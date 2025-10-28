package dto

import "time"

// CreateLeadSourceRequest represents request to create a new lead source
type CreateLeadSourceRequest struct {
	Name        string  `json:"name" binding:"required,min=3,max=100"`
	Code        string  `json:"code" binding:"required,min=2,max=50"`
	Description *string `json:"description" binding:"omitempty,max=500"`
	Color       *string `json:"color" binding:"omitempty,len=7"` // #FFFFFF format
	Icon        *string `json:"icon" binding:"omitempty,max=50"`
	IsActive    *bool   `json:"is_active" binding:"omitempty"`
	SortOrder   *int    `json:"sort_order" binding:"omitempty,min=0"`
}

// UpdateLeadSourceRequest represents request to update lead source
type UpdateLeadSourceRequest struct {
	Name        *string `json:"name" binding:"omitempty,min=3,max=100"`
	Code        *string `json:"code" binding:"omitempty,min=2,max=50"`
	Description *string `json:"description" binding:"omitempty,max=500"`
	Color       *string `json:"color" binding:"omitempty,len=7"`
	Icon        *string `json:"icon" binding:"omitempty,max=50"`
	IsActive    *bool   `json:"is_active" binding:"omitempty"`
	SortOrder   *int    `json:"sort_order" binding:"omitempty,min=0"`
}

// LeadSourceResponse represents lead source data in response
type LeadSourceResponse struct {
	ID          string     `json:"id"`
	CompanyID   string     `json:"company_id"`
	Name        string     `json:"name"`
	Code        string     `json:"code"`
	Description *string    `json:"description,omitempty"`
	Color       *string    `json:"color,omitempty"`
	Icon        *string    `json:"icon,omitempty"`
	IsActive    bool       `json:"is_active"`
	SortOrder   int        `json:"sort_order"`
	CreatedAt   time.Time  `json:"created_at"`
	CreatedBy   string     `json:"created_by"`
	UpdatedAt   time.Time  `json:"updated_at"`
	UpdatedBy   *string    `json:"updated_by,omitempty"`
	DeletedAt   *time.Time `json:"deleted_at,omitempty"`
	DeletedBy   *string    `json:"deleted_by,omitempty"`
}

// LeadSourceListResponse represents paginated lead source list
type LeadSourceListResponse struct {
	LeadSources []LeadSourceResponse `json:"lead_sources"`
	Total       int64                `json:"total"`
	Page        int                  `json:"page"`
	PageSize    int                  `json:"page_size"`
	TotalPages  int                  `json:"total_pages"`
}

// LeadSourceQueryParams represents query parameters for listing lead sources
type LeadSourceQueryParams struct {
	Page     int    `form:"page,default=1" binding:"omitempty,min=1"`
	PageSize int    `form:"page_size,default=10" binding:"omitempty,min=1,max=100"`
	Search   string `form:"search" binding:"omitempty,max=255"`
	IsActive *bool  `form:"is_active" binding:"omitempty"`
}
