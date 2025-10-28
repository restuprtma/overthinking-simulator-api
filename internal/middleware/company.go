package middleware

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// CompanyContext middleware ensures that user has a valid company context
// This middleware should be used after JWTAuth middleware
func CompanyContext() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Get company ID from JWT claims
		companyID := GetCompanyID(c)

		// Check if company ID exists
		if companyID == "" {
			c.JSON(http.StatusForbidden, gin.H{
				"success": false,
				"message": "Company context is required. User must be associated with a company.",
				"error":   "missing_company_context",
			})
			c.Abort()
			return
		}

		// Company ID is valid, continue
		c.Next()
	}
}
