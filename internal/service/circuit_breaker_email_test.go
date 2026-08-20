package service

import (
	"errors"
	"testing"

	"task-forge/internal/dto"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	"github.com/stretchr/testify/require"
)

type mockEmailSender struct {
	mock.Mock
}

func (m *mockEmailSender) SendInvitation(
	data dto.InvitationEmailData,
) error {
	args := m.Called(data)
	return args.Error(0)
}

var errEmailService = errors.New("email service unavailable")

func testInvitationData() dto.InvitationEmailData {
	return dto.InvitationEmailData{}
}

func TestCircuitBreakerEmailSender_SendInvitation_Success(t *testing.T) {
	sender := new(mockEmailSender)

	data := testInvitationData()

	sender.
		On("SendInvitation", data).
		Return(nil).
		Once()

	service := NewCircuitBreakerEmailSender(
		sender,
		zerolog.Nop(),
	)

	err := service.SendInvitation(data)

	require.NoError(t, err)

	sender.AssertExpectations(t)
}

func TestCircuitBreakerEmailSender_SendInvitation_Error(t *testing.T) {
	sender := new(mockEmailSender)

	data := testInvitationData()

	sender.
		On("SendInvitation", data).
		Return(errEmailService).
		Once()

	service := NewCircuitBreakerEmailSender(
		sender,
		zerolog.Nop(),
	)

	err := service.SendInvitation(data)

	require.Error(t, err)
	assert.ErrorIs(t, err, errEmailService)

	sender.AssertExpectations(t)
}

func TestCircuitBreakerEmailSender_OpensAfterFiveFailures(t *testing.T) {
	sender := new(mockEmailSender)

	data := testInvitationData()

	sender.
		On("SendInvitation", data).
		Return(errEmailService).
		Times(emailCircuitFailureThreshold)

	service := NewCircuitBreakerEmailSender(
		sender,
		zerolog.Nop(),
	)

	for range emailCircuitFailureThreshold {
		err := service.SendInvitation(data)

		require.Error(t, err)
		assert.ErrorIs(t, err, errEmailService)
	}

	err := service.SendInvitation(data)

	require.Error(t, err)

	sender.AssertNumberOfCalls(
		t,
		"SendInvitation",
		emailCircuitFailureThreshold,
	)
}

func TestCircuitBreakerEmailSender_DoesNotCallSenderWhenOpen(t *testing.T) {
	sender := new(mockEmailSender)

	data := testInvitationData()

	sender.
		On("SendInvitation", data).
		Return(errEmailService).
		Times(emailCircuitFailureThreshold)

	service := NewCircuitBreakerEmailSender(
		sender,
		zerolog.Nop(),
	)

	for range emailCircuitFailureThreshold {
		_ = service.SendInvitation(data)
	}

	err := service.SendInvitation(data)

	require.Error(t, err)

	sender.AssertNumberOfCalls(
		t,
		"SendInvitation",
		emailCircuitFailureThreshold,
	)
}

func TestCircuitBreakerEmailSender_SuccessResetsFailures(t *testing.T) {
	sender := new(mockEmailSender)

	data := testInvitationData()

	sender.
		On("SendInvitation", data).
		Return(errEmailService).
		Once()

	sender.
		On("SendInvitation", data).
		Return(nil).
		Once()

	sender.
		On("SendInvitation", data).
		Return(errEmailService).
		Times(emailCircuitFailureThreshold - 1)

	service := NewCircuitBreakerEmailSender(
		sender,
		zerolog.Nop(),
	)

	err := service.SendInvitation(data)
	require.Error(t, err)

	err = service.SendInvitation(data)
	require.NoError(t, err)

	for range emailCircuitFailureThreshold-1 {
		err = service.SendInvitation(data)
		require.Error(t, err)
	}

	sender.AssertNumberOfCalls(
		t,
		"SendInvitation",
		1+1+(emailCircuitFailureThreshold-1),
	)
}

func TestCircuitBreakerEmailSender_SuccessResetsConsecutiveFailures(t *testing.T) {
	sender := new(mockEmailSender)

	data := testInvitationData()

	sender.
		On("SendInvitation", data).
		Return(errEmailService).
		Times(4)

	sender.
		On("SendInvitation", data).
		Return(nil).
		Once()

	sender.
		On("SendInvitation", data).
		Return(errEmailService).
		Times(5)

	service := NewCircuitBreakerEmailSender(
		sender,
		zerolog.Nop(),
	)
	
	for range 4 {
		err := service.SendInvitation(data)
		require.Error(t, err)
	}

	err := service.SendInvitation(data)
	require.NoError(t, err)

	for range 5 {
		err := service.SendInvitation(data)
		require.Error(t, err)
	}

	err = service.SendInvitation(data)
	require.Error(t, err)

	sender.AssertNumberOfCalls(
		t,
		"SendInvitation",
		10,
	)
}