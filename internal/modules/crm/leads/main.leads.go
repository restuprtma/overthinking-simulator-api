package leads

import (
	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"
	"lakukan-be/internal/middleware"
	"lakukan-be/internal/modules/crm/leads/handler"
	"lakukan-be/internal/modules/crm/leads/repository"
	"lakukan-be/internal/modules/crm/leads/service"
)

type LeadModule struct {
	Handler *handler.LeadHandler
}

// Initialize initializes the lead module with dependency injection
func Initialize(db *pgxpool.Pool) *LeadModule {
	repo := repository.NewLeadRepository(db)
	svc := service.NewLeadService(repo)
	h := handler.NewLeadHandler(svc)

	return &LeadModule{
		Handler: h,
	}
}

// SetupRoutes registers all routes for lead module
func (m *LeadModule) SetupRoutes(router *gin.RouterGroup) {
	leads := router.Group("/leads")
	leads.Use(middleware.JWTAuth())
	leads.Use(middleware.CompanyContext())

	// List and get endpoints
	leads.GET("", middleware.RequirePermission("crm.leads:read"), m.Handler.GetAll)
	leads.GET("/:id", middleware.RequirePermission("crm.leads:read"), m.Handler.GetByID)

	// Create, update, delete endpoints
	leads.POST("", middleware.RequirePermission("crm.leads:create"), m.Handler.Create)
	leads.PUT("/:id", middleware.RequirePermission("crm.leads:update"), m.Handler.Update)
	leads.DELETE("/:id", middleware.RequirePermission("crm.leads:delete"), m.Handler.Delete)

	// Restore endpoint
	leads.POST("/:id/restore", middleware.RequirePermission("crm.leads:restore"), m.Handler.Restore)
}
