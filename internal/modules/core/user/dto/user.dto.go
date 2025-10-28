package dto

import "time"

// CreateUserRequest represents request to create a new user
type CreateUserRequest struct {
	Email    string   `json:"email" binding:"required,email"`
	Username string   `json:"username" binding:"required,min=3,max=50,alphanum"`
	Password string   `json:"password" binding:"required,min=8,max=72"`
	FullName *string  `json:"full_name" binding:"omitempty,min=3,max=255"`
	Phone    *string  `json:"phone" binding:"omitempty,min=10,max=20"`
	RoleIDs  []string `json:"role_ids" binding:"omitempty,dive,uuid"` // Optional: assign roles on creation
}

// UpdateUserRequest represents request to update user
type UpdateUserRequest struct {
	Email    *string  `json:"email" binding:"omitempty,email"`
	Username *string  `json:"username" binding:"omitempty,min=3,max=50,alphanum"`
	FullName *string  `json:"full_name" binding:"omitempty,min=3,max=255"`
	Phone    *string  `json:"phone" binding:"omitempty,min=10,max=20"`
	IsActive *bool    `json:"is_active" binding:"omitempty"`
	RoleIDs  []string `json:"role_ids" binding:"omitempty,dive,uuid"` // Optional: update/sync roles
}

// ChangePasswordRequest represents request to change password
type ChangePasswordRequest struct {
	OldPassword string `json:"old_password" binding:"required"`
	NewPassword string `json:"new_password" binding:"required,min=8,max=72"`
}

// RoleInfo represents basic role information in user response
type RoleInfo struct {
	ID          string  `json:"id"`
	Name        string  `json:"name"`
	Description *string `json:"description,omitempty"`
	IsSystem    bool    `json:"is_system"`
}

// UserResponse represents user data in response
type UserResponse struct {
	ID              string     `json:"id"`
	Email           string     `json:"email"`
	Username        string     `json:"username"`
	FullName        *string    `json:"full_name,omitempty"`
	Phone           *string    `json:"phone,omitempty"`
	IsActive        bool       `json:"is_active"`
	IsEmailVerified bool       `json:"is_email_verified"`
	LastLoginAt     *time.Time `json:"last_login_at,omitempty"`
	Roles           []RoleInfo `json:"roles,omitempty"`
	CreatedAt       time.Time  `json:"created_at"`
	CreatedBy       *string    `json:"created_by,omitempty"`
	UpdatedAt       time.Time  `json:"updated_at"`
	UpdatedBy       *string    `json:"updated_by,omitempty"`
	DeletedAt       *time.Time `json:"deleted_at,omitempty"`
	DeletedBy       *string    `json:"deleted_by,omitempty"`
}

// UserListResponse represents paginated user list
type UserListResponse struct {
	Users      []UserResponse `json:"users"`
	Total      int64          `json:"total"`
	Page       int            `json:"page"`
	PageSize   int            `json:"page_size"`
	TotalPages int            `json:"total_pages"`
}

// AssignRolesRequest represents request to assign roles to a user
type AssignRolesRequest struct {
	RoleIDs []string `json:"role_ids" binding:"required,min=1,dive,uuid"`
}

// SyncRolesRequest represents request to sync/replace all user roles
type SyncRolesRequest struct {
	RoleIDs []string `json:"role_ids" binding:"required,dive,uuid"` // Can be empty array to remove all roles
}

// UserQueryParams represents query parameters for listing users
type UserQueryParams struct {
	Page     int    `form:"page,default=1" binding:"omitempty,min=1"`
	PageSize int    `form:"page_size,default=10" binding:"omitempty,min=1,max=100"`
	Search   string `form:"search" binding:"omitempty,max=255"`      // Search across email, username, full_name
	FullName string `form:"full_name" binding:"omitempty,max=255"`   // Filter by full_name
	Username string `form:"username" binding:"omitempty,max=50"`     // Filter by username
	Email    string `form:"email" binding:"omitempty,email,max=255"` // Filter by email
	IsActive *bool  `form:"is_active" binding:"omitempty"`           // Filter by is_active status
}