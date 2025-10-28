package domain

import "time"

// AutoReplyRule represents an auto-reply rule in the CRM system
type AutoReplyRule struct {
	ID              string     `json:"id"`
	CompanyID       string     `json:"company_id"`
	Name            string     `json:"name"`
	Description     *string    `json:"description,omitempty"`
	IsEnabled       bool       `json:"is_enabled"`
	TriggerType     string     `json:"trigger_type"` // first_message, outside_business_hours, keyword_match, no_sales_available, high_priority_lead, ai_confidence_low, conversation_flow
	Conditions      *string    `json:"conditions,omitempty"` // JSONB stored as string
	MessageTemplate string     `json:"message_template"`
	DelaySeconds    int        `json:"delay_seconds"`
	Priority        int        `json:"priority"`
	UsageCount      int        `json:"usage_count"`
	LastUsedAt      *time.Time `json:"last_used_at,omitempty"`
	CreatedAt       time.Time  `json:"created_at"`
	CreatedBy       *string    `json:"created_by,omitempty"`
	UpdatedAt       time.Time  `json:"updated_at"`
	UpdatedBy       *string    `json:"updated_by,omitempty"`
	DeletedAt       *time.Time `json:"deleted_at,omitempty"`
	DeletedBy       *string    `json:"deleted_by,omitempty"`

	// Joined fields from related tables
	CreatedByName *string `json:"created_by_name,omitempty"`
	UpdatedByName *string `json:"updated_by_name,omitempty"`
	DeletedByName *string `json:"deleted_by_name,omitempty"`
}
