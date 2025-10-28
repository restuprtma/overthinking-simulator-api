package domain

import "time"

// Company represents a company entity in the system
type Company struct {
	ID          string     `json:"id"`
	OwnerID     string     `json:"owner_id"`
	Name        string     `json:"name"`
	Code        string     `json:"code"`
	TaxID       *string    `json:"tax_id,omitempty"`
	Phone       *string    `json:"phone,omitempty"`
	Email       *string    `json:"email,omitempty"`
	Website     *string    `json:"website,omitempty"`
	LogoURL     *string    `json:"logo_url,omitempty"`
	Address     *string    `json:"address,omitempty"`
	VillageID   *string    `json:"village_id,omitempty"`
	MaxUsers    int        `json:"max_users"`
	MaxBranches int        `json:"max_branches"`
	IsActive    bool       `json:"is_active"`
	CreatedAt   time.Time  `json:"created_at"`
	CreatedBy   *string    `json:"created_by,omitempty"`
	UpdatedAt   time.Time  `json:"updated_at"`
	UpdatedBy   *string    `json:"updated_by,omitempty"`
	DeletedAt   *time.Time `json:"deleted_at,omitempty"`
	DeletedBy   *string    `json:"deleted_by,omitempty"`
}

// CompanyBasic represents basic company information (for responses that don't need full details)
type CompanyBasic struct {
	ID      string  `json:"id"`
	Name    string  `json:"name"`
	Code    string  `json:"code"`
	LogoURL *string `json:"logo_url,omitempty"`
}
