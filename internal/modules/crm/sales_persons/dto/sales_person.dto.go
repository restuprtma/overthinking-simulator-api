package dto

import "time"

// CreateSalesPersonRequest is the DTO for creating a new sales person
type CreateSalesPersonRequest struct {
	CompanyUserID       string   `json:"company_user_id" binding:"required,uuid"`
	SalesCode           string   `json:"sales_code" binding:"required,min=2,max=50"`
	SalesName           *string  `json:"sales_name" binding:"omitempty,max=255"`
	SalesArea           *string  `json:"sales_area" binding:"omitempty,max=100"`
	SalesTarget         *float64 `json:"sales_target" binding:"omitempty,min=0"`
	CommissionRate      *float64 `json:"commission_rate" binding:"omitempty,min=0,max=100"`
	Whatsapp            *string  `json:"whatsapp" binding:"omitempty,max=20"`
	IsWhatsappConnected *bool    `json:"is_whatsapp_connected" binding:"omitempty"`
	IsActive            *bool    `json:"is_active" binding:"omitempty"`
	Notes               *string  `json:"notes" binding:"omitempty"`
}

// UpdateSalesPersonRequest is the DTO for updating a sales person
// All fields are optional for partial updates
type UpdateSalesPersonRequest struct {
	SalesCode           *string  `json:"sales_code" binding:"omitempty,min=2,max=50"`
	SalesName           *string  `json:"sales_name" binding:"omitempty,max=255"`
	SalesArea           *string  `json:"sales_area" binding:"omitempty,max=100"`
	SalesTarget         *float64 `json:"sales_target" binding:"omitempty,min=0"`
	CommissionRate      *float64 `json:"commission_rate" binding:"omitempty,min=0,max=100"`
	Whatsapp            *string  `json:"whatsapp" binding:"omitempty,max=20"`
	IsWhatsappConnected *bool    `json:"is_whatsapp_connected" binding:"omitempty"`
	IsActive            *bool    `json:"is_active" binding:"omitempty"`
	Notes               *string  `json:"notes" binding:"omitempty"`
}

// UserInfo represents basic user information
type UserInfo struct {
	ID       string `json:"id"`
	FullName string `json:"full_name"`
	Email    string `json:"email"`
}

// AuditInfo represents audit trail information
type AuditInfo struct {
	CreatedAt     time.Time  `json:"created_at"`
	CreatedBy     *string    `json:"created_by,omitempty"`
	CreatedByName *string    `json:"created_by_name,omitempty"`
	UpdatedAt     time.Time  `json:"updated_at"`
	UpdatedBy     *string    `json:"updated_by,omitempty"`
	UpdatedByName *string    `json:"updated_by_name,omitempty"`
	DeletedAt     *time.Time `json:"deleted_at,omitempty"`
	DeletedBy     *string    `json:"deleted_by,omitempty"`
	DeletedByName *string    `json:"deleted_by_name,omitempty"`
}

// SalesPersonResponse is the DTO for sales person response
type SalesPersonResponse struct {
	ID                  string     `json:"id"`
	CompanyUserID       string     `json:"company_user_id"`
	SalesCode           string     `json:"sales_code"`
	SalesName           *string    `json:"sales_name,omitempty"`
	SalesArea           *string    `json:"sales_area,omitempty"`
	SalesTarget         *float64   `json:"sales_target,omitempty"`
	CommissionRate      *float64   `json:"commission_rate,omitempty"`
	Whatsapp            *string    `json:"whatsapp,omitempty"`
	IsWhatsappConnected bool       `json:"is_whatsapp_connected"`
	IsActive            bool       `json:"is_active"`
	Notes               *string    `json:"notes,omitempty"`

	// Related user information
	User *UserInfo `json:"user,omitempty"`

	// Audit trail
	Audit *AuditInfo `json:"audit,omitempty"`
}

// SalesPersonQueryParams is the DTO for query parameters when listing sales persons
type SalesPersonQueryParams struct {
	Page                int     `form:"page,default=1" binding:"omitempty,min=1"`
	PageSize            int     `form:"page_size,default=10" binding:"omitempty,min=1,max=100"`
	Search              string  `form:"search" binding:"omitempty,max=255"`
	IsActive            *bool   `form:"is_active" binding:"omitempty"`
	IsWhatsappConnected *bool   `form:"is_whatsapp_connected" binding:"omitempty"`
	SalesArea           *string `form:"sales_area" binding:"omitempty,max=100"`
}
