package service

import (
	"crypto/hmac"
	"crypto/sha512"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"lakukan-be/internal/config"
	"lakukan-be/internal/modules/crm/chats/service"
	salesPersonsRepo "lakukan-be/internal/modules/crm/sales_persons/repository"
	salesPersonsService "lakukan-be/internal/modules/crm/sales_persons/service"
	"lakukan-be/internal/modules/crm/webhooks/dto"
	"lakukan-be/pkg/logger"
	"lakukan-be/pkg/waha"
)

type WAHAWebhookService struct {
	salesPersonRepo    *salesPersonsRepo.SalesPersonRepository
	whatsappSvc        *salesPersonsService.WhatsAppSessionService
	chatService        *service.ChatService
	webhookSecret      string
	wahaClient         *waha.Client
}

func NewWAHAWebhookService(
	salesPersonRepo *salesPersonsRepo.SalesPersonRepository,
	whatsappSvc *salesPersonsService.WhatsAppSessionService,
	chatService *service.ChatService,
	cfg *config.Config,
) *WAHAWebhookService {
	return &WAHAWebhookService{
		salesPersonRepo: salesPersonRepo,
		whatsappSvc:     whatsappSvc,
		chatService:     chatService,
		webhookSecret:   cfg.WAHA.WebhookSecret,
		wahaClient:      waha.NewClient(cfg.WAHA.BaseURL, cfg.WAHA.APIKey),
	}
}

// VerifyHMAC verifies the HMAC signature from WAHA webhook
// WAHA uses HMAC-SHA512 of the raw body (not including timestamp)
// Reference: https://waha.devlike.pro/docs/how-to/security/
func (s *WAHAWebhookService) VerifyHMAC(signature, timestamp, body string) bool {
	if s.webhookSecret == "" {
		logger.Warn("WAHA webhook secret not configured, skipping HMAC verification")
		return true
	}

	if signature == "" {
		logger.Warn("No HMAC signature provided in webhook")
		return false
	}

	// WAHA creates HMAC using SHA-512 of the raw body only
	h := hmac.New(sha512.New, []byte(s.webhookSecret))
	h.Write([]byte(body))
	expectedSignature := hex.EncodeToString(h.Sum(nil))

	isValid := hmac.Equal([]byte(signature), []byte(expectedSignature))

	if !isValid {
		logger.Warn("HMAC signature verification failed",
			logger.String("expected", expectedSignature),
			logger.String("received", signature),
			logger.String("timestamp", timestamp),
			logger.Int("body_length", len(body)),
			logger.String("body_preview", truncateString(body, 200)),
		)
	}

	return isValid
}

// truncateString truncates a string to maxLen characters
func truncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}

// HandleWebhook processes incoming WAHA webhook
func (s *WAHAWebhookService) HandleWebhook(payload dto.WAHAWebhookPayload) error {
	logger.Info("Processing WAHA webhook",
		logger.String("event", payload.Event),
		logger.String("session", payload.Session),
		logger.String("webhook_id", payload.ID),
	)

	// Extract metadata (may be empty for some events like engine.event)
	companyID := payload.Metadata["company_id"]
	salesPersonID := payload.Metadata["sales_person_id"]

	// For events without metadata, try to get sales person from session name
	if (companyID == "" || salesPersonID == "") && payload.Session != "" {
		// Find sales person by WAHA session name
		sp, err := s.salesPersonRepo.FindByWAHASessionName(payload.Session)
		if err == nil && sp != nil {
			salesPersonID = sp.ID
			companyID = sp.CompanyUserID
			logger.Debug("Extracted metadata from session name",
				logger.String("session", payload.Session),
				logger.String("sales_person_id", salesPersonID),
				logger.String("company_id", companyID),
			)
		} else {
			logger.Warn("Failed to find sales person by session name",
				logger.String("session", payload.Session),
				logger.Err(err),
			)
		}
	}

	// Still missing? Log error and skip
	if companyID == "" || salesPersonID == "" {
		logger.Error("Missing metadata in webhook payload and could not extract from session",
			logger.String("company_id", companyID),
			logger.String("sales_person_id", salesPersonID),
			logger.String("session", payload.Session),
		)
		return fmt.Errorf("missing company_id or sales_person_id in webhook metadata")
	}

	// Route to appropriate handler based on event type
	switch payload.Event {
	case "session.status":
		return s.handleSessionStatus(salesPersonID, payload)
	case "engine.event":
		return s.handleEngineEvent(companyID, salesPersonID, payload)
	case "message.any":
		return s.handleMessageAny(companyID, salesPersonID, payload)
	case "message.ack":
		return s.handleMessageAck(companyID, salesPersonID, payload)
	default:
		logger.Warn("Unknown webhook event type", logger.String("event", payload.Event))
		return nil // Not an error, just unhandled event
	}
}

// handleSessionStatus handles session.status events
func (s *WAHAWebhookService) handleSessionStatus(salesPersonID string, payload dto.WAHAWebhookPayload) error {
	logger.Info("Handling session.status event",
		logger.String("sales_person_id", salesPersonID),
	)

	// Parse status from payload
	statusPayload := dto.WAHASessionStatusPayload{}
	payloadBytes, err := json.Marshal(payload.Payload)
	if err != nil {
		logger.Error("Failed to marshal payload", logger.Err(err))
		return err
	}

	if err := json.Unmarshal(payloadBytes, &statusPayload); err != nil {
		logger.Error("Failed to parse session status payload", logger.Err(err))
		return err
	}

	logger.Info("Session status changed",
		logger.String("sales_person_id", salesPersonID),
		logger.String("status", statusPayload.Status),
	)

	// Update sales person connection status
	timestamp := time.Unix(payload.Timestamp/1000, 0)
	if err := s.whatsappSvc.UpdateConnectionStatus(salesPersonID, statusPayload.Status, timestamp); err != nil {
		logger.Error("Failed to update connection status", logger.Err(err))
		return err
	}

	return nil
}

// handleEngineEvent handles engine.event for debugging and special events like read-self
func (s *WAHAWebhookService) handleEngineEvent(companyID, salesPersonID string, payload dto.WAHAWebhookPayload) error {
	logger.Debug("Engine event received",
		logger.String("sales_person_id", salesPersonID),
		logger.String("company_id", companyID),
		logger.String("engine", payload.Engine),
		logger.Any("payload", payload.Payload),
	)

	// Parse payload to check if it's a read-self event
	dataMap, ok := payload.Payload["data"].(map[string]interface{})
	if !ok {
		return nil
	}

	eventType, _ := dataMap["Type"].(string)

	// Handle read-self event (when sales person reads messages)
	if eventType == "read-self" {
		chatID, _ := dataMap["Chat"].(string)
		if chatID == "" {
			return nil
		}

		// Extract phone number from Chat ID
		phoneNumber := extractPhoneNumber(chatID)
		if phoneNumber == "" {
			return nil
		}

		logger.Info("Read-self event detected, marking chat as read",
			logger.String("phone", phoneNumber),
			logger.String("chat_id", chatID),
			logger.String("company_id", companyID),
		)

		// Mark chat as read using the company ID from metadata
		if err := s.chatService.MarkChatAsReadByPhone(companyID, phoneNumber); err != nil {
			logger.Error("Failed to mark chat as read", logger.Err(err))
			// Don't return error - not critical
		} else {
			logger.Info("Chat marked as read successfully",
				logger.String("phone", phoneNumber),
				logger.String("company_id", companyID),
			)
		}
	}

	return nil
}

// handleMessageAny handles message.any events (incoming and outgoing messages)
func (s *WAHAWebhookService) handleMessageAny(companyID, salesPersonID string, payload dto.WAHAWebhookPayload) error {
	logger.Info("Handling message.any event",
		logger.String("company_id", companyID),
		logger.String("sales_person_id", salesPersonID),
	)

	// Parse message from payload
	messagePayload := dto.WAHAMessagePayload{}
	payloadBytes, err := json.Marshal(payload.Payload)
	if err != nil {
		logger.Error("Failed to marshal payload", logger.Err(err))
		return err
	}

	if err := json.Unmarshal(payloadBytes, &messagePayload); err != nil {
		logger.Error("Failed to parse message payload", logger.Err(err))
		return err
	}

	// Extract phone number from WhatsApp ID (format: 628123456789@c.us or GROUP_ID@g.us)
	var phoneNumber string
	var isGroup bool

	// Try to extract from "From" first (for incoming), then "To" (for outgoing)
	// Some webhooks may not have "To" field for incoming messages
	if messagePayload.From != "" {
		phoneNumber = extractPhoneNumber(messagePayload.From)
		isGroup = isGroupChat(messagePayload.From)

		if messagePayload.FromMe {
			logger.Info("Outgoing message detected (using From field)",
				logger.String("from", messagePayload.From),
				logger.String("to", messagePayload.To),
				logger.String("extracted_phone", phoneNumber),
				logger.Bool("is_group", isGroup),
			)
		} else {
			logger.Info("Incoming message detected",
				logger.String("from", messagePayload.From),
				logger.String("extracted_phone", phoneNumber),
				logger.Bool("is_group", isGroup),
			)
		}
	} else if messagePayload.To != "" {
		// Fallback to "To" field if "From" is empty
		phoneNumber = extractPhoneNumber(messagePayload.To)
		isGroup = isGroupChat(messagePayload.To)
		logger.Info("Message detected (using To field)",
			logger.String("to", messagePayload.To),
			logger.String("extracted_phone", phoneNumber),
			logger.Bool("is_group", isGroup),
			logger.Bool("from_me", messagePayload.FromMe),
		)
	}

	// Get the session name from the payload (needed for both group and contact lookup)
	sessionName := payload.Session
	if sessionName == "" {
		// Fallback: try to find session from sales person
		sp, err := s.salesPersonRepo.FindByID(salesPersonID, "")
		if err == nil && sp != nil && sp.WAHASessionName != nil {
			sessionName = *sp.WAHASessionName
		}
	}

	// Get contact/group name from WAHA API
	var contactName string
	if isGroup {
		// For group messages, get group name from WAHA API
		// Use the full Chat ID with @g.us suffix (e.g., "120363420566639944@g.us")
		groupID := messagePayload.From
		if groupID == "" {
			groupID = messagePayload.ChatID
		}

		logger.Info("Group message detected, fetching group info",
			logger.String("chat_id", messagePayload.ChatID),
			logger.String("from", messagePayload.From),
			logger.String("group_id", groupID),
		)

		if sessionName != "" && groupID != "" {
			// Fetch group information from WAHA API (pass full ID with @g.us)
			groupInfo, err := s.wahaClient.GetGroup(sessionName, groupID)
			if err != nil {
				logger.Warn("Failed to fetch group info from WAHA",
					logger.Err(err),
					logger.String("session", sessionName),
					logger.String("group_id", groupID),
				)
				// Continue processing even if we can't get group name
				contactName = fmt.Sprintf("Group %s", phoneNumber)
			} else {
				contactName = groupInfo.Name
				logger.Info("Group info retrieved",
					logger.String("group_id", groupID),
					logger.String("group_name", contactName),
				)
			}
		} else {
			logger.Warn("Session name or group ID not found, using default group name",
				logger.String("group_id", groupID),
			)
			contactName = fmt.Sprintf("Group %s", phoneNumber)
		}
	} else {
		// For individual messages, get contact name from WAHA API
		logger.Info("Individual message detected, fetching contact info",
			logger.String("chat_id", messagePayload.ChatID),
			logger.String("from", messagePayload.From),
			logger.String("contact_id", phoneNumber+"@c.us"),
		)

		if sessionName != "" {
			// Fetch contact information from WAHA API
			contactID := phoneNumber + "@c.us"
			contactInfo, err := s.wahaClient.GetContact(sessionName, contactID)
			if err != nil {
				logger.Warn("Failed to fetch contact info from WAHA",
					logger.Err(err),
					logger.String("session", sessionName),
					logger.String("contact_id", contactID),
				)
				// Continue with phone number as fallback
				contactName = phoneNumber
			} else {
				// Use name priority: Name > PushName > ShortName > Number
				if contactInfo.Name != "" {
					contactName = contactInfo.Name
				} else if contactInfo.PushName != "" {
					contactName = contactInfo.PushName
				} else if contactInfo.ShortName != "" {
					contactName = contactInfo.ShortName
				} else {
					contactName = phoneNumber
				}
				logger.Info("Contact info retrieved",
					logger.String("contact_id", contactID),
					logger.String("contact_name", contactName),
					logger.String("pushname", contactInfo.PushName),
				)
			}
		} else {
			logger.Warn("Session name not found, using phone number",
				logger.String("phone", phoneNumber),
			)
			contactName = phoneNumber
		}
	}

	if phoneNumber == "" {
		logger.Error("Failed to extract phone number from message",
			logger.String("from", messagePayload.From),
			logger.String("to", messagePayload.To),
			logger.String("chat_id", messagePayload.ChatID),
			logger.Bool("from_me", messagePayload.FromMe),
		)
		return fmt.Errorf("failed to extract phone number")
	}

	logger.Info("Processing message",
		logger.String("phone", phoneNumber),
		logger.Bool("from_me", messagePayload.FromMe),
		logger.String("type", messagePayload.Type),
		logger.String("message_id", messagePayload.ID),
		logger.Bool("is_group", isGroup),
		logger.String("contact_name", contactName),
	)

	// Delegate to chat service to handle message storage (pass contact/group name)
	if err := s.chatService.HandleWAHAMessage(companyID, salesPersonID, phoneNumber, messagePayload, payload, contactName); err != nil {
		logger.Error("Failed to handle WAHA message", logger.Err(err))
		return err
	}

	return nil
}

// handleMessageAck handles message.ack events (delivery status updates)
func (s *WAHAWebhookService) handleMessageAck(companyID, salesPersonID string, payload dto.WAHAWebhookPayload) error {
	logger.Info("Handling message.ack event",
		logger.String("company_id", companyID),
		logger.String("sales_person_id", salesPersonID),
	)

	// Parse ack from payload
	ackPayload := dto.WAHAMessageAckPayload{}
	payloadBytes, err := json.Marshal(payload.Payload)
	if err != nil {
		logger.Error("Failed to marshal payload", logger.Err(err))
		return err
	}

	if err := json.Unmarshal(payloadBytes, &ackPayload); err != nil {
		logger.Error("Failed to parse message ack payload", logger.Err(err))
		return err
	}

	// Check if this is a group message ACK by checking message ID format
	// Group message IDs contain @g.us
	// Example: true_120363420566639944@g.us_3EB034AC801FEAE6257293_6285172417240@c.us
	if strings.Contains(ackPayload.ID, "@g.us") {
		logger.Debug("Skipping ACK for group message",
			logger.String("message_id", ackPayload.ID),
		)
		return nil
	}

	logger.Info("Message ack received",
		logger.String("message_id", ackPayload.ID),
		logger.Int("ack", ackPayload.ACK),
	)

	// Update message delivery status
	if err := s.chatService.UpdateMessageAck(ackPayload.ID, ackPayload.ACK); err != nil {
		logger.Debug("Failed to update message ack (message may not exist)",
			logger.Err(err),
			logger.String("message_id", ackPayload.ID),
		)
		// Don't return error - ack updates are not critical
		// Message might not exist if it was skipped (e.g., old group messages before we added skip logic)
	}

	return nil
}

// extractPhoneNumber extracts phone number from WhatsApp ID (format: 628123456789@c.us)
func extractPhoneNumber(waID string) string {
	// Remove @c.us, @g.us, or @s.whatsapp.net suffix
	parts := strings.Split(waID, "@")
	if len(parts) > 0 {
		return parts[0]
	}
	return waID
}

// isGroupChat checks if WhatsApp ID is a group chat
func isGroupChat(waID string) bool {
	// Group chats have @g.us suffix
	// Example: 120363048670648481@g.us or GROUPID_TIMESTAMP@g.us
	return strings.HasSuffix(waID, "@g.us")
}
