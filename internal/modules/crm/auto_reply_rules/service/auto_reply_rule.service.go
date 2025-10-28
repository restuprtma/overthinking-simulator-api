package service

import (
	"encoding/json"
	"errors"
	"math"

	"github.com/google/uuid"
	"lakukan-be/internal/modules/crm/auto_reply_rules/domain"
	"lakukan-be/internal/modules/crm/auto_reply_rules/dto"
	"lakukan-be/internal/modules/crm/auto_reply_rules/repository"
	"lakukan-be/pkg/logger"
)

var (
	ErrAutoReplyRuleNotFound = errors.New("auto-reply rule not found")
)

type AutoReplyRuleService struct {
	repo *repository.AutoReplyRuleRepository
}

func NewAutoReplyRuleService(repo *repository.AutoReplyRuleRepository) *AutoReplyRuleService {
	return &AutoReplyRuleService{repo: repo}
}

// GetAll returns paginated list of auto-reply rules
func (s *AutoReplyRuleService) GetAll(companyID string, params dto.AutoReplyRuleQueryParams) ([]dto.AutoReplyRuleResponse, int, int, error) {
	logger.Info("Fetching auto-reply rules",
		logger.String("company_id", companyID),
		logger.Int("page", params.Page),
		logger.Int("page_size", params.PageSize),
	)

	// Calculate offset
	offset := (params.Page - 1) * params.PageSize

	// Get rules with total count
	rules, total, err := s.repo.FindAll(companyID, params.Search, params.TriggerType, params.IsEnabled, params.PageSize, offset)
	if err != nil {
		logger.Error("Failed to fetch auto-reply rules", logger.Err(err))
		return nil, 0, 0, err
	}

	// Convert to response DTOs
	responses := make([]dto.AutoReplyRuleResponse, len(rules))
	for i, rule := range rules {
		responses[i] = s.toResponse(rule)
	}

	// Calculate total pages
	totalPages := int(math.Ceil(float64(total) / float64(params.PageSize)))

	logger.Info("Successfully fetched auto-reply rules",
		logger.Int("total", total),
		logger.Int("count", len(responses)),
	)

	return responses, total, totalPages, nil
}

// GetByID returns a single auto-reply rule by ID
func (s *AutoReplyRuleService) GetByID(id, companyID string) (*dto.AutoReplyRuleResponse, error) {
	logger.Info("Fetching auto-reply rule by ID",
		logger.String("id", id),
		logger.String("company_id", companyID),
	)

	rule, err := s.repo.FindByID(id, companyID)
	if err != nil {
		logger.Error("Failed to fetch auto-reply rule", logger.Err(err))
		return nil, err
	}

	if rule == nil {
		return nil, ErrAutoReplyRuleNotFound
	}

	response := s.toResponse(rule)
	return &response, nil
}

// Create creates a new auto-reply rule
func (s *AutoReplyRuleService) Create(companyID, userID string, req dto.CreateAutoReplyRuleRequest) (*dto.AutoReplyRuleResponse, error) {
	logger.Info("Creating auto-reply rule",
		logger.String("company_id", companyID),
		logger.String("name", req.Name),
	)

	// Generate UUID
	id := uuid.New().String()

	// Marshal conditions to JSON string
	var conditionsJSON *string
	if req.Conditions != nil {
		conditionsBytes, err := json.Marshal(req.Conditions)
		if err != nil {
			logger.Error("Failed to marshal conditions", logger.Err(err))
			return nil, err
		}
		conditionsStr := string(conditionsBytes)
		conditionsJSON = &conditionsStr
	}

	// Set defaults
	isEnabled := true
	if req.IsEnabled != nil {
		isEnabled = *req.IsEnabled
	}

	delaySeconds := 30
	if req.DelaySeconds != nil {
		delaySeconds = *req.DelaySeconds
	}

	priority := 1
	if req.Priority != nil {
		priority = *req.Priority
	}

	// Create domain object
	rule := &domain.AutoReplyRule{
		ID:              id,
		CompanyID:       companyID,
		Name:            req.Name,
		Description:     req.Description,
		IsEnabled:       isEnabled,
		TriggerType:     req.TriggerType,
		Conditions:      conditionsJSON,
		MessageTemplate: req.MessageTemplate,
		DelaySeconds:    delaySeconds,
		Priority:        priority,
		UsageCount:      0,
		CreatedBy:       &userID,
		UpdatedBy:       &userID,
	}

	// Save to database
	if err := s.repo.Create(rule); err != nil {
		logger.Error("Failed to create auto-reply rule", logger.Err(err))
		return nil, err
	}

	logger.Info("Successfully created auto-reply rule", logger.String("id", id))

	// Fetch the created rule to get complete data
	return s.GetByID(id, companyID)
}

// Update updates an existing auto-reply rule
func (s *AutoReplyRuleService) Update(id, companyID, userID string, req dto.UpdateAutoReplyRuleRequest) (*dto.AutoReplyRuleResponse, error) {
	logger.Info("Updating auto-reply rule",
		logger.String("id", id),
		logger.String("company_id", companyID),
	)

	// Get existing rule
	existing, err := s.repo.FindByID(id, companyID)
	if err != nil {
		logger.Error("Failed to fetch auto-reply rule", logger.Err(err))
		return nil, err
	}

	if existing == nil {
		return nil, ErrAutoReplyRuleNotFound
	}

	// Update fields
	if req.Name != nil {
		existing.Name = *req.Name
	}
	if req.Description != nil {
		existing.Description = req.Description
	}
	if req.IsEnabled != nil {
		existing.IsEnabled = *req.IsEnabled
	}
	if req.TriggerType != nil {
		existing.TriggerType = *req.TriggerType
	}
	if req.Conditions != nil {
		conditionsBytes, err := json.Marshal(req.Conditions)
		if err != nil {
			logger.Error("Failed to marshal conditions", logger.Err(err))
			return nil, err
		}
		conditionsStr := string(conditionsBytes)
		existing.Conditions = &conditionsStr
	}
	if req.MessageTemplate != nil {
		existing.MessageTemplate = *req.MessageTemplate
	}
	if req.DelaySeconds != nil {
		existing.DelaySeconds = *req.DelaySeconds
	}
	if req.Priority != nil {
		existing.Priority = *req.Priority
	}

	existing.UpdatedBy = &userID

	// Save to database
	if err := s.repo.Update(existing); err != nil {
		logger.Error("Failed to update auto-reply rule", logger.Err(err))
		return nil, err
	}

	logger.Info("Successfully updated auto-reply rule", logger.String("id", id))

	// Fetch the updated rule
	return s.GetByID(id, companyID)
}

// Delete soft deletes an auto-reply rule
func (s *AutoReplyRuleService) Delete(id, companyID, userID string) error {
	logger.Info("Deleting auto-reply rule",
		logger.String("id", id),
		logger.String("company_id", companyID),
	)

	if err := s.repo.Delete(id, companyID, userID); err != nil {
		logger.Error("Failed to delete auto-reply rule", logger.Err(err))
		return err
	}

	logger.Info("Successfully deleted auto-reply rule", logger.String("id", id))
	return nil
}

// Restore restores a soft deleted auto-reply rule
func (s *AutoReplyRuleService) Restore(id, companyID string) error {
	logger.Info("Restoring auto-reply rule",
		logger.String("id", id),
		logger.String("company_id", companyID),
	)

	if err := s.repo.Restore(id, companyID); err != nil {
		logger.Error("Failed to restore auto-reply rule", logger.Err(err))
		return err
	}

	logger.Info("Successfully restored auto-reply rule", logger.String("id", id))
	return nil
}

// GetEnabledByTriggerType retrieves all enabled rules for a specific trigger type
func (s *AutoReplyRuleService) GetEnabledByTriggerType(companyID, triggerType string) ([]dto.AutoReplyRuleResponse, error) {
	logger.Info("Fetching enabled auto-reply rules by trigger type",
		logger.String("company_id", companyID),
		logger.String("trigger_type", triggerType),
	)

	rules, err := s.repo.FindEnabledByTriggerType(companyID, triggerType)
	if err != nil {
		logger.Error("Failed to fetch enabled auto-reply rules", logger.Err(err))
		return nil, err
	}

	// Convert to response DTOs
	responses := make([]dto.AutoReplyRuleResponse, len(rules))
	for i, rule := range rules {
		responses[i] = s.toResponse(rule)
	}

	return responses, nil
}

// IncrementUsageCount increments the usage count for a rule
func (s *AutoReplyRuleService) IncrementUsageCount(id string) error {
	return s.repo.IncrementUsageCount(id)
}

// toResponse converts domain model to response DTO
func (s *AutoReplyRuleService) toResponse(rule *domain.AutoReplyRule) dto.AutoReplyRuleResponse {
	response := dto.AutoReplyRuleResponse{
		ID:              rule.ID,
		CompanyID:       rule.CompanyID,
		Name:            rule.Name,
		Description:     rule.Description,
		IsEnabled:       rule.IsEnabled,
		TriggerType:     rule.TriggerType,
		MessageTemplate: rule.MessageTemplate,
		DelaySeconds:    rule.DelaySeconds,
		Priority:        rule.Priority,
		UsageCount:      rule.UsageCount,
		LastUsedAt:      rule.LastUsedAt,
	}

	// Unmarshal conditions from JSON string
	if rule.Conditions != nil {
		var conditions interface{}
		if err := json.Unmarshal([]byte(*rule.Conditions), &conditions); err == nil {
			response.Conditions = conditions
		}
	}

	// Audit info
	response.Audit = &dto.AuditInfo{
		CreatedAt:     rule.CreatedAt,
		CreatedBy:     rule.CreatedBy,
		CreatedByName: rule.CreatedByName,
		UpdatedAt:     rule.UpdatedAt,
		UpdatedBy:     rule.UpdatedBy,
		UpdatedByName: rule.UpdatedByName,
		DeletedAt:     rule.DeletedAt,
		DeletedBy:     rule.DeletedBy,
		DeletedByName: rule.DeletedByName,
	}

	return response
}
