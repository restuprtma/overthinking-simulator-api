package dto

// WAHAWebhookPayload represents the incoming webhook from WAHA
type WAHAWebhookPayload struct {
	ID          string                 `json:"id"`
	Timestamp   int64                  `json:"timestamp"`
	Event       string                 `json:"event"`
	Session     string                 `json:"session"`
	Metadata    map[string]string      `json:"metadata"`
	Me          *WAHAMeInfo            `json:"me,omitempty"`
	Payload     map[string]interface{} `json:"payload"`
	Engine      string                 `json:"engine,omitempty"`
	Environment map[string]interface{} `json:"environment,omitempty"`
}

// WAHAMeInfo represents the authenticated WhatsApp account info
type WAHAMeInfo struct {
	ID       string `json:"id"`
	PushName string `json:"pushName"`
}

// WAHASessionStatusPayload represents session.status event payload
type WAHASessionStatusPayload struct {
	Status string `json:"status"`
}

// WAHAMessagePayload represents message.any event payload
type WAHAMessagePayload struct {
	ID            string                 `json:"id"`
	Timestamp     int64                  `json:"timestamp"`
	From          string                 `json:"from"`
	To            string                 `json:"to"`
	ChatID        string                 `json:"chatId"`
	Body          string                 `json:"body"`
	Type          string                 `json:"type"` // chat, image, video, document, audio, voice, location, etc.
	FromMe        bool                   `json:"fromMe"`
	HasMedia      bool                   `json:"hasMedia,omitempty"`
	MediaURL      string                 `json:"mediaUrl,omitempty"`
	MediaMimeType string                 `json:"mimetype,omitempty"`
	Caption       string                 `json:"caption,omitempty"`
	ACK           int                    `json:"ack,omitempty"`
	Location      map[string]interface{} `json:"location,omitempty"`
}

// WAHAMessageAckPayload represents message.ack event payload
type WAHAMessageAckPayload struct {
	ID  string `json:"id"`
	ACK int    `json:"ack"`
}
