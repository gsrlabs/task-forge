package service

import (
	"fmt"
	"time"

	"github.com/rs/zerolog"
	"github.com/sony/gobreaker/v2"

	"task-forge/internal/dto"
	"task-forge/internal/email"
)

const (
	emailCircuitBreakerName = "email-service"

	emailCircuitFailureThreshold = 5
	emailCircuitTimeout          = 30 * time.Second
	emailCircuitInterval         = 60 * time.Second
)

type circuitBreakerEmailSender struct {
	sender  email.EmailSender
	breaker *gobreaker.CircuitBreaker[any]
	logger  zerolog.Logger
}

// NewCircuitBreakerEmailSender wraps EmailSender with Circuit Breaker.
func NewCircuitBreakerEmailSender(
	sender email.EmailSender,
	logger zerolog.Logger,
) email.EmailSender {
	settings := gobreaker.Settings{
		Name:        emailCircuitBreakerName,
		MaxRequests: 1,
		Interval:    emailCircuitInterval,
		Timeout:     emailCircuitTimeout,

		ReadyToTrip: func(counts gobreaker.Counts) bool {
			return counts.ConsecutiveFailures >= emailCircuitFailureThreshold
		},

		OnStateChange: func(
			name string,
			from gobreaker.State,
			to gobreaker.State,
		) {
			logger.Warn().
				Str("circuit", name).
				Str("from", from.String()).
				Str("to", to.String()).
				Msg("email circuit breaker state changed")
		},
	}

	return &circuitBreakerEmailSender{
		sender:  sender,
		breaker: gobreaker.NewCircuitBreaker[any](settings),
		logger:  logger,
	}
}

// SendInvitation sends an invitation through the Circuit Breaker.
func (s *circuitBreakerEmailSender) SendInvitation(
	data dto.InvitationEmailData,
) error {
	_, err := s.breaker.Execute(func() (any, error) {
		if err := s.sender.SendInvitation(data); err != nil {
			return nil, err
		}

		return nil, nil
	})

	if err != nil {
    return fmt.Errorf("send invitation email: %w", err)
	}

	return nil
}