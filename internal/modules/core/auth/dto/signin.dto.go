package dto

// SignInRequest represents the signin request payload
type SignInRequest struct {
	Email    string `json:"email" binding:"required"`    // Can be email or username
	Password string `json:"password" binding:"required"` // Min 8 characters
}

// SignInResponse represents the signin response
type SignInResponse struct {
	AccessToken  string      `json:"access_token"`
	RefreshToken string      `json:"refresh_token,omitempty"`
	TokenType    string      `json:"token_type"`
	ExpiresIn    int         `json:"expires_in"`
	User         interface{} `json:"user"`
}
