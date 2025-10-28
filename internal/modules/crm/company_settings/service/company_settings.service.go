package service

import (
	"encoding/json"
	"errors"

	"github.com/google/uuid"
	"lakukan-be/internal/modules/crm/company_settings/domain"
	"lakukan-be/internal/modules/crm/company_settings/dto"
	"lakukan-be/internal/modules/crm/company_settings/repository"
	"lakukan-be/pkg/logger"
)

var (
	ErrCompanySettingsNotFound = errors.New("company settings not found")
)

type CompanySettingsService struct {
	repo *repository.CompanySettingsRepository
}

func NewCompanySettingsService(repo *repository.CompanySettingsRepository) *CompanySettingsService {
	return &CompanySettingsService{repo: repo}
}

// Get retrieves company settings, creates default if not exists
func (s *CompanySettingsService) Get(companyID string) (*dto.CompanySettingsResponse, error) {
	logger.Info("Fetching company settings", logger.String("company_id", companyID))

	settings, err := s.repo.FindByCompanyID(companyID)
	if err != nil {
		logger.Error("Failed to fetch company settings", logger.Err(err))
		return nil, err
	}

	// If not found, create default settings
	if settings == nil {
		logger.Info("Company settings not found, creating default", logger.String("company_id", companyID))
		settings, err = s.createDefaultSettings(companyID)
		if err != nil {
			logger.Error("Failed to create default company settings", logger.Err(err))
			return nil, err
		}
	}

	response := s.toResponse(settings)
	return &response, nil
}

// Update updates company settings
func (s *CompanySettingsService) Update(companyID, userID string, req dto.UpdateCompanySettingsRequest) (*dto.CompanySettingsResponse, error) {
	logger.Info("Updating company settings", logger.String("company_id", companyID))

	// Get existing settings
	existing, err := s.repo.FindByCompanyID(companyID)
	if err != nil {
		logger.Error("Failed to fetch company settings", logger.Err(err))
		return nil, err
	}

	// If not found, create default first
	if existing == nil {
		existing, err = s.createDefaultSettings(companyID)
		if err != nil {
			logger.Error("Failed to create default company settings", logger.Err(err))
			return nil, err
		}
	}

	// Update fields if provided
	if req.AutoReplyEnabled != nil {
		existing.AutoReplyEnabled = *req.AutoReplyEnabled
	}
	if req.AutoReplyGlobalDelaySeconds != nil {
		existing.AutoReplyGlobalDelaySeconds = *req.AutoReplyGlobalDelaySeconds
	}
	if req.AutoReplyBusinessHoursStart != nil {
		existing.AutoReplyBusinessHoursStart = *req.AutoReplyBusinessHoursStart
	}
	if req.AutoReplyBusinessHoursEnd != nil {
		existing.AutoReplyBusinessHoursEnd = *req.AutoReplyBusinessHoursEnd
	}
	if req.AutoReplyWorkingDays != nil {
		existing.AutoReplyWorkingDays = req.AutoReplyWorkingDays
	}
	if req.ChatAutoAssignEnabled != nil {
		existing.ChatAutoAssignEnabled = *req.ChatAutoAssignEnabled
	}
	if req.ChatAssignmentStrategy != nil {
		existing.ChatAssignmentStrategy = *req.ChatAssignmentStrategy
	}
	if req.ChatIdleTimeoutMinutes != nil {
		existing.ChatIdleTimeoutMinutes = *req.ChatIdleTimeoutMinutes
	}
	if req.LeadAutoCategorization != nil {
		existing.LeadAutoCategorization = *req.LeadAutoCategorization
	}
	if req.LeadHotCriteria != nil {
		jsonBytes, _ := json.Marshal(req.LeadHotCriteria)
		jsonStr := string(jsonBytes)
		existing.LeadHotCriteria = &jsonStr
	}
	if req.LeadWarmCriteria != nil {
		jsonBytes, _ := json.Marshal(req.LeadWarmCriteria)
		jsonStr := string(jsonBytes)
		existing.LeadWarmCriteria = &jsonStr
	}
	if req.LeadColdCriteria != nil {
		jsonBytes, _ := json.Marshal(req.LeadColdCriteria)
		jsonStr := string(jsonBytes)
		existing.LeadColdCriteria = &jsonStr
	}
	if req.AIEnabled != nil {
		existing.AIEnabled = *req.AIEnabled
	}
	if req.AIConfidenceThreshold != nil {
		existing.AIConfidenceThreshold = *req.AIConfidenceThreshold
	}
	if req.AITrainingMode != nil {
		existing.AITrainingMode = *req.AITrainingMode
	}
	if req.NotificationsEnabled != nil {
		existing.NotificationsEnabled = *req.NotificationsEnabled
	}
	if req.NotificationChannels != nil {
		jsonBytes, _ := json.Marshal(req.NotificationChannels)
		jsonStr := string(jsonBytes)
		existing.NotificationChannels = &jsonStr
	}
	if req.Timezone != nil {
		existing.Timezone = *req.Timezone
	}

	existing.UpdatedBy = &userID

	// Save to database
	if err := s.repo.Update(existing); err != nil {
		logger.Error("Failed to update company settings", logger.Err(err))
		return nil, err
	}

	logger.Info("Successfully updated company settings", logger.String("company_id", companyID))

	// Fetch updated settings
	return s.Get(companyID)
}

// createDefaultSettings creates default settings for a company
func (s *CompanySettingsService) createDefaultSettings(companyID string) (*domain.CompanySettings, error) {
	workingDays := []int{1, 2, 3, 4, 5} // Monday to Friday

	settings := &domain.CompanySettings{
		ID:                            uuid.New().String(),
		CompanyID:                     companyID,
		AutoReplyEnabled:              true,
		AutoReplyGlobalDelaySeconds:   30,
		AutoReplyBusinessHoursStart:   "09:00:00",
		AutoReplyBusinessHoursEnd:     "17:00:00",
		AutoReplyWorkingDays:          workingDays,
		ChatAutoAssignEnabled:         true,
		ChatAssignmentStrategy:        "round_robin",
		ChatIdleTimeoutMinutes:        30,
		LeadAutoCategorization:        true,
		AIEnabled:                     true,
		AIConfidenceThreshold:         0.70,
		AITrainingMode:                false,
		NotificationsEnabled:          true,
		Timezone:                      "Asia/Jakarta",
	}

	if err := s.repo.Create(settings); err != nil {
		return nil, err
	}

	return settings, nil
}

// toResponse converts domain model to response DTO
func (s *CompanySettingsService) toResponse(settings *domain.CompanySettings) dto.CompanySettingsResponse {
	response := dto.CompanySettingsResponse{
		ID:                          settings.ID,
		CompanyID:                   settings.CompanyID,
		AutoReplyEnabled:            settings.AutoReplyEnabled,
		AutoReplyGlobalDelaySeconds: settings.AutoReplyGlobalDelaySeconds,
		AutoReplyBusinessHoursStart: settings.AutoReplyBusinessHoursStart,
		AutoReplyBusinessHoursEnd:   settings.AutoReplyBusinessHoursEnd,
		AutoReplyWorkingDays:        settings.AutoReplyWorkingDays,
		ChatAutoAssignEnabled:       settings.ChatAutoAssignEnabled,
		ChatAssignmentStrategy:      settings.ChatAssignmentStrategy,
		ChatIdleTimeoutMinutes:      settings.ChatIdleTimeoutMinutes,
		LeadAutoCategorization:      settings.LeadAutoCategorization,
		AIEnabled:                   settings.AIEnabled,
		AIConfidenceThreshold:       settings.AIConfidenceThreshold,
		AITrainingMode:              settings.AITrainingMode,
		NotificationsEnabled:        settings.NotificationsEnabled,
		Timezone:                    settings.Timezone,
		CreatedAt:                   settings.CreatedAt,
		CreatedBy:                   settings.CreatedBy,
		CreatedByName:               settings.CreatedByName,
		UpdatedAt:                   settings.UpdatedAt,
		UpdatedBy:                   settings.UpdatedBy,
		UpdatedByName:               settings.UpdatedByName,
	}

	// Unmarshal JSONB fields
	if settings.LeadHotCriteria != nil {
		var criteria interface{}
		if err := json.Unmarshal([]byte(*settings.LeadHotCriteria), &criteria); err == nil {
			response.LeadHotCriteria = criteria
		}
	}
	if settings.LeadWarmCriteria != nil {
		var criteria interface{}
		if err := json.Unmarshal([]byte(*settings.LeadWarmCriteria), &criteria); err == nil {
			response.LeadWarmCriteria = criteria
		}
	}
	if settings.LeadColdCriteria != nil {
		var criteria interface{}
		if err := json.Unmarshal([]byte(*settings.LeadColdCriteria), &criteria); err == nil {
			response.LeadColdCriteria = criteria
		}
	}
	if settings.NotificationChannels != nil {
		var channels interface{}
		if err := json.Unmarshal([]byte(*settings.NotificationChannels), &channels); err == nil {
			response.NotificationChannels = channels
		}
	}

	return response
}
