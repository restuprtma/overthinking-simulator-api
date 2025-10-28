package permission

import (
	"venturo-skeleton-go/internal/middleware"
	"venturo-skeleton-go/internal/modules/core/permission/handler"
	"venturo-skeleton-go/internal/modules/core/permission/repository"
	"venturo-skeleton-go/internal/modules/core/permission/service"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"
)

type PermissionModule struct {
	PermissionHandler *handler.PermissionHandler
	ModuleHandler     *handler.ModuleHandler
}

// Initialize initializes the permission module with all dependencies
func Initialize(db *pgxpool.Pool) *PermissionModule {
	// Initialize repositories
	permissionRepo := repository.NewPermissionRepository(db)
	moduleRepo := repository.NewModuleRepository(db)

	// Initialize services
	permissionService := service.NewPermissionService(permissionRepo)
	moduleService := service.NewModuleService(moduleRepo)

	// Initialize handlers
	permissionHandler := handler.NewPermissionHandler(permissionService)
	moduleHandler := handler.NewModuleHandler(moduleService)

	return &PermissionModule{
		PermissionHandler: permissionHandler,
		ModuleHandler:     moduleHandler,
	}
}

// SetupRoutes sets up permission and module routes with permission-based authorization
func (m *PermissionModule) SetupRoutes(router *gin.RouterGroup) {
	// =====================================================
	// PERMISSIONS ROUTES
	// =====================================================
	permissions := router.Group("/permissions")
	permissions.Use(middleware.JWTAuth())

	{
		// READ operations - Require core.permissions:read permission
		permissions.GET("",
			middleware.RequirePermission("core.permissions:read"),
			m.PermissionHandler.GetAll,
		)
		permissions.GET("/:id",
			middleware.RequirePermission("core.permissions:read"),
			m.PermissionHandler.GetByID,
		)
		permissions.GET("/actions",
			middleware.RequirePermission("core.permissions:read"),
			m.PermissionHandler.GetActions,
		)

		// CREATE - Require core.permissions:create permission
		permissions.POST("",
			middleware.RequirePermission("core.permissions:create"),
			m.PermissionHandler.Create,
		)

		// UPDATE - Require core.permissions:update permission
		permissions.PUT("/:id",
			middleware.RequirePermission("core.permissions:update"),
			m.PermissionHandler.Update,
		)

		// DELETE - Require core.permissions:delete permission
		permissions.DELETE("/:id",
			middleware.RequirePermission("core.permissions:delete"),
			m.PermissionHandler.Delete,
		)
	}

	// =====================================================
	// MODULES ROUTES
	// =====================================================
	modules := router.Group("/modules")
	modules.Use(middleware.JWTAuth())

	{
		// READ operations - Require core.permissions:read permission
		modules.GET("",
			middleware.RequirePermission("core.permissions:read"),
			m.ModuleHandler.GetAll,
		)
		modules.GET("/tree",
			middleware.RequirePermission("core.permissions:read"),
			m.ModuleHandler.GetTree,
		)
		modules.GET("/:id",
			middleware.RequirePermission("core.permissions:read"),
			m.ModuleHandler.GetByID,
		)

		// CREATE - Require core.permissions:create permission
		modules.POST("",
			middleware.RequirePermission("core.permissions:create"),
			m.ModuleHandler.Create,
		)

		// UPDATE - Require core.permissions:update permission
		modules.PUT("/:id",
			middleware.RequirePermission("core.permissions:update"),
			m.ModuleHandler.Update,
		)

		// DELETE - Require core.permissions:delete permission
		modules.DELETE("/:id",
			middleware.RequirePermission("core.permissions:delete"),
			m.ModuleHandler.Delete,
		)

		// RESTORE - Require core.permissions:update permission
		modules.POST("/:id/restore",
			middleware.RequirePermission("core.permissions:update"),
			m.ModuleHandler.Restore,
		)

		// =====================================================
		// NESTED RESOURCE: Module Permissions
		// =====================================================
		// GET /modules/:id/permissions - Get permissions for a specific module
		modules.GET("/:id/permissions",
			middleware.RequirePermission("core.permissions:read"),
			m.PermissionHandler.GetByModuleID,
		)

		// POST /modules/:id/permissions/apply-crud-template - Apply standard CRUD template
		modules.POST("/:id/permissions/apply-crud-template",
			middleware.RequirePermission("core.permissions:create"),
			m.PermissionHandler.ApplyCRUDTemplate,
		)
	}
}
