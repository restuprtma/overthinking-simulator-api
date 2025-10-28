package dto

import "time"

// CreateChatRequest is the DTO for creating a new chat (from WhatsApp webhook)
type CreateChatRequest struct {
	CustomerName           string   `json:"customer_name" binding:"required,min=2,max=255"`
	Phone                  string   `json:"phone" binding:"required,max=20"`
	Email                  *string  `json:"email" binding:"omitempty,email,max=255"`
	LeadID                 *string  `json:"lead_id" binding:"omitempty,uuid"`
	Platform               string   `json:"platform" binding:"required,oneof=whatsapp telegram email phone"`
	AssignedToCompanyUserID *string `json:"assigned_to_company_user_id" binding:"omitempty,uuid"`
	Category               *string  `json:"category" binding:"omitempty,oneof=hot warm cold"`

	// First message info (optional, for creating chat with initial message)
	FirstMessage           *CreateChatMessageRequest `json:"first_message" binding:"omitempty"`
}

// CreateChatMessageRequest is the DTO for creating a new message in a chat
type CreateChatMessageRequest struct {
	SenderType     string  `json:"sender_type" binding:"required,oneof=customer sales system auto_reply"`
	SenderID       *string `json:"sender_id" binding:"omitempty,uuid"`
	SenderName     *string `json:"sender_name" binding:"omitempty,max=255"`
	Content        string  `json:"content" binding:"required"`
	MessageType    string  `json:"message_type" binding:"required,oneof=text image video file voice location"`
	MediaURL       *string `json:"media_url" binding:"omitempty,max=500"`
	MediaMimeType  *string `json:"media_mime_type" binding:"omitempty,max=100"`
	MediaSizeBytes *int64  `json:"media_size_bytes" binding:"omitempty,min=0"`
	DeliveryStatus *string `json:"delivery_status" binding:"omitempty,oneof=sent delivered read failed"`
	Metadata       *string `json:"metadata" binding:"omitempty"`
}

// AssignedUserInfo represents basic assigned user information
type AssignedUserInfo struct {
	CompanyUserID string `json:"company_user_id"`
	UserID        string `json:"user_id"`
	FullName      string `json:"full_name"`
	Email         string `json:"email"`
}

// LeadInfo represents basic lead information
type LeadInfo struct {
	ID         string `json:"id"`
	Name       string `json:"name"`
	LeadNumber string `json:"lead_number"`
}

// ChatMessageResponse is the DTO for chat message response
type ChatMessageResponse struct {
	ID              string     `json:"id"`
	ChatID          string     `json:"chat_id"`
	SenderType      string     `json:"sender_type"`
	SenderID        *string    `json:"sender_id,omitempty"`
	SenderName      *string    `json:"sender_name,omitempty"`
	Content         string     `json:"content"`
	MessageType     string     `json:"message_type"`
	MediaURL        *string    `json:"media_url,omitempty"`
	MediaMimeType   *string    `json:"media_mime_type,omitempty"`
	MediaSizeBytes  *int64     `json:"media_size_bytes,omitempty"`
	IsRead          bool       `json:"is_read"`
	ReadAt          *time.Time `json:"read_at,omitempty"`
	IsSent          bool       `json:"is_sent"`
	SentAt          time.Time  `json:"sent_at"`
	DeliveryStatus  *string    `json:"delivery_status,omitempty"`
	CreatedAt       time.Time  `json:"created_at"`
}

// ChatResponse is the DTO for chat response
type ChatResponse struct {
	ID                     string     `json:"id"`
	CompanyID              string     `json:"company_id"`
	CustomerName           string     `json:"customer_name"`
	Phone                  string     `json:"phone"`
	Email                  *string    `json:"email,omitempty"`
	Platform               string     `json:"platform"`
	Status                 string     `json:"status"`
	Category               *string    `json:"category,omitempty"`
	LastMessageAt          *time.Time `json:"last_message_at,omitempty"`
	LastMessagePreview     *string    `json:"last_message_preview,omitempty"`
	LastMessageSender      *string    `json:"last_message_sender,omitempty"`
	UnreadCount            int        `json:"unread_count"`
	MessageCount           int        `json:"message_count"`
	AvgResponseTimeMinutes *int       `json:"avg_response_time_minutes,omitempty"`
	FirstResponseTimeMinutes *int     `json:"first_response_time_minutes,omitempty"`
	SentimentScore         *float64   `json:"sentiment_score,omitempty"`
	Tags                   []string   `json:"tags,omitempty"`
	CreatedAt              time.Time  `json:"created_at"`
	UpdatedAt              time.Time  `json:"updated_at"`

	// Related information
	Lead         *LeadInfo         `json:"lead,omitempty"`
	AssignedTo   *AssignedUserInfo `json:"assigned_to,omitempty"`
}

// ChatDetailResponse is the DTO for chat detail with messages
type ChatDetailResponse struct {
	Chat     ChatResponse          `json:"chat"`
	Messages []ChatMessageResponse `json:"messages"`
}

// ChatQueryParams is the DTO for query parameters when listing chats
type ChatQueryParams struct {
	Page                   int     `form:"page,default=1" binding:"omitempty,min=1"`
	PageSize               int     `form:"page_size,default=10" binding:"omitempty,min=1,max=100"`
	Search                 string  `form:"search" binding:"omitempty,max=255"`
	Platform               *string `form:"platform" binding:"omitempty,oneof=whatsapp telegram email phone"`
	Status                 *string `form:"status" binding:"omitempty,oneof=active archived closed"`
	Category               *string `form:"category" binding:"omitempty,oneof=hot warm cold"`
	AssignedToCompanyUserID *string `form:"assigned_to_company_user_id" binding:"omitempty,uuid"`
}
