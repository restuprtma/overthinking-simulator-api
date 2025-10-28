package company

import (
	"venturo-skeleton-go/internal/middleware"
	"venturo-skeleton-go/internal/modules/core/company/handler"
	"venturo-skeleton-go/internal/modules/core/company/repository"
	"venturo-skeleton-go/internal/modules/core/company/service"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"
)

type CompanyModule struct {
	Handler *handler.CompanyHandler
}

// Initialize initializes the company module with all dependencies
func Initialize(db *pgxpool.Pool) *CompanyModule {
	// Initialize repositories
	companyRepo := repository.NewCompanyRepository(db)
	companyUserRepo := repository.NewCompanyUserRepository(db)

	// Initialize services
	companyService := service.NewCompanyService(companyRepo, companyUserRepo)

	// Initialize handlers
	companyHandler := handler.NewCompanyHandler(companyService)

	return &CompanyModule{
		Handler: companyHandler,
	}
}

// SetupRoutes sets up company routes with permission-based authorization
func (m *CompanyModule) SetupRoutes(router *gin.RouterGroup) {
	companies := router.Group("/companies")

	// Apply JWT authentication to all company routes
	companies.Use(middleware.JWTAuth())

	{
		// READ operations - Require core.companies:read permission
		companies.GET("",
			middleware.RequirePermission("core.companies:read"),
			m.Handler.GetAll,
		)
		companies.GET("/:id",
			middleware.RequirePermission("core.companies:read"),
			m.Handler.GetByID,
		)

		// CREATE - Require core.companies:create permission
		companies.POST("",
			middleware.RequirePermission("core.companies:create"),
			m.Handler.Create,
		)

		// UPDATE - Require core.companies:update permission
		companies.PUT("/:id",
			middleware.RequirePermission("core.companies:update"),
			m.Handler.Update,
		)

		// DELETE - Require core.companies:delete permission
		companies.DELETE("/:id",
			middleware.RequirePermission("core.companies:delete"),
			m.Handler.Delete,
		)

		// Company Users Management
		// Get users in company - Require core.companies:read permission
		companies.GET("/:id/users",
			middleware.RequirePermission("core.companies:read"),
			m.Handler.GetUsers,
		)

		// Add user to company - Require core.companies:manage_users permission
		companies.POST("/:id/users",
			middleware.RequirePermission("core.companies:manage_users"),
			m.Handler.AddUser,
		)

		// Update user role in company - Require core.companies:manage_users permission
		companies.PUT("/:id/users/:user_id",
			middleware.RequirePermission("core.companies:manage_users"),
			m.Handler.UpdateUserRole,
		)

		// Remove user from company - Require core.companies:manage_users permission
		companies.DELETE("/:id/users/:user_id",
			middleware.RequirePermission("core.companies:manage_users"),
			m.Handler.RemoveUser,
		)
	}
}
