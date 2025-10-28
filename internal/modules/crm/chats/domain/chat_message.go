package domain

import "time"

// ChatMessage represents an individual message in a chat conversation
type ChatMessage struct {
	ID              string     `json:"id"`
	ChatID          string     `json:"chat_id"`
	SenderType      string     `json:"sender_type"` // customer, sales, system, auto_reply
	SenderID        *string    `json:"sender_id,omitempty"`
	SenderName      *string    `json:"sender_name,omitempty"`
	Content         string     `json:"content"`
	MessageType     string     `json:"message_type"` // text, image, video, file, voice, location
	MediaURL        *string    `json:"media_url,omitempty"`
	MediaMimeType   *string    `json:"media_mime_type,omitempty"`
	MediaSizeBytes  *int64     `json:"media_size_bytes,omitempty"`
	IsRead          bool       `json:"is_read"`
	ReadAt          *time.Time `json:"read_at,omitempty"`
	IsSent          bool       `json:"is_sent"`
	SentAt          time.Time  `json:"sent_at"`
	DeliveryStatus  *string    `json:"delivery_status,omitempty"` // sent, delivered, read, failed
	Metadata        *string    `json:"metadata,omitempty"` // JSONB stored as string
	CreatedAt       time.Time  `json:"created_at"`
}
