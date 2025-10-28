package service

import (
	"errors"
	"fmt"
	"time"

	"venturo-skeleton-go/internal/config"
	"venturo-skeleton-go/internal/modules/crm/sales_persons/dto"
	"venturo-skeleton-go/internal/modules/crm/sales_persons/repository"
	"venturo-skeleton-go/pkg/logger"
	"venturo-skeleton-go/pkg/waha"
)

var (
	ErrWhatsappNumberRequired = errors.New("whatsapp number is required for this sales person")
	ErrWhatsappAlreadyConnected = errors.New("whatsapp is already connected for this sales person")
	ErrWhatsappNotConnected = errors.New("whatsapp is not connected for this sales person")
	ErrSessionNotFound = errors.New("waha session not found")
)

type WhatsAppSessionService struct {
	repo       *repository.SalesPersonRepository
	wahaClient *waha.Client
	config     *config.Config
}

func NewWhatsAppSessionService(repo *repository.SalesPersonRepository, cfg *config.Config) *WhatsAppSessionService {
	return &WhatsAppSessionService{
		repo:       repo,
		wahaClient: waha.NewClient(cfg.WAHA.BaseURL, cfg.WAHA.APIKey),
		config:     cfg,
	}
}

// ConnectWhatsApp creates a WAHA session and returns pairing code
func (s *WhatsAppSessionService) ConnectWhatsApp(salesPersonID, companyID, userID string) (*dto.ConnectWhatsAppResponse, error) {
	logger.Info("Connecting WhatsApp for sales person",
		logger.String("sales_person_id", salesPersonID),
		logger.String("company_id", companyID),
	)

	// Get sales person
	sp, err := s.repo.FindByID(salesPersonID, companyID)
	if err != nil {
		logger.Error("Failed to find sales person", logger.Err(err))
		return nil, err
	}
	if sp == nil {
		return nil, ErrSalesPersonNotFound
	}

	// Check if WhatsApp number exists
	if sp.Whatsapp == nil || *sp.Whatsapp == "" {
		logger.Warn("Sales person does not have WhatsApp number", logger.String("sales_person_id", salesPersonID))
		return nil, ErrWhatsappNumberRequired
	}

	// Check if already connected with WORKING status
	if sp.IsWhatsappConnected && sp.WAHASessionStatus != nil && *sp.WAHASessionStatus == "WORKING" {
		logger.Warn("WhatsApp already connected", logger.String("sales_person_id", salesPersonID))
		return nil, ErrWhatsappAlreadyConnected
	}

	// Generate session name based on WhatsApp number (format: lakukan_62812345678)
	// Remove + prefix if present and use the phone number
	whatsappNumber := *sp.Whatsapp
	if len(whatsappNumber) > 0 && whatsappNumber[0] == '+' {
		whatsappNumber = whatsappNumber[1:]
	}
	sessionName := fmt.Sprintf("lakukan_%s", whatsappNumber)

	// Check if session already exists in WAHA
	var sessionResp *waha.SessionResponse
	var sessionIsNew bool
	existingSession, err := s.wahaClient.GetSession(sessionName)
	if err == nil && existingSession != nil {
		// Session already exists, update it with new metadata and config
		logger.Info("WAHA session already exists, updating metadata",
			logger.String("session_name", sessionName),
			logger.String("status", existingSession.Status),
			logger.String("company_id", companyID),
			logger.String("sales_person_id", salesPersonID),
		)

		updateReq := waha.CreateSessionRequest{
			Name:  sessionName,
			Start: true,
			Config: &waha.SessionConfig{
				Metadata: map[string]string{
					"company_id":      companyID,
					"sales_person_id": salesPersonID,
				},
				Webhooks: []waha.WebhookConfig{
					{
						URL:    s.config.WAHA.WebhookURL,
						Events: []string{"session.status", "engine.event", "message.any", "message.ack"},
						HMAC: &waha.HMACConfig{
							Key: s.config.WAHA.WebhookSecret,
						},
						Retries: &waha.RetryConfig{
							Policy:       "exponential",
							Attempts:     5,
							DelaySeconds: 2,
						},
					},
				},
			},
		}

		sessionResp, err = s.wahaClient.UpdateSession(sessionName, updateReq)
		if err != nil {
			logger.Error("Failed to update WAHA session metadata", logger.Err(err))
			// Don't fail, just use existing session
			sessionResp = existingSession
		} else {
			logger.Info("WAHA session metadata updated successfully",
				logger.String("session_name", sessionName),
			)
		}
		sessionIsNew = false
	} else {
		// Session doesn't exist, create new one
		logger.Info("Creating new WAHA session",
			logger.String("session_name", sessionName),
			logger.String("company_id", companyID),
			logger.String("sales_person_id", salesPersonID),
		)

		createReq := waha.CreateSessionRequest{
			Name:  sessionName,
			Start: true,
			Config: &waha.SessionConfig{
				Metadata: map[string]string{
					"company_id":      companyID,
					"sales_person_id": salesPersonID,
				},
				Webhooks: []waha.WebhookConfig{
					{
						URL:    s.config.WAHA.WebhookURL,
						Events: []string{"session.status", "engine.event", "message.any", "message.ack"},
						HMAC: &waha.HMACConfig{
							Key: s.config.WAHA.WebhookSecret,
						},
						Retries: &waha.RetryConfig{
							Policy:       "exponential",
							Attempts:     5,
							DelaySeconds: 2,
						},
					},
				},
			},
		}

		logger.Info("Sending create session request to WAHA",
			logger.String("session_name", sessionName),
			logger.String("webhook_url", s.config.WAHA.WebhookURL),
			logger.Any("metadata", createReq.Config.Metadata),
		)

		sessionResp, err = s.wahaClient.CreateSession(createReq)
		if err != nil {
			logger.Error("Failed to create WAHA session", logger.Err(err))
			return nil, fmt.Errorf("failed to create WAHA session: %w", err)
		}

		logger.Info("WAHA session created successfully",
			logger.String("session_name", sessionName),
			logger.String("status", sessionResp.Status),
		)
		sessionIsNew = true
	}

	// Request pairing code (use the formatted number without +)
	logger.Info("Requesting pairing code",
		logger.String("session_name", sessionName),
		logger.String("phone_number", whatsappNumber),
		logger.String("original_number", *sp.Whatsapp),
	)

	pairingResp, err := s.wahaClient.RequestPairingCode(sessionName, whatsappNumber)
	if err != nil {
		logger.Error("Failed to request pairing code", logger.Err(err))
		// Only cleanup if we just created the session
		if sessionIsNew {
			_ = s.wahaClient.DeleteSession(sessionName)
		}
		return nil, fmt.Errorf("failed to request pairing code: %w", err)
	}

	logger.Info("Pairing code generated successfully",
		logger.String("session_name", sessionName),
		logger.String("code", pairingResp.Code),
	)

	// Update sales person with WAHA session info
	now := time.Now()
	expiresAt := now.Add(2 * time.Minute) // Pairing codes typically expire in 1-2 minutes

	sp.WAHASessionName = &sessionName
	status := sessionResp.Status
	sp.WAHASessionStatus = &status
	sp.WAHAPairingCode = &pairingResp.Code
	sp.WAHAPairingCodeExpiresAt = &expiresAt
	sp.UpdatedAt = now
	sp.UpdatedBy = &userID

	if err := s.repo.UpdateWAHASession(sp); err != nil {
		logger.Error("Failed to update sales person with WAHA session info", logger.Err(err))
		// Session created but DB update failed - log but don't fail completely
		// The session will still work, just not tracked properly
	}

	return &dto.ConnectWhatsAppResponse{
		SalesPersonID:  salesPersonID,
		SessionName:    sessionName,
		WhatsappNumber: *sp.Whatsapp,
		PairingCode:    pairingResp.Code,
		Status:         sessionResp.Status,
		ExpiresAt:      expiresAt,
	}, nil
}

// GetWhatsAppStatus returns the current WhatsApp connection status
func (s *WhatsAppSessionService) GetWhatsAppStatus(salesPersonID, companyID string) (*dto.WhatsAppStatusResponse, error) {
	logger.Info("Getting WhatsApp status",
		logger.String("sales_person_id", salesPersonID),
		logger.String("company_id", companyID),
	)

	sp, err := s.repo.FindByID(salesPersonID, companyID)
	if err != nil {
		logger.Error("Failed to find sales person", logger.Err(err))
		return nil, err
	}
	if sp == nil {
		return nil, ErrSalesPersonNotFound
	}

	if sp.Whatsapp == nil {
		noPhoneStatus := "NO_PHONE_NUMBER"
		return &dto.WhatsAppStatusResponse{
			IsConnected:    false,
			Status:         &noPhoneStatus,
			WhatsappNumber: "",
		}, nil
	}

	// If there's a session, try to get live status from WAHA
	if sp.WAHASessionName != nil && *sp.WAHASessionName != "" {
		sessionResp, err := s.wahaClient.GetSession(*sp.WAHASessionName)
		if err != nil {
			logger.Warn("Failed to get WAHA session status",
				logger.Err(err),
				logger.String("session_name", *sp.WAHASessionName),
			)

			// If session not found (404) and status is STOPPED, clear the WAHA fields
			// This handles orphaned data where session was deleted externally
			if sp.WAHASessionStatus != nil && *sp.WAHASessionStatus == "STOPPED" {
				logger.Info("Session not found in WAHA but marked as STOPPED, clearing WAHA fields",
					logger.String("sales_person_id", salesPersonID),
				)
				// Clear all WAHA fields
				sp.WAHASessionName = nil
				sp.WAHASessionStatus = nil
				sp.WAHAPairingCode = nil
				sp.WAHAPairingCodeExpiresAt = nil
				sp.WAHALastSeenAt = nil
				sp.WAHAConnectedAt = nil
				sp.WAHADisconnectedAt = nil
				sp.IsWhatsappConnected = false

				// Update in database
				if err := s.repo.UpdateWAHASession(sp); err != nil {
					logger.Error("Failed to clear WAHA fields", logger.Err(err))
				}
			}
			// Fall through to use cached status from DB
		} else {
			// Session exists in WAHA, update status if different from DB
			if sp.WAHASessionStatus == nil || *sp.WAHASessionStatus != sessionResp.Status {
				isConnected := sessionResp.Status == "WORKING"
				now := time.Now()
				if err := s.repo.UpdateWAHAConnectionStatus(sp.ID, sessionResp.Status, isConnected, now); err != nil {
					logger.Error("Failed to update WAHA connection status", logger.Err(err))
				} else {
					sp.WAHASessionStatus = &sessionResp.Status
					sp.IsWhatsappConnected = isConnected
					sp.WAHALastSeenAt = &now
				}
			}
		}
	} else if sp.WAHASessionStatus != nil {
		// Edge case: Has status but no session name (data inconsistency)
		// This shouldn't happen normally, but clean it up if it does
		logger.Warn("Sales person has WAHA status but no session name, clearing status",
			logger.String("sales_person_id", salesPersonID),
			logger.String("status", *sp.WAHASessionStatus),
		)
		sp.WAHASessionStatus = nil
		sp.WAHAPairingCode = nil
		sp.WAHAPairingCodeExpiresAt = nil
		sp.IsWhatsappConnected = false

		if err := s.repo.UpdateWAHASession(sp); err != nil {
			logger.Error("Failed to clear inconsistent WAHA status", logger.Err(err))
		}
	}

	// Build response
	response := &dto.WhatsAppStatusResponse{
		IsConnected:    sp.IsWhatsappConnected,
		Status:         sp.WAHASessionStatus, // Will be nil if never connected
		WhatsappNumber: *sp.Whatsapp,
		SessionName:    sp.WAHASessionName,
		ConnectedAt:    sp.WAHAConnectedAt,
		LastSeenAt:     sp.WAHALastSeenAt,
	}

	return response, nil
}

// DisconnectWhatsApp disconnects and deletes the WAHA session
func (s *WhatsAppSessionService) DisconnectWhatsApp(salesPersonID, companyID, userID string, logout bool) error {
	logger.Info("Disconnecting WhatsApp",
		logger.String("sales_person_id", salesPersonID),
		logger.String("company_id", companyID),
		logger.Bool("logout", logout),
	)

	sp, err := s.repo.FindByID(salesPersonID, companyID)
	if err != nil {
		logger.Error("Failed to find sales person", logger.Err(err))
		return err
	}
	if sp == nil {
		return ErrSalesPersonNotFound
	}

	if sp.WAHASessionName == nil || *sp.WAHASessionName == "" {
		logger.Warn("No WAHA session to disconnect", logger.String("sales_person_id", salesPersonID))
		return ErrWhatsappNotConnected
	}

	sessionName := *sp.WAHASessionName

	// Logout or just stop session
	if logout {
		if err := s.wahaClient.LogoutSession(sessionName); err != nil {
			logger.Error("Failed to logout WAHA session", logger.Err(err))
			// Continue anyway to update DB
		}
	} else {
		if err := s.wahaClient.StopSession(sessionName); err != nil {
			logger.Error("Failed to stop WAHA session", logger.Err(err))
			// Continue anyway to update DB
		}
	}

	// Delete session
	if err := s.wahaClient.DeleteSession(sessionName); err != nil {
		logger.Error("Failed to delete WAHA session", logger.Err(err))
		// Continue anyway to update DB
	}

	// Update sales person
	now := time.Now()
	stoppedStatus := "STOPPED"
	sp.WAHASessionStatus = &stoppedStatus
	sp.IsWhatsappConnected = false
	sp.WAHADisconnectedAt = &now
	sp.WAHAPairingCode = nil
	sp.WAHAPairingCodeExpiresAt = nil
	sp.UpdatedAt = now
	sp.UpdatedBy = &userID

	if logout {
		// Clear session name if logging out
		sp.WAHASessionName = nil
	}

	if err := s.repo.UpdateWAHASession(sp); err != nil {
		logger.Error("Failed to update sales person after disconnect", logger.Err(err))
		return err
	}

	logger.Info("WhatsApp disconnected successfully", logger.String("sales_person_id", salesPersonID))
	return nil
}

// RestartWhatsApp restarts the WAHA session
func (s *WhatsAppSessionService) RestartWhatsApp(salesPersonID, companyID, userID string) error {
	logger.Info("Restarting WhatsApp session",
		logger.String("sales_person_id", salesPersonID),
		logger.String("company_id", companyID),
	)

	sp, err := s.repo.FindByID(salesPersonID, companyID)
	if err != nil {
		logger.Error("Failed to find sales person", logger.Err(err))
		return err
	}
	if sp == nil {
		return ErrSalesPersonNotFound
	}

	if sp.WAHASessionName == nil || *sp.WAHASessionName == "" {
		logger.Warn("No WAHA session to restart", logger.String("sales_person_id", salesPersonID))
		return ErrSessionNotFound
	}

	sessionName := *sp.WAHASessionName

	if err := s.wahaClient.RestartSession(sessionName); err != nil {
		logger.Error("Failed to restart WAHA session", logger.Err(err))
		return fmt.Errorf("failed to restart WAHA session: %w", err)
	}

	// Update status to STARTING
	now := time.Now()
	startingStatus := "STARTING"
	sp.WAHASessionStatus = &startingStatus
	sp.UpdatedAt = now
	sp.UpdatedBy = &userID

	if err := s.repo.UpdateWAHASession(sp); err != nil {
		logger.Error("Failed to update sales person after restart", logger.Err(err))
		return err
	}

	logger.Info("WhatsApp session restarted successfully", logger.String("sales_person_id", salesPersonID))
	return nil
}

// UpdateConnectionStatus updates the connection status from webhook
func (s *WhatsAppSessionService) UpdateConnectionStatus(salesPersonID, status string, timestamp time.Time) error {
	logger.Info("Updating WhatsApp connection status from webhook",
		logger.String("sales_person_id", salesPersonID),
		logger.String("status", status),
	)

	isConnected := status == "WORKING"

	if err := s.repo.UpdateWAHAConnectionStatus(salesPersonID, status, isConnected, timestamp); err != nil {
		logger.Error("Failed to update WAHA connection status", logger.Err(err))
		return err
	}

	logger.Info("WhatsApp connection status updated successfully",
		logger.String("sales_person_id", salesPersonID),
		logger.String("status", status),
		logger.Bool("is_connected", isConnected),
	)

	return nil
}
