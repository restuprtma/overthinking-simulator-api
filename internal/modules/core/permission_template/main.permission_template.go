package permission_template

import (
	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"

	"lakukan-be/internal/middleware"
	"lakukan-be/internal/modules/core/permission_template/handler"
	"lakukan-be/internal/modules/core/permission_template/repository"
	"lakukan-be/internal/modules/core/permission_template/service"
)

type PermissionTemplateModule struct {
	Handler *handler.PermissionTemplateHandler
}

// Initialize initializes the permission template module
func Initialize(db *pgxpool.Pool) *PermissionTemplateModule {
	// Repository layer
	templateRepo := repository.NewPermissionTemplateRepository(db)

	// Service layer
	templateService := service.NewPermissionTemplateService(templateRepo)

	// Handler layer
	templateHandler := handler.NewPermissionTemplateHandler(templateService)

	return &PermissionTemplateModule{
		Handler: templateHandler,
	}
}

// SetupRoutes sets up the routes for the permission template module
func (m *PermissionTemplateModule) SetupRoutes(router *gin.RouterGroup) {
	templates := router.Group("/permission-templates")
	templates.Use(middleware.JWTAuth())
	{
		templates.GET("", middleware.RequirePermission("core.permission_templates:read"), m.Handler.GetAll)
		templates.GET("/:id", middleware.RequirePermission("core.permission_templates:read"), m.Handler.GetByID)
		templates.POST("", middleware.RequirePermission("core.permission_templates:create"), m.Handler.Create)
		templates.PUT("/:id", middleware.RequirePermission("core.permission_templates:update"), m.Handler.Update)
		templates.DELETE("/:id", middleware.RequirePermission("core.permission_templates:delete"), m.Handler.Delete)
		templates.POST("/:id/restore", middleware.RequirePermission("core.permission_templates:restore"), m.Handler.Restore)
	}
}
