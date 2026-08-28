package settings

import (
	"context"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"

	"venturo-skeleton-go/internal/middleware"
	"venturo-skeleton-go/internal/modules/reflection/settings/handler"
	"venturo-skeleton-go/internal/modules/reflection/settings/repository"
	"venturo-skeleton-go/internal/modules/reflection/settings/service"
)

type Module struct {
	Handler *handler.Handler
	Service *service.Service
}

// Initialize wires the settings module dependencies.
func Initialize(db *pgxpool.Pool, fallbackAPIKeys, fallbackModels []string) *Module {
	repo := repository.NewSettingsRepository(db)
	svc := service.NewService(repo, fallbackAPIKeys, fallbackModels)
	h := handler.NewHandler(svc)

	return &Module{
		Handler: h,
		Service: svc,
	}
}

// SetupRoutes registers the settings routes (JWT-protected).
func (m *Module) SetupRoutes(router *gin.RouterGroup) {
	s := router.Group("/settings")
	s.Use(middleware.JWTAuth())
	{
		s.GET("/gemini-credentials", m.Handler.Get)
		s.PUT("/gemini-credentials", m.Handler.Update)
	}
}

// CredentialsProvider returns the active credential list at request time.
// The reflection service (Phase 4) uses this to fetch key/model pairs.
func (m *Module) CredentialsProvider(ctx context.Context) ([]service.GeminiCredential, error) {
	return m.Service.GetCredentials(ctx)
}
