package domain

import "time"

// RoleModuleTemplate represents the mapping between roles, modules, and permission templates
// This is a UI helper to remember which permission template was selected for each module
type RoleModuleTemplate struct {
	ID                   string     `db:"id" json:"id"`
	RoleID               string     `db:"role_id" json:"role_id"`
	ModuleID             string     `db:"module_id" json:"module_id"`
	PermissionTemplateID string     `db:"permission_template_id" json:"permission_template_id"`
	CreatedAt            time.Time  `db:"created_at" json:"created_at"`
	CreatedBy            *string    `db:"created_by" json:"created_by,omitempty"`
	UpdatedAt            time.Time  `db:"updated_at" json:"updated_at"`
	UpdatedBy            *string    `db:"updated_by" json:"updated_by,omitempty"`
	DeletedAt            *time.Time `db:"deleted_at" json:"deleted_at,omitempty"`
	DeletedBy            *string    `db:"deleted_by" json:"deleted_by,omitempty"`
}
