package dto

import "time"

// UpdateCompanySettingsRequest is the DTO for updating company settings
// All fields are optional for partial updates
type UpdateCompanySettingsRequest struct {
	// Auto-Reply Settings
	AutoReplyEnabled            *bool   `json:"auto_reply_enabled" binding:"omitempty"`
	AutoReplyGlobalDelaySeconds *int    `json:"auto_reply_global_delay_seconds" binding:"omitempty,min=0"`
	AutoReplyBusinessHoursStart *string `json:"auto_reply_business_hours_start" binding:"omitempty"` // Format: "HH:MM:SS"
	AutoReplyBusinessHoursEnd   *string `json:"auto_reply_business_hours_end" binding:"omitempty"`   // Format: "HH:MM:SS"
	AutoReplyWorkingDays        []int   `json:"auto_reply_working_days" binding:"omitempty,dive,min=0,max=6"`

	// Chat Settings
	ChatAutoAssignEnabled  *bool   `json:"chat_auto_assign_enabled" binding:"omitempty"`
	ChatAssignmentStrategy *string `json:"chat_assignment_strategy" binding:"omitempty,oneof=round_robin least_busy manual"`
	ChatIdleTimeoutMinutes *int    `json:"chat_idle_timeout_minutes" binding:"omitempty,min=1"`

	// Lead Settings
	LeadAutoCategorization *bool        `json:"lead_auto_categorization_enabled" binding:"omitempty"`
	LeadHotCriteria        interface{}  `json:"lead_hot_criteria" binding:"omitempty"`
	LeadWarmCriteria       interface{}  `json:"lead_warm_criteria" binding:"omitempty"`
	LeadColdCriteria       interface{}  `json:"lead_cold_criteria" binding:"omitempty"`

	// AI Settings
	AIEnabled             *bool    `json:"ai_enabled" binding:"omitempty"`
	AIConfidenceThreshold *float64 `json:"ai_confidence_threshold" binding:"omitempty,min=0,max=1"`
	AITrainingMode        *bool    `json:"ai_training_mode" binding:"omitempty"`

	// Notification Settings
	NotificationsEnabled *bool       `json:"notifications_enabled" binding:"omitempty"`
	NotificationChannels interface{} `json:"notification_channels" binding:"omitempty"`

	// Timezone
	Timezone *string `json:"timezone" binding:"omitempty"`
}

// CompanySettingsResponse is the DTO for company settings response
type CompanySettingsResponse struct {
	ID        string `json:"id"`
	CompanyID string `json:"company_id"`

	// Auto-Reply Settings
	AutoReplyEnabled            bool   `json:"auto_reply_enabled"`
	AutoReplyGlobalDelaySeconds int    `json:"auto_reply_global_delay_seconds"`
	AutoReplyBusinessHoursStart string `json:"auto_reply_business_hours_start"`
	AutoReplyBusinessHoursEnd   string `json:"auto_reply_business_hours_end"`
	AutoReplyWorkingDays        []int  `json:"auto_reply_working_days"`

	// Chat Settings
	ChatAutoAssignEnabled  bool   `json:"chat_auto_assign_enabled"`
	ChatAssignmentStrategy string `json:"chat_assignment_strategy"`
	ChatIdleTimeoutMinutes int    `json:"chat_idle_timeout_minutes"`

	// Lead Settings
	LeadAutoCategorization bool        `json:"lead_auto_categorization_enabled"`
	LeadHotCriteria        interface{} `json:"lead_hot_criteria,omitempty"`
	LeadWarmCriteria       interface{} `json:"lead_warm_criteria,omitempty"`
	LeadColdCriteria       interface{} `json:"lead_cold_criteria,omitempty"`

	// AI Settings
	AIEnabled             bool    `json:"ai_enabled"`
	AIConfidenceThreshold float64 `json:"ai_confidence_threshold"`
	AITrainingMode        bool    `json:"ai_training_mode"`

	// Notification Settings
	NotificationsEnabled bool        `json:"notifications_enabled"`
	NotificationChannels interface{} `json:"notification_channels,omitempty"`

	// Timezone
	Timezone string `json:"timezone"`

	// Audit
	CreatedAt     time.Time `json:"created_at"`
	CreatedBy     *string   `json:"created_by,omitempty"`
	CreatedByName *string   `json:"created_by_name,omitempty"`
	UpdatedAt     time.Time `json:"updated_at"`
	UpdatedBy     *string   `json:"updated_by,omitempty"`
	UpdatedByName *string   `json:"updated_by_name,omitempty"`
}
