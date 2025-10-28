package dto

import "time"

// ModuleResponse represents module data in response
type ModuleResponse struct {
	ID          string  `json:"id"`
	Code        string  `json:"code"`
	Name        string  `json:"name"`
	Description *string `json:"description,omitempty"`
	Icon        *string `json:"icon,omitempty"`
	Color       *string `json:"color,omitempty"`
	SortOrder   int     `json:"sort_order"`
	IsActive    bool    `json:"is_active"`
}

// CreatePermissionRequest represents request to create a new permission
type CreatePermissionRequest struct {
	ModuleID    string  `json:"module_id" binding:"required,uuid"`
	Resource    string  `json:"resource" binding:"required,min=2,max=100"`
	Action      string  `json:"action" binding:"required,min=2,max=50"`
	Description *string `json:"description" binding:"omitempty,max=500"`
}

// UpdatePermissionRequest represents request to update permission
type UpdatePermissionRequest struct {
	ModuleID    *string `json:"module_id" binding:"omitempty,uuid"`
	Resource    *string `json:"resource" binding:"omitempty,min=2,max=100"`
	Action      *string `json:"action" binding:"omitempty,min=2,max=50"`
	Description *string `json:"description" binding:"omitempty,max=500"`
}

// PermissionResponse represents permission data in response
type PermissionResponse struct {
	ID          string          `json:"id"`
	ModuleID    string          `json:"module_id"`
	Module      *ModuleResponse `json:"module,omitempty"`
	Resource    string          `json:"resource"`
	Action      string          `json:"action"`
	Description *string         `json:"description,omitempty"`
	CreatedAt   time.Time       `json:"created_at"`
	CreatedBy   *string         `json:"created_by,omitempty"`
	UpdatedAt   time.Time       `json:"updated_at"`
	UpdatedBy   *string         `json:"updated_by,omitempty"`
	DeletedAt   *time.Time      `json:"deleted_at,omitempty"`
	DeletedBy   *string         `json:"deleted_by,omitempty"`
}

// PermissionListResponse represents paginated permission list
type PermissionListResponse struct {
	Permissions []PermissionResponse `json:"permissions"`
	Total       int64                `json:"total"`
	Page        int                  `json:"page"`
	PageSize    int                  `json:"page_size"`
	TotalPages  int                  `json:"total_pages"`
}

// PermissionQueryParams represents query parameters for listing permissions
type PermissionQueryParams struct {
	Page     int    `form:"page,default=1" binding:"omitempty,min=1"`
	PageSize int    `form:"page_size,default=10" binding:"omitempty,min=1,max=100"`
	Search   string `form:"search" binding:"omitempty,max=255"`
	ModuleID string `form:"module_id" binding:"omitempty,uuid"`
	Resource string `form:"resource" binding:"omitempty,max=100"`
	Action   string `form:"action" binding:"omitempty,max=50"`
}

// ResourceGroupedByModule represents resources grouped by module
type ResourceGroupedByModule struct {
	Module    ModuleResponse `json:"module"`
	Resources []string       `json:"resources"`
}

// ApplyCRUDTemplateRequest represents request to apply standard CRUD permissions
type ApplyCRUDTemplateRequest struct {
	ModuleID string `json:"module_id"` // Set from URL param
	Resource string `json:"resource" binding:"required,min=2,max=100"`
}

// CRUDTemplateResponse represents the result of applying CRUD template
type CRUDTemplateResponse struct {
	Created []PermissionResponse `json:"created"`
	Skipped []string             `json:"skipped"` // Actions that already exist
	Total   int                  `json:"total"`
}
