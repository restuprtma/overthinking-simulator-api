package reflection

import (
	"context"
	_ "embed"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"

	"venturo-skeleton-go/internal/middleware"
	"venturo-skeleton-go/internal/modules/reflection/handler"
	"venturo-skeleton-go/internal/modules/reflection/repository"
	"venturo-skeleton-go/internal/modules/reflection/service"
	settingsService "venturo-skeleton-go/internal/modules/reflection/settings/service"
)

//go:embed data/distortions.json
var distortionsJSON []byte

type ReflectionModule struct {
	Handler *handler.ReflectionHandler
}

// Initialize wires the reflection module dependencies.
func Initialize(db *pgxpool.Pool, timeout time.Duration, credentialsProvider func(ctx context.Context) ([]settingsService.GeminiCredential, error)) *ReflectionModule {
	repo := repository.NewReflectionRepository(db)
	svc := service.NewService(repo, timeout, distortionsJSON, credentialsProvider)
	h := handler.NewReflectionHandler(svc)

	return &ReflectionModule{
		Handler: h,
	}
}

// SetupRoutes registers the reflection routes (JWT-protected).
func (m *ReflectionModule) SetupRoutes(router *gin.RouterGroup) {
	reflections := router.Group("/reflections")
	reflections.Use(middleware.JWTAuth())
	{
		reflections.POST("", m.Handler.Create)
		reflections.GET("", m.Handler.List)
		reflections.GET("/:id", m.Handler.Get)
	}
}
