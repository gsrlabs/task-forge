// internal/dto/email.go
package dto

type InvitationEmailData struct {
	RecipientEmail string
	TeamName       string
	InviterName    string
	Role           string
	InviteURL      string
}
