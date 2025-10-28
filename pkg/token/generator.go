package token

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
)

// GenerateSecureToken generates a cryptographically secure random token
func GenerateSecureToken(length int) (string, error) {
	bytes := make([]byte, length)
	if _, err := rand.Read(bytes); err != nil {
		return "", fmt.Errorf("failed to generate secure token: %w", err)
	}
	return hex.EncodeToString(bytes), nil
}

// GenerateBase64Token generates a base64 encoded random token
func GenerateBase64Token(length int) (string, error) {
	bytes := make([]byte, length)
	if _, err := rand.Read(bytes); err != nil {
		return "", fmt.Errorf("failed to generate base64 token: %w", err)
	}
	return base64.URLEncoding.EncodeToString(bytes), nil
}

// GenerateEmailVerificationToken generates a token for email verification
func GenerateEmailVerificationToken() (string, error) {
	return GenerateSecureToken(32) // 64 character hex string
}

// GeneratePasswordResetToken generates a token for password reset
func GeneratePasswordResetToken() (string, error) {
	return GenerateSecureToken(32) // 64 character hex string
}

// GenerateRefreshToken generates a token for refresh tokens
func GenerateRefreshToken() (string, error) {
	return GenerateBase64Token(64) // Base64 encoded 64-byte token
}

// HashRefreshToken creates a SHA256 hash of the refresh token for storage
func HashRefreshToken(token string) string {
	hash := sha256.Sum256([]byte(token))
	return hex.EncodeToString(hash[:])
}
