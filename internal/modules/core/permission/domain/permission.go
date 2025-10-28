package domain

import "time"

// Permission represents a permission entity in the system
type Permission struct {
	ID          string     `json:"id"`
	ModuleID    string     `json:"module_id"`
	Module      *Module    `json:"module,omitempty"`
	Resource    string     `json:"resource"`
	Action      string     `json:"action"`
	Description *string    `json:"description,omitempty"`
	CreatedAt   time.Time  `json:"created_at"`
	CreatedBy   *string    `json:"created_by,omitempty"`
	UpdatedAt   time.Time  `json:"updated_at"`
	UpdatedBy   *string    `json:"updated_by,omitempty"`
	DeletedAt   *time.Time `json:"deleted_at,omitempty"`
	DeletedBy   *string    `json:"deleted_by,omitempty"`
}
