package dto

import "time"

// CreateAutoReplyRuleRequest is the DTO for creating a new auto-reply rule
type CreateAutoReplyRuleRequest struct {
	Name            string      `json:"name" binding:"required,min=2,max=255"`
	Description     *string     `json:"description" binding:"omitempty"`
	IsEnabled       *bool       `json:"is_enabled" binding:"omitempty"`
	TriggerType     string      `json:"trigger_type" binding:"required,oneof=first_message outside_business_hours keyword_match no_sales_available high_priority_lead ai_confidence_low conversation_flow"`
	Conditions      interface{} `json:"conditions" binding:"omitempty"` // Can be map or struct, will be marshaled to JSONB
	MessageTemplate string      `json:"message_template" binding:"required,min=1"`
	DelaySeconds    *int        `json:"delay_seconds" binding:"omitempty,min=0"`
	Priority        *int        `json:"priority" binding:"omitempty,min=1"`
}

// UpdateAutoReplyRuleRequest is the DTO for updating an auto-reply rule
// All fields are optional for partial updates
type UpdateAutoReplyRuleRequest struct {
	Name            *string     `json:"name" binding:"omitempty,min=2,max=255"`
	Description     *string     `json:"description" binding:"omitempty"`
	IsEnabled       *bool       `json:"is_enabled" binding:"omitempty"`
	TriggerType     *string     `json:"trigger_type" binding:"omitempty,oneof=first_message outside_business_hours keyword_match no_sales_available high_priority_lead ai_confidence_low conversation_flow"`
	Conditions      interface{} `json:"conditions" binding:"omitempty"`
	MessageTemplate *string     `json:"message_template" binding:"omitempty,min=1"`
	DelaySeconds    *int        `json:"delay_seconds" binding:"omitempty,min=0"`
	Priority        *int        `json:"priority" binding:"omitempty,min=1"`
}

// AutoReplyRuleConditions represents the structure of conditions JSONB
type AutoReplyRuleConditions struct {
	Keywords            []string `json:"keywords,omitempty"`             // For keyword_match trigger
	TimeRange           *TimeRange `json:"timeRange,omitempty"`          // For outside_business_hours trigger
	SalesAssigned       []string `json:"salesAssigned,omitempty"`        // UUID array for specific sales assignment
	LeadStatus          []string `json:"leadStatus,omitempty"`           // Lead status filtering
	MinConfidenceScore  *float64 `json:"minConfidenceScore,omitempty"`   // For AI confidence threshold
}

// TimeRange represents time range for business hours
type TimeRange struct {
	Start string `json:"start"` // Format: "HH:MM"
	End   string `json:"end"`   // Format: "HH:MM"
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

// AutoReplyRuleResponse is the DTO for auto-reply rule response
type AutoReplyRuleResponse struct {
	ID              string      `json:"id"`
	CompanyID       string      `json:"company_id"`
	Name            string      `json:"name"`
	Description     *string     `json:"description,omitempty"`
	IsEnabled       bool        `json:"is_enabled"`
	TriggerType     string      `json:"trigger_type"`
	Conditions      interface{} `json:"conditions,omitempty"` // Will be unmarshaled from JSONB
	MessageTemplate string      `json:"message_template"`
	DelaySeconds    int         `json:"delay_seconds"`
	Priority        int         `json:"priority"`
	UsageCount      int         `json:"usage_count"`
	LastUsedAt      *time.Time  `json:"last_used_at,omitempty"`

	// Audit trail
	Audit *AuditInfo `json:"audit,omitempty"`
}

// AutoReplyRuleQueryParams is the DTO for query parameters when listing auto-reply rules
type AutoReplyRuleQueryParams struct {
	Page        int     `form:"page,default=1" binding:"omitempty,min=1"`
	PageSize    int     `form:"page_size,default=10" binding:"omitempty,min=1,max=100"`
	Search      string  `form:"search" binding:"omitempty,max=255"`
	TriggerType *string `form:"trigger_type" binding:"omitempty,oneof=first_message outside_business_hours keyword_match no_sales_available high_priority_lead ai_confidence_low conversation_flow"`
	IsEnabled   *bool   `form:"is_enabled" binding:"omitempty"`
}
