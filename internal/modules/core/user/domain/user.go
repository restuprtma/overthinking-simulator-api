package domain

import "time"

// User represents a user entity in the system
type User struct {
	ID              string     `json:"id"`
	Email           string     `json:"email"`
	Username        string     `json:"username"`
	PasswordHash    string     `json:"-"` // Hide from JSON responses
	FullName        *string    `json:"full_name,omitempty"`
	Phone           *string    `json:"phone,omitempty"`
	IsActive        bool       `json:"is_active"`
	IsEmailVerified bool       `json:"is_email_verified"`
	LastLoginAt     *time.Time `json:"last_login_at,omitempty"`
	CreatedAt       time.Time  `json:"created_at"`
	CreatedBy       *string    `json:"created_by,omitempty"`
	UpdatedAt       time.Time  `json:"updated_at"`
	UpdatedBy       *string    `json:"updated_by,omitempty"`
	DeletedAt       *time.Time `json:"deleted_at,omitempty"`
	DeletedBy       *string    `json:"deleted_by,omitempty"`
}

// ToPublic converts User to UserPublic (without sensitive data)
func (u *User) ToPublic() *UserPublic {
	return &UserPublic{
		ID:              u.ID,
		Email:           u.Email,
		Username:        u.Username,
		FullName:        u.FullName,
		Phone:           u.Phone,
		IsActive:        u.IsActive,
		IsEmailVerified: u.IsEmailVerified,
		LastLoginAt:     u.LastLoginAt,
	}
}

// UserPublic represents public user information (without sensitive data)
type UserPublic struct {
	ID              string     `json:"id"`
	Email           string     `json:"email"`
	Username        string     `json:"username"`
	FullName        *string    `json:"full_name,omitempty"`
	Phone           *string    `json:"phone,omitempty"`
	IsActive        bool       `json:"is_active"`
	IsEmailVerified bool       `json:"is_email_verified"`
	LastLoginAt     *time.Time `json:"last_login_at,omitempty"`
}