package chats

import (
	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"
	"venturo-skeleton-go/internal/middleware"
	"venturo-skeleton-go/internal/modules/crm/chats/handler"
	"venturo-skeleton-go/internal/modules/crm/chats/repository"
	"venturo-skeleton-go/internal/modules/crm/chats/service"
)

type ChatModule struct {
	Handler *handler.ChatHandler
}

// Initialize initializes the chat module with dependency injection
func Initialize(db *pgxpool.Pool) *ChatModule {
	chatRepo := repository.NewChatRepository(db)
	messageRepo := repository.NewChatMessageRepository(db)
	svc := service.NewChatService(chatRepo, messageRepo)
	h := handler.NewChatHandler(svc)

	return &ChatModule{
		Handler: h,
	}
}

// SetupRoutes registers all routes for chat module
func (m *ChatModule) SetupRoutes(router *gin.RouterGroup) {
	chats := router.Group("/chats")
	chats.Use(middleware.JWTAuth())
	chats.Use(middleware.CompanyContext())

	// Create endpoint (for WhatsApp webhook)
	chats.POST("", middleware.RequirePermission("crm.chats:create"), m.Handler.Create)

	// Get all chats
	chats.GET("", middleware.RequirePermission("crm.chats:read"), m.Handler.GetAll)

	// Get chat detail with messages
	chats.GET("/:id", middleware.RequirePermission("crm.chats:read"), m.Handler.GetByIDWithMessages)
}
