package dto

import "time"

// CreatePermissionTemplateRequest represents the request to create a permission template
type CreatePermissionTemplateRequest struct {
	Name        string   `json:"name" binding:"required,min=3,max=100"`
	Description *string  `json:"description" binding:"omitempty,max=1000"`
	Actions     []string `json:"actions" binding:"required,min=1,dive,min=1,max=50"`
}

// UpdatePermissionTemplateRequest represents the request to update a permission template
type UpdatePermissionTemplateRequest struct {
	Name        string   `json:"name" binding:"required,min=3,max=100"`
	Description *string  `json:"description" binding:"omitempty,max=1000"`
	Actions     []string `json:"actions" binding:"required,min=1,dive,min=1,max=50"`
}

// PermissionTemplateResponse represents a single permission template response
type PermissionTemplateResponse struct {
	ID          string     `json:"id"`
	Name        string     `json:"name"`
	Description *string    `json:"description,omitempty"`
	IsSystem    bool       `json:"is_system"`
	Actions     []string   `json:"actions"`
	CreatedAt   time.Time  `json:"created_at"`
	CreatedBy   *string    `json:"created_by,omitempty"`
	UpdatedAt   time.Time  `json:"updated_at"`
	UpdatedBy   *string    `json:"updated_by,omitempty"`
	DeletedAt   *time.Time `json:"deleted_at,omitempty"`
	DeletedBy   *string    `json:"deleted_by,omitempty"`
}

// PermissionTemplateListResponse represents the list response with pagination
type PermissionTemplateListResponse struct {
	Templates  []PermissionTemplateResponse `json:"templates"`
	Total      int64                        `json:"total"`
	Page       int                          `json:"page"`
	PageSize   int                          `json:"page_size"`
	TotalPages int                          `json:"total_pages"`
}

// PermissionTemplateQueryParams represents query parameters for listing templates
type PermissionTemplateQueryParams struct {
	Page          int    `form:"page,default=1" binding:"omitempty,min=1"`
	PageSize      int    `form:"page_size,default=10" binding:"omitempty,min=1,max=100"`
	Search        string `form:"search" binding:"omitempty,max=255"`
	IncludeSystem *bool  `form:"include_system" binding:"omitempty"`
}
