package dto

import "time"

// CreateRoleRequest represents request to create a new role
type CreateRoleRequest struct {
	Name            string            `json:"name" binding:"required,min=3,max=100"`
	Description     *string           `json:"description" binding:"omitempty,max=1000"`
	ModuleTemplates map[string]string `json:"module_templates" binding:"omitempty"` // map[module_id]permission_template_id
	Permissions     []string          `json:"permissions" binding:"omitempty"`      // For backward compatibility
}

// UpdateRoleRequest represents request to update role
type UpdateRoleRequest struct {
	Name            *string           `json:"name" binding:"omitempty,min=3,max=100"`
	Description     *string           `json:"description" binding:"omitempty,max=1000"`
	ModuleTemplates map[string]string `json:"module_templates" binding:"omitempty"` // map[module_id]permission_template_id
	Permissions     []string          `json:"permissions" binding:"omitempty"`      // For backward compatibility
}

// RoleResponse represents role data in response
type RoleResponse struct {
	ID          string     `json:"id"`
	Name        string     `json:"name"`
	Description *string    `json:"description,omitempty"`
	IsSystem    bool       `json:"is_system"`
	CreatedAt   time.Time  `json:"created_at"`
	CreatedBy   *string    `json:"created_by,omitempty"`
	UpdatedAt   time.Time  `json:"updated_at"`
	UpdatedBy   *string    `json:"updated_by,omitempty"`
	DeletedAt   *time.Time `json:"deleted_at,omitempty"`
	DeletedBy   *string    `json:"deleted_by,omitempty"`
}

// RoleListResponse represents paginated role list
type RoleListResponse struct {
	Roles      []RoleResponse `json:"roles"`
	Total      int64          `json:"total"`
	Page       int            `json:"page"`
	PageSize   int            `json:"page_size"`
	TotalPages int            `json:"total_pages"`
}

// RoleQueryParams represents query parameters for listing roles
type RoleQueryParams struct {
	Page          int    `form:"page,default=1" binding:"omitempty,min=1"`
	PageSize      int    `form:"page_size,default=10" binding:"omitempty,min=1,max=100"`
	Search        string `form:"search" binding:"omitempty,max=255"`
	IncludeSystem *bool  `form:"include_system" binding:"omitempty"`
}

// PermissionResponse represents permission data in response
type PermissionResponse struct {
	ID          string `json:"id"`
	Resource    string `json:"resource"`
	Action      string `json:"action"`
	Description string `json:"description"`
}

// RoleWithPermissionsResponse represents role with its permissions
type RoleWithPermissionsResponse struct {
	ID              string                       `json:"id"`
	Name            string                       `json:"name"`
	Description     *string                      `json:"description,omitempty"`
	IsSystem        bool                         `json:"is_system"`
	Permissions     []PermissionResponse         `json:"permissions"`
	ModuleTemplates []RoleModuleTemplateResponse `json:"module_templates,omitempty"`
	CreatedAt       time.Time                    `json:"created_at"`
	CreatedBy       *string                      `json:"created_by,omitempty"`
	UpdatedAt       time.Time                    `json:"updated_at"`
	UpdatedBy       *string                      `json:"updated_by,omitempty"`
	DeletedAt       *time.Time                   `json:"deleted_at,omitempty"`
	DeletedBy       *string                      `json:"deleted_by,omitempty"`
}

// RoleModuleTemplateResponse represents a module-template mapping
type RoleModuleTemplateResponse struct {
	ModuleID               string `json:"module_id"`
	ModuleCode             string `json:"module_code"`
	ModuleName             string `json:"module_name"`
	PermissionTemplateID   string `json:"permission_template_id"`
	PermissionTemplateName string `json:"permission_template_name"`
}

// AssignPermissionsRequest represents request to assign permissions to a role
type AssignPermissionsRequest struct {
	PermissionIDs []string `json:"permission_ids" binding:"required,min=1,dive,uuid"`
}
