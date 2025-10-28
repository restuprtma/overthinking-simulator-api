package handler

import (
	"errors"
	"net/http"

	"venturo-skeleton-go/internal/middleware"
	"venturo-skeleton-go/internal/modules/core/auth/dto"
	"venturo-skeleton-go/internal/modules/core/auth/service"
	"venturo-skeleton-go/internal/shared/response"

	"github.com/gin-gonic/gin"
)

type AuthHandler struct {
	authService *service.AuthService
}

func NewAuthHandler(authService *service.AuthService) *AuthHandler {
	return &AuthHandler{
		authService: authService,
	}
}

// SignIn handles user signin request
// @Summary User signin
// @Description Authenticate user with email/username and password
// @Tags Authentication
// @Accept json
// @Produce json
// @Param request body dto.SignInRequest true "Signin credentials"
// @Success 200 {object} response.Response{data=dto.SignInResponse} "Sign in successful"
// @Failure 400 {object} response.Response "Invalid request payload"
// @Failure 401 {object} response.Response "Invalid credentials or user not active"
// @Failure 500 {object} response.Response "Internal server error"
// @Router /auth/signin [post]
func (h *AuthHandler) SignIn(c *gin.Context) {
	var req dto.SignInRequest

	// Bind and validate request
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, "Invalid request payload", err.Error())
		return
	}

	// Call service
	result, err := h.authService.SignIn(&req)
	if err != nil {
		switch {
		case errors.Is(err, service.ErrInvalidCredentials):
			response.Error(c, http.StatusUnauthorized, "Invalid credentials", "")
		case errors.Is(err, service.ErrUserNotActive):
			response.Error(c, http.StatusUnauthorized, "User account is not active", "")
		default:
			response.Error(c, http.StatusInternalServerError, "Failed to sign in", err.Error())
		}
		return
	}

	response.Success(c, http.StatusOK, "Sign in successful", result)
}

// SignUp handles user registration request
// @Summary User registration
// @Description Register a new user account
// @Tags Authentication
// @Accept json
// @Produce json
// @Param request body dto.SignUpRequest true "Registration details"
// @Success 201 {object} response.Response{data=dto.SignUpResponse} "Registration successful"
// @Failure 400 {object} response.Response "Invalid request payload or validation error"
// @Failure 409 {object} response.Response "Email or username already exists"
// @Failure 500 {object} response.Response "Internal server error"
// @Router /auth/signup [post]
func (h *AuthHandler) SignUp(c *gin.Context) {
	var req dto.SignUpRequest

	// Bind and validate request
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, "Invalid request payload", err.Error())
		return
	}

	// Call service
	result, err := h.authService.SignUp(&req)
	if err != nil {
		switch {
		case errors.Is(err, service.ErrEmailAlreadyExists):
			response.Error(c, http.StatusConflict, "Email already exists", "")
		case errors.Is(err, service.ErrUsernameAlreadyExists):
			response.Error(c, http.StatusConflict, "Username already exists", "")
		case errors.Is(err, service.ErrInvalidEmail):
			response.Error(c, http.StatusBadRequest, "Invalid email format", "")
		case errors.Is(err, service.ErrInvalidUsername):
			response.Error(c, http.StatusBadRequest, "Invalid username format", err.Error())
		case errors.Is(err, service.ErrInvalidPassword):
			response.Error(c, http.StatusBadRequest, "Password does not meet requirements", err.Error())
		default:
			response.Error(c, http.StatusInternalServerError, "Failed to register user", err.Error())
		}
		return
	}

	response.Success(c, http.StatusCreated, "Registration successful", result)
}

// VerifyEmail handles email verification request
// @Summary Verify email address
// @Description Verify user email with verification token
// @Tags Authentication
// @Accept json
// @Produce json
// @Param request body dto.VerifyEmailRequest true "Verification token"
// @Success 200 {object} response.Response{data=dto.VerifyEmailResponse} "Email verified successfully"
// @Failure 400 {object} response.Response "Invalid token or already verified"
// @Failure 500 {object} response.Response "Internal server error"
// @Router /auth/verify-email [post]
func (h *AuthHandler) VerifyEmail(c *gin.Context) {
	var req dto.VerifyEmailRequest

	// Bind and validate request
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, "Invalid request payload", err.Error())
		return
	}

	// Call service
	result, err := h.authService.VerifyEmail(&req)
	if err != nil {
		response.Error(c, http.StatusBadRequest, err.Error(), "")
		return
	}

	response.Success(c, http.StatusOK, "Email verified successfully", result)
}

// ResendVerification handles resend verification email request
// @Summary Resend verification email
// @Description Resend email verification link to user
// @Tags Authentication
// @Accept json
// @Produce json
// @Param request body dto.ResendVerificationRequest true "User email"
// @Success 200 {object} response.Response{data=dto.ResendVerificationResponse} "Verification email sent"
// @Failure 400 {object} response.Response "Invalid request or limit reached"
// @Failure 500 {object} response.Response "Internal server error"
// @Router /auth/resend-verification [post]
func (h *AuthHandler) ResendVerification(c *gin.Context) {
	var req dto.ResendVerificationRequest

	// Bind and validate request
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, "Invalid request payload", err.Error())
		return
	}

	// Call service
	result, err := h.authService.ResendVerificationEmail(&req)
	if err != nil {
		response.Error(c, http.StatusBadRequest, err.Error(), "")
		return
	}

	response.Success(c, http.StatusOK, "Verification email sent", result)
}

// RefreshToken handles refresh token request
// @Summary Refresh access token
// @Description Generate new access and refresh tokens using a valid refresh token
// @Tags Authentication
// @Accept json
// @Produce json
// @Param request body dto.RefreshTokenRequest true "Refresh token"
// @Success 200 {object} response.Response{data=dto.RefreshTokenResponse} "Token refreshed successfully"
// @Failure 400 {object} response.Response "Invalid request payload"
// @Failure 401 {object} response.Response "Invalid or expired refresh token"
// @Failure 500 {object} response.Response "Internal server error"
// @Router /auth/refresh [post]
func (h *AuthHandler) RefreshToken(c *gin.Context) {
	var req dto.RefreshTokenRequest

	// Bind and validate request
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, "Invalid request payload", err.Error())
		return
	}

	// Call service
	result, err := h.authService.RefreshToken(&req)
	if err != nil {
		switch {
		case errors.Is(err, service.ErrInvalidRefreshToken):
			response.Error(c, http.StatusUnauthorized, "Invalid or expired refresh token", "")
		case errors.Is(err, service.ErrRefreshTokenRevoked):
			response.Error(c, http.StatusUnauthorized, "Refresh token has been revoked", "")
		case errors.Is(err, service.ErrUserNotActive):
			response.Error(c, http.StatusUnauthorized, "User account is not active", "")
		case errors.Is(err, service.ErrUserNotFound):
			response.Error(c, http.StatusUnauthorized, "User not found", "")
		default:
			response.Error(c, http.StatusInternalServerError, "Failed to refresh token", err.Error())
		}
		return
	}

	response.Success(c, http.StatusOK, "Token refreshed successfully", result)
}

// RevokeToken handles revoke token request
// @Summary Revoke refresh token
// @Description Revoke a refresh token to log out from a specific device
// @Tags Authentication
// @Accept json
// @Produce json
// @Param request body dto.RevokeTokenRequest true "Refresh token to revoke"
// @Success 200 {object} response.Response{data=dto.RevokeTokenResponse} "Token revoked successfully"
// @Failure 400 {object} response.Response "Invalid request payload"
// @Failure 500 {object} response.Response "Internal server error"
// @Router /auth/revoke [post]
func (h *AuthHandler) RevokeToken(c *gin.Context) {
	var req dto.RevokeTokenRequest

	// Bind and validate request
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, "Invalid request payload", err.Error())
		return
	}

	// Call service
	result, err := h.authService.RevokeToken(&req)
	if err != nil {
		response.Error(c, http.StatusInternalServerError, "Failed to revoke token", err.Error())
		return
	}

	response.Success(c, http.StatusOK, "Token revoked successfully", result)
}

// SwitchCompany handles switch company request
// @Summary Switch company
// @Description Switch user's active company and get new access token with updated company context. This endpoint updates the user's primary company and returns new JWT tokens containing the new company ID and name in the claims. Use this when a user wants to change their active company context.
// @Tags Authentication - Company Management
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param request body dto.SwitchCompanyRequest true "Company ID to switch to"
// @Success 200 {object} response.Response{data=dto.SwitchCompanyResponse} "Company switched successfully. Returns new access token and refresh token with updated company information in JWT claims (company_id and company_name)."
// @Failure 400 {object} response.Response "Invalid request payload - Missing or invalid company_id"
// @Failure 401 {object} response.Response "Unauthorized - Invalid or missing authentication token"
// @Failure 403 {object} response.Response "Forbidden - User is not a member of this company"
// @Failure 404 {object} response.Response "Not Found - Company not found or user not found"
// @Failure 500 {object} response.Response "Internal server error"
// @Router /core/v1/auth/switch-company [post]
func (h *AuthHandler) SwitchCompany(c *gin.Context) {
	var req dto.SwitchCompanyRequest

	// Bind and validate request
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, "Invalid request payload", err.Error())
		return
	}

	// Get user ID from context (set by JWT middleware)
	userID, err := middleware.GetUserID(c)
	if err != nil {
		response.Error(c, http.StatusUnauthorized, "User not authenticated", "")
		return
	}

	// Call service
	result, err := h.authService.SwitchCompany(userID, &req)
	if err != nil {
		switch {
		case errors.Is(err, service.ErrUserNotFound):
			response.Error(c, http.StatusNotFound, "User not found", "")
		case errors.Is(err, service.ErrUserNotActive):
			response.Error(c, http.StatusUnauthorized, "User account is not active", "")
		case errors.Is(err, service.ErrNotCompanyMember):
			response.Error(c, http.StatusForbidden, "You are not a member of this company", "")
		case errors.Is(err, service.ErrCompanyNotFound):
			response.Error(c, http.StatusNotFound, "Company not found", "")
		default:
			response.Error(c, http.StatusInternalServerError, "Failed to switch company", err.Error())
		}
		return
	}

	response.Success(c, http.StatusOK, "Company switched successfully", result)
}

// GetUserCompanies handles get user companies request
// @Summary Get user companies
// @Description Get all companies that the authenticated user is a member of. Returns a list of companies with basic information including ID, name, code, and logo URL.
// @Tags Authentication - Company Management
// @Accept json
// @Produce json
// @Security BearerAuth
// @Success 200 {object} response.Response{data=dto.GetUserCompaniesResponse} "User companies retrieved successfully"
// @Failure 401 {object} response.Response "Unauthorized - Invalid or missing authentication token"
// @Failure 500 {object} response.Response "Internal server error"
// @Router /core/v1/auth/companies [get]
func (h *AuthHandler) GetUserCompanies(c *gin.Context) {
	// Get user ID from context (set by JWT middleware)
	userID, err := middleware.GetUserID(c)
	if err != nil {
		response.Error(c, http.StatusUnauthorized, "User not authenticated", "")
		return
	}

	// Call service
	result, err := h.authService.GetUserCompanies(userID)
	if err != nil {
		response.Error(c, http.StatusInternalServerError, "Failed to get user companies", err.Error())
		return
	}

	response.Success(c, http.StatusOK, "User companies retrieved successfully", result)
}