package repository

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"venturo-skeleton-go/internal/modules/crm/leads/domain"
)

type LeadRepository struct {
	db *pgxpool.Pool
}

func NewLeadRepository(db *pgxpool.Pool) *LeadRepository {
	return &LeadRepository{db: db}
}

// FindByID retrieves a lead by ID and company ID
func (r *LeadRepository) FindByID(id, companyID string) (*domain.Lead, error) {
	query := `
		SELECT
			l.id, l.company_id, l.lead_number, l.name, l.contact_person, l.phone, l.email,
			l.category, l.status, l.source_id, l.assigned_to_company_user_id, l.deal_value,
			l.last_contact_at, l.next_follow_up_at, l.response_time_minutes, l.ai_highlights,
			l.notes, l.address, l.city, l.province, l.postal_code,
			l.created_at, l.created_by, l.updated_at, l.updated_by, l.deleted_at, l.deleted_by,
			ls.name as source_name,
			u.id as assigned_to_user_id, u.full_name as assigned_to_user_name, u.email as assigned_to_user_email,
			uc.full_name as created_by_name, uu.full_name as updated_by_name, ud.full_name as deleted_by_name
		FROM crm.leads l
		LEFT JOIN crm.lead_sources ls ON l.source_id = ls.id
		LEFT JOIN core.company_users cu ON l.assigned_to_company_user_id = cu.id
		LEFT JOIN core.users u ON cu.user_id = u.id
		LEFT JOIN core.users uc ON l.created_by = uc.id::text
		LEFT JOIN core.users uu ON l.updated_by = uu.id::text
		LEFT JOIN core.users ud ON l.deleted_by = ud.id::text
		WHERE l.id = $1 AND l.company_id = $2 AND l.deleted_at IS NULL
	`

	var lead domain.Lead
	err := r.db.QueryRow(context.Background(), query, id, companyID).Scan(
		&lead.ID, &lead.CompanyID, &lead.LeadNumber, &lead.Name, &lead.ContactPerson, &lead.Phone, &lead.Email,
		&lead.Category, &lead.Status, &lead.SourceID, &lead.AssignedToCompanyUserID, &lead.DealValue,
		&lead.LastContactAt, &lead.NextFollowUpAt, &lead.ResponseTimeMinutes, &lead.AIHighlights,
		&lead.Notes, &lead.Address, &lead.City, &lead.Province, &lead.PostalCode,
		&lead.CreatedAt, &lead.CreatedBy, &lead.UpdatedAt, &lead.UpdatedBy, &lead.DeletedAt, &lead.DeletedBy,
		&lead.SourceName,
		&lead.AssignedToUserID, &lead.AssignedToUserName, &lead.AssignedToUserEmail,
		&lead.CreatedByName, &lead.UpdatedByName, &lead.DeletedByName,
	)

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}

	return &lead, nil
}

// FindByPhone retrieves a lead by phone and company ID
func (r *LeadRepository) FindByPhone(phone, companyID string) (*domain.Lead, error) {
	query := `
		SELECT
			l.id, l.company_id, l.lead_number, l.name, l.contact_person, l.phone, l.email,
			l.category, l.status, l.source_id, l.assigned_to_company_user_id, l.deal_value,
			l.last_contact_at, l.next_follow_up_at, l.response_time_minutes, l.ai_highlights,
			l.notes, l.address, l.city, l.province, l.postal_code,
			l.created_at, l.created_by, l.updated_at, l.updated_by, l.deleted_at, l.deleted_by,
			ls.name as source_name,
			u.id as assigned_to_user_id, u.full_name as assigned_to_user_name, u.email as assigned_to_user_email,
			uc.full_name as created_by_name, uu.full_name as updated_by_name, ud.full_name as deleted_by_name
		FROM crm.leads l
		LEFT JOIN crm.lead_sources ls ON l.source_id = ls.id
		LEFT JOIN core.company_users cu ON l.assigned_to_company_user_id = cu.id
		LEFT JOIN core.users u ON cu.user_id = u.id
		LEFT JOIN core.users uc ON l.created_by = uc.id::text
		LEFT JOIN core.users uu ON l.updated_by = uu.id::text
		LEFT JOIN core.users ud ON l.deleted_by = ud.id::text
		WHERE l.phone = $1 AND l.company_id = $2 AND l.deleted_at IS NULL
	`

	var lead domain.Lead
	err := r.db.QueryRow(context.Background(), query, phone, companyID).Scan(
		&lead.ID, &lead.CompanyID, &lead.LeadNumber, &lead.Name, &lead.ContactPerson, &lead.Phone, &lead.Email,
		&lead.Category, &lead.Status, &lead.SourceID, &lead.AssignedToCompanyUserID, &lead.DealValue,
		&lead.LastContactAt, &lead.NextFollowUpAt, &lead.ResponseTimeMinutes, &lead.AIHighlights,
		&lead.Notes, &lead.Address, &lead.City, &lead.Province, &lead.PostalCode,
		&lead.CreatedAt, &lead.CreatedBy, &lead.UpdatedAt, &lead.UpdatedBy, &lead.DeletedAt, &lead.DeletedBy,
		&lead.SourceName,
		&lead.AssignedToUserID, &lead.AssignedToUserName, &lead.AssignedToUserEmail,
		&lead.CreatedByName, &lead.UpdatedByName, &lead.DeletedByName,
	)

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}

	return &lead, nil
}

// FindAll retrieves all leads with pagination and filters
// Optimized for list view - only joins essential data, no audit names
func (r *LeadRepository) FindAll(companyID string, limit, offset int, search string, category, status, sourceID, assignedToCompanyUserID *string) ([]*domain.Lead, error) {
	query := `
		SELECT
			l.id, l.company_id, l.lead_number, l.name, l.contact_person, l.phone, l.email,
			l.category, l.status, l.source_id, l.assigned_to_company_user_id, l.deal_value,
			l.last_contact_at, l.next_follow_up_at, l.response_time_minutes,
			l.notes, l.address, l.city, l.province, l.postal_code,
			l.created_at, l.created_by, l.updated_at, l.updated_by,
			ls.name as source_name,
			u.id as assigned_to_user_id, u.full_name as assigned_to_user_name, u.email as assigned_to_user_email
		FROM crm.leads l
		LEFT JOIN crm.lead_sources ls ON l.source_id = ls.id
		LEFT JOIN core.company_users cu ON l.assigned_to_company_user_id = cu.id
		LEFT JOIN core.users u ON cu.user_id = u.id
		WHERE l.company_id = $1 AND l.deleted_at IS NULL
	`

	args := []interface{}{companyID}
	argPos := 2

	// Add search filter
	if search != "" {
		query += fmt.Sprintf(` AND (l.name ILIKE $%d OR l.phone ILIKE $%d OR l.email ILIKE $%d OR l.contact_person ILIKE $%d)`, argPos, argPos, argPos, argPos)
		args = append(args, "%"+search+"%")
		argPos++
	}

	// Add category filter
	if category != nil && *category != "" {
		query += fmt.Sprintf(` AND l.category = $%d`, argPos)
		args = append(args, *category)
		argPos++
	}

	// Add status filter
	if status != nil && *status != "" {
		query += fmt.Sprintf(` AND l.status = $%d`, argPos)
		args = append(args, *status)
		argPos++
	}

	// Add source_id filter
	if sourceID != nil && *sourceID != "" {
		query += fmt.Sprintf(` AND l.source_id = $%d`, argPos)
		args = append(args, *sourceID)
		argPos++
	}

	// Add assigned_to_company_user_id filter
	if assignedToCompanyUserID != nil && *assignedToCompanyUserID != "" {
		query += fmt.Sprintf(` AND l.assigned_to_company_user_id = $%d`, argPos)
		args = append(args, *assignedToCompanyUserID)
		argPos++
	}

	query += ` ORDER BY l.created_at DESC`
	query += fmt.Sprintf(` LIMIT $%d OFFSET $%d`, argPos, argPos+1)
	args = append(args, limit, offset)

	rows, err := r.db.Query(context.Background(), query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var leads []*domain.Lead
	for rows.Next() {
		var lead domain.Lead
		err := rows.Scan(
			&lead.ID, &lead.CompanyID, &lead.LeadNumber, &lead.Name, &lead.ContactPerson, &lead.Phone, &lead.Email,
			&lead.Category, &lead.Status, &lead.SourceID, &lead.AssignedToCompanyUserID, &lead.DealValue,
			&lead.LastContactAt, &lead.NextFollowUpAt, &lead.ResponseTimeMinutes,
			&lead.Notes, &lead.Address, &lead.City, &lead.Province, &lead.PostalCode,
			&lead.CreatedAt, &lead.CreatedBy, &lead.UpdatedAt, &lead.UpdatedBy,
			&lead.SourceName,
			&lead.AssignedToUserID, &lead.AssignedToUserName, &lead.AssignedToUserEmail,
		)
		if err != nil {
			return nil, err
		}
		// Audit names are not loaded in list view for performance
		// Use FindByID if you need full audit trail information
		leads = append(leads, &lead)
	}

	if err = rows.Err(); err != nil {
		return nil, err
	}

	return leads, nil
}

// Count returns the total count of leads with filters
// Optimized to match FindAll query structure
func (r *LeadRepository) Count(companyID, search string, category, status, sourceID, assignedToCompanyUserID *string) (int, error) {
	query := `
		SELECT COUNT(DISTINCT l.id)
		FROM crm.leads l
		WHERE l.company_id = $1 AND l.deleted_at IS NULL
	`

	args := []interface{}{companyID}
	argPos := 2

	// Add search filter
	if search != "" {
		query += fmt.Sprintf(` AND (l.name ILIKE $%d OR l.phone ILIKE $%d OR l.email ILIKE $%d OR l.contact_person ILIKE $%d)`, argPos, argPos, argPos, argPos)
		args = append(args, "%"+search+"%")
		argPos++
	}

	// Add category filter
	if category != nil && *category != "" {
		query += fmt.Sprintf(` AND l.category = $%d`, argPos)
		args = append(args, *category)
		argPos++
	}

	// Add status filter
	if status != nil && *status != "" {
		query += fmt.Sprintf(` AND l.status = $%d`, argPos)
		args = append(args, *status)
		argPos++
	}

	// Add source_id filter
	if sourceID != nil && *sourceID != "" {
		query += fmt.Sprintf(` AND l.source_id = $%d`, argPos)
		args = append(args, *sourceID)
		argPos++
	}

	// Add assigned_to_company_user_id filter
	if assignedToCompanyUserID != nil && *assignedToCompanyUserID != "" {
		query += fmt.Sprintf(` AND l.assigned_to_company_user_id = $%d`, argPos)
		args = append(args, *assignedToCompanyUserID)
		argPos++
	}

	var count int
	err := r.db.QueryRow(context.Background(), query, args...).Scan(&count)
	if err != nil {
		return 0, err
	}

	return count, nil
}

// Create inserts a new lead
func (r *LeadRepository) Create(lead *domain.Lead) error {
	query := `
		INSERT INTO crm.leads (
			id, company_id, lead_number, name, contact_person, phone, email,
			category, status, source_id, assigned_to_company_user_id, deal_value,
			next_follow_up_at, notes, address, city, province, postal_code,
			created_at, created_by, updated_at, updated_by
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17, $18, $19, $20, $21, $22)
	`

	_, err := r.db.Exec(context.Background(), query,
		lead.ID, lead.CompanyID, lead.LeadNumber, lead.Name, lead.ContactPerson, lead.Phone, lead.Email,
		lead.Category, lead.Status, lead.SourceID, lead.AssignedToCompanyUserID, lead.DealValue,
		lead.NextFollowUpAt, lead.Notes, lead.Address, lead.City, lead.Province, lead.PostalCode,
		lead.CreatedAt, lead.CreatedBy, lead.UpdatedAt, lead.UpdatedBy,
	)

	return err
}

// Update modifies an existing lead
func (r *LeadRepository) Update(lead *domain.Lead) error {
	query := `
		UPDATE crm.leads
		SET name = $1, contact_person = $2, phone = $3, email = $4,
		    category = $5, status = $6, source_id = $7, assigned_to_company_user_id = $8,
		    deal_value = $9, last_contact_at = $10, next_follow_up_at = $11,
		    notes = $12, address = $13, city = $14, province = $15, postal_code = $16,
		    updated_at = $17, updated_by = $18
		WHERE id = $19 AND company_id = $20 AND deleted_at IS NULL
	`

	result, err := r.db.Exec(context.Background(), query,
		lead.Name, lead.ContactPerson, lead.Phone, lead.Email,
		lead.Category, lead.Status, lead.SourceID, lead.AssignedToCompanyUserID,
		lead.DealValue, lead.LastContactAt, lead.NextFollowUpAt,
		lead.Notes, lead.Address, lead.City, lead.Province, lead.PostalCode,
		lead.UpdatedAt, lead.UpdatedBy, lead.ID, lead.CompanyID,
	)

	if err != nil {
		return err
	}

	if result.RowsAffected() == 0 {
		return pgx.ErrNoRows
	}

	return nil
}

// Delete soft deletes a lead
func (r *LeadRepository) Delete(id, companyID, deletedBy string) error {
	query := `
		UPDATE crm.leads
		SET deleted_at = $1, deleted_by = $2
		WHERE id = $3 AND company_id = $4 AND deleted_at IS NULL
	`

	result, err := r.db.Exec(context.Background(), query, time.Now(), deletedBy, id, companyID)
	if err != nil {
		return err
	}

	if result.RowsAffected() == 0 {
		return pgx.ErrNoRows
	}

	return nil
}

// Restore restores a soft-deleted lead
func (r *LeadRepository) Restore(id, companyID string) error {
	query := `
		UPDATE crm.leads
		SET deleted_at = NULL, deleted_by = NULL
		WHERE id = $1 AND company_id = $2 AND deleted_at IS NOT NULL
	`

	result, err := r.db.Exec(context.Background(), query, id, companyID)
	if err != nil {
		return err
	}

	if result.RowsAffected() == 0 {
		return pgx.ErrNoRows
	}

	return nil
}
