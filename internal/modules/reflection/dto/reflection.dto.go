package dto

import (
	"time"

	"venturo-skeleton-go/internal/modules/reflection/domain"
)

// CreateReflectionRequest is the payload to start a new reflection.
type CreateReflectionRequest struct {
	Thought string `json:"thought" binding:"required"`
}

// ReflectionSummary is the list-view projection of a reflection.
type ReflectionSummary struct {
	ID              string    `json:"id"`
	Thought         string    `json:"thought"`
	SafetyTriggered bool      `json:"safety_triggered"`
	CreatedAt       time.Time `json:"created_at"`
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

// ReflectionResponse is the full-detail representation of a reflection.
type ReflectionResponse struct {
	ID                   string               `json:"id"`
	UserID               string               `json:"user_id"`
	Thought              string               `json:"thought"`
	DetectedDistortions  []domain.Distortion  `json:"detected_distortions"`
	CoreFear             string               `json:"core_fear"`
	Dialog               []domain.DialogTurn  `json:"dialog"`
	ActionableSuggestion string               `json:"actionable_suggestion"`
	SafetyTriggered      bool                 `json:"safety_triggered"`
	SafetyResponse       *string              `json:"safety_response,omitempty"`
	CreatedAt            time.Time            `json:"created_at"`
}
