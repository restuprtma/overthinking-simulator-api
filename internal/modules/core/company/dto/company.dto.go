package dto

import "time"

// CreateCompanyRequest represents request to create a new company
type CreateCompanyRequest struct {
	Name        string  `json:"name" binding:"required,min=3,max=255"`
	Code        string  `json:"code" binding:"required,min=2,max=50,alphanum"`
	TaxID       *string `json:"tax_id" binding:"omitempty,max=50"`
	Phone       *string `json:"phone" binding:"omitempty,min=10,max=20"`
	Email       *string `json:"email" binding:"omitempty,email,max=255"`
	Website     *string `json:"website" binding:"omitempty,url,max=255"`
	LogoURL     *string `json:"logo_url" binding:"omitempty,url,max=500"`
	Address     *string `json:"address" binding:"omitempty,max=1000"`
	VillageID   *string `json:"village_id" binding:"omitempty,uuid"`
	MaxUsers    *int    `json:"max_users" binding:"omitempty,min=1,max=1000"`
	MaxBranches *int    `json:"max_branches" binding:"omitempty,min=1,max=100"`
}

// UpdateCompanyRequest represents request to update company
type UpdateCompanyRequest struct {
	Name        *string `json:"name" binding:"omitempty,min=3,max=255"`
	TaxID       *string `json:"tax_id" binding:"omitempty,max=50"`
	Phone       *string `json:"phone" binding:"omitempty,min=10,max=20"`
	Email       *string `json:"email" binding:"omitempty,email,max=255"`
	Website     *string `json:"website" binding:"omitempty,url,max=255"`
	LogoURL     *string `json:"logo_url" binding:"omitempty,url,max=500"`
	Address     *string `json:"address" binding:"omitempty,max=1000"`
	VillageID   *string `json:"village_id" binding:"omitempty,uuid"`
	MaxUsers    *int    `json:"max_users" binding:"omitempty,min=1,max=1000"`
	MaxBranches *int    `json:"max_branches" binding:"omitempty,min=1,max=100"`
	IsActive    *bool   `json:"is_active" binding:"omitempty"`
}

// CompanyResponse represents company data in response
type CompanyResponse struct {
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

// CompanyListResponse represents paginated company list
type CompanyListResponse struct {
	Companies  []CompanyResponse `json:"companies"`
	Total      int64             `json:"total"`
	Page       int               `json:"page"`
	PageSize   int               `json:"page_size"`
	TotalPages int               `json:"total_pages"`
}

// CompanyQueryParams represents query parameters for listing companies
type CompanyQueryParams struct {
	Page     int    `form:"page,default=1" binding:"omitempty,min=1"`
	PageSize int    `form:"page_size,default=10" binding:"omitempty,min=1,max=100"`
	Search   string `form:"search" binding:"omitempty,max=255"`
	IsActive *bool  `form:"is_active" binding:"omitempty"`
	OwnerID  string `form:"owner_id" binding:"omitempty,uuid"`
}

// AddUserToCompanyRequest represents request to add user to company
type AddUserToCompanyRequest struct {
	UserID string  `json:"user_id" binding:"required,uuid"`
	RoleID *string `json:"role_id"` // Company-specific role ID. If null, user inherits global role. UUID validation happens in service layer
	Role   string  `json:"role" binding:"omitempty,oneof=OWNER ADMIN MEMBER VIEWER"` // DEPRECATED: Use role_id instead
}

// UpdateCompanyUserRequest represents request to update company user
type UpdateCompanyUserRequest struct {
	RoleID   *string `json:"role_id"` // Company-specific role ID. Set to null to use global role. UUID validation happens in service layer
	Role     *string `json:"role" binding:"omitempty,oneof=OWNER ADMIN MEMBER VIEWER"` // DEPRECATED: Use role_id instead
	IsActive *bool   `json:"is_active" binding:"omitempty"`
}

// CompanyUserResponse represents company user data in response
type CompanyUserResponse struct {
	ID        string     `json:"id"`
	CompanyID string     `json:"company_id"`
	UserID    string     `json:"user_id"`
	RoleID    *string    `json:"role_id,omitempty"` // Company-specific role ID. Null means inherits global role
	Role      string     `json:"role"`              // DEPRECATED: Use role_id instead. Kept for backward compatibility
	IsPrimary bool       `json:"is_primary"`
	IsActive  bool       `json:"is_active"`
	// User information
	UserEmail    *string `json:"user_email,omitempty"`
	UserUsername *string `json:"user_username,omitempty"`
	UserFullName *string `json:"user_full_name,omitempty"`
	// Timestamps
	JoinedAt  time.Time  `json:"joined_at"`
	CreatedAt time.Time  `json:"created_at"`
	CreatedBy string     `json:"created_by"`
	UpdatedAt time.Time  `json:"updated_at"`
	UpdatedBy *string    `json:"updated_by,omitempty"`
	DeletedAt *time.Time `json:"deleted_at,omitempty"`
	DeletedBy *string    `json:"deleted_by,omitempty"`
}

// CompanyUserListResponse represents list of users in a company
type CompanyUserListResponse struct {
	Users      []CompanyUserResponse `json:"users"`
	Total      int64                 `json:"total"`
	Page       int                   `json:"page"`
	PageSize   int                   `json:"page_size"`
	TotalPages int                   `json:"total_pages"`
}
