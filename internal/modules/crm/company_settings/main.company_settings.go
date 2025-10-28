package company_settings

import (
	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"
	"lakukan-be/internal/middleware"
	"lakukan-be/internal/modules/crm/company_settings/handler"
	"lakukan-be/internal/modules/crm/company_settings/repository"
	"lakukan-be/internal/modules/crm/company_settings/service"
)

type CompanySettingsModule struct {
	Handler *handler.CompanySettingsHandler
}

// Initialize initializes the company settings module with dependency injection
func Initialize(db *pgxpool.Pool) *CompanySettingsModule {
	repo := repository.NewCompanySettingsRepository(db)
	svc := service.NewCompanySettingsService(repo)
	h := handler.NewCompanySettingsHandler(svc)

	return &CompanySettingsModule{
		Handler: h,
	}
}

// SetupRoutes registers all routes for company settings module
func (m *CompanySettingsModule) SetupRoutes(router *gin.RouterGroup) {
	settings := router.Group("/settings")
	settings.Use(middleware.JWTAuth())
	settings.Use(middleware.CompanyContext())

	// Get and update endpoints
	settings.GET("", middleware.RequirePermission("crm.company_settings:read"), m.Handler.Get)
	settings.PUT("", middleware.RequirePermission("crm.company_settings:update"), m.Handler.Update)
}
