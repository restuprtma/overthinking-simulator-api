package dto

import "time"

// ConnectWhatsAppResponse is the response when connecting WhatsApp
type ConnectWhatsAppResponse struct {
	SalesPersonID string    `json:"sales_person_id"`
	SessionName   string    `json:"session_name"`
	WhatsappNumber string   `json:"whatsapp_number"`
	PairingCode   string    `json:"pairing_code"`
	Status        string    `json:"status"`
	ExpiresAt     time.Time `json:"expires_at"`
}

// WhatsAppStatusResponse is the response for WhatsApp connection status
type WhatsAppStatusResponse struct {
	IsConnected    bool       `json:"is_connected"`
	Status         *string    `json:"status,omitempty"` // nil if never connected
	WhatsappNumber string     `json:"whatsapp_number"`
	SessionName    *string    `json:"session_name,omitempty"`
	ConnectedAt    *time.Time `json:"connected_at,omitempty"`
	LastSeenAt     *time.Time `json:"last_seen_at,omitempty"`
}

// DisconnectWhatsAppRequest is the request to disconnect WhatsApp (optional body)
type DisconnectWhatsAppRequest struct {
	Logout bool `json:"logout"` // If true, logout and delete session; if false, just stop
}
