package dto

import "time"

// CreateLeadRequest is the DTO for creating a new lead
type CreateLeadRequest struct {
	Name                   string    `json:"name" binding:"required,min=2,max=255"`
	ContactPerson          *string   `json:"contact_person" binding:"omitempty,max=255"`
	Phone                  string    `json:"phone" binding:"required,max=20"`
	Email                  *string   `json:"email" binding:"omitempty,email,max=255"`
	Category               string    `json:"category" binding:"required,oneof=hot warm cold"`
	Status                 *string   `json:"status" binding:"omitempty,oneof=new in_progress follow_up negotiation won lost"`
	SourceID               string    `json:"source_id" binding:"required,uuid"`
	AssignedToCompanyUserID *string  `json:"assigned_to_company_user_id" binding:"omitempty,uuid"`
	DealValue              *float64  `json:"deal_value" binding:"omitempty,min=0"`
	NextFollowUpAt         *time.Time `json:"next_follow_up_at" binding:"omitempty"`
	Notes                  *string   `json:"notes" binding:"omitempty"`
	Address                *string   `json:"address" binding:"omitempty"`
	City                   *string   `json:"city" binding:"omitempty,max=100"`
	Province               *string   `json:"province" binding:"omitempty,max=100"`
	PostalCode             *string   `json:"postal_code" binding:"omitempty,max=10"`
}

// UpdateLeadRequest is the DTO for updating a lead
// All fields are optional for partial updates
type UpdateLeadRequest struct {
	Name                   *string    `json:"name" binding:"omitempty,min=2,max=255"`
	ContactPerson          *string    `json:"contact_person" binding:"omitempty,max=255"`
	Phone                  *string    `json:"phone" binding:"omitempty,max=20"`
	Email                  *string    `json:"email" binding:"omitempty,email,max=255"`
	Category               *string    `json:"category" binding:"omitempty,oneof=hot warm cold"`
	Status                 *string    `json:"status" binding:"omitempty,oneof=new in_progress follow_up negotiation won lost"`
	SourceID               *string    `json:"source_id" binding:"omitempty,uuid"`
	AssignedToCompanyUserID *string   `json:"assigned_to_company_user_id" binding:"omitempty,uuid"`
	DealValue              *float64   `json:"deal_value" binding:"omitempty,min=0"`
	LastContactAt          *time.Time `json:"last_contact_at" binding:"omitempty"`
	NextFollowUpAt         *time.Time `json:"next_follow_up_at" binding:"omitempty"`
	Notes                  *string    `json:"notes" binding:"omitempty"`
	Address                *string    `json:"address" binding:"omitempty"`
	City                   *string    `json:"city" binding:"omitempty,max=100"`
	Province               *string    `json:"province" binding:"omitempty,max=100"`
	PostalCode             *string    `json:"postal_code" binding:"omitempty,max=10"`
}

// LeadSourceInfo represents basic lead source information
type LeadSourceInfo struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	Code string `json:"code"`
}

// AssignedUserInfo represents basic assigned user information
type AssignedUserInfo struct {
	CompanyUserID string `json:"company_user_id"`
	UserID        string `json:"user_id"`
	FullName      string `json:"full_name"`
	Email         string `json:"email"`
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

// LeadResponse is the DTO for lead response
type LeadResponse struct {
	ID                     string     `json:"id"`
	CompanyID              string     `json:"company_id"`
	LeadNumber             *string    `json:"lead_number,omitempty"`
	Name                   string     `json:"name"`
	ContactPerson          *string    `json:"contact_person,omitempty"`
	Phone                  string     `json:"phone"`
	Email                  *string    `json:"email,omitempty"`
	Category               string     `json:"category"`
	Status                 string     `json:"status"`
	DealValue              *float64   `json:"deal_value,omitempty"`
	LastContactAt          *time.Time `json:"last_contact_at,omitempty"`
	NextFollowUpAt         *time.Time `json:"next_follow_up_at,omitempty"`
	ResponseTimeMinutes    *int       `json:"response_time_minutes,omitempty"`
	Notes                  *string    `json:"notes,omitempty"`
	Address                *string    `json:"address,omitempty"`
	City                   *string    `json:"city,omitempty"`
	Province               *string    `json:"province,omitempty"`
	PostalCode             *string    `json:"postal_code,omitempty"`

	// Related information
	Source       *LeadSourceInfo   `json:"source,omitempty"`
	AssignedTo   *AssignedUserInfo `json:"assigned_to,omitempty"`

	// Audit trail
	Audit *AuditInfo `json:"audit,omitempty"`
}

// LeadQueryParams is the DTO for query parameters when listing leads
type LeadQueryParams struct {
	Page                   int     `form:"page,default=1" binding:"omitempty,min=1"`
	PageSize               int     `form:"page_size,default=10" binding:"omitempty,min=1,max=100"`
	Search                 string  `form:"search" binding:"omitempty,max=255"`
	Category               *string `form:"category" binding:"omitempty,oneof=hot warm cold"`
	Status                 *string `form:"status" binding:"omitempty,oneof=new in_progress follow_up negotiation won lost"`
	SourceID               *string `form:"source_id" binding:"omitempty,uuid"`
	AssignedToCompanyUserID *string `form:"assigned_to_company_user_id" binding:"omitempty,uuid"`
}
