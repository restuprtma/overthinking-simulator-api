package middleware

import (
	"net/http"
	"strings"

	"venturo-skeleton-go/internal/shared/response"
	jwtpkg "venturo-skeleton-go/pkg/jwt"
	"venturo-skeleton-go/pkg/logger"

	"github.com/gin-gonic/gin"
)

// JWTAuth is a middleware that validates JWT tokens
func JWTAuth() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Extract token from Authorization header
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			logger.Warn("Missing authorization header")
			response.Error(c, http.StatusUnauthorized, "Unauthorized", "Authorization header required")
			c.Abort()
			return
		}

		// Check if header has "Bearer " prefix
		parts := strings.SplitN(authHeader, " ", 2)
		if len(parts) != 2 || parts[0] != "Bearer" {
			logger.Warn("Invalid authorization header format")
			response.Error(c, http.StatusUnauthorized, "Unauthorized", "Invalid authorization header format. Expected: Bearer <token>")
			c.Abort()
			return
		}

		tokenString := parts[1]

		// Parse and validate token
		claims, err := jwtpkg.ParseToken(tokenString)
		if err != nil {
			logger.Warn("Invalid token", logger.Err(err))

			// Return specific error message based on error type
			var message string
			switch err {
			case jwtpkg.ErrExpiredToken:
				message = "Token has expired"
			case jwtpkg.ErrInvalidSignature:
				message = "Invalid token signature"
			default:
				message = "Invalid token"
			}

			response.Error(c, http.StatusUnauthorized, "Unauthorized", message)
			c.Abort()
			return
		}

		// Store claims in context for later use
		SetUserContext(c, claims)

		logger.Info("User authenticated",
			logger.String("user_id", claims.UserID),
			logger.String("email", claims.Email),
		)

		c.Next()
	}
}

// OptionalAuth is a middleware that extracts JWT if present but doesn't require it
// Useful for endpoints that work differently for authenticated vs unauthenticated users
func OptionalAuth() gin.HandlerFunc {
	return func(c *gin.Context) {
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			// No token provided, continue without authentication
			c.Next()
			return
		}

		parts := strings.SplitN(authHeader, " ", 2)
		if len(parts) != 2 || parts[0] != "Bearer" {
			// Invalid format, continue without authentication
			c.Next()
			return
		}

		tokenString := parts[1]
		claims, err := jwtpkg.ParseToken(tokenString)
		if err != nil {
			// Invalid token, continue without authentication
			c.Next()
			return
		}

		// Valid token, store claims in context
		SetUserContext(c, claims)

		logger.Info("Optional auth: User authenticated",
			logger.String("user_id", claims.UserID),
		)

		c.Next()
	}
}
