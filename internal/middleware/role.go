package middleware

import (
	"net/http"

	"venturo-skeleton-go/internal/shared/response"
	"venturo-skeleton-go/pkg/logger"

	"github.com/gin-gonic/gin"
)

// RequireRole checks if user has at least one of the required roles
// Must be used after JWTAuth middleware
func RequireRole(roles ...string) gin.HandlerFunc {
	return func(c *gin.Context) {
		claims, err := GetUserFromContext(c)
		if err != nil {
			logger.Warn("User not authenticated for role check")
			response.Error(c, http.StatusUnauthorized, "Unauthorized", "Authentication required")
			c.Abort()
			return
		}

		// Check if user has any of the required roles
		if !claims.HasAnyRole(roles...) {
			logger.Warn("User lacks required role",
				logger.String("user_id", claims.UserID),
				logger.String("required_roles", joinStrings(roles)),
				logger.String("user_roles", joinStrings(claims.Roles)),
			)
			response.Error(c, http.StatusForbidden, "Forbidden", "Insufficient permissions")
			c.Abort()
			return
		}

		logger.Info("Role check passed",
			logger.String("user_id", claims.UserID),
			logger.String("matched_role", findMatchingRole(claims.Roles, roles)),
		)

		c.Next()
	}
}

// RequireAllRoles checks if user has all the specified roles
// Must be used after JWTAuth middleware
func RequireAllRoles(roles ...string) gin.HandlerFunc {
	return func(c *gin.Context) {
		claims, err := GetUserFromContext(c)
		if err != nil {
			logger.Warn("User not authenticated for role check")
			response.Error(c, http.StatusUnauthorized, "Unauthorized", "Authentication required")
			c.Abort()
			return
		}

		// Check if user has all required roles
		if !claims.HasAllRoles(roles...) {
			logger.Warn("User lacks all required roles",
				logger.String("user_id", claims.UserID),
				logger.String("required_roles", joinStrings(roles)),
				logger.String("user_roles", joinStrings(claims.Roles)),
			)
			response.Error(c, http.StatusForbidden, "Forbidden", "Insufficient permissions")
			c.Abort()
			return
		}

		logger.Info("All roles check passed",
			logger.String("user_id", claims.UserID),
		)

		c.Next()
	}
}

// RequirePermission checks if user has a specific permission
// Must be used after JWTAuth middleware
func RequirePermission(permission string) gin.HandlerFunc {
	return func(c *gin.Context) {
		claims, err := GetUserFromContext(c)
		if err != nil {
			logger.Warn("User not authenticated for permission check")
			response.Error(c, http.StatusUnauthorized, "Unauthorized", "Authentication required")
			c.Abort()
			return
		}

		// Check if user has the required permission
		if !claims.HasPermission(permission) {
			logger.Warn("User lacks required permission",
				logger.String("user_id", claims.UserID),
				logger.String("required_permission", permission),
				logger.String("user_permissions", joinStrings(claims.Permissions)),
			)
			response.Error(c, http.StatusForbidden, "Forbidden", "Insufficient permissions")
			c.Abort()
			return
		}

		logger.Info("Permission check passed",
			logger.String("user_id", claims.UserID),
			logger.String("permission", permission),
		)

		c.Next()
	}
}

// RequireAnyPermission checks if user has at least one of the specified permissions
// Must be used after JWTAuth middleware
func RequireAnyPermission(permissions ...string) gin.HandlerFunc {
	return func(c *gin.Context) {
		claims, err := GetUserFromContext(c)
		if err != nil {
			logger.Warn("User not authenticated for permission check")
			response.Error(c, http.StatusUnauthorized, "Unauthorized", "Authentication required")
			c.Abort()
			return
		}

		// Check if user has any of the required permissions
		if !claims.HasAnyPermission(permissions...) {
			logger.Warn("User lacks required permissions",
				logger.String("user_id", claims.UserID),
				logger.String("required_permissions", joinStrings(permissions)),
				logger.String("user_permissions", joinStrings(claims.Permissions)),
			)
			response.Error(c, http.StatusForbidden, "Forbidden", "Insufficient permissions")
			c.Abort()
			return
		}

		logger.Info("Permission check passed",
			logger.String("user_id", claims.UserID),
		)

		c.Next()
	}
}

// RequireAllPermissions checks if user has all the specified permissions
// Must be used after JWTAuth middleware
func RequireAllPermissions(permissions ...string) gin.HandlerFunc {
	return func(c *gin.Context) {
		claims, err := GetUserFromContext(c)
		if err != nil {
			logger.Warn("User not authenticated for permission check")
			response.Error(c, http.StatusUnauthorized, "Unauthorized", "Authentication required")
			c.Abort()
			return
		}

		// Check if user has all required permissions
		if !claims.HasAllPermissions(permissions...) {
			logger.Warn("User lacks all required permissions",
				logger.String("user_id", claims.UserID),
				logger.String("required_permissions", joinStrings(permissions)),
				logger.String("user_permissions", joinStrings(claims.Permissions)),
			)
			response.Error(c, http.StatusForbidden, "Forbidden", "Insufficient permissions")
			c.Abort()
			return
		}

		logger.Info("All permissions check passed",
			logger.String("user_id", claims.UserID),
		)

		c.Next()
	}
}

// Helper functions

// joinStrings joins string slice with comma separator
func joinStrings(strs []string) string {
	result := ""
	for i, str := range strs {
		if i > 0 {
			result += ", "
		}
		result += str
	}
	return result
}

// findMatchingRole finds the first matching role between user roles and required roles
func findMatchingRole(userRoles, requiredRoles []string) string {
	for _, userRole := range userRoles {
		for _, reqRole := range requiredRoles {
			if userRole == reqRole {
				return userRole
			}
		}
	}
	return ""
}
