package domain

import "time"

// CompanySettings represents CRM settings for a company
type CompanySettings struct {
	ID         string    `json:"id"`
	CompanyID  string    `json:"company_id"`

	// Auto-Reply Settings
	AutoReplyEnabled              bool      `json:"auto_reply_enabled"`
	AutoReplyGlobalDelaySeconds   int       `json:"auto_reply_global_delay_seconds"`
	AutoReplyBusinessHoursStart   string    `json:"auto_reply_business_hours_start"` // TIME stored as string
	AutoReplyBusinessHoursEnd     string    `json:"auto_reply_business_hours_end"`   // TIME stored as string
	AutoReplyWorkingDays          []int     `json:"auto_reply_working_days"`         // Array of integers

	// Chat Settings
	ChatAutoAssignEnabled  bool      `json:"chat_auto_assign_enabled"`
	ChatAssignmentStrategy string    `json:"chat_assignment_strategy"` // round_robin, least_busy, manual
	ChatIdleTimeoutMinutes int       `json:"chat_idle_timeout_minutes"`

	// Lead Settings
	LeadAutoCategorization bool     `json:"lead_auto_categorization_enabled"`
	LeadHotCriteria        *string   `json:"lead_hot_criteria,omitempty"`  // JSONB stored as string
	LeadWarmCriteria       *string   `json:"lead_warm_criteria,omitempty"` // JSONB stored as string
	LeadColdCriteria       *string   `json:"lead_cold_criteria,omitempty"` // JSONB stored as string

	// AI Settings
	AIEnabled              bool      `json:"ai_enabled"`
	AIConfidenceThreshold  float64   `json:"ai_confidence_threshold"`
	AITrainingMode         bool      `json:"ai_training_mode"`

	// Notification Settings
	NotificationsEnabled   bool      `json:"notifications_enabled"`
	NotificationChannels   *string   `json:"notification_channels,omitempty"` // JSONB stored as string

	// Timezone
	Timezone               string    `json:"timezone"`

	CreatedAt              time.Time  `json:"created_at"`
	CreatedBy              *string    `json:"created_by,omitempty"`
	UpdatedAt              time.Time  `json:"updated_at"`
	UpdatedBy              *string    `json:"updated_by,omitempty"`

	// Joined fields
	CreatedByName          *string    `json:"created_by_name,omitempty"`
	UpdatedByName          *string    `json:"updated_by_name,omitempty"`
}
