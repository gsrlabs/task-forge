// internal/dto/dto.go
package dto

// MessageResponse standard response with a message.
type MessageResponse struct {
	Message string `json:"message"`
}

// ErrorResponse is a standard response with an error.
type ErrorResponse struct {
	Error   string `json:"error"`
	Details string `json:"details,omitempty"`
}
