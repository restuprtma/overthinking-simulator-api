package service

import (
	"context"
	cryptorand "crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"

	"venturo-skeleton-go/internal/config"
	"venturo-skeleton-go/internal/modules/core/auth/domain"
	"venturo-skeleton-go/internal/modules/core/auth/dto"
	"venturo-skeleton-go/internal/modules/core/auth/repository"
	userDomain "venturo-skeleton-go/internal/modules/core/user/domain"
	userRepo "venturo-skeleton-go/internal/modules/core/user/repository"
	"venturo-skeleton-go/pkg/crypto"
	pkgfirebase "venturo-skeleton-go/pkg/firebase"
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

	// Google sign-in errors
	ErrFirebaseNotConfigured = errors.New("google sign-in not configured")
	ErrInvalidGoogleToken    = errors.New("invalid google id token")
	ErrGoogleEmailMissing    = errors.New("google account did not return an email")
	ErrUnexpectedProvider    = errors.New("unexpected firebase sign-in provider")
)

const firebaseSignInProviderGoogle = "google.com"

const (
	MaxFailedLoginAttempts = 5
	LockDuration           = 15 * time.Minute
	RefreshTokenExpiry     = 7 * 24 * time.Hour // 7 days
	AccessTokenExpiryHours = 24
)

// RoleRepository interface for role operations
type RoleRepository interface {
	GetUserPermissions(ctx context.Context, userID string, companyID *string) ([]string, error)
	GetUserRoleNames(ctx context.Context, userID string, companyID *string) ([]string, error)
}

// PermissionReader is the seam GetMe needs from the authz package.
type PermissionReader interface {
	GetPermissions(ctx context.Context, userID, companyID string) ([]string, error)
}

// FirebaseVerifier is the seam the auth service uses to verify Firebase ID tokens.
type FirebaseVerifier interface {
	VerifyIDToken(ctx context.Context, idToken string) (*pkgfirebase.VerifiedToken, error)
}

type AuthService struct {
	userRepo         *userRepo.UserRepository
	userIdentityRepo *userRepo.UserIdentityRepository
	refreshTokenRepo *repository.RefreshTokenRepository
	roleRepo         RoleRepository
	permissionReader PermissionReader
	firebase         FirebaseVerifier
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

func (s *AuthService) SetUserIdentityRepo(repo *userRepo.UserIdentityRepository) {
	s.userIdentityRepo = repo
}

func (s *AuthService) SetFirebaseVerifier(f FirebaseVerifier) {
	s.firebase = f
}

func (s *AuthService) SetRoleRepo(repo RoleRepository) {
	s.roleRepo = repo
}

func (s *AuthService) SetPermissionReader(r PermissionReader) {
	s.permissionReader = r
}

// SignUp registers a new user
func (s *AuthService) SignUp(ctx context.Context, req *dto.SignUpRequest) (*dto.SignUpResponse, error) {
	exists, err := s.userRepo.EmailExists(ctx, req.Email, nil)
	if err != nil {
		return nil, err
	}
	if exists {
		return nil, ErrEmailAlreadyExists
	}

	exists, err = s.userRepo.UsernameExists(ctx, req.Username, nil)
	if err != nil {
		return nil, err
	}
	if exists {
		return nil, ErrUsernameAlreadyExists
	}

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

	logger.Info("User signed up",
		logger.String("user_id", user.ID),
		logger.String("email", user.Email))

	return &dto.SignUpResponse{
		Message: "User registered successfully.",
		User: dto.UserInfo{
			ID:       user.ID,
			Email:    user.Email,
			Username: user.Username,
			FullName: user.FullName,
		},
	}, nil
}

// SignIn authenticates a user and returns tokens
func (s *AuthService) SignIn(ctx context.Context, req *dto.SignInRequest, deviceInfo domain.DeviceInfo, ipAddress string) (*dto.SignInResponse, error) {
	user, err := s.userRepo.FindByEmailOrUsername(ctx, req.Login)
	if err != nil {
		return nil, err
	}
	if user == nil {
		logger.Warn("User not found", logger.String("login", req.Login))
		return nil, ErrInvalidCredentials
	}

	if !user.IsActive {
		logger.Warn("Inactive user login attempt", logger.String("user_id", user.ID))
		return nil, ErrUserNotActive
	}

	if user.IsLocked() {
		logger.Warn("Locked user login attempt", logger.String("user_id", user.ID))
		return nil, ErrUserLocked
	}

	if s.config != nil && s.config.Security.EmailVerificationRequired && !user.IsEmailVerified {
		logger.Warn("Unverified email login attempt", logger.String("user_id", user.ID))
		return nil, ErrEmailNotVerified
	}

	if !crypto.ComparePassword(user.PasswordHash, req.Password) {
		_ = s.userRepo.IncrementFailedLogin(ctx, user.ID)

		if user.FailedLoginCount+1 >= MaxFailedLoginAttempts {
			lockUntil := time.Now().Add(LockDuration)
			_ = s.userRepo.LockUser(ctx, user.ID, lockUntil)
			logger.Warn("User locked due to failed login attempts", logger.String("user_id", user.ID))
		}

		logger.Warn("Invalid password", logger.String("user_id", user.ID))
		return nil, ErrInvalidCredentials
	}

	_ = s.userRepo.UpdateLastLogin(ctx, user.ID)

	var roles, permissions []string
	if s.roleRepo != nil {
		roles, _ = s.roleRepo.GetUserRoleNames(ctx, user.ID, nil)
		permissions, _ = s.roleRepo.GetUserPermissions(ctx, user.ID, nil)
	}

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

	refreshTokenStr, err := token.GenerateSecureToken(32)
	if err != nil {
		logger.Error("Failed to generate refresh token", logger.Err(err))
		return nil, err
	}

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
		logger.String("email", user.Email))

	if permissions == nil {
		permissions = []string{}
	}
	if roles == nil {
		roles = []string{}
	}

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
		Roles:       roles,
		Permissions: permissions,
	}, nil
}

// RefreshToken refreshes the access token using a refresh token
func (s *AuthService) RefreshToken(ctx context.Context, refreshTokenStr string) (*dto.RefreshTokenResponse, error) {
	tokenHash := hashToken(refreshTokenStr)

	refreshToken, err := s.refreshTokenRepo.FindByTokenHash(ctx, tokenHash)
	if err != nil {
		return nil, err
	}
	if refreshToken == nil {
		return nil, ErrInvalidRefreshToken
	}

	if refreshToken.IsRevoked() {
		return nil, ErrRefreshTokenRevoked
	}

	if refreshToken.IsExpired() {
		return nil, ErrRefreshTokenExpired
	}

	user, err := s.userRepo.FindByID(ctx, refreshToken.UserID)
	if err != nil {
		return nil, err
	}
	if user == nil || !user.IsActive {
		return nil, ErrUserNotActive
	}

	_ = s.refreshTokenRepo.UpdateLastUsed(ctx, refreshToken.ID)

	var roles []string
	if s.roleRepo != nil {
		roles, _ = s.roleRepo.GetUserRoleNames(ctx, user.ID, nil)
	}

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

	newRefreshTokenStr, err := token.GenerateSecureToken(32)
	if err != nil {
		logger.Error("Failed to generate new refresh token", logger.Err(err))
		return nil, err
	}

	_ = s.refreshTokenRepo.Revoke(ctx, refreshToken.ID)

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
	return s.refreshTokenRepo.RevokeByTokenHash(ctx, tokenHash)
}

// LogoutAll revokes all refresh tokens for a user
func (s *AuthService) LogoutAll(ctx context.Context, userID string) error {
	return s.refreshTokenRepo.RevokeAllByUserID(ctx, userID)
}

// SwitchCompany stub
func (s *AuthService) SwitchCompany(ctx context.Context, userID, companyID string) (*dto.SwitchCompanyResponse, error) {
	return nil, ErrCompanyNotFound
}

// GetMe returns the signed-in user's profile, roles, and permissions
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

	if s.roleRepo != nil {
		resp.Roles, _ = s.roleRepo.GetUserRoleNames(ctx, userID, nil)
	}

	switch {
	case s.permissionReader != nil:
		resp.Permissions, err = s.permissionReader.GetPermissions(ctx, userID, "")
		if err != nil {
			logger.Warn("GetMe: permission reader failed, falling back to role repo", logger.Err(err))
			if s.roleRepo != nil {
				resp.Permissions, _ = s.roleRepo.GetUserPermissions(ctx, userID, nil)
			}
		}
	case s.roleRepo != nil:
		resp.Permissions, _ = s.roleRepo.GetUserPermissions(ctx, userID, nil)
	}

	if resp.Permissions == nil {
		resp.Permissions = []string{}
	}
	if resp.Roles == nil {
		resp.Roles = []string{}
	}

	return resp, nil
}

// GetMyCompanies returns empty list
func (s *AuthService) GetMyCompanies(ctx context.Context, userID string) ([]dto.MyCompanyResponse, error) {
	return []dto.MyCompanyResponse{}, nil
}

func containsRole(roles []string, target string) bool {
	for _, r := range roles {
		if r == target {
			return true
		}
	}
	return false
}

// SignInWithGoogle handles Google OAuth sign-in/sign-up
func (s *AuthService) SignInWithGoogle(
	ctx context.Context,
	req *dto.GoogleSignInRequest,
	deviceInfo domain.DeviceInfo,
	ipAddress string,
) (*dto.GoogleSignInResponse, error) {
	if s.firebase == nil {
		return nil, ErrFirebaseNotConfigured
	}
	if s.userIdentityRepo == nil {
		return nil, errors.New("user identity repository not set")
	}

	tok, err := s.firebase.VerifyIDToken(ctx, req.IDToken)
	if err != nil {
		logger.Warn("Firebase ID token verification failed", logger.Err(err))
		return nil, ErrInvalidGoogleToken
	}

	if tok.SignInProvider != "" && tok.SignInProvider != firebaseSignInProviderGoogle {
		logger.Warn("Rejected Firebase token with unexpected sign-in provider",
			logger.String("sign_in_provider", tok.SignInProvider))
		return nil, ErrUnexpectedProvider
	}

	if tok.Email == "" {
		return nil, ErrGoogleEmailMissing
	}
	email := strings.ToLower(strings.TrimSpace(tok.Email))

	identity, err := s.userIdentityRepo.FindByProvider(ctx, userDomain.IdentityProviderGoogle, tok.UID)
	if err != nil {
		return nil, err
	}

	var (
		user      *userDomain.User
		isNewUser bool
	)

	switch {
	case identity != nil:
		user, err = s.userRepo.FindByID(ctx, identity.UserID)
		if err != nil {
			return nil, err
		}
		if user == nil || !user.IsActive || user.DeletedAt != nil {
			return nil, ErrUserNotActive
		}
		_ = s.userIdentityRepo.UpdateOnLogin(ctx, identity.ID, &email, marshalProfile(tok))

	default:
		existing, err := s.userRepo.FindByEmail(ctx, email)
		if err != nil {
			return nil, err
		}
		if existing != nil {
			if !existing.IsActive || existing.DeletedAt != nil {
				return nil, ErrUserNotActive
			}
			user = existing
			if err := s.linkGoogleIdentity(ctx, user.ID, tok, email); err != nil {
				return nil, err
			}
		} else {
			user, err = s.provisionGoogleUser(ctx, tok, email)
			if err != nil {
				return nil, err
			}
			isNewUser = true
		}
	}

	_ = s.userRepo.UpdateLastLogin(ctx, user.ID)

	return s.issueGoogleTokens(ctx, user, isNewUser, deviceInfo, ipAddress)
}

func (s *AuthService) provisionGoogleUser(
	ctx context.Context,
	tok *pkgfirebase.VerifiedToken,
	email string,
) (*userDomain.User, error) {
	now := time.Now()

	username, err := s.allocateUsernameFromEmail(ctx, email)
	if err != nil {
		return nil, err
	}

	displayName := strings.TrimSpace(tok.Name)
	if displayName == "" {
		displayName = usernameFromEmail(email)
	}

	var avatarURL *string
	if tok.Picture != "" {
		pic := tok.Picture
		avatarURL = &pic
	}

	full := displayName
	user := &userDomain.User{
		ID:              uuid.New().String(),
		Email:           email,
		Username:        username,
		FullName:        &full,
		AvatarURL:       avatarURL,
		IsActive:        true,
		IsEmailVerified: tok.EmailVerified,
		CreatedAt:       now,
		UpdatedAt:       now,
	}
	if tok.EmailVerified {
		v := now
		user.EmailVerifiedAt = &v
	}

	if err := s.userRepo.CreateExternal(ctx, user); err != nil {
		return nil, err
	}

	if err := s.linkGoogleIdentity(ctx, user.ID, tok, email); err != nil {
		return nil, err
	}

	logger.Info("User provisioned via Google sign-in",
		logger.String("user_id", user.ID),
		logger.String("email", user.Email))

	return user, nil
}

func (s *AuthService) linkGoogleIdentity(
	ctx context.Context,
	userID string,
	tok *pkgfirebase.VerifiedToken,
	email string,
) error {
	now := time.Now()
	identity := &userDomain.UserIdentity{
		ID:             uuid.New().String(),
		UserID:         userID,
		Provider:       userDomain.IdentityProviderGoogle,
		ProviderUserID: tok.UID,
		Email:          &email,
		RawProfile:     marshalProfile(tok),
		LastLoginAt:    &now,
		CreatedAt:      now,
		UpdatedAt:      now,
	}
	if err := s.userIdentityRepo.Create(ctx, identity); err != nil {
		logger.Error("Failed to link google identity", logger.Err(err))
		return err
	}
	return nil
}

func (s *AuthService) issueGoogleTokens(
	ctx context.Context,
	user *userDomain.User,
	isNewUser bool,
	deviceInfo domain.DeviceInfo,
	ipAddress string,
) (*dto.GoogleSignInResponse, error) {
	var roles, permissions []string
	if s.roleRepo != nil {
		roles, _ = s.roleRepo.GetUserRoleNames(ctx, user.ID, nil)
		permissions, _ = s.roleRepo.GetUserPermissions(ctx, user.ID, nil)
	}

	fullName := ""
	if user.FullName != nil {
		fullName = *user.FullName
	}
	isSuperAdmin := containsRole(roles, jwt.RoleSuperAdmin)
	accessToken, err := jwt.GenerateToken(
		user.ID, "", "", "", "",
		user.Email, user.Username, fullName, isSuperAdmin, roles,
	)
	if err != nil {
		logger.Error("Failed to generate access token", logger.Err(err))
		return nil, err
	}

	refreshTokenStr, err := token.GenerateSecureToken(32)
	if err != nil {
		logger.Error("Failed to generate refresh token", logger.Err(err))
		return nil, err
	}

	refreshToken := &domain.RefreshToken{
		ID:         uuid.New().String(),
		UserID:     user.ID,
		TokenHash:  hashToken(refreshTokenStr),
		DeviceInfo: deviceInfo,
		IPAddress:  &ipAddress,
		ExpiresAt:  time.Now().Add(RefreshTokenExpiry),
		CreatedAt:  time.Now(),
	}
	if err := s.refreshTokenRepo.Create(ctx, refreshToken); err != nil {
		logger.Error("Failed to store refresh token", logger.Err(err))
		return nil, err
	}

	if permissions == nil {
		permissions = []string{}
	}
	if roles == nil {
		roles = []string{}
	}

	return &dto.GoogleSignInResponse{
		AccessToken:  accessToken,
		RefreshToken: refreshTokenStr,
		TokenType:    "Bearer",
		ExpiresIn:    AccessTokenExpiryHours * 3600,
		IsNewUser:    isNewUser,
		User: dto.UserInfo{
			ID:       user.ID,
			Email:    user.Email,
			Username: user.Username,
			FullName: user.FullName,
		},
		Roles:       roles,
		Permissions: permissions,
	}, nil
}

func (s *AuthService) allocateUsernameFromEmail(ctx context.Context, email string) (string, error) {
	base := sanitizeUsername(usernameFromEmail(email))
	if base == "" {
		base = "user-" + shortHex(4)
	}

	candidate := base
	for attempt := 0; attempt < 5; attempt++ {
		exists, err := s.userRepo.UsernameExists(ctx, candidate, nil)
		if err != nil {
			return "", err
		}
		if !exists {
			return candidate, nil
		}
		suffix := "-" + shortHex(4)
		maxBase := 100 - len(suffix)
		if len(base) > maxBase {
			base = base[:maxBase]
		}
		candidate = base + suffix
	}
	return "", errors.New("could not allocate unique username after retries")
}

func usernameFromEmail(email string) string {
	at := strings.IndexByte(email, '@')
	if at <= 0 {
		return ""
	}
	return strings.ToLower(strings.TrimSpace(email[:at]))
}

func sanitizeUsername(raw string) string {
	lower := strings.ToLower(strings.TrimSpace(raw))
	b := make([]byte, 0, len(lower))
	for i := 0; i < len(lower); i++ {
		ch := lower[i]
		switch {
		case ch >= 'a' && ch <= 'z',
			ch >= '0' && ch <= '9',
			ch == '.', ch == '-', ch == '_':
			b = append(b, ch)
		}
	}
	out := string(b)
	if len(out) < 3 {
		return ""
	}
	if len(out) > 100 {
		out = out[:100]
	}
	return out
}

func shortHex(n int) string {
	buf := make([]byte, n)
	if _, err := cryptoRand(buf); err != nil {
		return hex.EncodeToString([]byte(time.Now().Format("0405")))[:n*2]
	}
	return hex.EncodeToString(buf)
}

var cryptoRand = func(b []byte) (int, error) {
	return cryptorand.Read(b)
}

func truncateName(s string, maxRunes int) string {
	r := []rune(s)
	if len(r) <= maxRunes {
		return s
	}
	return string(r[:maxRunes])
}

func marshalProfile(tok *pkgfirebase.VerifiedToken) []byte {
	b, err := json.Marshal(tok.Claims)
	if err != nil {
		return []byte("{}")
	}
	return b
}

func hashToken(tokenStr string) string {
	h := sha256.New()
	h.Write([]byte(tokenStr))
	return hex.EncodeToString(h.Sum(nil))
}

func GetDeviceInfoFromUserAgent(userAgent string) domain.DeviceInfo {
	info := domain.DeviceInfo{}
	ua := strings.ToLower(userAgent)

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
