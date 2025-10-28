package service

import (
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"time"

	"github.com/google/uuid"
	"lakukan-be/internal/modules/crm/chats/domain"
	"lakukan-be/internal/modules/crm/chats/dto"
	"lakukan-be/internal/modules/crm/chats/repository"
	webhooksDto "lakukan-be/internal/modules/crm/webhooks/dto"
	"lakukan-be/pkg/logger"
)

var (
	ErrChatNotFound = errors.New("chat not found")
)

type ChatService struct {
	chatRepo    *repository.ChatRepository
	messageRepo *repository.ChatMessageRepository
}

func NewChatService(chatRepo *repository.ChatRepository, messageRepo *repository.ChatMessageRepository) *ChatService {
	return &ChatService{
		chatRepo:    chatRepo,
		messageRepo: messageRepo,
	}
}

// GetAll returns paginated list of chats
func (s *ChatService) GetAll(companyID string, params dto.ChatQueryParams) ([]dto.ChatResponse, int, int, error) {
	logger.Info("Fetching chats",
		logger.String("company_id", companyID),
		logger.Int("page", params.Page),
		logger.Int("page_size", params.PageSize),
	)

	// Calculate offset
	offset := (params.Page - 1) * params.PageSize

	// Get total count
	total, err := s.chatRepo.Count(companyID, params.Search, params.Platform, params.Status, params.Category, params.AssignedToCompanyUserID)
	if err != nil {
		logger.Error("Failed to count chats", logger.Err(err))
		return nil, 0, 0, err
	}

	// Get chats
	chats, err := s.chatRepo.FindAll(companyID, params.PageSize, offset, params.Search, params.Platform, params.Status, params.Category, params.AssignedToCompanyUserID)
	if err != nil {
		logger.Error("Failed to fetch chats", logger.Err(err))
		return nil, 0, 0, err
	}

	// Convert to response DTOs
	responses := make([]dto.ChatResponse, len(chats))
	for i, chat := range chats {
		responses[i] = s.toChatResponse(chat)
	}

	// Calculate total pages
	totalPages := int(math.Ceil(float64(total) / float64(params.PageSize)))

	logger.Info("Successfully fetched chats",
		logger.Int("total", total),
		logger.Int("count", len(responses)),
	)

	return responses, total, totalPages, nil
}

// GetByIDWithMessages returns a single chat with all its messages
func (s *ChatService) GetByIDWithMessages(id, companyID string) (*dto.ChatDetailResponse, error) {
	logger.Info("Fetching chat with messages",
		logger.String("id", id),
		logger.String("company_id", companyID),
	)

	// Get chat
	chat, err := s.chatRepo.FindByID(id, companyID)
	if err != nil {
		logger.Error("Failed to fetch chat", logger.Err(err))
		return nil, err
	}

	if chat == nil {
		logger.Warn("Chat not found",
			logger.String("id", id),
			logger.String("company_id", companyID),
		)
		return nil, ErrChatNotFound
	}

	// Get messages
	messages, err := s.messageRepo.FindByChatID(id)
	if err != nil {
		logger.Error("Failed to fetch messages", logger.Err(err))
		return nil, err
	}

	// Convert to response
	chatResponse := s.toChatResponse(chat)
	messageResponses := make([]dto.ChatMessageResponse, len(messages))
	for i, msg := range messages {
		messageResponses[i] = s.toMessageResponse(msg)
	}

	response := &dto.ChatDetailResponse{
		Chat:     chatResponse,
		Messages: messageResponses,
	}

	logger.Info("Successfully fetched chat with messages",
		logger.String("id", id),
		logger.Int("message_count", len(messages)),
	)

	return response, nil
}

// Create creates a new chat (for WhatsApp webhook)
func (s *ChatService) Create(companyID string, req dto.CreateChatRequest) (*dto.ChatResponse, error) {
	logger.Info("Creating new chat",
		logger.String("company_id", companyID),
		logger.String("phone", req.Phone),
		logger.String("platform", req.Platform),
	)

	// Check if chat already exists for this phone and platform
	existing, err := s.chatRepo.FindByPhoneAndPlatform(req.Phone, req.Platform, companyID)
	if err != nil {
		logger.Error("Failed to check existing chat", logger.Err(err))
		return nil, err
	}

	// If chat exists, update it and optionally add first message
	if existing != nil {
		logger.Info("Chat already exists, updating",
			logger.String("chat_id", existing.ID),
		)

		// If there's a first message, create it
		if req.FirstMessage != nil {
			if err := s.createMessage(existing.ID, req.FirstMessage); err != nil {
				logger.Error("Failed to create first message", logger.Err(err))
				return nil, err
			}
		}

		// Refresh chat data
		updated, err := s.chatRepo.FindByID(existing.ID, companyID)
		if err != nil {
			logger.Error("Failed to fetch updated chat", logger.Err(err))
			return nil, err
		}

		response := s.toChatResponse(updated)
		return &response, nil
	}

	// Create new chat
	now := time.Now()
	chat := &domain.Chat{
		ID:                     uuid.New().String(),
		CompanyID:              companyID,
		LeadID:                 req.LeadID,
		CustomerName:           req.CustomerName,
		Phone:                  req.Phone,
		Email:                  req.Email,
		AssignedToCompanyUserID: req.AssignedToCompanyUserID,
		Platform:               req.Platform,
		Status:                 "active", // Default status
		Category:               req.Category,
		UnreadCount:            0,
		MessageCount:           0,
		Tags:                   []string{},
		CreatedAt:              now,
		UpdatedAt:              now,
	}

	// Save chat to database
	if err := s.chatRepo.Create(chat); err != nil {
		logger.Error("Failed to create chat", logger.Err(err))
		return nil, err
	}

	// If there's a first message, create it
	if req.FirstMessage != nil {
		if err := s.createMessage(chat.ID, req.FirstMessage); err != nil {
			logger.Error("Failed to create first message", logger.Err(err))
			return nil, err
		}

		// Update chat with message info
		chat.MessageCount = 1
		chat.LastMessageAt = &now
		preview := req.FirstMessage.Content
		if len(preview) > 100 {
			preview = preview[:100] + "..."
		}
		chat.LastMessagePreview = &preview
		sender := req.FirstMessage.SenderType
		chat.LastMessageSender = &sender

		if err := s.chatRepo.Update(chat); err != nil {
			logger.Error("Failed to update chat with message info", logger.Err(err))
		}
	}

	logger.Info("Successfully created chat",
		logger.String("id", chat.ID),
	)

	// Fetch created chat with joined fields
	created, err := s.chatRepo.FindByID(chat.ID, companyID)
	if err != nil {
		logger.Error("Failed to fetch created chat", logger.Err(err))
		return nil, err
	}

	response := s.toChatResponse(created)
	return &response, nil
}

// createMessage creates a new message in a chat
func (s *ChatService) createMessage(chatID string, req *dto.CreateChatMessageRequest) error {
	now := time.Now()

	message := &domain.ChatMessage{
		ID:             uuid.New().String(),
		ChatID:         chatID,
		SenderType:     req.SenderType,
		SenderID:       req.SenderID,
		SenderName:     req.SenderName,
		Content:        req.Content,
		MessageType:    req.MessageType,
		MediaURL:       req.MediaURL,
		MediaMimeType:  req.MediaMimeType,
		MediaSizeBytes: req.MediaSizeBytes,
		IsRead:         false,
		IsSent:         true,
		SentAt:         now,
		DeliveryStatus: req.DeliveryStatus,
		Metadata:       req.Metadata,
		CreatedAt:      now,
	}

	return s.messageRepo.Create(message)
}

// toChatResponse converts domain entity to response DTO
func (s *ChatService) toChatResponse(chat *domain.Chat) dto.ChatResponse {
	response := dto.ChatResponse{
		ID:                     chat.ID,
		CompanyID:              chat.CompanyID,
		CustomerName:           chat.CustomerName,
		Phone:                  chat.Phone,
		Email:                  chat.Email,
		Platform:               chat.Platform,
		Status:                 chat.Status,
		Category:               chat.Category,
		LastMessageAt:          chat.LastMessageAt,
		LastMessagePreview:     chat.LastMessagePreview,
		LastMessageSender:      chat.LastMessageSender,
		UnreadCount:            chat.UnreadCount,
		MessageCount:           chat.MessageCount,
		AvgResponseTimeMinutes: chat.AvgResponseTimeMinutes,
		FirstResponseTimeMinutes: chat.FirstResponseTimeMinutes,
		SentimentScore:         chat.SentimentScore,
		Tags:                   chat.Tags,
		CreatedAt:              chat.CreatedAt,
		UpdatedAt:              chat.UpdatedAt,
	}

	// Build lead info
	if chat.LeadID != nil && chat.LeadName != nil {
		response.Lead = &dto.LeadInfo{
			ID:         *chat.LeadID,
			Name:       *chat.LeadName,
			LeadNumber: "", // Not loaded for performance
		}
	}

	// Build assigned user info
	if chat.AssignedToCompanyUserID != nil && chat.AssignedToUserID != nil && chat.AssignedToUserName != nil && chat.AssignedToUserEmail != nil {
		response.AssignedTo = &dto.AssignedUserInfo{
			CompanyUserID: *chat.AssignedToCompanyUserID,
			UserID:        *chat.AssignedToUserID,
			FullName:      *chat.AssignedToUserName,
			Email:         *chat.AssignedToUserEmail,
		}
	}

	return response
}

// toMessageResponse converts domain entity to response DTO
func (s *ChatService) toMessageResponse(msg *domain.ChatMessage) dto.ChatMessageResponse {
	return dto.ChatMessageResponse{
		ID:             msg.ID,
		ChatID:         msg.ChatID,
		SenderType:     msg.SenderType,
		SenderID:       msg.SenderID,
		SenderName:     msg.SenderName,
		Content:        msg.Content,
		MessageType:    msg.MessageType,
		MediaURL:       msg.MediaURL,
		MediaMimeType:  msg.MediaMimeType,
		MediaSizeBytes: msg.MediaSizeBytes,
		IsRead:         msg.IsRead,
		ReadAt:         msg.ReadAt,
		IsSent:         msg.IsSent,
		SentAt:         msg.SentAt,
		DeliveryStatus: msg.DeliveryStatus,
		CreatedAt:      msg.CreatedAt,
	}
}

// HandleWAHAMessage processes incoming message from WAHA webhook and stores it
func (s *ChatService) HandleWAHAMessage(companyID, salesPersonID, phoneNumber string, messagePayload webhooksDto.WAHAMessagePayload, webhookPayload webhooksDto.WAHAWebhookPayload, contactName string) error {
	logger.Info("Handling WAHA message",
		logger.String("company_id", companyID),
		logger.String("sales_person_id", salesPersonID),
		logger.String("phone", phoneNumber),
		logger.String("message_id", messagePayload.ID),
		logger.String("contact_name", contactName),
	)

	// Find or create chat
	chat, err := s.findOrCreateChat(companyID, salesPersonID, phoneNumber, contactName)
	if err != nil {
		logger.Error("Failed to find or create chat", logger.Err(err))
		return err
	}

	// Check if message already exists (avoid duplicates)
	existing, err := s.messageRepo.FindByWAHAMessageID(messagePayload.ID)
	if err != nil {
		logger.Error("Failed to check existing message", logger.Err(err))
		// Continue anyway, better to have duplicate than miss a message
	}
	if existing != nil {
		logger.Info("Message already exists, skipping", logger.String("message_id", messagePayload.ID))
		return nil
	}

	// Create chat message
	now := time.Now()
	sentAt := time.Unix(messagePayload.Timestamp/1000, 0)

	// Determine sender type
	senderType := "customer"
	if messagePayload.FromMe {
		senderType = "sales"
	}

	// Map message type from WAHA to internal format
	messageType := mapWAHAMessageType(messagePayload.Type)

	// Prepare content - use caption if available for media messages
	content := messagePayload.Body
	if messagePayload.Caption != "" {
		content = messagePayload.Caption
	}

	// Map delivery status from ACK
	deliveryStatus := mapWAHAAckToDeliveryStatus(messagePayload.ACK)

	// Prepare metadata with full WAHA payload
	metadataBytes, err := json.Marshal(map[string]interface{}{
		"waha_message_id": messagePayload.ID,
		"waha_event_type": "message.any",
		"waha_timestamp":  messagePayload.Timestamp,
		"waha_ack_status": messagePayload.ACK,
		"waha_from_me":    messagePayload.FromMe,
		"waha_chat_id":    messagePayload.ChatID,
		"webhook_payload": webhookPayload,
	})
	if err != nil {
		logger.Error("Failed to marshal metadata", logger.Err(err))
		return err
	}
	metadataStr := string(metadataBytes)

	message := &domain.ChatMessage{
		ID:             uuid.New().String(),
		ChatID:         chat.ID,
		SenderType:     senderType,
		SenderID:       nil, // We don't have sender ID from WAHA
		SenderName:     nil, // We could extract from webhook payload if needed
		Content:        content,
		MessageType:    messageType,
		MediaURL:       getStringPtr(messagePayload.MediaURL),
		MediaMimeType:  getStringPtr(messagePayload.MediaMimeType),
		MediaSizeBytes: nil, // Not provided by WAHA
		IsRead:         messagePayload.FromMe, // Own messages are auto-read
		ReadAt:         nil,
		IsSent:         true,
		SentAt:         sentAt,
		DeliveryStatus: &deliveryStatus,
		Metadata:       &metadataStr,
		CreatedAt:      now,
	}

	if err := s.messageRepo.Create(message); err != nil {
		logger.Error("Failed to create chat message", logger.Err(err))
		return err
	}

	// Update chat with last message info
	chat.LastMessageAt = &sentAt
	previewContent := content
	if len(previewContent) > 100 {
		previewContent = previewContent[:100]
	}
	chat.LastMessagePreview = &previewContent
	lastSender := "customer"
	if messagePayload.FromMe {
		lastSender = "sales"
	}
	chat.LastMessageSender = &lastSender
	chat.MessageCount += 1
	if !messagePayload.FromMe {
		chat.UnreadCount += 1
	}
	chat.UpdatedAt = now

	if err := s.chatRepo.Update(chat); err != nil {
		logger.Error("Failed to update chat", logger.Err(err))
		// Don't fail the whole operation if chat update fails
	}

	logger.Info("WAHA message handled successfully",
		logger.String("message_id", message.ID),
		logger.String("chat_id", chat.ID),
	)

	return nil
}

// UpdateMessageAck updates message acknowledgment status
func (s *ChatService) UpdateMessageAck(wahaMessageID string, ackStatus int) error {
	logger.Info("Updating message ack",
		logger.String("waha_message_id", wahaMessageID),
		logger.Int("ack", ackStatus),
	)

	message, err := s.messageRepo.FindByWAHAMessageID(wahaMessageID)
	if err != nil {
		logger.Error("Failed to find message by WAHA ID", logger.Err(err))
		return err
	}

	if message == nil {
		logger.Warn("Message not found for ack update", logger.String("waha_message_id", wahaMessageID))
		return fmt.Errorf("message not found")
	}

	// Update delivery status
	deliveryStatus := mapWAHAAckToDeliveryStatus(ackStatus)
	message.DeliveryStatus = &deliveryStatus

	// If read, set read_at
	if ackStatus == 3 || ackStatus == 4 {
		now := time.Now()
		message.IsRead = true
		message.ReadAt = &now
	}

	if err := s.messageRepo.Update(message); err != nil {
		logger.Error("Failed to update message ack", logger.Err(err))
		return err
	}

	logger.Info("Message ack updated successfully", logger.String("message_id", message.ID))
	return nil
}

// findOrCreateChat finds existing chat or creates a new one
func (s *ChatService) findOrCreateChat(companyID, salesPersonID, phoneNumber, contactName string) (*domain.Chat, error) {
	// Try to find existing chat (phone, platform, companyID)
	chat, err := s.chatRepo.FindByPhoneAndPlatform(phoneNumber, "whatsapp", companyID)
	if err != nil {
		logger.Error("Failed to find chat", logger.Err(err))
		return nil, err
	}

	if chat != nil {
		// If contact name is provided and different from existing, update it
		if contactName != "" && chat.CustomerName != contactName {
			chat.CustomerName = contactName
			chat.UpdatedAt = time.Now()
			if err := s.chatRepo.Update(chat); err != nil {
				logger.Error("Failed to update contact name", logger.Err(err))
				// Continue anyway, not critical
			} else {
				logger.Info("Updated contact name",
					logger.String("chat_id", chat.ID),
					logger.String("old_name", chat.CustomerName),
					logger.String("new_name", contactName),
				)
			}
		}
		return chat, nil
	}

	// Determine customer name
	customerName := phoneNumber // Default to phone number
	if contactName != "" {
		customerName = contactName // Use contact/group name if provided
	}

	// Create new chat
	now := time.Now()
	chat = &domain.Chat{
		ID:                      uuid.New().String(),
		CompanyID:               companyID,
		LeadID:                  nil, // Can be linked later
		CustomerName:            customerName,
		Phone:                   phoneNumber,
		Email:                   nil,
		AssignedToCompanyUserID: &salesPersonID,
		Platform:                "whatsapp",
		Status:                  "active",
		Category:                nil,
		LastMessageAt:           nil,
		LastMessagePreview:      nil,
		LastMessageSender:       nil,
		UnreadCount:             0,
		MessageCount:            0,
		AIInsights:              nil,
		AvgResponseTimeMinutes:  nil,
		FirstResponseTimeMinutes: nil,
		SentimentScore:          nil,
		Tags:                    nil,
		CreatedAt:               now,
		CreatedBy:               &salesPersonID,
		UpdatedAt:               now,
		UpdatedBy:               &salesPersonID,
	}

	if err := s.chatRepo.Create(chat); err != nil {
		logger.Error("Failed to create chat", logger.Err(err))
		return nil, err
	}

	logger.Info("Created new chat",
		logger.String("chat_id", chat.ID),
		logger.String("phone", phoneNumber),
		logger.String("customer_name", customerName),
		logger.String("contact_name", contactName),
	)
	return chat, nil
}

// MarkChatAsReadByPhone marks all unread messages in a chat as read (when sales person opens chat)
func (s *ChatService) MarkChatAsReadByPhone(companyID, phoneNumber string) error {
	logger.Info("Marking chat as read by phone",
		logger.String("company_id", companyID),
		logger.String("phone", phoneNumber),
	)

	// Find chat by phone and platform
	chat, err := s.chatRepo.FindByPhoneAndPlatform(phoneNumber, "whatsapp", companyID)
	if err != nil {
		logger.Error("Failed to find chat", logger.Err(err))
		return err
	}

	if chat == nil {
		logger.Warn("Chat not found for marking as read",
			logger.String("phone", phoneNumber),
		)
		return nil // Not an error, just no chat to mark
	}

	// If already no unread messages, skip
	if chat.UnreadCount == 0 {
		logger.Debug("Chat already has no unread messages", logger.String("chat_id", chat.ID))
		return nil
	}

	// Reset unread count to 0
	chat.UnreadCount = 0
	chat.UpdatedAt = time.Now()

	if err := s.chatRepo.Update(chat); err != nil {
		logger.Error("Failed to update chat unread count", logger.Err(err))
		return err
	}

	logger.Info("Chat marked as read successfully",
		logger.String("chat_id", chat.ID),
		logger.String("phone", phoneNumber),
	)

	return nil
}

// Helper functions

func mapWAHAMessageType(wahaType string) string {
	switch wahaType {
	case "chat":
		return "text"
	case "image", "video", "document", "audio", "voice":
		return wahaType
	case "location":
		return "location"
	default:
		return "text"
	}
}

func mapWAHAAckToDeliveryStatus(ack int) string {
	switch ack {
	case -1:
		return "failed"
	case 0:
		return "sent"
	case 1:
		return "sent"
	case 2:
		return "delivered"
	case 3, 4:
		return "read"
	default:
		return "sent"
	}
}

func getStringPtr(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}
