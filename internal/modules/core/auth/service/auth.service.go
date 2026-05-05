package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"

	"venturo-skeleton-go/internal/config"
	"venturo-skeleton-go/internal/modules/core/auth/domain"
	"venturo-skeleton-go/internal/modules/core/auth/dto"
	"venturo-skeleton-go/internal/modules/core/auth/repository"
	branchDomain "venturo-skeleton-go/internal/modules/core/branch/domain"
	clientDomain "venturo-skeleton-go/internal/modules/core/client/domain"
	companyDomain "venturo-skeleton-go/internal/modules/core/company/domain"
	userDomain "venturo-skeleton-go/internal/modules/core/user/domain"
	userRepo "venturo-skeleton-go/internal/modules/core/user/repository"
	"venturo-skeleton-go/pkg/crypto"
	"venturo-skeleton-go/pkg/jwt"
	"venturo-skeleton-go/pkg/logger"
	"venturo-skeleton-go/pkg/token"
)

var (
	ErrInvalidCredentials    = errors.New("invalid credentials")
	ErrUserNotActive         = errors.New("user is not active")
	ErrUserLocked            = errors.New("user account is locked")
	ErrEmailNotVerified      = errors.New("email not verified")
	ErrEmailAlreadyExists    = errors.New("email already exists")
	ErrUsernameAlreadyExists = errors.New("username already exists")
	ErrInvalidRefreshToken   = errors.New("invalid refresh token")
	ErrRefreshTokenExpired   = errors.New("refresh token expired")
	ErrRefreshTokenRevoked   = errors.New("refresh token revoked")
	ErrCompanyNotFound       = errors.New("company not found")
	ErrNotCompanyMember      = errors.New("user is not a member of this company")
)

const (
	MaxFailedLoginAttempts = 5
	LockDuration           = 15 * time.Minute
	RefreshTokenExpiry     = 7 * 24 * time.Hour // 7 days
	AccessTokenExpiryHours = 24
)

// CompanyUserRepository interface for company user operations
type CompanyUserRepository interface {
	IsMember(ctx context.Context, userID, companyID string) (bool, error)
	GetPrimaryCompany(ctx context.Context, userID string) (*companyDomain.CompanyUser, error)
	FindByUser(ctx context.Context, userID string) ([]companyDomain.CompanyUser, error)
	FindByUserWithDetails(ctx context.Context, userID string) ([]companyDomain.UserCompanyDetail, error)
	Create(ctx context.Context, cu *companyDomain.CompanyUser) error
}

// CompanyRepository interface for company operations
type CompanyRepository interface {
	FindByID(ctx context.Context, id string) (*companyDomain.Company, error)
	Create(ctx context.Context, company *companyDomain.Company) error
}

// RoleRepository interface for role operations
type RoleRepository interface {
	GetUserPermissions(ctx context.Context, userID string, companyID *string) ([]string, error)
	GetUserRoleNames(ctx context.Context, userID string, companyID *string) ([]string, error)
}

// BranchRepository interface for branch operations
type BranchRepository interface {
	Create(ctx context.Context, branch *branchDomain.Branch) error
}

// ClientService is the slice of the client service that auth needs:
//   - CreateForUser: SignUp provisions a new client for a registrant.
//   - GetByID:       SignIn / SwitchCompany resolve client_id + slug
//                    from the caller's company so both show up in the
//                    JWT and the response payload.
type ClientService interface {
	CreateForUser(ctx context.Context, userID, username, displayName string) (*clientDomain.Client, error)
	GetByID(ctx context.Context, id string) (*clientDomain.Client, error)
}

// PermissionReader is the narrow seam GetMe needs from the authz
// package. Implemented by *authz.Service — accepted as an interface so
// auth doesn't import authz directly (and so tests can fake it).
type PermissionReader interface {
	GetPermissions(ctx context.Context, userID, companyID string) ([]string, error)
}

type AuthService struct {
	userRepo         *userRepo.UserRepository
	refreshTokenRepo *repository.RefreshTokenRepository
	companyUserRepo  CompanyUserRepository
	companyRepo      CompanyRepository
	roleRepo         RoleRepository
	branchRepo       BranchRepository
	clientService    ClientService
	permissionReader PermissionReader
	config           *config.Config
}

func NewAuthService(
	userRepo *userRepo.UserRepository,
	refreshTokenRepo *repository.RefreshTokenRepository,
	cfg *config.Config,
) *AuthService {
	return &AuthService{
		userRepo:         userRepo,
		refreshTokenRepo: refreshTokenRepo,
		config:           cfg,
	}
}

// SetCompanyUserRepo sets the company user repository (for dependency injection after initialization)
func (s *AuthService) SetCompanyUserRepo(repo CompanyUserRepository) {
	s.companyUserRepo = repo
}

// SetCompanyRepo sets the company repository
func (s *AuthService) SetCompanyRepo(repo CompanyRepository) {
	s.companyRepo = repo
}

// SetRoleRepo sets the role repository
func (s *AuthService) SetRoleRepo(repo RoleRepository) {
	s.roleRepo = repo
}

// SetBranchRepo sets the branch repository
func (s *AuthService) SetBranchRepo(repo BranchRepository) {
	s.branchRepo = repo
}

// SetPermissionReader wires the authz-backed permission cache used by
// GetMe. Without this, GetMe falls back to the role repo directly —
// correct, but bypasses Redis.
func (s *AuthService) SetPermissionReader(r PermissionReader) {
	s.permissionReader = r
}

// resolveClientInfo looks up a client by id and returns its
// (id, slug, *ClientInfo). On any failure (client service missing,
// lookup error, or unknown id) it returns empty strings and nil — the
// caller silently degrades so a missing client never blocks login.
// Used by SignIn / SwitchCompany to enrich the JWT and response.
func (s *AuthService) resolveClientInfo(ctx context.Context, clientID string) (string, string, *dto.ClientInfo) {
	if s.clientService == nil || clientID == "" {
		return "", "", nil
	}
	c, err := s.clientService.GetByID(ctx, clientID)
	if err != nil || c == nil {
		return "", "", nil
	}
	return c.ID, c.Slug, &dto.ClientInfo{
		ID:   c.ID,
		Slug: c.Slug,
		Name: c.Name,
	}
}

// SetClientService wires the client service so SignUp can provision a
// new client row (one-per-registrant) before the first company is created.
func (s *AuthService) SetClientService(svc ClientService) {
	s.clientService = svc
}

// SignUp registers a new user
func (s *AuthService) SignUp(ctx context.Context, req *dto.SignUpRequest) (*dto.SignUpResponse, error) {
	// Check email uniqueness
	exists, err := s.userRepo.EmailExists(ctx, req.Email, nil)
	if err != nil {
		return nil, err
	}
	if exists {
		return nil, ErrEmailAlreadyExists
	}

	// Check username uniqueness
	exists, err = s.userRepo.UsernameExists(ctx, req.Username, nil)
	if err != nil {
		return nil, err
	}
	if exists {
		return nil, ErrUsernameAlreadyExists
	}

	// Hash password
	passwordHash, err := crypto.HashPassword(req.Password)
	if err != nil {
		logger.Error("Failed to hash password", logger.Err(err))
		return nil, err
	}

	now := time.Now()
	user := &userDomain.User{
		ID:              uuid.New().String(),
		Email:           req.Email,
		Username:        req.Username,
		PasswordHash:    passwordHash,
		FullName:        req.FullName,
		Phone:           req.Phone,
		IsActive:        true,
		IsEmailVerified: true,
		EmailVerifiedAt: &now,
		CreatedAt:       now,
		UpdatedAt:       now,
	}

	err = s.userRepo.Create(ctx, user)
	if err != nil {
		return nil, err
	}

	// Provision a client (registration-level tenant) for this user before
	// creating any companies. One signup = one client; companies created
	// later (via POST /core/v1/companies) will inherit this client_id.
	if s.clientService == nil {
		return nil, errors.New("client service not set")
	}
	var newClient *clientDomain.Client
	var displayName string
	if user.FullName != nil {
		displayName = *user.FullName
	}
	newClient, err = s.clientService.CreateForUser(ctx, user.ID, user.Username, displayName)
	if err != nil {
		logger.Error("Failed to create client during signup", logger.Err(err))
		return nil, err
	}

	// Create company scoped to the new client.
	companyID := uuid.New().String()
	newCompany := &companyDomain.Company{
		ID:        companyID,
		ClientID:  newClient.ID,
		Name:      req.CompanyName,
		Type:      companyDomain.CompanyTypeHolding,
		OwnerID:   user.ID,
		Sort:      1,
		IsActive:  true,
		CreatedAt: now,
		CreatedBy: &user.ID,
		UpdatedAt: now,
		UpdatedBy: &user.ID,
	}

	if s.companyRepo == nil {
		return nil, errors.New("company repository not set")
	}
	err = s.companyRepo.Create(ctx, newCompany)
	if err != nil {
		logger.Error("Failed to create company during signup", logger.Err(err))
		return nil, err
	}

	// Link user to company as primary member with the default admin role
	// (so the owner has full company-scoped permissions out of the box).
	if s.companyUserRepo == nil {
		return nil, errors.New("company user repository not set")
	}
	var defaultRoleID *string
	if s.config != nil && s.config.Auth.DefaultAdminRoleID != "" {
		id := s.config.Auth.DefaultAdminRoleID
		defaultRoleID = &id
	}
	companyUser := &companyDomain.CompanyUser{
		ID:        uuid.New().String(),
		CompanyID: companyID,
		UserID:    user.ID,
		RoleID:    defaultRoleID,
		IsPrimary: true,
		IsActive:  true,
		JoinedAt:  now,
		CreatedAt: now,
		CreatedBy: &user.ID,
		UpdatedAt: now,
		UpdatedBy: &user.ID,
	}
	err = s.companyUserRepo.Create(ctx, companyUser)
	if err != nil {
		logger.Error("Failed to link user to company during signup", logger.Err(err))
		return nil, err
	}

	// Create default branch "Cabang Pusat"
	if s.branchRepo == nil {
		return nil, errors.New("branch repository not set")
	}
	defaultBranch := &branchDomain.Branch{
		ID:        uuid.New().String(),
		CompanyID: companyID,
		Name:      "Cabang Pusat",
		Sort:      1,
		IsDefault: true,
		IsActive:  true,
		CreatedAt: now,
		CreatedBy: &user.ID,
		UpdatedAt: now,
		UpdatedBy: &user.ID,
	}
	err = s.branchRepo.Create(ctx, defaultBranch)
	if err != nil {
		logger.Error("Failed to create default branch during signup", logger.Err(err))
		return nil, err
	}

	logger.Info("User signed up with company",
		logger.String("user_id", user.ID),
		logger.String("email", user.Email),
		logger.String("company_id", companyID),
		logger.String("company_name", req.CompanyName))

	return &dto.SignUpResponse{
		Message: "User registered successfully.",
		User: dto.UserInfo{
			ID:       user.ID,
			Email:    user.Email,
			Username: user.Username,
			FullName: user.FullName,
		},
		Company: dto.CompanyInfo{
			ID:   companyID,
			Name: req.CompanyName,
		},
	}, nil
}

// SignIn authenticates a user and returns tokens
func (s *AuthService) SignIn(ctx context.Context, req *dto.SignInRequest, deviceInfo domain.DeviceInfo, ipAddress string) (*dto.SignInResponse, error) {
	// Find user by email or username
	user, err := s.userRepo.FindByEmailOrUsername(ctx, req.Login)
	if err != nil {
		return nil, err
	}
	if user == nil {
		logger.Warn("User not found", logger.String("login", req.Login))
		return nil, ErrInvalidCredentials
	}

	// Check if user is active
	if !user.IsActive {
		logger.Warn("Inactive user login attempt", logger.String("user_id", user.ID))
		return nil, ErrUserNotActive
	}

	// Check if user is locked
	if user.IsLocked() {
		logger.Warn("Locked user login attempt", logger.String("user_id", user.ID))
		return nil, ErrUserLocked
	}

	// Check email verification if required
	if s.config != nil && s.config.Security.EmailVerificationRequired && !user.IsEmailVerified {
		logger.Warn("Unverified email login attempt", logger.String("user_id", user.ID))
		return nil, ErrEmailNotVerified
	}

	// Verify password
	if !crypto.ComparePassword(user.PasswordHash, req.Password) {
		// Increment failed login count
		_ = s.userRepo.IncrementFailedLogin(ctx, user.ID)

		// Lock user if too many failed attempts
		if user.FailedLoginCount+1 >= MaxFailedLoginAttempts {
			lockUntil := time.Now().Add(LockDuration)
			_ = s.userRepo.LockUser(ctx, user.ID, lockUntil)
			logger.Warn("User locked due to failed login attempts", logger.String("user_id", user.ID))
		}

		logger.Warn("Invalid password", logger.String("user_id", user.ID))
		return nil, ErrInvalidCredentials
	}

	// Update last login
	_ = s.userRepo.UpdateLastLogin(ctx, user.ID)

	// Try to load primary company for auto-scoped permissions
	var companyID, companyName string
	var companyInfo *dto.CompanyInfo
	var clientID, clientSlug string
	var clientInfo *dto.ClientInfo

	if s.companyUserRepo != nil {
		primaryMembership, _ := s.companyUserRepo.GetPrimaryCompany(ctx, user.ID)
		if primaryMembership != nil {
			companyID = primaryMembership.CompanyID
			if s.companyRepo != nil {
				company, _ := s.companyRepo.FindByID(ctx, companyID)
				if company != nil {
					companyName = company.Name
					companyInfo = &dto.CompanyInfo{
						ID:   company.ID,
						Name: company.Name,
					}
					clientID, clientSlug, clientInfo = s.resolveClientInfo(ctx, company.ClientID)
				}
			}
		}
	}

	// Get user roles and permissions (scoped to primary company if available)
	var roles, permissions []string
	if s.roleRepo != nil {
		if companyID != "" {
			roles, _ = s.roleRepo.GetUserRoleNames(ctx, user.ID, &companyID)
			permissions, _ = s.roleRepo.GetUserPermissions(ctx, user.ID, &companyID)
		} else {
			roles, _ = s.roleRepo.GetUserRoleNames(ctx, user.ID, nil)
			permissions, _ = s.roleRepo.GetUserPermissions(ctx, user.ID, nil)
		}
	}

	// Generate access token with company context
	fullName := ""
	if user.FullName != nil {
		fullName = *user.FullName
	}
	isSuperAdmin := containsRole(roles, jwt.RoleSuperAdmin)
	accessToken, err := jwt.GenerateToken(user.ID, companyID, companyName, clientID, clientSlug, user.Email, user.Username, fullName, isSuperAdmin, roles)
	if err != nil {
		logger.Error("Failed to generate access token", logger.Err(err))
		return nil, err
	}

	// Generate refresh token
	refreshTokenStr, err := token.GenerateSecureToken(32)
	if err != nil {
		logger.Error("Failed to generate refresh token", logger.Err(err))
		return nil, err
	}

	// Store refresh token hash in database
	refreshToken := &domain.RefreshToken{
		ID:         uuid.New().String(),
		UserID:     user.ID,
		TokenHash:  hashToken(refreshTokenStr),
		DeviceInfo: deviceInfo,
		IPAddress:  &ipAddress,
		ExpiresAt:  time.Now().Add(RefreshTokenExpiry),
		CreatedAt:  time.Now(),
	}

	err = s.refreshTokenRepo.Create(ctx, refreshToken)
	if err != nil {
		logger.Error("Failed to store refresh token", logger.Err(err))
		return nil, err
	}

	logger.Info("User signed in",
		logger.String("user_id", user.ID),
		logger.String("email", user.Email),
		logger.String("company_id", companyID))

	return &dto.SignInResponse{
		AccessToken:  accessToken,
		RefreshToken: refreshTokenStr,
		TokenType:    "Bearer",
		ExpiresIn:    AccessTokenExpiryHours * 3600,
		User: dto.UserInfo{
			ID:       user.ID,
			Email:    user.Email,
			Username: user.Username,
			FullName: user.FullName,
		},
		Company:     companyInfo,
		Client:      clientInfo,
		Roles:       roles,
		Permissions: permissions,
	}, nil
}

// RefreshToken refreshes the access token using a refresh token
func (s *AuthService) RefreshToken(ctx context.Context, refreshTokenStr string) (*dto.RefreshTokenResponse, error) {
	tokenHash := hashToken(refreshTokenStr)

	// Find refresh token
	refreshToken, err := s.refreshTokenRepo.FindByTokenHash(ctx, tokenHash)
	if err != nil {
		return nil, err
	}
	if refreshToken == nil {
		return nil, ErrInvalidRefreshToken
	}

	// Check if token is revoked
	if refreshToken.IsRevoked() {
		return nil, ErrRefreshTokenRevoked
	}

	// Check if token is expired
	if refreshToken.IsExpired() {
		return nil, ErrRefreshTokenExpired
	}

	// Get user
	user, err := s.userRepo.FindByID(ctx, refreshToken.UserID)
	if err != nil {
		return nil, err
	}
	if user == nil || !user.IsActive {
		return nil, ErrUserNotActive
	}

	// Update last used
	_ = s.refreshTokenRepo.UpdateLastUsed(ctx, refreshToken.ID)

	// Get user roles. Permissions are not needed here — the refresh
	// response returns only the new token pair, and permissions are
	// looked up from Redis on subsequent requests via authz.
	var roles []string
	if s.roleRepo != nil {
		roles, _ = s.roleRepo.GetUserRoleNames(ctx, user.ID, nil)
	}

	// Generate new access token
	fullName := ""
	if user.FullName != nil {
		fullName = *user.FullName
	}
	isSuperAdmin := containsRole(roles, jwt.RoleSuperAdmin)
	accessToken, err := jwt.GenerateToken(user.ID, "", "", "", "", user.Email, user.Username, fullName, isSuperAdmin, roles)
	if err != nil {
		logger.Error("Failed to generate access token", logger.Err(err))
		return nil, err
	}

	// Generate new refresh token (token rotation)
	newRefreshTokenStr, err := token.GenerateSecureToken(32)
	if err != nil {
		logger.Error("Failed to generate new refresh token", logger.Err(err))
		return nil, err
	}

	// Revoke old refresh token
	_ = s.refreshTokenRepo.Revoke(ctx, refreshToken.ID)

	// Store new refresh token
	newRefreshToken := &domain.RefreshToken{
		ID:         uuid.New().String(),
		UserID:     user.ID,
		TokenHash:  hashToken(newRefreshTokenStr),
		DeviceInfo: refreshToken.DeviceInfo,
		IPAddress:  refreshToken.IPAddress,
		ExpiresAt:  time.Now().Add(RefreshTokenExpiry),
		CreatedAt:  time.Now(),
	}

	err = s.refreshTokenRepo.Create(ctx, newRefreshToken)
	if err != nil {
		logger.Error("Failed to store new refresh token", logger.Err(err))
		return nil, err
	}

	return &dto.RefreshTokenResponse{
		AccessToken:  accessToken,
		RefreshToken: newRefreshTokenStr,
		TokenType:    "Bearer",
		ExpiresIn:    AccessTokenExpiryHours * 3600,
	}, nil
}

// Logout revokes a refresh token
func (s *AuthService) Logout(ctx context.Context, refreshTokenStr string) error {
	tokenHash := hashToken(refreshTokenStr)

	err := s.refreshTokenRepo.RevokeByTokenHash(ctx, tokenHash)
	if err != nil {
		logger.Error("Failed to revoke refresh token", logger.Err(err))
		return err
	}

	return nil
}

// LogoutAll revokes all refresh tokens for a user
func (s *AuthService) LogoutAll(ctx context.Context, userID string) error {
	err := s.refreshTokenRepo.RevokeAllByUserID(ctx, userID)
	if err != nil {
		logger.Error("Failed to revoke all refresh tokens", logger.Err(err))
		return err
	}

	return nil
}

// SwitchCompany switches the user's active company context
func (s *AuthService) SwitchCompany(ctx context.Context, userID, companyID string) (*dto.SwitchCompanyResponse, error) {
	// Check if user is a member of the company
	if s.companyUserRepo == nil {
		return nil, errors.New("company user repository not set")
	}

	isMember, err := s.companyUserRepo.IsMember(ctx, userID, companyID)
	if err != nil {
		return nil, err
	}
	if !isMember {
		return nil, ErrNotCompanyMember
	}

	// Get user
	user, err := s.userRepo.FindByID(ctx, userID)
	if err != nil {
		return nil, err
	}
	if user == nil {
		return nil, ErrUserNotActive
	}

	// Fetch company details
	var companyName string
	var clientID, clientSlug string
	var clientInfo *dto.ClientInfo
	if s.companyRepo != nil {
		company, _ := s.companyRepo.FindByID(ctx, companyID)
		if company != nil {
			companyName = company.Name
			clientID, clientSlug, clientInfo = s.resolveClientInfo(ctx, company.ClientID)
		}
	}
	_ = clientInfo // switch-company response doesn't carry client info today; claim covers the FE

	// Get user roles for the company. Permissions live in Redis and are
	// fetched on the request path; we still load them here to return in
	// the response body so the FE can update its UI without decoding
	// the (now permission-less) JWT.
	var roles, permissions []string
	if s.roleRepo != nil {
		roles, _ = s.roleRepo.GetUserRoleNames(ctx, userID, &companyID)
		permissions, _ = s.roleRepo.GetUserPermissions(ctx, userID, &companyID)
	}

	// Generate new access token with company context
	fullName := ""
	if user.FullName != nil {
		fullName = *user.FullName
	}
	isSuperAdmin := containsRole(roles, jwt.RoleSuperAdmin)
	accessToken, err := jwt.GenerateToken(user.ID, companyID, companyName, clientID, clientSlug, user.Email, user.Username, fullName, isSuperAdmin, roles)
	if err != nil {
		logger.Error("Failed to generate access token", logger.Err(err))
		return nil, err
	}

	// Generate new refresh token
	refreshTokenStr, err := token.GenerateSecureToken(32)
	if err != nil {
		logger.Error("Failed to generate refresh token", logger.Err(err))
		return nil, err
	}

	// Store refresh token
	refreshToken := &domain.RefreshToken{
		ID:        uuid.New().String(),
		UserID:    user.ID,
		TokenHash: hashToken(refreshTokenStr),
		ExpiresAt: time.Now().Add(RefreshTokenExpiry),
		CreatedAt: time.Now(),
	}

	err = s.refreshTokenRepo.Create(ctx, refreshToken)
	if err != nil {
		logger.Error("Failed to store refresh token", logger.Err(err))
		return nil, err
	}

	return &dto.SwitchCompanyResponse{
		AccessToken:  accessToken,
		RefreshToken: refreshTokenStr,
		TokenType:    "Bearer",
		ExpiresIn:    AccessTokenExpiryHours * 3600,
		Company: dto.CompanyInfo{
			ID:   companyID,
			Name: companyName,
		},
		Roles:       roles,
		Permissions: permissions,
	}, nil
}

// GetMe returns the signed-in user's current profile, active company,
// client context, roles, and *effective* permissions. Permissions come
// from the authz cache (Redis) when wired; otherwise the role repo is
// consulted directly. Intended for FE rehydration after a page reload
// or re-sync after an unexpected 403.
func (s *AuthService) GetMe(ctx context.Context, userID, companyID string, isSuperAdmin bool) (*dto.MeResponse, error) {
	user, err := s.userRepo.FindByID(ctx, userID)
	if err != nil {
		return nil, err
	}
	if user == nil {
		return nil, ErrUserNotActive
	}

	resp := &dto.MeResponse{
		User: dto.UserInfo{
			ID:       user.ID,
			Email:    user.Email,
			Username: user.Username,
			FullName: user.FullName,
		},
		IsSuperAdmin: isSuperAdmin,
	}

	// Resolve company + client context when the caller has switched.
	var companyScope *string
	if companyID != "" && s.companyRepo != nil {
		company, _ := s.companyRepo.FindByID(ctx, companyID)
		if company != nil {
			resp.Company = &dto.CompanyInfo{
				ID:   company.ID,
				Name: company.Name,
			}
			_, _, resp.Client = s.resolveClientInfo(ctx, company.ClientID)
			cid := company.ID
			companyScope = &cid
		}
	}

	// Roles — always from the role repo. The list is small so there's
	// no cache layer in front of it.
	if s.roleRepo != nil {
		resp.Roles, _ = s.roleRepo.GetUserRoleNames(ctx, userID, companyScope)
	}

	// Permissions — prefer the Redis-backed authz cache. Fall back to
	// the role repo if the reader isn't wired (tests / bootstrapping).
	switch {
	case s.permissionReader != nil:
		scope := ""
		if companyScope != nil {
			scope = *companyScope
		}
		resp.Permissions, err = s.permissionReader.GetPermissions(ctx, userID, scope)
		if err != nil {
			logger.Warn("GetMe: permission reader failed, falling back to role repo", logger.Err(err))
			if s.roleRepo != nil {
				resp.Permissions, _ = s.roleRepo.GetUserPermissions(ctx, userID, companyScope)
			}
		}
	case s.roleRepo != nil:
		resp.Permissions, _ = s.roleRepo.GetUserPermissions(ctx, userID, companyScope)
	}

	if resp.Permissions == nil {
		resp.Permissions = []string{}
	}
	if resp.Roles == nil {
		resp.Roles = []string{}
	}

	return resp, nil
}

// GetMyCompanies returns the list of companies the user has access to
func (s *AuthService) GetMyCompanies(ctx context.Context, userID string) ([]dto.MyCompanyResponse, error) {
	if s.companyUserRepo == nil {
		return []dto.MyCompanyResponse{}, nil
	}

	details, err := s.companyUserRepo.FindByUserWithDetails(ctx, userID)
	if err != nil {
		return nil, err
	}

	companies := make([]dto.MyCompanyResponse, 0, len(details))
	for _, d := range details {
		companies = append(companies, dto.MyCompanyResponse{
			ID:        d.CompanyID,
			Name:      d.CompanyName,
			Type:      d.CompanyType,
			LogoURL:   d.LogoURL,
			ParentID:  d.ParentID,
			IsPrimary: d.IsPrimary,
			IsOwner:   d.OwnerID == userID,
			RoleName:  d.RoleName,
			RoleCode:  d.RoleCode,
		})
	}

	return companies, nil
}

// containsRole checks if a role exists in the roles slice
func containsRole(roles []string, target string) bool {
	for _, r := range roles {
		if r == target {
			return true
		}
	}
	return false
}

// hashToken creates a SHA-256 hash of the token
func hashToken(tokenStr string) string {
	h := sha256.New()
	h.Write([]byte(tokenStr))
	return hex.EncodeToString(h.Sum(nil))
}


// GetDeviceInfoFromUserAgent extracts device info from user agent string
func GetDeviceInfoFromUserAgent(userAgent string) domain.DeviceInfo {
	info := domain.DeviceInfo{}

	ua := strings.ToLower(userAgent)

	// Detect browser
	switch {
	case strings.Contains(ua, "chrome"):
		info.Browser = "Chrome"
	case strings.Contains(ua, "firefox"):
		info.Browser = "Firefox"
	case strings.Contains(ua, "safari"):
		info.Browser = "Safari"
	case strings.Contains(ua, "edge"):
		info.Browser = "Edge"
	default:
		info.Browser = "Unknown"
	}

	// Detect OS
	switch {
	case strings.Contains(ua, "windows"):
		info.OS = "Windows"
	case strings.Contains(ua, "mac"):
		info.OS = "macOS"
	case strings.Contains(ua, "linux"):
		info.OS = "Linux"
	case strings.Contains(ua, "android"):
		info.OS = "Android"
	case strings.Contains(ua, "iphone"), strings.Contains(ua, "ipad"):
		info.OS = "iOS"
	default:
		info.OS = "Unknown"
	}

	// Detect type
	switch {
	case strings.Contains(ua, "mobile"), strings.Contains(ua, "android"), strings.Contains(ua, "iphone"):
		info.Type = "mobile"
	case strings.Contains(ua, "tablet"), strings.Contains(ua, "ipad"):
		info.Type = "tablet"
	default:
		info.Type = "desktop"
	}

	info.Name = info.Browser + " on " + info.OS

	return info
}
