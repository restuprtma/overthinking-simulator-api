package user

import (
	"venturo-skeleton-go/internal/middleware"
	"venturo-skeleton-go/internal/modules/core/user/handler"
	"venturo-skeleton-go/internal/modules/core/user/repository"
	"venturo-skeleton-go/internal/modules/core/user/service"
	roleRepo "venturo-skeleton-go/internal/modules/core/role/repository"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"
)

type UserModule struct {
	Handler *handler.UserHandler
}

// Initialize initializes the user module with all dependencies
func Initialize(db *pgxpool.Pool) *UserModule {
	// Initialize repositories
	userRepo := repository.NewUserRepository(db)
	roleRepository := roleRepo.NewRoleRepository(db)

	// Initialize services
	userService := service.NewUserService(userRepo, roleRepository)

	// Initialize handlers
	userHandler := handler.NewUserHandler(userService)

	return &UserModule{
		Handler: userHandler,
	}
}

// SetupRoutes sets up user routes with permission-based authorization
func (m *UserModule) SetupRoutes(router *gin.RouterGroup) {
	users := router.Group("/users")

	// Apply JWT authentication to all user routes
	users.Use(middleware.JWTAuth())

	{
		// READ operations - Require core.users:read permission
		users.GET("",
			middleware.RequirePermission("core.users:read"),
			m.Handler.GetAll,
		)
		users.GET("/:id",
			middleware.RequirePermission("core.users:read"),
			m.Handler.GetByID,
		)

		// CREATE - Require core.users:create permission
		users.POST("",
			middleware.RequirePermission("core.users:create"),
			m.Handler.Create,
		)

		// UPDATE - Require core.users:update permission
		users.PUT("/:id",
			middleware.RequirePermission("core.users:update"),
			m.Handler.Update,
		)

		// DELETE - Require core.users:delete permission
		users.DELETE("/:id",
			middleware.RequirePermission("core.users:delete"),
			m.Handler.Delete,
		)

		// RESTORE - Require core.users:restore permission
		users.POST("/:id/restore",
			middleware.RequirePermission("core.users:restore"),
			m.Handler.Restore,
		)

		// CHANGE PASSWORD - Any authenticated user
		// Handler will check if user can only change their own password
		users.POST("/:id/change-password", m.Handler.ChangePassword)

		// ROLE MANAGEMENT - Require core.users:update permission
		users.POST("/:id/roles",
			middleware.RequirePermission("core.users:update"),
			m.Handler.AssignRoles,
		)
		users.DELETE("/:id/roles/:roleId",
			middleware.RequirePermission("core.users:update"),
			m.Handler.RemoveRole,
		)
		users.PUT("/:id/roles",
			middleware.RequirePermission("core.users:update"),
			m.Handler.SyncRoles,
		)
	}
}