package lead_sources

import (
	"lakukan-be/internal/middleware"
	"lakukan-be/internal/modules/crm/lead_sources/handler"
	"lakukan-be/internal/modules/crm/lead_sources/repository"
	"lakukan-be/internal/modules/crm/lead_sources/service"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"
)

type LeadSourceModule struct {
	Handler *handler.LeadSourceHandler
}

// Initialize initializes the lead source module with all dependencies
func Initialize(db *pgxpool.Pool) *LeadSourceModule {
	// Initialize repositories
	repo := repository.NewLeadSourceRepository(db)

	// Initialize services
	svc := service.NewLeadSourceService(repo)

	// Initialize handlers
	h := handler.NewLeadSourceHandler(svc)

	return &LeadSourceModule{
		Handler: h,
	}
}

// SetupRoutes sets up lead source routes with permission-based authorization
func (m *LeadSourceModule) SetupRoutes(router *gin.RouterGroup) {
	leadSources := router.Group("/lead-sources")

	// Apply JWT authentication and company context to all routes
	leadSources.Use(middleware.JWTAuth())
	leadSources.Use(middleware.CompanyContext())

	{
		// READ operations - Require lead_sources:read permission
		leadSources.GET("",
			middleware.RequirePermission("crm.lead_sources:read"),
			m.Handler.GetAll,
		)
		leadSources.GET("/:id",
			middleware.RequirePermission("crm.lead_sources:read"),
			m.Handler.GetByID,
		)

		// CREATE - Require lead_sources:create permission
		leadSources.POST("",
			middleware.RequirePermission("crm.lead_sources:create"),
			m.Handler.Create,
		)

		// UPDATE - Require lead_sources:update permission
		leadSources.PUT("/:id",
			middleware.RequirePermission("crm.lead_sources:update"),
			m.Handler.Update,
		)

		// DELETE - Require lead_sources:delete permission
		leadSources.DELETE("/:id",
			middleware.RequirePermission("crm.lead_sources:delete"),
			m.Handler.Delete,
		)

		// RESTORE - Require lead_sources:restore permission
		leadSources.POST("/:id/restore",
			middleware.RequirePermission("crm.lead_sources:restore"),
			m.Handler.Restore,
		)
	}
}
