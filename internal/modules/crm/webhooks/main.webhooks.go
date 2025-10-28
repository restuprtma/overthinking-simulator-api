package webhooks

import (
	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"
	"venturo-skeleton-go/internal/config"
	chatsService "venturo-skeleton-go/internal/modules/crm/chats/service"
	salesPersonsRepo "venturo-skeleton-go/internal/modules/crm/sales_persons/repository"
	salesPersonsService "venturo-skeleton-go/internal/modules/crm/sales_persons/service"
	"venturo-skeleton-go/internal/modules/crm/webhooks/handler"
	"venturo-skeleton-go/internal/modules/crm/webhooks/service"

	chatsRepo "venturo-skeleton-go/internal/modules/crm/chats/repository"
)

type WebhooksModule struct {
	Handler *handler.WAHAWebhookHandler
}

// Initialize initializes the webhooks module with dependency injection
func Initialize(db *pgxpool.Pool, cfg *config.Config) *WebhooksModule {
	// Initialize dependencies
	salesPersonRepo := salesPersonsRepo.NewSalesPersonRepository(db)
	whatsappSvc := salesPersonsService.NewWhatsAppSessionService(salesPersonRepo, cfg)

	// Initialize chat service
	chatRepo := chatsRepo.NewChatRepository(db)
	chatMessageRepo := chatsRepo.NewChatMessageRepository(db)
	chatSvc := chatsService.NewChatService(chatRepo, chatMessageRepo)

	// Initialize webhook service
	webhookSvc := service.NewWAHAWebhookService(salesPersonRepo, whatsappSvc, chatSvc, cfg)

	// Initialize handler
	h := handler.NewWAHAWebhookHandler(webhookSvc)

	return &WebhooksModule{
		Handler: h,
	}
}

// SetupRoutes registers all routes for webhooks module
func (m *WebhooksModule) SetupRoutes(router *gin.RouterGroup) {
	webhooks := router.Group("/webhooks")

	// No authentication required for webhooks (uses HMAC verification instead)
	webhooks.POST("/waha", m.Handler.ReceiveWebhook)
}
