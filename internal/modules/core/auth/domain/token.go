package domain

import "time"

// EmailVerificationToken represents an email verification token
type EmailVerificationToken struct {
	ID           string
	UserID       string
	Token        string
	ExpiresAt    time.Time
	VerifiedAt   *time.Time
	ResentCount  int
	LastResentAt *time.Time
	CreatedAt    time.Time
}

// PasswordResetToken represents a password reset token
type PasswordResetToken struct {
	ID        string
	UserID    string
	Token     string
	ExpiresAt time.Time
	UsedAt    *time.Time
	IPAddress *string
	UserAgent *string
	CreatedAt time.Time
}

// RefreshToken represents a refresh token for session management
type RefreshToken struct {
	ID         string
	UserID     string
	TokenHash  string
	DeviceName *string
	DeviceID   *string
	IPAddress  *string
	UserAgent  *string
	ExpiresAt  time.Time
	LastUsedAt *time.Time
	RevokedAt  *time.Time
	CreatedAt  time.Time
}

// LoginAttempt represents a login attempt record
type LoginAttempt struct {
	ID            string
	UserID        *string
	Email         *string
	Username      *string
	IPAddress     *string
	UserAgent     *string
	Status        string // success, failed, locked
	FailureReason *string
	CreatedAt     time.Time
}

// AuditLog represents an audit log entry
type AuditLog struct {
	ID         string
	UserID     *string
	Action     string
	Resource   *string
	ResourceID *string
	IPAddress  *string
	UserAgent  *string
	Metadata   map[string]interface{}
	CreatedAt  time.Time
}
