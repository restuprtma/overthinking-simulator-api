package handler

import (
	"encoding/json"
	"io"
	"net/http"

	"github.com/gin-gonic/gin"
	"lakukan-be/internal/modules/crm/webhooks/dto"
	"lakukan-be/internal/modules/crm/webhooks/service"
	"lakukan-be/internal/shared/response"
	"lakukan-be/pkg/logger"
)

type WAHAWebhookHandler struct {
	service *service.WAHAWebhookService
}

func NewWAHAWebhookHandler(service *service.WAHAWebhookService) *WAHAWebhookHandler {
	return &WAHAWebhookHandler{service: service}
}

// ReceiveWebhook godoc
// @Summary Receive WAHA webhook
// @Description Receive and process webhooks from WAHA (WhatsApp HTTP API)
// @Tags CRM - Webhooks
// @Accept json
// @Produce json
// @Param X-Webhook-Request-Id header string false "Webhook request ID"
// @Param X-Webhook-Timestamp header string false "Webhook timestamp"
// @Param X-Webhook-Hmac header string false "HMAC signature for verification"
// @Param payload body dto.WAHAWebhookPayload true "Webhook payload"
// @Success 200 {object} response.Response
// @Failure 400 {object} response.Response
// @Failure 401 {object} response.Response
// @Failure 500 {object} response.Response
// @Router /crm/v1/webhooks/waha [post]
func (h *WAHAWebhookHandler) ReceiveWebhook(c *gin.Context) {
	// Read raw body for HMAC verification
	bodyBytes, err := io.ReadAll(c.Request.Body)
	if err != nil {
		logger.Error("Failed to read webhook body", logger.Err(err))
		response.Error(c, http.StatusBadRequest, "Failed to read request body", err.Error())
		return
	}

	// Extract headers
	requestID := c.GetHeader("X-Webhook-Request-Id")
	timestamp := c.GetHeader("X-Webhook-Timestamp")
	hmacSignature := c.GetHeader("X-Webhook-Hmac")

	logger.Info("Received WAHA webhook",
		logger.String("request_id", requestID),
		logger.String("timestamp", timestamp),
		logger.String("hmac", hmacSignature),
	)

	// Verify HMAC signature
	if !h.service.VerifyHMAC(hmacSignature, timestamp, string(bodyBytes)) {
		logger.Warn("HMAC verification failed for webhook",
			logger.String("request_id", requestID),
		)
		response.Error(c, http.StatusUnauthorized, "Invalid webhook signature", "")
		return
	}

	// Parse webhook payload from the body bytes (since body already read)
	var payload dto.WAHAWebhookPayload
	if err := json.Unmarshal(bodyBytes, &payload); err != nil {
		logger.Error("Failed to parse webhook payload", logger.Err(err))
		response.Error(c, http.StatusBadRequest, "Invalid webhook payload", err.Error())
		return
	}

	logger.Info("Webhook parsed successfully",
		logger.String("event", payload.Event),
		logger.String("session", payload.Session),
	)

	// Process webhook asynchronously to respond quickly
	// WAHA expects 200 OK response within reasonable time
	go func() {
		if err := h.service.HandleWebhook(payload); err != nil {
			logger.Error("Failed to process webhook",
				logger.Err(err),
				logger.String("event", payload.Event),
				logger.String("session", payload.Session),
			)
		}
	}()

	// Respond immediately with success
	response.Success(c, http.StatusOK, "Webhook received", nil)
}
