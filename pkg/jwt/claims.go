package jwt

import (
	"github.com/golang-jwt/jwt/v5"
)

// Claims represents custom JWT claims for Lakukan
type Claims struct {
	UserID      string   `json:"user_id"`
	CompanyID   string   `json:"company_id"`   // Primary company ID for multi-tenancy
	CompanyName string   `json:"company_name"` // Company name for display purposes
	Email       string   `json:"email"`
	Username    string   `json:"username"`
	FullName    string   `json:"full_name"`
	Roles       []string `json:"roles"`
	Permissions []string `json:"permissions"`
	jwt.RegisteredClaims
}

// HasRole checks if user has a specific role
func (c *Claims) HasRole(role string) bool {
	for _, r := range c.Roles {
		if r == role {
			return true
		}
	}
	return false
}

// HasAnyRole checks if user has at least one of the specified roles
func (c *Claims) HasAnyRole(roles ...string) bool {
	for _, role := range roles {
		if c.HasRole(role) {
			return true
		}
	}
	return false
}

// HasAllRoles checks if user has all specified roles
func (c *Claims) HasAllRoles(roles ...string) bool {
	for _, role := range roles {
		if !c.HasRole(role) {
			return false
		}
	}
	return true
}

// HasPermission checks if user has a specific permission
func (c *Claims) HasPermission(permission string) bool {
	for _, p := range c.Permissions {
		if p == permission {
			return true
		}
	}
	return false
}

// HasAnyPermission checks if user has at least one of the specified permissions
func (c *Claims) HasAnyPermission(permissions ...string) bool {
	for _, permission := range permissions {
		if c.HasPermission(permission) {
			return true
		}
	}
	return false
}

// HasAllPermissions checks if user has all specified permissions
func (c *Claims) HasAllPermissions(permissions ...string) bool {
	for _, permission := range permissions {
		if !c.HasPermission(permission) {
			return false
		}
	}
	return true
}
