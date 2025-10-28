package domain

import "time"

// Module represents a module entity in the system for grouping permissions
// Supports hierarchical tree structure for dynamic menu generation
type Module struct {
	ID          string     `json:"id"`
	Code        string     `json:"code"`
	Name        string     `json:"name"`
	Description *string    `json:"description,omitempty"`
	Icon        *string    `json:"icon,omitempty"`
	Color       *string    `json:"color,omitempty"`
	SortOrder   int        `json:"sort_order"`
	IsActive    bool       `json:"is_active"`

	// Tree structure fields
	ParentID *string `json:"parent_id,omitempty"`
	Depth    int     `json:"depth"`

	// Frontend-matching fields (MenuItem/ChildItem interface)
	Heading       *string `json:"heading,omitempty"`
	To            *string `json:"to,omitempty"`
	URL           *string `json:"url,omitempty"`
	Permission    *string `json:"permission,omitempty"`
	Tooltip       *string `json:"tooltip,omitempty"`
	IsMenuVisible bool    `json:"is_menu_visible"`

	// Related data counts (for UX badges/indicators)
	PermissionsCount int `json:"permissions_count"`

	// Audit fields
	CreatedAt time.Time  `json:"created_at"`
	CreatedBy *string    `json:"created_by,omitempty"`
	UpdatedAt time.Time  `json:"updated_at"`
	UpdatedBy *string    `json:"updated_by,omitempty"`
	DeletedAt *time.Time `json:"deleted_at,omitempty"`
	DeletedBy *string    `json:"deleted_by,omitempty"`
}
