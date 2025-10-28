package domain

import "time"

// PermissionTemplate represents a permission template entity
type PermissionTemplate struct {
	ID          string     `db:"id" json:"id"`
	Name        string     `db:"name" json:"name"`
	Description *string    `db:"description" json:"description,omitempty"`
	IsSystem    bool       `db:"is_system" json:"is_system"`
	CreatedAt   time.Time  `db:"created_at" json:"created_at"`
	CreatedBy   *string    `db:"created_by" json:"created_by,omitempty"`
	UpdatedAt   time.Time  `db:"updated_at" json:"updated_at"`
	UpdatedBy   *string    `db:"updated_by" json:"updated_by,omitempty"`
	DeletedAt   *time.Time `db:"deleted_at" json:"deleted_at,omitempty"`
	DeletedBy   *string    `db:"deleted_by" json:"deleted_by,omitempty"`
}

// PermissionTemplateAction represents the actions included in a template
type PermissionTemplateAction struct {
	PermissionTemplateID string    `db:"permission_template_id" json:"permission_template_id"`
	Action               string    `db:"action" json:"action"`
	CreatedAt            time.Time `db:"created_at" json:"created_at"`
	CreatedBy            *string   `db:"created_by" json:"created_by,omitempty"`
}
