package dto

// VerifyEmailRequest represents the email verification request
type VerifyEmailRequest struct {
	Token string `json:"token" binding:"required"`
}

// VerifyEmailResponse represents the email verification response
type VerifyEmailResponse struct {
	Message string `json:"message"`
}

// ResendVerificationRequest represents resend verification email request
type ResendVerificationRequest struct {
	Email string `json:"email" binding:"required,email"`
}

// ResendVerificationResponse represents resend verification response
type ResendVerificationResponse struct {
	Message string `json:"message"`
}
