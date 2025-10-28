package jwt

import (
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

var (
	ErrInvalidToken     = errors.New("invalid token")
	ErrExpiredToken     = errors.New("token has expired")
	ErrTokenNotProvided = errors.New("token not provided")
	ErrInvalidSignature = errors.New("invalid token signature")
)

// GetSecret returns JWT secret from environment variable
func GetSecret() []byte {
	secret := os.Getenv("JWT_SECRET")
	if secret == "" {
		// Default secret for development only
		// In production, JWT_SECRET MUST be set in .env
		secret = "wizhub-development-secret-change-in-production"
	}
	return []byte(secret)
}

// GetExpirationTime returns token expiration duration from environment
func GetExpirationTime() time.Duration {
	expStr := os.Getenv("JWT_EXPIRATION")
	if expStr == "" {
		return 24 * time.Hour // Default 24 hours
	}

	duration, err := time.ParseDuration(expStr)
	if err != nil {
		return 24 * time.Hour // Fallback to 24 hours
	}

	return duration
}

// GenerateToken generates a new JWT token with user claims
func GenerateToken(userID, companyID, companyName, email, username string, fullName string, roles, permissions []string) (string, error) {
	now := time.Now()
	expirationTime := GetExpirationTime()

	claims := &Claims{
		UserID:      userID,
		CompanyID:   companyID,
		CompanyName: companyName,
		Email:       email,
		Username:    username,
		FullName:    fullName,
		Roles:       roles,
		Permissions: permissions,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(now.Add(expirationTime)),
			IssuedAt:  jwt.NewNumericDate(now),
			NotBefore: jwt.NewNumericDate(now),
			Issuer:    "lakukan-api",
			Subject:   userID,
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenString, err := token.SignedString(GetSecret())
	if err != nil {
		return "", fmt.Errorf("failed to sign token: %w", err)
	}

	return tokenString, nil
}

// ParseToken parses and validates a JWT token string
func ParseToken(tokenString string) (*Claims, error) {
	if tokenString == "" {
		return nil, ErrTokenNotProvided
	}

	token, err := jwt.ParseWithClaims(tokenString, &Claims{}, func(token *jwt.Token) (interface{}, error) {
		// Validate signing method
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return GetSecret(), nil
	})

	if err != nil {
		if errors.Is(err, jwt.ErrTokenExpired) {
			return nil, ErrExpiredToken
		}
		if errors.Is(err, jwt.ErrSignatureInvalid) {
			return nil, ErrInvalidSignature
		}
		return nil, fmt.Errorf("%w: %v", ErrInvalidToken, err)
	}

	claims, ok := token.Claims.(*Claims)
	if !ok || !token.Valid {
		return nil, ErrInvalidToken
	}

	return claims, nil
}

// ValidateToken validates a token and returns claims if valid
func ValidateToken(tokenString string) (*Claims, error) {
	return ParseToken(tokenString)
}

// RefreshToken generates a new token from an existing valid token
// This can be used to extend user session
func RefreshToken(oldTokenString string) (string, error) {
	claims, err := ParseToken(oldTokenString)
	if err != nil {
		// Allow refresh even if token is expired (but not if invalid signature)
		if !errors.Is(err, ErrExpiredToken) {
			return "", err
		}
	}

	// Generate new token with same claims
	return GenerateToken(
		claims.UserID,
		claims.CompanyID,
		claims.CompanyName,
		claims.Email,
		claims.Username,
		claims.FullName,
		claims.Roles,
		claims.Permissions,
	)
}
