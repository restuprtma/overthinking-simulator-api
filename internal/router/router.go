package router

import (
	"context"
	"time"
	"venturo-skeleton-go/internal/config"
	"venturo-skeleton-go/internal/middleware"

	// Core modules
	"venturo-skeleton-go/internal/modules/core/auth"
	"venturo-skeleton-go/internal/modules/core/role"
	"venturo-skeleton-go/internal/modules/core/user"
	userRepo "venturo-skeleton-go/internal/modules/core/user/repository"

	"venturo-skeleton-go/internal/shared/audit"
	"venturo-skeleton-go/internal/shared/authz"

	pkgfirebase "venturo-skeleton-go/pkg/firebase"
	"venturo-skeleton-go/pkg/logger"

	"github.com/gin-contrib/cors"
	ginzap "github.com/gin-contrib/zap"
	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"
	"go.uber.org/zap"
)

func Setup(router *gin.Engine, db *pgxpool.Pool, cfg *config.Config) {
	log := logger.GetLogger()

	router.Use(ginzap.Ginzap(log, time.RFC3339, true))
	router.Use(ginzap.RecoveryWithZap(log, true))

	router.Use(cors.New(cors.Config{
		AllowOrigins:     []string{"http://localhost:5173", "http://localhost:3000", "http://localhost:3001", "http://localhost:8081"},
		AllowMethods:     []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"},
		AllowHeaders:     []string{"Origin", "Content-Type", "Accept", "Authorization", "X-API-Key"},
		ExposeHeaders:    []string{"Content-Length"},
		AllowCredentials: true,
		MaxAge:           12 * time.Hour,
	}))

	router.GET("/health", func(c *gin.Context) {
		c.JSON(200, gin.H{
			"status":  "ok",
			"message": "Overthinking Simulator API is running",
		})
	})

	coreV1 := router.Group("/core/v1")
	{
		authModule := auth.Initialize(db, cfg)
		authModule.SetupRoutes(coreV1)
		authModule.Service.SetUserIdentityRepo(userRepo.NewUserIdentityRepository(db))

		if cfg.Firebase.ProjectID != "" {
			fbClient, err := pkgfirebase.New(context.Background(), cfg.Firebase.ProjectID, cfg.Firebase.CredentialsJSON)
			if err != nil {
				log.Fatal("Failed to initialize Firebase Admin", zap.Error(err))
			}
			authModule.Service.SetFirebaseVerifier(fbClient)
			log.Info("Firebase Admin initialized", zap.String("project_id", cfg.Firebase.ProjectID))
		}

		userModule := user.Initialize(db)
		userModule.SetupRoutes(coreV1)

		roleModule := role.Initialize(db)
		roleModule.SetupRoutes(coreV1)

		authzService := authz.NewService(roleModule.Repository)
		middleware.SetAuthzService(authzService)
		roleModule.Service.SetPermissionCacheInvalidator(authzService)
		authModule.Service.SetPermissionReader(authzService)
	}

	auditModule := audit.Initialize(db)
	auditModule.SetupRoutes(coreV1)

	log.Info("Routes setup completed", zap.Int("routes", len(router.Routes())))
}
