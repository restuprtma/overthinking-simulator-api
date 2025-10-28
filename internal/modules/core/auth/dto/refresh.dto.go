package dto

// RefreshTokenRequest represents the refresh token request payload
type RefreshTokenRequest struct {
	RefreshToken string `json:"refresh_token" binding:"required"`
}

// RefreshTokenResponse represents the refresh token response
type RefreshTokenResponse struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	TokenType    string `json:"token_type"`
	ExpiresIn    int    `json:"expires_in"`
}

// RevokeTokenRequest represents the revoke token request payload
type RevokeTokenRequest struct {
	RefreshToken string `json:"refresh_token" binding:"required"`
}

// RevokeTokenResponse represents the revoke token response
type RevokeTokenResponse struct {
	Message string `json:"message"`
}
