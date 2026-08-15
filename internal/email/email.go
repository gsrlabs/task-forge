// internal/email/email.go
package email

import (
	"bytes"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"net/http"
	"net/smtp"
	"strings"
	"task-forge/internal/config"
	"task-forge/internal/dto"
	"time"

	"github.com/rs/zerolog"
)

const (
	mailtrapAPIURL = "https://send.api.mailtrap.io/api/send"

	ModeConsole  = "console"
	ModeMailtrap = "mailtrap"
	ModeSMTP     = "smtp"
)

// EmailSender interface for sending emails
type EmailSender interface {
	SendInvitation(data dto.InvitationEmailData) error
}

// FACTORY
func NewEmailSender(mode string, smtpCfg config.SMTPConfig, mailtrapCfg config.MailtrapConfig, logger zerolog.Logger) EmailSender {
	switch strings.ToLower(strings.TrimSpace(mode)) {
	case ModeConsole, "debug", "dev":
		logger.Info().Msg("📧 Email sender initialized in CONSOLE mode (no real emails will be sent)")
		return newConsoleSender(logger)

	case ModeMailtrap:
		logger.Info().Msg("📧 Email sender initialized in MAILTRAP mode")
		return newMailtrapSender(mailtrapCfg, logger)

	case ModeSMTP, "":
		logger.Info().
			Str("host", smtpCfg.Host).
			Int("port", smtpCfg.Port).
			Str("from", smtpCfg.From).
			Bool("has_auth", smtpCfg.Username != "" && smtpCfg.Password != "").
			Msg("📧 Email sender initialized in SMTP mode")
		return newSMTPSender(smtpCfg, logger)

	default:
		logger.Warn().
			Str("mode", mode).
			Msg("⚠️ Unknown email mode, falling back to SMTP")
		return newSMTPSender(smtpCfg, logger)
	}
}


// CONSOLE SENDER
type consoleSender struct {
	logger zerolog.Logger
}

func newConsoleSender(logger zerolog.Logger) EmailSender {
	return &consoleSender{logger: logger}
}

func (s *consoleSender) SendInvitation(data dto.InvitationEmailData) error {

	fmt.Printf("\n%s\n", strings.Repeat("═", 69))
	fmt.Printf("%s invitation to join the %s\n", data.RecipientEmail, data.TeamName)
	fmt.Printf("%s\n\n", strings.Repeat("═", 69))

	return nil
}


// MAILTRAP SENDER
type mailtrapSender struct {
	apiURL     string
	apiToken   string
	fromEmail  string
	fromName   string
	httpClient *http.Client
	logger     zerolog.Logger
}

func newMailtrapSender(cfg config.MailtrapConfig, logger zerolog.Logger) EmailSender {
	return &mailtrapSender{
		apiURL:     mailtrapAPIURL,
		apiToken:   cfg.APIKey,
		fromEmail:  cfg.FromEmail,
		fromName:   cfg.FromName,
		httpClient: &http.Client{Timeout: 10 * time.Second},
		logger:     logger,
	}
}

// MailtrapRequest DTO for Mailtrap API
type MailtrapRequest struct {
	From     MailtrapFrom `json:"from"`
	To       []MailtrapTo `json:"to"`
	Subject  string       `json:"subject"`
	HTML     string       `json:"html"`
	Category string       `json:"category"`
}

type MailtrapFrom struct {
	Email string `json:"email"`
	Name  string `json:"name"`
}

type MailtrapTo struct {
	Email string `json:"email"`
}

func (m *mailtrapSender) SendInvitation(data dto.InvitationEmailData) error {
	htmlBody, err := buildInvitationTemplate(data)
	if err != nil {
		return fmt.Errorf("build invitation email: %w", err)
	}

	payload := MailtrapRequest{
		From: MailtrapFrom{
			Email: m.fromEmail,
			Name:  m.fromName,
		},
		To: []MailtrapTo{
			{Email: data.RecipientEmail},
		},
		Subject:  "Invitation to join " + data.TeamName,
		HTML:     htmlBody,
		Category: "Participant's invitation",
	}

	bodyBytes, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("mailtrap marshal payload: %w", err)
	}

	req, err := http.NewRequest("POST", m.apiURL, bytes.NewReader(bodyBytes))
	if err != nil {
		return fmt.Errorf("mailtrap create request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+m.apiToken)
	req.Header.Set("Content-Type", "application/json")

	resp, err := m.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("mailtrap send request: %w", err)
	}
	defer func() {
		if closeErr := resp.Body.Close(); closeErr != nil {
			m.logger.Warn().Err(closeErr).Msg("mailtrap response body close error")
		}
	}()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusAccepted {
		return fmt.Errorf("mailtrap returned status: %d", resp.StatusCode)
	}

	m.logger.Info().
		Str("from", m.fromEmail).
		Str("to", data.RecipientEmail).
		Msg("✅ message sent via Mailtrap")

	return nil
}

// SMTP SENDER
type smtpSender struct {
	host     string
	port     int
	from     string
	username string
	password string
	logger   zerolog.Logger
}

func newSMTPSender(cfg config.SMTPConfig, logger zerolog.Logger) EmailSender {
	passExist := "not exist"
	if cfg.Username != "" {
		cfg.Password = "exist"
	}

	logger.Debug().
		Str("from", cfg.Host).
		Int("port", cfg.Port).
		Str("from", cfg.From).
		Str("user", cfg.Username).
		Str("password", passExist)

	return &smtpSender{
		host:     cfg.Host,
		port:     cfg.Port,
		from:     cfg.From,
		username: cfg.Username,
		password: cfg.Password,
		logger:   logger,
	}
}

func (s *smtpSender) SendInvitation(data dto.InvitationEmailData) error {
	if s.host == "" || s.port == 0 || s.from == "" {
		return fmt.Errorf("smtp configuration is incomplete")
	}

	addr := fmt.Sprintf("%s:%d", s.host, s.port)
	htmlBody, err := buildInvitationTemplate(data)
	if err != nil {
		return fmt.Errorf("build invitation email: %w", err)
	}
	subject := "Invitation to join " + data.TeamName

	msg := fmt.Sprintf(`From: %s
To: %s
Subject: %s
MIME-Version: 1.0
Content-Type: text/html; charset="UTF-8"
Date: %s

%s`,
		s.from, data.RecipientEmail, subject,
		time.Now().Format(time.RFC1123Z),
		htmlBody,
	)

	var auth smtp.Auth
	if s.username != "" && s.password != "" {
		auth = smtp.PlainAuth("", s.username, s.password, s.host)
	}

	if s.port == 465 {
		return s.deliverSMTPOverTLS(addr, auth, data.RecipientEmail, msg, "SMTPS (465 implicit TLS)")
	}

	err = smtp.SendMail(addr, auth, s.from, []string{data.RecipientEmail}, []byte(msg))
	if err == nil {
		s.logSuccess(data.RecipientEmail, "SMTP (STARTTLS)")
		return nil
	}

	s.logger.Warn().
		Err(err).
		Str("host", s.host).
		Int("port", s.port).
		Msg("⚠️ smtp.SendMail failed, trying TLS dial (legacy fallback)")

	if s.port != 587 {
		return fmt.Errorf("failed to send email via SMTP: %w", err)
	}

	return s.deliverSMTPOverTLS(addr, auth, data.RecipientEmail, msg, "SMTP (manual TLS after SendMail failure)")
}

func (s *smtpSender) deliverSMTPOverTLS(addr string, auth smtp.Auth, toEmail, msg, logLabel string) error {
	tlsConfig := &tls.Config{
		ServerName:         s.host,
		MinVersion:         tls.VersionTLS12,
		InsecureSkipVerify: false,
	}

	conn, err := tls.Dial("tcp", addr, tlsConfig)
	if err != nil {
		return fmt.Errorf("tls dial failed to %s: %w", addr, err)
	}
	defer func() {
		if closeErr := conn.Close(); closeErr != nil {
			s.logger.Warn().Err(closeErr).Msg("tls connection close error")
		}
	}()

	c, err := smtp.NewClient(conn, s.host)
	if err != nil {
		return fmt.Errorf("smtp client creation failed: %w", err)
	}
	defer func() {
		if closeErr := c.Close(); closeErr != nil {
			s.logger.Warn().Err(closeErr).Msg("smtp client close error")
		}
	}()

	if auth != nil {
		if err = c.Auth(auth); err != nil {
			return fmt.Errorf("smtp auth failed: %w", err)
		}
	}

	if err = c.Mail(s.from); err != nil {
		return fmt.Errorf("smtp mail from failed: %w", err)
	}
	if err = c.Rcpt(toEmail); err != nil {
		return fmt.Errorf("smtp rcpt to failed: %w", err)
	}

	w, err := c.Data()
	if err != nil {
		return fmt.Errorf("smtp data failed: %w", err)
	}

	if _, err = w.Write([]byte(msg)); err != nil {
		_ = w.Close()
		return fmt.Errorf("writing message failed: %w", err)
	}

	if err = w.Close(); err != nil {
		return fmt.Errorf("closing data writer failed: %w", err)
	}

	if err = c.Quit(); err != nil {
		s.logger.Warn().Err(err).Msg("smtp quit failed")
	}

	s.logSuccess(toEmail, logLabel)
	return nil
}

func (s *smtpSender) logSuccess(toEmail, mode string) {
	s.logger.Info().
		Str("host", s.host).
		Int("port", s.port).
		Str("to", toEmail).
		Str("mode", mode).
		Msg("✅ message sent via SMTP")
}
