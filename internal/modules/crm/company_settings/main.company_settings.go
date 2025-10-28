package company_settings

import (
	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"
	"venturo-skeleton-go/internal/middleware"
	"venturo-skeleton-go/internal/modules/crm/company_settings/handler"
	"venturo-skeleton-go/internal/modules/crm/company_settings/repository"
	"venturo-skeleton-go/internal/modules/crm/company_settings/service"
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
