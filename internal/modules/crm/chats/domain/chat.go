package domain

import "time"

// Chat represents a chat session with a customer
type Chat struct {
	ID                     string     `json:"id"`
	CompanyID              string     `json:"company_id"`
	LeadID                 *string    `json:"lead_id,omitempty"`
	CustomerName           string     `json:"customer_name"`
	Phone                  string     `json:"phone"`
	Email                  *string    `json:"email,omitempty"`
	AssignedToCompanyUserID *string   `json:"assigned_to_company_user_id,omitempty"`
	Platform               string     `json:"platform"` // whatsapp, telegram, email, phone
	Status                 string     `json:"status"`   // active, archived, closed
	Category               *string    `json:"category,omitempty"` // hot, warm, cold
	LastMessageAt          *time.Time `json:"last_message_at,omitempty"`
	LastMessagePreview     *string    `json:"last_message_preview,omitempty"`
	LastMessageSender      *string    `json:"last_message_sender,omitempty"` // customer, sales, system
	UnreadCount            int        `json:"unread_count"`
	MessageCount           int        `json:"message_count"`
	AIInsights             *string    `json:"ai_insights,omitempty"` // JSONB stored as string
	AvgResponseTimeMinutes *int       `json:"avg_response_time_minutes,omitempty"`
	FirstResponseTimeMinutes *int     `json:"first_response_time_minutes,omitempty"`
	SentimentScore         *float64   `json:"sentiment_score,omitempty"`
	Tags                   []string   `json:"tags,omitempty"`
	CreatedAt              time.Time  `json:"created_at"`
	CreatedBy              *string    `json:"created_by,omitempty"`
	UpdatedAt              time.Time  `json:"updated_at"`
	UpdatedBy              *string    `json:"updated_by,omitempty"`
	DeletedAt              *time.Time `json:"deleted_at,omitempty"`
	DeletedBy              *string    `json:"deleted_by,omitempty"`

	// Joined fields from related tables
	LeadName               *string `json:"lead_name,omitempty"`
	AssignedToUserID       *string `json:"assigned_to_user_id,omitempty"`
	AssignedToUserName     *string `json:"assigned_to_user_name,omitempty"`
	AssignedToUserEmail    *string `json:"assigned_to_user_email,omitempty"`
}
