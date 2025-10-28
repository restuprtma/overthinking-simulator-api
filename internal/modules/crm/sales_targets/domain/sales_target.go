package domain

import "time"

// SalesTarget represents a sales target entity in the CRM system
type SalesTarget struct {
	ID                    string     `json:"id"`
	CompanyID             string     `json:"company_id"`
	Year                  int        `json:"year"`
	Month                 int        `json:"month"`
	TargetType            string     `json:"target_type"` // team, individual
	UserID                *string    `json:"user_id,omitempty"`
	TargetAmount          float64    `json:"target_amount"`
	AchievedAmount        float64    `json:"achieved_amount"`
	AchievementPercentage float64    `json:"achievement_percentage"`
	Notes                 *string    `json:"notes,omitempty"`
	CreatedAt             time.Time  `json:"created_at"`
	CreatedBy             *string    `json:"created_by,omitempty"`
	UpdatedAt             time.Time  `json:"updated_at"`
	UpdatedBy             *string    `json:"updated_by,omitempty"`
	DeletedAt             *time.Time `json:"deleted_at,omitempty"`
	DeletedBy             *string    `json:"deleted_by,omitempty"`
}
