package role

import (
	"lakukan-be/internal/middleware"
	"lakukan-be/internal/modules/core/role/handler"
	"lakukan-be/internal/modules/core/role/repository"
	"lakukan-be/internal/modules/core/role/service"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"
)

type RoleModule struct {
	Handler *handler.RoleHandler
}

// Initialize initializes the role module with all dependencies
func Initialize(db *pgxpool.Pool) *RoleModule {
	// Initialize repositories
	roleRepo := repository.NewRoleRepository(db)
	roleModuleTemplateRepo := repository.NewRoleModuleTemplateRepository(db)

	// Initialize services
	roleService := service.NewRoleService(roleRepo, roleModuleTemplateRepo, db)

	// Initialize handlers
	roleHandler := handler.NewRoleHandler(roleService)

	return &RoleModule{
		Handler: roleHandler,
	}
}

// SetupRoutes sets up role routes with permission-based authorization
func (m *RoleModule) SetupRoutes(router *gin.RouterGroup) {
	roles := router.Group("/roles")

	// Apply JWT authentication to all role routes
	roles.Use(middleware.JWTAuth())

	{
		// READ operations - Require core.roles:read permission
		roles.GET("",
			middleware.RequirePermission("core.roles:read"),
			m.Handler.GetAll,
		)
		roles.GET("/:id",
			middleware.RequirePermission("core.roles:read"),
			m.Handler.GetByID,
		)

		// CREATE - Require core.roles:create permission
		roles.POST("",
			middleware.RequirePermission("core.roles:create"),
			m.Handler.Create,
		)

		// UPDATE - Require core.roles:update permission
		roles.PUT("/:id",
			middleware.RequirePermission("core.roles:update"),
			m.Handler.Update,
		)

		// DELETE - Require core.roles:delete permission
		roles.DELETE("/:id",
			middleware.RequirePermission("core.roles:delete"),
			m.Handler.Delete,
		)

		// RESTORE - Require core.roles:restore permission
		roles.POST("/:id/restore",
			middleware.RequirePermission("core.roles:restore"),
			m.Handler.Restore,
		)

		// ROLE-PERMISSION MANAGEMENT
		// Get permissions for a role - Require core.roles:read permission
		roles.GET("/:id/permissions",
			middleware.RequirePermission("core.roles:read"),
			m.Handler.GetRolePermissions,
		)

		// Assign permissions to a role (replace all) - Require core.roles:update permission
		roles.PUT("/:id/permissions",
			middleware.RequirePermission("core.roles:update"),
			m.Handler.AssignPermissions,
		)

		// Add single permission to a role - Require core.roles:update permission
		roles.POST("/:id/permissions/:permission_id",
			middleware.RequirePermission("core.roles:update"),
			m.Handler.AddPermission,
		)

		// Remove single permission from a role - Require core.roles:update permission
		roles.DELETE("/:id/permissions/:permission_id",
			middleware.RequirePermission("core.roles:update"),
			m.Handler.RemovePermission,
		)
	}
}
