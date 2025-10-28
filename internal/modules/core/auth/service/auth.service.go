package service

import (
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"venturo-skeleton-go/internal/config"
	authdomain "venturo-skeleton-go/internal/modules/core/auth/domain"
	"venturo-skeleton-go/internal/modules/core/auth/dto"
	authrepo "venturo-skeleton-go/internal/modules/core/auth/repository"
	companydomain "venturo-skeleton-go/internal/modules/core/company/domain"
	"venturo-skeleton-go/internal/modules/core/user/domain"
	"venturo-skeleton-go/internal/modules/core/user/repository"
	"venturo-skeleton-go/pkg/crypto"
	"venturo-skeleton-go/pkg/email"
	jwtpkg "venturo-skeleton-go/pkg/jwt"
	"venturo-skeleton-go/pkg/logger"
	"venturo-skeleton-go/pkg/token"
	"venturo-skeleton-go/pkg/validator"

	"github.com/google/uuid"
)

var (
	ErrInvalidCredentials    = errors.New("invalid credentials")
	ErrUserNotActive         = errors.New("user is not active")
	ErrUserNotFound          = errors.New("user not found")
	ErrEmailAlreadyExists    = errors.New("email already exists")
	ErrUsernameAlreadyExists = errors.New("username already exists")
	ErrInvalidPassword       = errors.New("invalid password format")
	ErrInvalidEmail          = errors.New("invalid email format")
	ErrInvalidUsername       = errors.New("invalid username format")
	ErrInvalidRefreshToken   = errors.New("invalid or expired refresh token")
	ErrRefreshTokenRevoked   = errors.New("refresh token has been revoked")
	ErrCompanyNotFound       = errors.New("company not found")
	ErrNotCompanyMember      = errors.New("user is not a member of this company")
)

type AuthService struct {
	userRepo        *repository.UserRepository
	tokenRepo       *authrepo.TokenRepository
	companyUserRepo CompanyUserRepository
	emailSvc        email.EmailService
	config          *config.Config
}

// CompanyUserRepository interface for getting primary company
type CompanyUserRepository interface {
	GetPrimaryCompanyID(userID string) (string, error)
	GetPrimaryCompany(userID string) (*companydomain.CompanyBasic, error)
	SetPrimaryCompany(userID, companyID string) error
	GetUserCompanies(userID string) ([]companydomain.CompanyBasic, error)
	GetUserRolesAndPermissionsInCompany(userID, companyID string) ([]string, []string, error)
}

func NewAuthService(userRepo *repository.UserRepository, tokenRepo *authrepo.TokenRepository, companyUserRepo CompanyUserRepository, emailSvc email.EmailService, cfg *config.Config) *AuthService {
	return &AuthService{
		userRepo:        userRepo,
		tokenRepo:       tokenRepo,
		companyUserRepo: companyUserRepo,
		emailSvc:        emailSvc,
		config:          cfg,
	}
}

// SignIn authenticates a user with email/username and password
func (s *AuthService) SignIn(req *dto.SignInRequest) (*dto.SignInResponse, error) {
	logger.Info("SignIn attempt", logger.String("login", req.Email))

	// Find user by email or username with roles and permissions
	var user *domain.User
	var roles, permissions []string
	var err error

	// Check if login is email (contains @) or username
	if strings.Contains(req.Email, "@") {
		user, roles, permissions, err = s.userRepo.FindByEmailWithRoles(req.Email)
		logger.Debug("Finding user by email", logger.String("email", req.Email))
	} else {
		user, roles, permissions, err = s.userRepo.FindByUsernameWithRoles(req.Email)
		logger.Debug("Finding user by username", logger.String("username", req.Email))
	}

	if err != nil {
		logger.Error("Database error during user lookup", logger.Err(err))
		return nil, err
	}

	if user == nil {
		logger.Warn("User not found", logger.String("login", req.Email))
		return nil, ErrInvalidCredentials
	}

	// Check if user is active
	if !user.IsActive {
		logger.Warn("Inactive user login attempt", logger.String("user_id", user.ID))
		return nil, ErrUserNotActive
	}

	// Verify password
	if !crypto.ComparePassword(user.PasswordHash, req.Password) {
		logger.Warn("Invalid password attempt", logger.String("user_id", user.ID))
		return nil, ErrInvalidCredentials
	}

	// Get primary company for user
	company, err := s.companyUserRepo.GetPrimaryCompany(user.ID)
	companyID := ""
	companyName := ""
	if err != nil {
		logger.Error("Failed to get primary company", logger.Err(err), logger.String("user_id", user.ID))
		// Continue without company (user might not have company yet)
	} else if company != nil {
		companyID = company.ID
		companyName = company.Name
	}

	// Get company-specific roles if user has a company
	// This implements hierarchical role system: company role overrides global role
	if companyID != "" {
		companyRoles, companyPermissions, err := s.companyUserRepo.GetUserRolesAndPermissionsInCompany(user.ID, companyID)
		if err == nil && len(companyRoles) > 0 {
			// Override with company-specific roles
			roles = companyRoles
			permissions = companyPermissions

			logger.Info("Using company-specific roles",
				logger.String("user_id", user.ID),
				logger.String("company_id", companyID),
				logger.Int("roles_count", len(roles)),
				logger.Int("permissions_count", len(permissions)),
			)
		} else {
			// Keep using global roles
			logger.Info("Using global roles (no company-specific role)",
				logger.String("user_id", user.ID),
				logger.String("company_id", companyID),
				logger.Int("roles_count", len(roles)),
			)
		}
	}

	// Generate JWT access token with user info, roles, permissions, and company
	accessToken, err := jwtpkg.GenerateToken(
		user.ID,
		companyID,
		companyName,
		user.Email,
		user.Username,
		*user.FullName,
		roles,
		permissions,
	)
	if err != nil {
		logger.Error("Failed to generate access token", logger.Err(err))
		return nil, err
	}

	// Calculate token expiration time in seconds
	expiresIn := int(jwtpkg.GetExpirationTime().Seconds())

	// Generate refresh token
	refreshTokenStr, err := token.GenerateRefreshToken()
	if err != nil {
		logger.Error("Failed to generate refresh token", logger.Err(err))
		return nil, err
	}

	// Hash the refresh token for storage
	refreshTokenHash := token.HashRefreshToken(refreshTokenStr)

	// Get refresh token expiration (default 7 days)
	refreshExpiration := getRefreshTokenExpiration()

	// Create refresh token record
	refreshToken := &authdomain.RefreshToken{
		ID:        uuid.New().String(),
		UserID:    user.ID,
		TokenHash: refreshTokenHash,
		ExpiresAt: time.Now().Add(refreshExpiration),
		CreatedAt: time.Now(),
	}

	// Save refresh token to database
	if err := s.tokenRepo.CreateRefreshToken(refreshToken); err != nil {
		logger.Error("Failed to save refresh token", logger.Err(err))
		// Don't fail the login if refresh token fails
		refreshTokenStr = ""
	}

	// Update last login
	if err := s.userRepo.UpdateLastLogin(user.ID); err != nil {
		logger.Error("Failed to update last login", logger.Err(err), logger.String("user_id", user.ID))
		// Don't fail the login
	}

	// Update last login time in response
	now := time.Now()
	user.LastLoginAt = &now

	logger.Info("SignIn successful",
		logger.String("user_id", user.ID),
		logger.String("email", user.Email),
		logger.Int("roles_count", len(roles)),
		logger.Int("permissions_count", len(permissions)),
		logger.String("company_id", companyID),
	)

	return &dto.SignInResponse{
		AccessToken:  accessToken,
		RefreshToken: refreshTokenStr,
		TokenType:    "Bearer",
		ExpiresIn:    expiresIn,
		User:         user.ToPublic(),
	}, nil
}

// SignUp registers a new user
func (s *AuthService) SignUp(req *dto.SignUpRequest) (*dto.SignUpResponse, error) {
	logger.Info("SignUp attempt", logger.String("email", req.Email))

	// Normalize email
	req.Email = validator.NormalizeEmail(req.Email)

	// Validate email format
	if err := validator.ValidateEmail(req.Email); err != nil {
		logger.Warn("Invalid email format", logger.String("email", req.Email))
		return nil, ErrInvalidEmail
	}

	// Validate username format
	if err := validator.ValidateUsername(req.Username); err != nil {
		logger.Warn("Invalid username format", logger.String("username", req.Username))
		return nil, ErrInvalidUsername
	}

	// Validate password strength
	if err := validator.ValidatePasswordWithEmail(req.Password, req.Email); err != nil {
		logger.Warn("Weak password", logger.Err(err))
		return nil, ErrInvalidPassword
	}

	// Check if email already exists
	existingUser, _, _, err := s.userRepo.FindByEmailWithRoles(req.Email)
	if err == nil && existingUser != nil {
		logger.Warn("Email already exists", logger.String("email", req.Email))
		return nil, ErrEmailAlreadyExists
	}

	// Check if username already exists
	existingUser, _, _, err = s.userRepo.FindByUsernameWithRoles(req.Username)
	if err == nil && existingUser != nil {
		logger.Warn("Username already exists", logger.String("username", req.Username))
		return nil, ErrUsernameAlreadyExists
	}

	// Hash password
	passwordHash, err := crypto.HashPassword(req.Password)
	if err != nil {
		logger.Error("Failed to hash password", logger.Err(err))
		return nil, err
	}

	// Create user
	userID := uuid.New().String()
	fullName := req.FullName

	// Check if email verification is required
	requireEmailVerification := s.config.Security.EmailVerificationRequired

	now := time.Now()
	user := &domain.User{
		ID:              userID,
		Email:           req.Email,
		Username:        req.Username,
		PasswordHash:    passwordHash,
		FullName:        &fullName,
		IsActive:        true,
		IsEmailVerified: !requireEmailVerification, // Auto-verify if not required
		CreatedAt:       now,
		CreatedBy:       &userID, // Self-reference: user creates themselves during signup
		UpdatedAt:       now,
		UpdatedBy:       &userID, // Self-reference: user creates themselves during signup
	}

	// Save user to database
	if err := s.userRepo.Create(user); err != nil {
		logger.Error("Failed to create user", logger.Err(err))
		return nil, err
	}

	// Only handle email verification if required
	if requireEmailVerification {
		// Generate email verification token
		verificationToken, err := token.GenerateEmailVerificationToken()
		if err != nil {
			logger.Error("Failed to generate verification token", logger.Err(err))
			return nil, err
		}

		// Get expiration duration from env or use default (48 hours)
		expirationDuration := getEmailVerificationExpiration()
		expiresAt := time.Now().Add(expirationDuration)

		// Save verification token
		if err := s.tokenRepo.CreateEmailVerificationToken(userID, verificationToken, expiresAt); err != nil {
			logger.Error("Failed to save verification token", logger.Err(err))
			return nil, err
		}

		// Send verification email (async, don't fail signup if email fails)
		go func() {
			if err := s.emailSvc.SendVerificationEmail(user.Email, *user.FullName, verificationToken); err != nil {
				logger.Error("Failed to send verification email",
					logger.Err(err),
					logger.String("email", user.Email),
				)
			}
		}()
	}

	logger.Info("SignUp successful",
		logger.String("user_id", user.ID),
		logger.String("email", user.Email),
		logger.String("username", user.Username),
		logger.Bool("email_verification_required", requireEmailVerification),
	)

	// Set appropriate message based on email verification requirement
	message := "Registration successful!"
	if requireEmailVerification {
		message = "Registration successful. Please check your email to verify your account."
	}

	return &dto.SignUpResponse{
		Message: message,
		User:    user.ToPublic(),
	}, nil
}

// getEmailVerificationExpiration returns email verification expiration duration
func getEmailVerificationExpiration() time.Duration {
	expStr := os.Getenv("EMAIL_VERIFICATION_EXPIRATION")
	if expStr == "" {
		return 48 * time.Hour // Default 48 hours
	}

	duration, err := time.ParseDuration(expStr)
	if err != nil {
		return 48 * time.Hour
	}

	return duration
}

// getPasswordResetExpiration returns password reset expiration duration
func getPasswordResetExpiration() time.Duration {
	expStr := os.Getenv("PASSWORD_RESET_EXPIRATION")
	if expStr == "" {
		return 1 * time.Hour // Default 1 hour
	}

	duration, err := time.ParseDuration(expStr)
	if err != nil {
		return 1 * time.Hour
	}

	return duration
}

// getRefreshTokenExpiration returns refresh token expiration duration
func getRefreshTokenExpiration() time.Duration {
	expStr := os.Getenv("REFRESH_TOKEN_EXPIRATION")
	if expStr == "" {
		return 7 * 24 * time.Hour // Default 7 days
	}

	duration, err := time.ParseDuration(expStr)
	if err != nil {
		return 7 * 24 * time.Hour
	}

	return duration
}

// VerifyEmail verifies user's email with token
func (s *AuthService) VerifyEmail(req *dto.VerifyEmailRequest) (*dto.VerifyEmailResponse, error) {
	logger.Info("Email verification attempt", logger.String("token", req.Token))

	// Find verification token
	verificationToken, err := s.tokenRepo.FindEmailVerificationToken(req.Token)
	if err != nil {
		logger.Warn("Invalid verification token", logger.Err(err))
		return nil, errors.New("invalid or expired verification token")
	}

	// Check if already verified
	if verificationToken.VerifiedAt != nil {
		logger.Warn("Email already verified", logger.String("user_id", verificationToken.UserID))
		return nil, errors.New("email already verified")
	}

	// Check if token expired
	if time.Now().After(verificationToken.ExpiresAt) {
		logger.Warn("Verification token expired", logger.String("user_id", verificationToken.UserID))
		return nil, errors.New("verification token has expired")
	}

	// Mark email as verified in token table
	if err := s.tokenRepo.MarkEmailAsVerified(req.Token); err != nil {
		logger.Error("Failed to mark email as verified", logger.Err(err))
		return nil, err
	}

	// Update user's email_verified status
	if err := s.userRepo.UpdateEmailVerified(verificationToken.UserID, true); err != nil {
		logger.Error("Failed to update user email verified status", logger.Err(err))
		return nil, err
	}

	// Get user info for welcome email
	user, _, _, err := s.userRepo.FindByIDWithRoles(verificationToken.UserID)
	if err == nil && user != nil && user.FullName != nil {
		// Send welcome email (async)
		go func() {
			if err := s.emailSvc.SendWelcomeEmail(user.Email, *user.FullName); err != nil {
				logger.Error("Failed to send welcome email",
					logger.Err(err),
					logger.String("email", user.Email),
				)
			}
		}()
	}

	logger.Info("Email verified successfully", logger.String("user_id", verificationToken.UserID))

	return &dto.VerifyEmailResponse{
		Message: "Email verified successfully. You can now sign in.",
	}, nil
}

// ResendVerificationEmail resends verification email
func (s *AuthService) ResendVerificationEmail(req *dto.ResendVerificationRequest) (*dto.ResendVerificationResponse, error) {
	logger.Info("Resend verification email attempt", logger.String("email", req.Email))

	// Normalize email
	email := validator.NormalizeEmail(req.Email)

	// Find user by email
	user, _, _, err := s.userRepo.FindByEmailWithRoles(email)
	if err != nil || user == nil {
		logger.Warn("User not found", logger.String("email", email))
		// Don't reveal if user exists or not
		return &dto.ResendVerificationResponse{
			Message: "If an account exists with this email, a verification email has been sent.",
		}, nil
	}

	// Check if already verified
	if user.IsEmailVerified {
		logger.Warn("Email already verified", logger.String("user_id", user.ID))
		return nil, errors.New("email already verified")
	}

	// Check resend count limit
	maxResend := getMaxVerificationResend()
	currentCount, err := s.tokenRepo.GetEmailVerificationResendCount(user.ID)
	if err == nil && currentCount >= maxResend {
		logger.Warn("Max resend limit reached", logger.String("user_id", user.ID))
		return nil, errors.New("maximum resend limit reached, please contact support")
	}

	// Generate new verification token
	verificationToken, err := token.GenerateEmailVerificationToken()
	if err != nil {
		logger.Error("Failed to generate verification token", logger.Err(err))
		return nil, err
	}

	// Get expiration duration
	expirationDuration := getEmailVerificationExpiration()
	expiresAt := time.Now().Add(expirationDuration)

	// Save new verification token
	if err := s.tokenRepo.CreateEmailVerificationToken(user.ID, verificationToken, expiresAt); err != nil {
		logger.Error("Failed to save verification token", logger.Err(err))
		return nil, err
	}

	// Increment resend count
	if err := s.tokenRepo.IncrementEmailVerificationResend(user.ID); err != nil {
		logger.Error("Failed to increment resend count", logger.Err(err))
		// Don't fail the operation
	}

	// Send verification email
	go func() {
		userName := "User"
		if user.FullName != nil {
			userName = *user.FullName
		}
		if err := s.emailSvc.SendVerificationEmail(user.Email, userName, verificationToken); err != nil {
			logger.Error("Failed to send verification email",
				logger.Err(err),
				logger.String("email", user.Email),
			)
		}
	}()

	logger.Info("Verification email resent", logger.String("user_id", user.ID))

	return &dto.ResendVerificationResponse{
		Message: "Verification email has been sent. Please check your inbox.",
	}, nil
}

// getMaxVerificationResend returns max verification resend count
func getMaxVerificationResend() int {
	maxStr := os.Getenv("MAX_VERIFICATION_RESEND")
	if maxStr == "" {
		return 5 // Default
	}

	var max int
	if _, err := fmt.Sscanf(maxStr, "%d", &max); err != nil {
		return 5
	}

	return max
}

// RefreshToken generates new access and refresh tokens using a valid refresh token
func (s *AuthService) RefreshToken(req *dto.RefreshTokenRequest) (*dto.RefreshTokenResponse, error) {
	logger.Info("Refresh token attempt")

	// Hash the provided refresh token
	tokenHash := token.HashRefreshToken(req.RefreshToken)

	// Find refresh token in database
	refreshToken, err := s.tokenRepo.FindRefreshToken(tokenHash)
	if err != nil {
		logger.Warn("Invalid refresh token", logger.Err(err))
		return nil, ErrInvalidRefreshToken
	}

	// Check if token is revoked
	if refreshToken.RevokedAt != nil {
		logger.Warn("Refresh token has been revoked", logger.String("user_id", refreshToken.UserID))
		return nil, ErrRefreshTokenRevoked
	}

	// Check if token is expired
	if time.Now().After(refreshToken.ExpiresAt) {
		logger.Warn("Refresh token has expired", logger.String("user_id", refreshToken.UserID))
		return nil, ErrInvalidRefreshToken
	}

	// Get user with roles and permissions
	user, roles, permissions, err := s.userRepo.FindByIDWithRoles(refreshToken.UserID)
	if err != nil || user == nil {
		logger.Error("User not found for refresh token", logger.Err(err), logger.String("user_id", refreshToken.UserID))
		return nil, ErrUserNotFound
	}

	// Check if user is still active
	if !user.IsActive {
		logger.Warn("Inactive user trying to refresh token", logger.String("user_id", user.ID))
		return nil, ErrUserNotActive
	}

	// Get primary company for user
	company, err := s.companyUserRepo.GetPrimaryCompany(user.ID)
	companyID := ""
	companyName := ""
	if err != nil {
		logger.Error("Failed to get primary company", logger.Err(err), logger.String("user_id", user.ID))
		// Continue without company
	} else if company != nil {
		companyID = company.ID
		companyName = company.Name
	}

	// Get company-specific roles if user has a company
	// This implements hierarchical role system: company role overrides global role
	if companyID != "" {
		companyRoles, companyPermissions, err := s.companyUserRepo.GetUserRolesAndPermissionsInCompany(user.ID, companyID)
		if err == nil && len(companyRoles) > 0 {
			// Override with company-specific roles
			roles = companyRoles
			permissions = companyPermissions

			logger.Debug("Refresh token: using company-specific roles",
				logger.String("user_id", user.ID),
				logger.String("company_id", companyID),
				logger.Int("roles_count", len(roles)),
			)
		}
	}

	// Generate new access token
	accessToken, err := jwtpkg.GenerateToken(
		user.ID,
		companyID,
		companyName,
		user.Email,
		user.Username,
		*user.FullName,
		roles,
		permissions,
	)
	if err != nil {
		logger.Error("Failed to generate access token", logger.Err(err))
		return nil, err
	}

	// Generate new refresh token
	newRefreshTokenStr, err := token.GenerateRefreshToken()
	if err != nil {
		logger.Error("Failed to generate new refresh token", logger.Err(err))
		return nil, err
	}

	// Hash the new refresh token
	newRefreshTokenHash := token.HashRefreshToken(newRefreshTokenStr)

	// Revoke old refresh token
	if err := s.tokenRepo.RevokeRefreshToken(tokenHash); err != nil {
		logger.Error("Failed to revoke old refresh token", logger.Err(err))
		// Continue anyway
	}

	// Create new refresh token record
	newRefreshToken := &authdomain.RefreshToken{
		ID:         uuid.New().String(),
		UserID:     user.ID,
		TokenHash:  newRefreshTokenHash,
		DeviceName: refreshToken.DeviceName,
		DeviceID:   refreshToken.DeviceID,
		IPAddress:  refreshToken.IPAddress,
		UserAgent:  refreshToken.UserAgent,
		ExpiresAt:  time.Now().Add(getRefreshTokenExpiration()),
		CreatedAt:  time.Now(),
	}

	// Save new refresh token
	if err := s.tokenRepo.CreateRefreshToken(newRefreshToken); err != nil {
		logger.Error("Failed to save new refresh token", logger.Err(err))
		return nil, err
	}

	// Update last used timestamp for the old token (for audit purposes)
	_ = s.tokenRepo.UpdateRefreshTokenLastUsed(tokenHash)

	// Calculate token expiration time in seconds
	expiresIn := int(jwtpkg.GetExpirationTime().Seconds())

	logger.Info("Token refresh successful",
		logger.String("user_id", user.ID),
		logger.String("email", user.Email),
	)

	return &dto.RefreshTokenResponse{
		AccessToken:  accessToken,
		RefreshToken: newRefreshTokenStr,
		TokenType:    "Bearer",
		ExpiresIn:    expiresIn,
	}, nil
}

// RevokeToken revokes a refresh token
func (s *AuthService) RevokeToken(req *dto.RevokeTokenRequest) (*dto.RevokeTokenResponse, error) {
	logger.Info("Revoke token attempt")

	// Hash the provided refresh token
	tokenHash := token.HashRefreshToken(req.RefreshToken)

	// Find refresh token in database
	refreshToken, err := s.tokenRepo.FindRefreshToken(tokenHash)
	if err != nil {
		logger.Warn("Invalid refresh token for revocation", logger.Err(err))
		// Return success even if token not found (idempotent operation)
		return &dto.RevokeTokenResponse{
			Message: "Token revoked successfully",
		}, nil
	}

	// Revoke the token
	if err := s.tokenRepo.RevokeRefreshToken(tokenHash); err != nil {
		logger.Error("Failed to revoke refresh token", logger.Err(err))
		return nil, err
	}

	logger.Info("Token revoked successfully", logger.String("user_id", refreshToken.UserID))

	return &dto.RevokeTokenResponse{
		Message: "Token revoked successfully",
	}, nil
}

// SwitchCompany switches user's active company and generates new token with updated company context
func (s *AuthService) SwitchCompany(userID string, req *dto.SwitchCompanyRequest) (*dto.SwitchCompanyResponse, error) {
	logger.Info("Switch company attempt", logger.String("user_id", userID), logger.String("company_id", req.CompanyID))

	// Get user with roles and permissions
	user, roles, permissions, err := s.userRepo.FindByIDWithRoles(userID)
	if err != nil || user == nil {
		logger.Error("User not found", logger.Err(err), logger.String("user_id", userID))
		return nil, ErrUserNotFound
	}

	// Check if user is still active
	if !user.IsActive {
		logger.Warn("Inactive user trying to switch company", logger.String("user_id", user.ID))
		return nil, ErrUserNotActive
	}

	// Set the company as primary for this user
	err = s.companyUserRepo.SetPrimaryCompany(userID, req.CompanyID)
	if err != nil {
		logger.Error("Failed to set primary company", logger.Err(err), logger.String("user_id", userID), logger.String("company_id", req.CompanyID))
		if err.Error() == "user is not a member of this company" {
			return nil, ErrNotCompanyMember
		}
		return nil, err
	}

	// Get the switched company details
	company, err := s.companyUserRepo.GetPrimaryCompany(userID)
	if err != nil || company == nil {
		logger.Error("Failed to get company details after switch", logger.Err(err), logger.String("company_id", req.CompanyID))
		return nil, ErrCompanyNotFound
	}

	// Get company-specific roles for the new company
	// This implements hierarchical role system: company role overrides global role
	companyRoles, companyPermissions, err := s.companyUserRepo.GetUserRolesAndPermissionsInCompany(userID, req.CompanyID)
	if err == nil && len(companyRoles) > 0 {
		// Override with company-specific roles
		roles = companyRoles
		permissions = companyPermissions

		logger.Info("Switched to company with company-specific roles",
			logger.String("user_id", userID),
			logger.String("company_id", req.CompanyID),
			logger.String("company_name", company.Name),
			logger.Int("roles_count", len(roles)),
			logger.Int("permissions_count", len(permissions)),
		)
	} else {
		// Keep using global roles
		logger.Info("Switched to company with global roles",
			logger.String("user_id", userID),
			logger.String("company_id", req.CompanyID),
			logger.String("company_name", company.Name),
			logger.Int("roles_count", len(roles)),
		)
	}

	// Generate new access token with updated company ID
	accessToken, err := jwtpkg.GenerateToken(
		user.ID,
		req.CompanyID,
		company.Name,
		user.Email,
		user.Username,
		*user.FullName,
		roles,
		permissions,
	)
	if err != nil {
		logger.Error("Failed to generate access token", logger.Err(err))
		return nil, err
	}

	// Generate new refresh token
	refreshTokenStr, err := token.GenerateRefreshToken()
	if err != nil {
		logger.Error("Failed to generate refresh token", logger.Err(err))
		return nil, err
	}

	// Hash the refresh token for storage
	refreshTokenHash := token.HashRefreshToken(refreshTokenStr)

	// Get refresh token expiration
	refreshExpiration := getRefreshTokenExpiration()

	// Create refresh token record
	refreshToken := &authdomain.RefreshToken{
		ID:        uuid.New().String(),
		UserID:    user.ID,
		TokenHash: refreshTokenHash,
		ExpiresAt: time.Now().Add(refreshExpiration),
		CreatedAt: time.Now(),
	}

	// Save refresh token to database
	if err := s.tokenRepo.CreateRefreshToken(refreshToken); err != nil {
		logger.Error("Failed to save refresh token", logger.Err(err))
		// Don't fail the switch if refresh token fails
		refreshTokenStr = ""
	}

	// Calculate token expiration time in seconds
	expiresIn := int(jwtpkg.GetExpirationTime().Seconds())

	logger.Info("Company switched successfully",
		logger.String("user_id", user.ID),
		logger.String("company_id", req.CompanyID),
		logger.String("company_name", company.Name),
	)

	return &dto.SwitchCompanyResponse{
		AccessToken:  accessToken,
		RefreshToken: refreshTokenStr,
		TokenType:    "Bearer",
		ExpiresIn:    expiresIn,
		Company:      company,
	}, nil
}

// GetUserCompanies gets all companies that the user is a member of
func (s *AuthService) GetUserCompanies(userID string) (*dto.GetUserCompaniesResponse, error) {
	logger.Info("Get user companies attempt", logger.String("user_id", userID))

	// Get user's companies
	companies, err := s.companyUserRepo.GetUserCompanies(userID)
	if err != nil {
		logger.Error("Failed to get user companies", logger.Err(err), logger.String("user_id", userID))
		return nil, err
	}

	// Convert to DTO
	companyInfos := make([]dto.CompanyInfo, len(companies))
	for i, company := range companies {
		companyInfos[i] = dto.CompanyInfo{
			ID:      company.ID,
			Name:    company.Name,
			Code:    company.Code,
			LogoURL: company.LogoURL,
		}
	}

	logger.Info("Get user companies successful",
		logger.String("user_id", userID),
		logger.Int("companies_count", len(companies)),
	)

	return &dto.GetUserCompaniesResponse{
		Companies: companyInfos,
	}, nil
}
