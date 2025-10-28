package sales_persons

import (
	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"
	"lakukan-be/internal/config"
	"lakukan-be/internal/middleware"
	"lakukan-be/internal/modules/crm/sales_persons/handler"
	"lakukan-be/internal/modules/crm/sales_persons/repository"
	"lakukan-be/internal/modules/crm/sales_persons/service"
)

type SalesPersonModule struct {
	Handler *handler.SalesPersonHandler
}

// Initialize initializes the sales person module with dependency injection
func Initialize(db *pgxpool.Pool, cfg *config.Config) *SalesPersonModule {
	repo := repository.NewSalesPersonRepository(db)
	svc := service.NewSalesPersonService(repo)
	whatsappSvc := service.NewWhatsAppSessionService(repo, cfg)
	h := handler.NewSalesPersonHandler(svc, whatsappSvc)

	return &SalesPersonModule{
		Handler: h,
	}
}

// SetupRoutes registers all routes for sales person module
func (m *SalesPersonModule) SetupRoutes(router *gin.RouterGroup) {
	salesPersons := router.Group("/sales-persons")
	salesPersons.Use(middleware.JWTAuth())
	salesPersons.Use(middleware.CompanyContext())

	// List and get endpoints
	salesPersons.GET("", middleware.RequirePermission("crm.sales_persons:read"), m.Handler.GetAll)
	salesPersons.GET("/:id", middleware.RequirePermission("crm.sales_persons:read"), m.Handler.GetByID)

	// Create, update, delete endpoints
	salesPersons.POST("", middleware.RequirePermission("crm.sales_persons:create"), m.Handler.Create)
	salesPersons.PUT("/:id", middleware.RequirePermission("crm.sales_persons:update"), m.Handler.Update)
	salesPersons.DELETE("/:id", middleware.RequirePermission("crm.sales_persons:delete"), m.Handler.Delete)

	// Restore endpoint
	salesPersons.POST("/:id/restore", middleware.RequirePermission("crm.sales_persons:restore"), m.Handler.Restore)

	// WhatsApp endpoints
	salesPersons.POST("/:id/whatsapp/connect", middleware.RequirePermission("crm.sales_persons:manage"), m.Handler.ConnectWhatsApp)
	salesPersons.GET("/:id/whatsapp/status", middleware.RequirePermission("crm.sales_persons:read"), m.Handler.GetWhatsAppStatus)
	salesPersons.POST("/:id/whatsapp/disconnect", middleware.RequirePermission("crm.sales_persons:manage"), m.Handler.DisconnectWhatsApp)
	salesPersons.POST("/:id/whatsapp/restart", middleware.RequirePermission("crm.sales_persons:manage"), m.Handler.RestartWhatsApp)
}
