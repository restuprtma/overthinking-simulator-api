package auto_reply_rules

import (
	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"
	"venturo-skeleton-go/internal/middleware"
	"venturo-skeleton-go/internal/modules/crm/auto_reply_rules/handler"
	"venturo-skeleton-go/internal/modules/crm/auto_reply_rules/repository"
	"venturo-skeleton-go/internal/modules/crm/auto_reply_rules/service"
)

type AutoReplyRuleModule struct {
	Handler *handler.AutoReplyRuleHandler
}

// Initialize initializes the auto-reply rule module with dependency injection
func Initialize(db *pgxpool.Pool) *AutoReplyRuleModule {
	repo := repository.NewAutoReplyRuleRepository(db)
	svc := service.NewAutoReplyRuleService(repo)
	h := handler.NewAutoReplyRuleHandler(svc)

	return &AutoReplyRuleModule{
		Handler: h,
	}
}

// SetupRoutes registers all routes for auto-reply rule module
func (m *AutoReplyRuleModule) SetupRoutes(router *gin.RouterGroup) {
	autoReply := router.Group("/auto-reply-rules")
	autoReply.Use(middleware.JWTAuth())
	autoReply.Use(middleware.CompanyContext())

	// List and get endpoints
	autoReply.GET("", middleware.RequirePermission("crm.auto_reply_rules:read"), m.Handler.GetAll)
	autoReply.GET("/:id", middleware.RequirePermission("crm.auto_reply_rules:read"), m.Handler.GetByID)

	// Create, update, delete endpoints
	autoReply.POST("", middleware.RequirePermission("crm.auto_reply_rules:create"), m.Handler.Create)
	autoReply.PUT("/:id", middleware.RequirePermission("crm.auto_reply_rules:update"), m.Handler.Update)
	autoReply.DELETE("/:id", middleware.RequirePermission("crm.auto_reply_rules:delete"), m.Handler.Delete)

	// Restore endpoint
	autoReply.POST("/:id/restore", middleware.RequirePermission("crm.auto_reply_rules:restore"), m.Handler.Restore)
}
