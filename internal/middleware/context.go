package middleware

import (
	"errors"

	jwtpkg "venturo-skeleton-go/pkg/jwt"

	"github.com/gin-gonic/gin"
)

const (
	// UserContextKey is the key used to store user claims in gin.Context
	UserContextKey = "user"
)

var (
	ErrUserNotFoundInContext = errors.New("user not found in context")
)

// SetUserContext stores user claims in gin.Context
func SetUserContext(c *gin.Context, claims *jwtpkg.Claims) {
	c.Set(UserContextKey, claims)
}

// GetUserFromContext retrieves user claims from gin.Context
func GetUserFromContext(c *gin.Context) (*jwtpkg.Claims, error) {
	value, exists := c.Get(UserContextKey)
	if !exists {
		return nil, ErrUserNotFoundInContext
	}

	claims, ok := value.(*jwtpkg.Claims)
	if !ok {
		return nil, ErrUserNotFoundInContext
	}

	return claims, nil
}

// MustGetUserFromContext retrieves user claims and panics if not found
// Use this only in handlers that are protected by JWTAuth middleware
func MustGetUserFromContext(c *gin.Context) *jwtpkg.Claims {
	claims, err := GetUserFromContext(c)
	if err != nil {
		panic(err)
	}
	return claims
}

// GetUserID retrieves user ID from context
func GetUserID(c *gin.Context) (string, error) {
	claims, err := GetUserFromContext(c)
	if err != nil {
		return "", err
	}
	return claims.UserID, nil
}

// GetUserEmail retrieves user email from context
func GetUserEmail(c *gin.Context) (string, error) {
	claims, err := GetUserFromContext(c)
	if err != nil {
		return "", err
	}
	return claims.Email, nil
}

// GetUserRoles retrieves user roles from context
func GetUserRoles(c *gin.Context) ([]string, error) {
	claims, err := GetUserFromContext(c)
	if err != nil {
		return nil, err
	}
	return claims.Roles, nil
}

// GetUserPermissions retrieves user permissions from context
func GetUserPermissions(c *gin.Context) ([]string, error) {
	claims, err := GetUserFromContext(c)
	if err != nil {
		return nil, err
	}
	return claims.Permissions, nil
}

// GetCompanyID retrieves company ID from context
func GetCompanyID(c *gin.Context) string {
	claims, err := GetUserFromContext(c)
	if err != nil {
		return ""
	}
	return claims.CompanyID
}

// MustGetUserID retrieves user ID from context and panics if not found
// Use this only in handlers that are protected by JWTAuth middleware
func MustGetUserID(c *gin.Context) string {
	claims := MustGetUserFromContext(c)
	return claims.UserID
}
