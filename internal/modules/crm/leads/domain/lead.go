package domain

import "time"

// Lead represents a prospect/lead in the CRM system
type Lead struct {
	ID                     string     `json:"id"`
	CompanyID              string     `json:"company_id"`
	LeadNumber             *string    `json:"lead_number,omitempty"`
	Name                   string     `json:"name"`
	ContactPerson          *string    `json:"contact_person,omitempty"`
	Phone                  string     `json:"phone"`
	Email                  *string    `json:"email,omitempty"`
	Category               string     `json:"category"` // hot, warm, cold
	Status                 string     `json:"status"`   // new, in_progress, follow_up, negotiation, won, lost
	SourceID               string     `json:"source_id"`
	AssignedToCompanyUserID *string   `json:"assigned_to_company_user_id,omitempty"`
	DealValue              *float64   `json:"deal_value,omitempty"`
	LastContactAt          *time.Time `json:"last_contact_at,omitempty"`
	NextFollowUpAt         *time.Time `json:"next_follow_up_at,omitempty"`
	ResponseTimeMinutes    *int       `json:"response_time_minutes,omitempty"`
	AIHighlights           *string    `json:"ai_highlights,omitempty"` // JSONB stored as string
	Notes                  *string    `json:"notes,omitempty"`
	Address                *string    `json:"address,omitempty"`
	City                   *string    `json:"city,omitempty"`
	Province               *string    `json:"province,omitempty"`
	PostalCode             *string    `json:"postal_code,omitempty"`
	CreatedAt              time.Time  `json:"created_at"`
	CreatedBy              *string    `json:"created_by,omitempty"`
	UpdatedAt              time.Time  `json:"updated_at"`
	UpdatedBy              *string    `json:"updated_by,omitempty"`
	DeletedAt              *time.Time `json:"deleted_at,omitempty"`
	DeletedBy              *string    `json:"deleted_by,omitempty"`

	// Joined fields from related tables
	SourceName             *string `json:"source_name,omitempty"`
	AssignedToUserID       *string `json:"assigned_to_user_id,omitempty"`
	AssignedToUserName     *string `json:"assigned_to_user_name,omitempty"`
	AssignedToUserEmail    *string `json:"assigned_to_user_email,omitempty"`
	CreatedByName          *string `json:"created_by_name,omitempty"`
	UpdatedByName          *string `json:"updated_by_name,omitempty"`
	DeletedByName          *string `json:"deleted_by_name,omitempty"`
}
