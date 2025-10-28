package main

import (
	_ "venturo-skeleton-go/docs/swagger" // Import generated swagger docs
	"venturo-skeleton-go/internal/config"
	"venturo-skeleton-go/internal/database"
	"venturo-skeleton-go/internal/router"
	"venturo-skeleton-go/pkg/logger"

	"github.com/gin-gonic/gin"
)

// @title Lakukan API
// @version 1.0
// @description Backend API untuk Lakukan - Multi-tenant Business Management Platform
// @termsOfService http://swagger.io/terms/

// @contact.name API Support
// @contact.url http://www.lakukan.com/support
// @contact.email support@lakukan.com

// @license.name Apache 2.0
// @license.url http://www.apache.org/licenses/LICENSE-2.0.html

// @host localhost:8080
// @BasePath /core/v1

// @securityDefinitions.apikey BearerAuth
// @in header
// @name Authorization
// @description Type "Bearer" followed by a space and JWT token.

func main() {
	// Load configuration
	cfg := config.Load()

	// Initialize logger
	if err := logger.Initialize(cfg.Server.Env); err != nil {
		panic("Failed to initialize logger: " + err.Error())
	}
	defer logger.Sync()

	logger.Info("Starting Lakukan API")

	// Initialize database
	db, err := database.New(cfg.Database.GetDSN())
	if err != nil {
		logger.Fatal("Failed to connect to database")
	}
	defer db.Close()

	logger.Info("Database connected successfully")

	// Initialize Gin router
	ginRouter := gin.New() // Use gin.New() instead of gin.Default() since we use custom middleware

	// Setup routes (all modules initialized inside router)
	router.Setup(ginRouter, db.Pool, cfg)

	// Start server
	serverAddr := ":" + cfg.Server.Port
	logger.Info("Starting server on " + serverAddr + " (Environment: " + cfg.Server.Env + ")")

	if err := ginRouter.Run(serverAddr); err != nil {
		logger.Fatal("Failed to start server")
	}
}
