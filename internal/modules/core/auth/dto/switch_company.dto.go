package dto

// SwitchCompanyRequest represents the switch company request payload
type SwitchCompanyRequest struct {
	CompanyID string `json:"company_id" binding:"required" example:"550e8400-e29b-41d4-a716-446655440000"` // Company ID to switch to
}

// SwitchCompanyResponse represents the switch company response
type SwitchCompanyResponse struct {
	AccessToken  string      `json:"access_token" example:"eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9..."`  // JWT access token with updated company information (includes company_id and company_name in claims)
	RefreshToken string      `json:"refresh_token,omitempty" example:"a1b2c3d4e5f6..."`              // New refresh token for extended session
	TokenType    string      `json:"token_type" example:"Bearer"`                                    // Token type (always "Bearer")
	ExpiresIn    int         `json:"expires_in" example:"86400"`                                     // Token expiration time in seconds
	Company      interface{} `json:"company" swaggertype:"object,string"`                            // Switched company basic info (id, name, code, logo_url)
}
