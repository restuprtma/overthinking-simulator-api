package dto

import (
	"time"
)

// CreateReflectionRequest is the payload to start a new reflection.
type CreateReflectionRequest struct {
	Thought string `json:"thought" binding:"required"`
}

// ContinueRequest is the payload for continuing an interactive conversation.
type ContinueRequest struct {
	UserMessage string `json:"user_message" binding:"required"`
}

// ReflectionSummary is the list-view projection of a reflection.
type ReflectionSummary struct {
	ID                string    `json:"id"`
	Thought           string    `json:"thought"`
	SafetyTriggered   bool      `json:"safety_triggered"`
	ConversationState string    `json:"conversation_state"`
	TotalTurns        int       `json:"total_turns"`
	CreatedAt         time.Time `json:"created_at"`
}

// ReflectionListQuery holds pagination parameters for listing reflections.
type ReflectionListQuery struct {
	Page  int `form:"page"`
	Limit int `form:"limit"`
}

// Defaults fills zero values with sane defaults.
func (q *ReflectionListQuery) Defaults() {
	if q.Page == 0 {
		q.Page = 1
	}
	if q.Limit == 0 {
		q.Limit = 10
	}
}

// NOTE: the create/continue/detail handlers serialize the domain types directly
// (domain.Reflection and domain.ContinueResponse), so there are deliberately no
// response DTOs mirroring them here — duplicates only drift out of sync.
