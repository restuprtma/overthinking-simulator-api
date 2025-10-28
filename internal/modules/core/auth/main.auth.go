package auth

import (
	"lakukan-be/internal/config"
	"lakukan-be/internal/middleware"
	"lakukan-be/internal/modules/core/auth/handler"
	authrepo "lakukan-be/internal/modules/core/auth/repository"
	"lakukan-be/internal/modules/core/auth/service"
	companyrepo "lakukan-be/internal/modules/core/company/repository"
	"lakukan-be/internal/modules/core/user/repository"
	"lakukan-be/pkg/email"
	"lakukan-be/pkg/logger"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"
)

type AuthModule struct {
	Handler *handler.AuthHandler
}

// Initialize initializes the auth module with all dependencies
func Initialize(db *pgxpool.Pool, cfg *config.Config) *AuthModule {
	// Initialize repositories
	userRepo := repository.NewUserRepository(db)
	tokenRepo := authrepo.NewTokenRepository(db)
	companyUserRepo := companyrepo.NewCompanyUserRepository(db)

	// Initialize email service
	var emailSvc email.EmailService
	smtpSvc, err := email.NewSMTPEmailService()
	if err != nil {
		logger.Error("Failed to initialize email service, using NoOp fallback", logger.Err(err))
		// Create a no-op email service for development
		emailSvc = &email.NoOpEmailService{}
	} else {
		emailSvc = smtpSvc
	}

	// Initialize services
	authService := service.NewAuthService(userRepo, tokenRepo, companyUserRepo, emailSvc, cfg)

	// Initialize handlers
	authHandler := handler.NewAuthHandler(authService)

	return &AuthModule{
		Handler: authHandler,
	}
}

// SetupRoutes sets up auth routes
func (m *AuthModule) SetupRoutes(router *gin.RouterGroup) {
	auth := router.Group("/auth")
	{
		// Public routes (no authentication required)
		auth.POST("/signin", m.Handler.SignIn)
		auth.POST("/signup", m.Handler.SignUp)
		auth.POST("/verify-email", m.Handler.VerifyEmail)
		auth.POST("/resend-verification", m.Handler.ResendVerification)
		auth.POST("/refresh", m.Handler.RefreshToken)
		auth.POST("/revoke", m.Handler.RevokeToken)

		// Protected routes (authentication required)
		auth.POST("/switch-company", middleware.JWTAuth(), m.Handler.SwitchCompany)
		auth.GET("/companies", middleware.JWTAuth(), m.Handler.GetUserCompanies)

		// auth.POST("/signout", m.Handler.SignOut)
		// auth.POST("/forgot-password", m.Handler.ForgotPassword)
		// auth.POST("/reset-password", m.Handler.ResetPassword)
	}
}