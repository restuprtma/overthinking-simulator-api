package dto

// GetUserCompaniesResponse represents the user companies response
type GetUserCompaniesResponse struct {
	Companies []CompanyInfo `json:"companies"` // List of companies the user is a member of
}

// CompanyInfo represents basic company information
type CompanyInfo struct {
	ID      string  `json:"id" example:"550e8400-e29b-41d4-a716-446655440000"`                         // Unique company identifier (UUID)
	Name    string  `json:"name" example:"Acme Corporation"`                                           // Company name
	Code    string  `json:"code" example:"ACME001"`                                                    // Company code (unique identifier)
	LogoURL *string `json:"logo_url,omitempty" example:"https://example.com/logos/acme-corporation.png"` // Company logo URL (optional)
}
