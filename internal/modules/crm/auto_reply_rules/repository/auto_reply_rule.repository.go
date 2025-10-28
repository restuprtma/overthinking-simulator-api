package repository

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"venturo-skeleton-go/internal/modules/crm/auto_reply_rules/domain"
)

type AutoReplyRuleRepository struct {
	db *pgxpool.Pool
}

func NewAutoReplyRuleRepository(db *pgxpool.Pool) *AutoReplyRuleRepository {
	return &AutoReplyRuleRepository{db: db}
}

// FindByID retrieves an auto-reply rule by ID and company ID
func (r *AutoReplyRuleRepository) FindByID(id, companyID string) (*domain.AutoReplyRule, error) {
	query := `
		SELECT
			ar.id, ar.company_id, ar.name, ar.description, ar.is_enabled,
			ar.trigger_type, ar.conditions, ar.message_template, ar.delay_seconds, ar.priority,
			ar.usage_count, ar.last_used_at,
			ar.created_at, ar.created_by, ar.updated_at, ar.updated_by, ar.deleted_at, ar.deleted_by,
			uc.full_name as created_by_name, uu.full_name as updated_by_name, ud.full_name as deleted_by_name
		FROM crm.auto_reply_rules ar
		LEFT JOIN core.users uc ON ar.created_by = uc.id::text
		LEFT JOIN core.users uu ON ar.updated_by = uu.id::text
		LEFT JOIN core.users ud ON ar.deleted_by = ud.id::text
		WHERE ar.id = $1 AND ar.company_id = $2 AND ar.deleted_at IS NULL
	`

	var rule domain.AutoReplyRule
	err := r.db.QueryRow(context.Background(), query, id, companyID).Scan(
		&rule.ID, &rule.CompanyID, &rule.Name, &rule.Description, &rule.IsEnabled,
		&rule.TriggerType, &rule.Conditions, &rule.MessageTemplate, &rule.DelaySeconds, &rule.Priority,
		&rule.UsageCount, &rule.LastUsedAt,
		&rule.CreatedAt, &rule.CreatedBy, &rule.UpdatedAt, &rule.UpdatedBy, &rule.DeletedAt, &rule.DeletedBy,
		&rule.CreatedByName, &rule.UpdatedByName, &rule.DeletedByName,
	)

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}

	return &rule, nil
}

// FindAll retrieves all auto-reply rules with pagination and filtering
func (r *AutoReplyRuleRepository) FindAll(companyID string, search string, triggerType *string, isEnabled *bool, limit, offset int) ([]*domain.AutoReplyRule, int, error) {
	// Base query
	baseQuery := `
		FROM crm.auto_reply_rules ar
		LEFT JOIN core.users uc ON ar.created_by = uc.id::text
		LEFT JOIN core.users uu ON ar.updated_by = uu.id::text
		LEFT JOIN core.users ud ON ar.deleted_by = ud.id::text
		WHERE ar.company_id = $1 AND ar.deleted_at IS NULL
	`

	// Build WHERE clause
	whereClause := ""
	args := []interface{}{companyID}
	argCount := 1

	if search != "" {
		argCount++
		whereClause += fmt.Sprintf(" AND (ar.name ILIKE $%d OR ar.description ILIKE $%d OR ar.message_template ILIKE $%d)", argCount, argCount, argCount)
		args = append(args, "%"+search+"%")
	}

	if triggerType != nil {
		argCount++
		whereClause += fmt.Sprintf(" AND ar.trigger_type = $%d", argCount)
		args = append(args, *triggerType)
	}

	if isEnabled != nil {
		argCount++
		whereClause += fmt.Sprintf(" AND ar.is_enabled = $%d", argCount)
		args = append(args, *isEnabled)
	}

	// Count query
	countQuery := "SELECT COUNT(*) " + baseQuery + whereClause
	var total int
	err := r.db.QueryRow(context.Background(), countQuery, args...).Scan(&total)
	if err != nil {
		return nil, 0, err
	}

	// Data query
	dataQuery := `
		SELECT
			ar.id, ar.company_id, ar.name, ar.description, ar.is_enabled,
			ar.trigger_type, ar.conditions, ar.message_template, ar.delay_seconds, ar.priority,
			ar.usage_count, ar.last_used_at,
			ar.created_at, ar.created_by, ar.updated_at, ar.updated_by, ar.deleted_at, ar.deleted_by,
			uc.full_name as created_by_name, uu.full_name as updated_by_name, ud.full_name as deleted_by_name
	` + baseQuery + whereClause + fmt.Sprintf(" ORDER BY ar.priority ASC, ar.created_at DESC LIMIT $%d OFFSET $%d", argCount+1, argCount+2)

	args = append(args, limit, offset)

	rows, err := r.db.Query(context.Background(), dataQuery, args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var rules []*domain.AutoReplyRule
	for rows.Next() {
		var rule domain.AutoReplyRule
		err := rows.Scan(
			&rule.ID, &rule.CompanyID, &rule.Name, &rule.Description, &rule.IsEnabled,
			&rule.TriggerType, &rule.Conditions, &rule.MessageTemplate, &rule.DelaySeconds, &rule.Priority,
			&rule.UsageCount, &rule.LastUsedAt,
			&rule.CreatedAt, &rule.CreatedBy, &rule.UpdatedAt, &rule.UpdatedBy, &rule.DeletedAt, &rule.DeletedBy,
			&rule.CreatedByName, &rule.UpdatedByName, &rule.DeletedByName,
		)
		if err != nil {
			return nil, 0, err
		}
		rules = append(rules, &rule)
	}

	return rules, total, nil
}

// Create creates a new auto-reply rule
func (r *AutoReplyRuleRepository) Create(rule *domain.AutoReplyRule) error {
	query := `
		INSERT INTO crm.auto_reply_rules (
			id, company_id, name, description, is_enabled, trigger_type, conditions,
			message_template, delay_seconds, priority, created_by, updated_by
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)
		RETURNING created_at, updated_at
	`

	err := r.db.QueryRow(context.Background(), query,
		rule.ID, rule.CompanyID, rule.Name, rule.Description, rule.IsEnabled, rule.TriggerType,
		rule.Conditions, rule.MessageTemplate, rule.DelaySeconds, rule.Priority,
		rule.CreatedBy, rule.UpdatedBy,
	).Scan(&rule.CreatedAt, &rule.UpdatedAt)

	return err
}

// Update updates an existing auto-reply rule
func (r *AutoReplyRuleRepository) Update(rule *domain.AutoReplyRule) error {
	query := `
		UPDATE crm.auto_reply_rules
		SET
			name = $1, description = $2, is_enabled = $3, trigger_type = $4, conditions = $5,
			message_template = $6, delay_seconds = $7, priority = $8,
			updated_by = $9, updated_at = CURRENT_TIMESTAMP
		WHERE id = $10 AND company_id = $11 AND deleted_at IS NULL
		RETURNING updated_at
	`

	err := r.db.QueryRow(context.Background(), query,
		rule.Name, rule.Description, rule.IsEnabled, rule.TriggerType, rule.Conditions,
		rule.MessageTemplate, rule.DelaySeconds, rule.Priority,
		rule.UpdatedBy, rule.ID, rule.CompanyID,
	).Scan(&rule.UpdatedAt)

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return errors.New("auto-reply rule not found or already deleted")
		}
		return err
	}

	return nil
}

// Delete soft deletes an auto-reply rule
func (r *AutoReplyRuleRepository) Delete(id, companyID, deletedBy string) error {
	query := `
		UPDATE crm.auto_reply_rules
		SET deleted_at = $1, deleted_by = $2
		WHERE id = $3 AND company_id = $4 AND deleted_at IS NULL
	`

	result, err := r.db.Exec(context.Background(), query, time.Now(), deletedBy, id, companyID)
	if err != nil {
		return err
	}

	if result.RowsAffected() == 0 {
		return errors.New("auto-reply rule not found or already deleted")
	}

	return nil
}

// Restore restores a soft deleted auto-reply rule
func (r *AutoReplyRuleRepository) Restore(id, companyID string) error {
	query := `
		UPDATE crm.auto_reply_rules
		SET deleted_at = NULL, deleted_by = NULL
		WHERE id = $1 AND company_id = $2 AND deleted_at IS NOT NULL
	`

	result, err := r.db.Exec(context.Background(), query, id, companyID)
	if err != nil {
		return err
	}

	if result.RowsAffected() == 0 {
		return errors.New("auto-reply rule not found or not deleted")
	}

	return nil
}

// IncrementUsageCount increments the usage count and updates last_used_at
func (r *AutoReplyRuleRepository) IncrementUsageCount(id string) error {
	query := `
		UPDATE crm.auto_reply_rules
		SET usage_count = usage_count + 1, last_used_at = CURRENT_TIMESTAMP
		WHERE id = $1
	`

	_, err := r.db.Exec(context.Background(), query, id)
	return err
}

// FindEnabledByTriggerType retrieves all enabled rules for a specific trigger type
func (r *AutoReplyRuleRepository) FindEnabledByTriggerType(companyID, triggerType string) ([]*domain.AutoReplyRule, error) {
	query := `
		SELECT
			ar.id, ar.company_id, ar.name, ar.description, ar.is_enabled,
			ar.trigger_type, ar.conditions, ar.message_template, ar.delay_seconds, ar.priority,
			ar.usage_count, ar.last_used_at,
			ar.created_at, ar.created_by, ar.updated_at, ar.updated_by, ar.deleted_at, ar.deleted_by,
			uc.full_name as created_by_name, uu.full_name as updated_by_name, ud.full_name as deleted_by_name
		FROM crm.auto_reply_rules ar
		LEFT JOIN core.users uc ON ar.created_by = uc.id::text
		LEFT JOIN core.users uu ON ar.updated_by = uu.id::text
		LEFT JOIN core.users ud ON ar.deleted_by = ud.id::text
		WHERE ar.company_id = $1 AND ar.trigger_type = $2 AND ar.is_enabled = TRUE AND ar.deleted_at IS NULL
		ORDER BY ar.priority ASC
	`

	rows, err := r.db.Query(context.Background(), query, companyID, triggerType)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var rules []*domain.AutoReplyRule
	for rows.Next() {
		var rule domain.AutoReplyRule
		err := rows.Scan(
			&rule.ID, &rule.CompanyID, &rule.Name, &rule.Description, &rule.IsEnabled,
			&rule.TriggerType, &rule.Conditions, &rule.MessageTemplate, &rule.DelaySeconds, &rule.Priority,
			&rule.UsageCount, &rule.LastUsedAt,
			&rule.CreatedAt, &rule.CreatedBy, &rule.UpdatedAt, &rule.UpdatedBy, &rule.DeletedAt, &rule.DeletedBy,
			&rule.CreatedByName, &rule.UpdatedByName, &rule.DeletedByName,
		)
		if err != nil {
			return nil, err
		}
		rules = append(rules, &rule)
	}

	return rules, nil
}
