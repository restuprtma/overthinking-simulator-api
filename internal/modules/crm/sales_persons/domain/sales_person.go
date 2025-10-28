package domain

import "time"

// SalesPerson represents a sales/marketing representative in a company
type SalesPerson struct {
	ID                   string     `json:"id"`
	CompanyUserID        string     `json:"company_user_id"`
	SalesCode            string     `json:"sales_code"`
	SalesName            *string    `json:"sales_name,omitempty"`
	SalesArea            *string    `json:"sales_area,omitempty"`
	SalesTarget          *float64   `json:"sales_target,omitempty"`
	CommissionRate       *float64   `json:"commission_rate,omitempty"`
	Whatsapp                 *string    `json:"whatsapp,omitempty"`
	IsWhatsappConnected      bool       `json:"is_whatsapp_connected"`
	WAHASessionName          *string    `json:"waha_session_name,omitempty"`
	WAHASessionStatus        *string    `json:"waha_session_status,omitempty"`
	WAHAPairingCode          *string    `json:"waha_pairing_code,omitempty"`
	WAHAPairingCodeExpiresAt *time.Time `json:"waha_pairing_code_expires_at,omitempty"`
	WAHALastSeenAt           *time.Time `json:"waha_last_seen_at,omitempty"`
	WAHAConnectedAt          *time.Time `json:"waha_connected_at,omitempty"`
	WAHADisconnectedAt       *time.Time `json:"waha_disconnected_at,omitempty"`
	IsActive                 bool       `json:"is_active"`
	Notes                    *string    `json:"notes,omitempty"`
	CreatedAt            time.Time  `json:"created_at"`
	CreatedBy            *string    `json:"created_by,omitempty"` // UUID stored as string
	UpdatedAt            time.Time  `json:"updated_at"`
	UpdatedBy            *string    `json:"updated_by,omitempty"` // UUID stored as string
	DeletedAt            *time.Time `json:"deleted_at,omitempty"`
	DeletedBy            *string    `json:"deleted_by,omitempty"` // UUID stored as string

	// Joined fields from company_users and users tables
	UserID               *string    `json:"user_id,omitempty"`
	UserFullName         *string    `json:"user_full_name,omitempty"`
	UserEmail            *string    `json:"user_email,omitempty"`
	CreatedByName        *string    `json:"created_by_name,omitempty"`
	UpdatedByName        *string    `json:"updated_by_name,omitempty"`
	DeletedByName        *string    `json:"deleted_by_name,omitempty"`
}
