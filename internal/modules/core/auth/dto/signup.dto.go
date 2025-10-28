package dto

// SignUpRequest represents the signup request payload
type SignUpRequest struct {
	Email    string `json:"email" binding:"required,email"`
	Username string `json:"username" binding:"required,min=3,max=20"`
	Password string `json:"password" binding:"required,min=8"`
	FullName string `json:"full_name" binding:"required"`
}

// SignUpResponse represents the signup response
type SignUpResponse struct {
	Message string      `json:"message"`
	User    interface{} `json:"user"`
}
