package router

import (
	"time"

	"github.com/gin-contrib/cors"
	ginzap "github.com/gin-contrib/zap"
	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"
	swaggerFiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"
	"go.uber.org/zap"

	"venturo-skeleton-go/internal/config"
	"venturo-skeleton-go/internal/modules/core/auth"
	"venturo-skeleton-go/internal/modules/core/company"
	"venturo-skeleton-go/internal/modules/core/permission"
	"venturo-skeleton-go/internal/modules/core/permission_template"
	"venturo-skeleton-go/internal/modules/core/role"
	"venturo-skeleton-go/internal/modules/core/user"
	"venturo-skeleton-go/pkg/logger"
)

func Setup(router *gin.Engine, db *pgxpool.Pool, cfg *config.Config) {
	// Get logger instance
	log := logger.GetLogger()

	// Ginzap middleware for logging HTTP requests
	router.Use(ginzap.Ginzap(log, time.RFC3339, true))

	// Recovery middleware with Zap (handles panics)
	router.Use(ginzap.RecoveryWithZap(log, true))

	// CORS middleware configuration
	router.Use(cors.New(cors.Config{
		AllowOrigins:     []string{"http://localhost:5173", "http://localhost:3000"},
		AllowMethods:     []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"},
		AllowHeaders:     []string{"Origin", "Content-Type", "Accept", "Authorization"},
		ExposeHeaders:    []string{"Content-Length"},
		AllowCredentials: true,
		MaxAge:           12 * time.Hour,
	}))

	// Swagger documentation endpoints
	router.GET("/swagger/*any", ginSwagger.WrapHandler(swaggerFiles.Handler))

	// Health check endpoint
	router.GET("/health", func(c *gin.Context) {
		c.JSON(200, gin.H{
			"status":  "ok",
			"message": "API is running",
		})
	})

	// API v1 routes
	v1 := router.Group("/core/v1")
	{
		// Core modules
		// Initialize and setup auth module
		authModule := auth.Initialize(db, cfg)
		authModule.SetupRoutes(v1)

		// Initialize and setup user module
		userModule := user.Initialize(db)
		userModule.SetupRoutes(v1)

		// Initialize and setup role module
		roleModule := role.Initialize(db)
		roleModule.SetupRoutes(v1)

		// Initialize and setup permission module
		permissionModule := permission.Initialize(db)
		permissionModule.SetupRoutes(v1)

		// Initialize and setup permission template module
		permissionTemplateModule := permission_template.Initialize(db)
		permissionTemplateModule.SetupRoutes(v1)

		// Initialize and setup company module
		companyModule := company.Initialize(db)
		companyModule.SetupRoutes(v1)
	}

	log.Info("Routes setup completed", zap.Int("routes", len(router.Routes())))
}
