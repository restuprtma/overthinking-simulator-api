package dto

import "time"

// CreateModuleRequest represents request to create a new module
type CreateModuleRequest struct {
	Code        string  `json:"code" binding:"required,min=2,max=50"`
	Name        string  `json:"name" binding:"required,min=2,max=100"`
	Description *string `json:"description" binding:"omitempty,max=500"`
	Icon        *string `json:"icon" binding:"omitempty,max=50"`
	Color       *string `json:"color" binding:"omitempty,max=20"`
	SortOrder   int     `json:"sort_order" binding:"omitempty,min=0"`
	IsActive    *bool   `json:"is_active" binding:"omitempty"`

	// Tree structure fields
	ParentID *string `json:"parent_id" binding:"omitempty,uuid"`

	// Frontend-matching fields
	Heading       *string `json:"heading" binding:"omitempty,max=500"`
	To            *string `json:"to" binding:"omitempty,max=255"`
	URL           *string `json:"url" binding:"omitempty,max=255"`
	Permission    *string `json:"permission" binding:"omitempty,max=100"`
	Tooltip       *string `json:"tooltip" binding:"omitempty,max=255"`
	IsMenuVisible *bool   `json:"is_menu_visible" binding:"omitempty"`
}

// UpdateModuleRequest represents request to update module
type UpdateModuleRequest struct {
	Code        *string `json:"code" binding:"omitempty,min=2,max=50"`
	Name        *string `json:"name" binding:"omitempty,min=2,max=100"`
	Description *string `json:"description" binding:"omitempty,max=500"`
	Icon        *string `json:"icon" binding:"omitempty,max=50"`
	Color       *string `json:"color" binding:"omitempty,max=20"`
	SortOrder   *int    `json:"sort_order" binding:"omitempty,min=0"`
	IsActive    *bool   `json:"is_active" binding:"omitempty"`

	// Tree structure fields
	ParentID *string `json:"parent_id" binding:"omitempty,uuid"`

	// Frontend-matching fields
	Heading       *string `json:"heading" binding:"omitempty,max=500"`
	To            *string `json:"to" binding:"omitempty,max=255"`
	URL           *string `json:"url" binding:"omitempty,max=255"`
	Permission    *string `json:"permission" binding:"omitempty,max=100"`
	Tooltip       *string `json:"tooltip" binding:"omitempty,max=255"`
	IsMenuVisible *bool   `json:"is_menu_visible" binding:"omitempty"`
}

// ModuleDetailResponse represents detailed module data in response
type ModuleDetailResponse struct {
	ID          string     `json:"id"`
	Code        string     `json:"code"`
	Name        string     `json:"name"`
	Description *string    `json:"description,omitempty"`
	Icon        *string    `json:"icon,omitempty"`
	Color       *string    `json:"color,omitempty"`
	SortOrder   int        `json:"sort_order"`
	IsActive    bool       `json:"is_active"`

	// Tree structure fields
	ParentID     *string `json:"parent_id,omitempty"`
	Depth        int     `json:"depth"`
	DisplayOrder *int    `json:"display_order,omitempty"` // Explicit display order for frontend rendering (only in tree endpoint)

	// Frontend-matching fields
	Heading       *string `json:"heading,omitempty"`
	To            *string `json:"to,omitempty"`
	URL           *string `json:"url,omitempty"`
	Permission    *string `json:"permission,omitempty"`
	Tooltip       *string `json:"tooltip,omitempty"`
	IsMenuVisible bool    `json:"is_menu_visible"`

	// Related data counts (for UX badges/indicators)
	PermissionsCount int `json:"permissions_count"`

	// Audit fields
	CreatedAt time.Time  `json:"created_at"`
	CreatedBy *string    `json:"created_by,omitempty"`
	UpdatedAt time.Time  `json:"updated_at"`
	UpdatedBy *string    `json:"updated_by,omitempty"`
	DeletedAt *time.Time `json:"deleted_at,omitempty"`
	DeletedBy *string    `json:"deleted_by,omitempty"`
}

// ModuleListResponse represents paginated module list
type ModuleListResponse struct {
	Modules    []ModuleDetailResponse `json:"modules"`
	Total      int64                  `json:"total"`
	Page       int                    `json:"page"`
	PageSize   int                    `json:"page_size"`
	TotalPages int                    `json:"total_pages"`
}

// ModuleQueryParams represents query parameters for listing modules
type ModuleQueryParams struct {
	Page     int    `form:"page,default=1" binding:"omitempty,min=1"`
	PageSize int    `form:"page_size,default=10" binding:"omitempty,min=1,max=100"`
	Search   string `form:"search" binding:"omitempty,max=255"`
	IsActive *bool  `form:"is_active" binding:"omitempty"`

	// Tree structure filters
	ParentID      *string `form:"parent_id" binding:"omitempty,uuid"`
	Depth         *int    `form:"depth" binding:"omitempty,min=0"`
	IsMenuVisible *bool   `form:"is_menu_visible" binding:"omitempty"`
}

// ModuleWithSubModules represents a module with its sub-modules
type ModuleWithSubModules struct {
	Module     ModuleDetailResponse   `json:"module"`      // Root module (depth = 0)
	SubModules []ModuleDetailResponse `json:"sub_modules"` // Sub-modules belonging to this module, ordered by depth and sort_order
}

// ModuleTreeResponse represents module tree structure grouped by root modules
type ModuleTreeResponse []ModuleWithSubModules
