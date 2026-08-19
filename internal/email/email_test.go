// internal/email/email_test.go
package email

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"task-forge/internal/config"
	"task-forge/internal/dto"
	"testing"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/require"
)



func TestNewEmailSender(t *testing.T) {
	logger := zerolog.Nop()

	tests := []struct {
		name string
		mode string
		want EmailSender
	}{
		{
			name: "console mode",
			mode: "console",
			want: &consoleSender{},
		},
		{
			name: "debug mode",
			mode: "debug",
			want: &consoleSender{},
		},
		{
			name: "dev mode",
			mode: "dev",
			want: &consoleSender{},
		},
		{
			name: "mailtrap mode",
			mode: "mailtrap",
			want: &mailtrapSender{},
		},
		{
			name: "smtp mode",
			mode: "smtp",
			want: &smtpSender{},
		},
		{
			name: "empty mode defaults to smtp",
			mode: "",
			want: &smtpSender{},
		},
		{
			name: "unknown mode falls back to smtp",
			mode: "unknown",
			want: &smtpSender{},
		},
		{
			name: "uppercase mode",
			mode: "MAILTRAP",
			want: &mailtrapSender{},
		},
		{
			name: "mode with spaces",
			mode: "  console  ",
			want: &consoleSender{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := NewEmailSender(
				tt.mode,
				config.SMTPConfig{},
				config.MailtrapConfig{},
				logger,
			)

			require.IsType(t, tt.want, got)
		})
	}
}

func TestSMTPSender_SendInvitation_InvalidConfig(t *testing.T) {
	logger := zerolog.Nop()

	tests := []struct {
		name string
		cfg  config.SMTPConfig
	}{
		{
			name: "missing host",
			cfg: config.SMTPConfig{
				Port: 1025,
				From: "noreply@example.com",
			},
		},
		{
			name: "missing port",
			cfg: config.SMTPConfig{
				Host: "localhost",
				From: "noreply@example.com",
			},
		},
		{
			name: "missing from",
			cfg: config.SMTPConfig{
				Host: "localhost",
				Port: 1025,
			},
		},
	}

	data := dto.InvitationEmailData{
		RecipientEmail: "user@example.com",
		TeamName:       "Marketing Team",
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sender := newSMTPSender(tt.cfg, logger)

			err := sender.SendInvitation(data)

			require.Error(t, err)
			require.ErrorContains(t, err, "smtp configuration is incomplete")
		})
	}
}

func TestMailtrapSender_SendInvitation_Success(t *testing.T) {
	var received MailtrapRequest

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, http.MethodPost, r.Method)
		require.Equal(t, "application/json", r.Header.Get("Content-Type"))
		require.Equal(t, "Bearer test-token", r.Header.Get("Authorization"))

		err := json.NewDecoder(r.Body).Decode(&received)
		require.NoError(t, err)

		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	sender := &mailtrapSender{
		apiURL:     server.URL,
		apiToken:   "test-token",
		fromEmail:  "noreply@example.com",
		fromName:   "Task Forge",
		httpClient: server.Client(),
		logger:     zerolog.Nop(),
	}

	data := dto.InvitationEmailData{
		RecipientEmail: "user@example.com",
		TeamName:       "Marketing Team",
	}

	err := sender.SendInvitation(data)

	require.NoError(t, err)

	require.Equal(t, "noreply@example.com", received.From.Email)
	require.Equal(t, "Task Forge", received.From.Name)
	require.Equal(t, "user@example.com", received.To[0].Email)
	require.Equal(t, "Invitation to join Marketing Team", received.Subject)
	require.Equal(t, "Participant's invitation", received.Category)


	require.NotEmpty(t, received.HTML)
}

func TestMailtrapSender_SendInvitation_Accepted(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusAccepted)
	}))
	defer server.Close()

	sender := &mailtrapSender{
		apiURL:     server.URL,
		apiToken:   "test-token",
		fromEmail:  "noreply@example.com",
		fromName:   "Task Forge",
		httpClient: server.Client(),
		logger:     zerolog.Nop(),
	}

	err := sender.SendInvitation(dto.InvitationEmailData{
		RecipientEmail: "user@example.com",
		TeamName:       "Marketing Team",
	})

	require.NoError(t, err)
}

func TestMailtrapSender_SendInvitation_HTTPError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "internal server error", http.StatusInternalServerError)
	}))
	defer server.Close()

	sender := &mailtrapSender{
		apiURL:     server.URL,
		apiToken:   "test-token",
		fromEmail:  "noreply@example.com",
		fromName:   "Task Forge",
		httpClient: server.Client(),
		logger:     zerolog.Nop(),
	}

	data := dto.InvitationEmailData{
		RecipientEmail: "user@example.com",
		TeamName:       "Marketing Team",
	}

	err := sender.SendInvitation(data)

	require.Error(t, err)
	require.ErrorContains(t, err, "mailtrap returned status: 500")
}

func TestMailtrapSender_SendInvitation_NetworkError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
	}))
	
	server.Close()

	sender := &mailtrapSender{
		apiURL:     server.URL,
		apiToken:   "test-token",
		fromEmail:  "noreply@example.com",
		fromName:   "Task Forge",
		httpClient: &http.Client{},
		logger:     zerolog.Nop(),
	}

	data := dto.InvitationEmailData{
		RecipientEmail: "user@example.com",
		TeamName:       "Marketing Team",
	}

	err := sender.SendInvitation(data)

	require.Error(t, err)
	require.ErrorContains(t, err, "mailtrap send request")
}

func TestConsoleSender_SendInvitation(t *testing.T) {
	sender := newConsoleSender(zerolog.Nop())

	err := sender.SendInvitation(dto.InvitationEmailData{
		RecipientEmail: "user@example.com",
		TeamName:       "Marketing Team",
	})

	require.NoError(t, err)
}

func TestNewSMTPSender_PreservesPassword(t *testing.T) {
	cfg := config.SMTPConfig{
		Host:     "smtp.example.com",
		Port:     587,
		From:     "noreply@example.com",
		Username: "user",
		Password: "secret",
	}

	sender := newSMTPSender(cfg, zerolog.Nop())

	smtpSender, ok := sender.(*smtpSender)
	require.True(t, ok)

	require.Equal(t, "secret", smtpSender.password)
}