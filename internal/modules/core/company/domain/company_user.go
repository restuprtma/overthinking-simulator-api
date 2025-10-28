package domain

import "time"

// CompanyUser represents the relationship between a user and a company
type CompanyUser struct {
	ID        string     `json:"id"`
	CompanyID string     `json:"company_id"`
	UserID    string     `json:"user_id"`
	RoleID    *string    `json:"role_id,omitempty"` // Company-specific role ID (UUID). If NULL, inherits global role
	Role      string     `json:"role"`              // DEPRECATED: Use RoleID instead. Kept for backward compatibility
	IsPrimary bool       `json:"is_primary"`
	IsActive  bool       `json:"is_active"`
	InvitedBy *string    `json:"invited_by,omitempty"`
	// User information (joined from users table)
	UserEmail    *string `json:"user_email,omitempty"`
	UserUsername *string `json:"user_username,omitempty"`
	UserFullName *string `json:"user_full_name,omitempty"`
	// Timestamps
	JoinedAt  time.Time  `json:"joined_at"`
	CreatedAt time.Time  `json:"created_at"`
	CreatedBy *string    `json:"created_by,omitempty"`
	UpdatedAt time.Time  `json:"updated_at"`
	UpdatedBy *string    `json:"updated_by,omitempty"`
	DeletedAt *time.Time `json:"deleted_at,omitempty"`
	DeletedBy *string    `json:"deleted_by,omitempty"`
}

// CompanyUserRole constants
const (
	RoleOwner  = "OWNER"
	RoleAdmin  = "ADMIN"
	RoleMember = "MEMBER"
	RoleViewer = "VIEWER"
)

// IsValidRole checks if the role is valid
func IsValidRole(role string) bool {
	switch role {
	case RoleOwner, RoleAdmin, RoleMember, RoleViewer:
		return true
	default:
		return false
	}
}
