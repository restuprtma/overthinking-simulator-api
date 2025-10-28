package repository

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"lakukan-be/internal/modules/crm/company_settings/domain"
)

type CompanySettingsRepository struct {
	db *pgxpool.Pool
}

func NewCompanySettingsRepository(db *pgxpool.Pool) *CompanySettingsRepository {
	return &CompanySettingsRepository{db: db}
}

// FindByCompanyID retrieves company settings by company ID
func (r *CompanySettingsRepository) FindByCompanyID(companyID string) (*domain.CompanySettings, error) {
	query := `
		SELECT
			cs.id, cs.company_id,
			cs.auto_reply_enabled, cs.auto_reply_global_delay_seconds,
			cs.auto_reply_business_hours_start, cs.auto_reply_business_hours_end,
			cs.auto_reply_working_days,
			cs.chat_auto_assign_enabled, cs.chat_assignment_strategy, cs.chat_idle_timeout_minutes,
			cs.lead_auto_categorization_enabled, cs.lead_hot_criteria, cs.lead_warm_criteria, cs.lead_cold_criteria,
			cs.ai_enabled, cs.ai_confidence_threshold, cs.ai_training_mode,
			cs.notifications_enabled, cs.notification_channels,
			cs.timezone,
			cs.created_at, cs.created_by, cs.updated_at, cs.updated_by,
			uc.full_name as created_by_name, uu.full_name as updated_by_name
		FROM crm.company_settings cs
		LEFT JOIN core.users uc ON cs.created_by = uc.id::text
		LEFT JOIN core.users uu ON cs.updated_by = uu.id::text
		WHERE cs.company_id = $1
	`

	var settings domain.CompanySettings
	err := r.db.QueryRow(context.Background(), query, companyID).Scan(
		&settings.ID, &settings.CompanyID,
		&settings.AutoReplyEnabled, &settings.AutoReplyGlobalDelaySeconds,
		&settings.AutoReplyBusinessHoursStart, &settings.AutoReplyBusinessHoursEnd,
		&settings.AutoReplyWorkingDays,
		&settings.ChatAutoAssignEnabled, &settings.ChatAssignmentStrategy, &settings.ChatIdleTimeoutMinutes,
		&settings.LeadAutoCategorization, &settings.LeadHotCriteria, &settings.LeadWarmCriteria, &settings.LeadColdCriteria,
		&settings.AIEnabled, &settings.AIConfidenceThreshold, &settings.AITrainingMode,
		&settings.NotificationsEnabled, &settings.NotificationChannels,
		&settings.Timezone,
		&settings.CreatedAt, &settings.CreatedBy, &settings.UpdatedAt, &settings.UpdatedBy,
		&settings.CreatedByName, &settings.UpdatedByName,
	)

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}

	return &settings, nil
}

// Create creates default company settings
func (r *CompanySettingsRepository) Create(settings *domain.CompanySettings) error {
	query := `
		INSERT INTO crm.company_settings (
			id, company_id,
			auto_reply_enabled, auto_reply_global_delay_seconds,
			auto_reply_business_hours_start, auto_reply_business_hours_end,
			auto_reply_working_days,
			chat_auto_assign_enabled, chat_assignment_strategy, chat_idle_timeout_minutes,
			lead_auto_categorization_enabled, lead_hot_criteria, lead_warm_criteria, lead_cold_criteria,
			ai_enabled, ai_confidence_threshold, ai_training_mode,
			notifications_enabled, notification_channels,
			timezone,
			created_by, updated_by
		) VALUES (
			$1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17, $18, $19, $20, $21, $22
		)
		RETURNING created_at, updated_at
	`

	err := r.db.QueryRow(context.Background(), query,
		settings.ID, settings.CompanyID,
		settings.AutoReplyEnabled, settings.AutoReplyGlobalDelaySeconds,
		settings.AutoReplyBusinessHoursStart, settings.AutoReplyBusinessHoursEnd,
		settings.AutoReplyWorkingDays,
		settings.ChatAutoAssignEnabled, settings.ChatAssignmentStrategy, settings.ChatIdleTimeoutMinutes,
		settings.LeadAutoCategorization, settings.LeadHotCriteria, settings.LeadWarmCriteria, settings.LeadColdCriteria,
		settings.AIEnabled, settings.AIConfidenceThreshold, settings.AITrainingMode,
		settings.NotificationsEnabled, settings.NotificationChannels,
		settings.Timezone,
		settings.CreatedBy, settings.UpdatedBy,
	).Scan(&settings.CreatedAt, &settings.UpdatedAt)

	return err
}

// Update updates company settings
func (r *CompanySettingsRepository) Update(settings *domain.CompanySettings) error {
	query := `
		UPDATE crm.company_settings
		SET
			auto_reply_enabled = $1,
			auto_reply_global_delay_seconds = $2,
			auto_reply_business_hours_start = $3,
			auto_reply_business_hours_end = $4,
			auto_reply_working_days = $5,
			chat_auto_assign_enabled = $6,
			chat_assignment_strategy = $7,
			chat_idle_timeout_minutes = $8,
			lead_auto_categorization_enabled = $9,
			lead_hot_criteria = $10,
			lead_warm_criteria = $11,
			lead_cold_criteria = $12,
			ai_enabled = $13,
			ai_confidence_threshold = $14,
			ai_training_mode = $15,
			notifications_enabled = $16,
			notification_channels = $17,
			timezone = $18,
			updated_by = $19,
			updated_at = CURRENT_TIMESTAMP
		WHERE company_id = $20
		RETURNING updated_at
	`

	err := r.db.QueryRow(context.Background(), query,
		settings.AutoReplyEnabled,
		settings.AutoReplyGlobalDelaySeconds,
		settings.AutoReplyBusinessHoursStart,
		settings.AutoReplyBusinessHoursEnd,
		settings.AutoReplyWorkingDays,
		settings.ChatAutoAssignEnabled,
		settings.ChatAssignmentStrategy,
		settings.ChatIdleTimeoutMinutes,
		settings.LeadAutoCategorization,
		settings.LeadHotCriteria,
		settings.LeadWarmCriteria,
		settings.LeadColdCriteria,
		settings.AIEnabled,
		settings.AIConfidenceThreshold,
		settings.AITrainingMode,
		settings.NotificationsEnabled,
		settings.NotificationChannels,
		settings.Timezone,
		settings.UpdatedBy,
		settings.CompanyID,
	).Scan(&settings.UpdatedAt)

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return errors.New("company settings not found")
		}
		return err
	}

	return nil
}
