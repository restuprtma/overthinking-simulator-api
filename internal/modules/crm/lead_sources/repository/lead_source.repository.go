package repository

import (
	"context"
	"errors"
	"fmt"
	"time"

	"venturo-skeleton-go/internal/modules/crm/lead_sources/domain"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type LeadSourceRepository struct {
	db *pgxpool.Pool
}

func NewLeadSourceRepository(db *pgxpool.Pool) *LeadSourceRepository {
	return &LeadSourceRepository{db: db}
}

// FindByID finds a lead source by ID
func (r *LeadSourceRepository) FindByID(id, companyID string) (*domain.LeadSource, error) {
	ctx := context.Background()
	query := `
		SELECT ls.id, ls.company_id, ls.name, ls.code, ls.description, ls.color, ls.icon, ls.is_active, ls.sort_order,
		       ls.created_at, COALESCE(u1.full_name, ls.created_by::TEXT) as created_by,
		       ls.updated_at, COALESCE(u2.full_name, ls.updated_by::TEXT) as updated_by,
		       ls.deleted_at, COALESCE(u3.full_name, ls.deleted_by::TEXT) as deleted_by
		FROM crm.lead_sources ls
		LEFT JOIN core.users u1 ON ls.created_by = u1.id
		LEFT JOIN core.users u2 ON ls.updated_by = u2.id
		LEFT JOIN core.users u3 ON ls.deleted_by = u3.id
		WHERE ls.id = $1 AND ls.company_id = $2 AND ls.deleted_at IS NULL
	`

	var ls domain.LeadSource
	err := r.db.QueryRow(ctx, query, id, companyID).Scan(
		&ls.ID, &ls.CompanyID, &ls.Name, &ls.Code, &ls.Description, &ls.Color, &ls.Icon,
		&ls.IsActive, &ls.SortOrder, &ls.CreatedAt, &ls.CreatedBy, &ls.UpdatedAt,
		&ls.UpdatedBy, &ls.DeletedAt, &ls.DeletedBy,
	)

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil // Not found
		}
		return nil, err
	}

	return &ls, nil
}

// FindByCode finds a lead source by code within a company
func (r *LeadSourceRepository) FindByCode(code, companyID string) (*domain.LeadSource, error) {
	ctx := context.Background()
	query := `
		SELECT ls.id, ls.company_id, ls.name, ls.code, ls.description, ls.color, ls.icon, ls.is_active, ls.sort_order,
		       ls.created_at, COALESCE(u1.full_name, ls.created_by::TEXT) as created_by,
		       ls.updated_at, COALESCE(u2.full_name, ls.updated_by::TEXT) as updated_by,
		       ls.deleted_at, COALESCE(u3.full_name, ls.deleted_by::TEXT) as deleted_by
		FROM crm.lead_sources ls
		LEFT JOIN core.users u1 ON ls.created_by = u1.id
		LEFT JOIN core.users u2 ON ls.updated_by = u2.id
		LEFT JOIN core.users u3 ON ls.deleted_by = u3.id
		WHERE ls.code = $1 AND ls.company_id = $2 AND ls.deleted_at IS NULL
	`

	var ls domain.LeadSource
	err := r.db.QueryRow(ctx, query, code, companyID).Scan(
		&ls.ID, &ls.CompanyID, &ls.Name, &ls.Code, &ls.Description, &ls.Color, &ls.Icon,
		&ls.IsActive, &ls.SortOrder, &ls.CreatedAt, &ls.CreatedBy, &ls.UpdatedAt,
		&ls.UpdatedBy, &ls.DeletedAt, &ls.DeletedBy,
	)

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil // Not found
		}
		return nil, err
	}

	return &ls, nil
}

// FindAll finds all lead sources with pagination and filters
func (r *LeadSourceRepository) FindAll(companyID string, limit, offset int, search string, isActive *bool) ([]domain.LeadSource, error) {
	ctx := context.Background()
	query := `
		SELECT ls.id, ls.company_id, ls.name, ls.code, ls.description, ls.color, ls.icon, ls.is_active, ls.sort_order,
		       ls.created_at, COALESCE(u1.full_name, ls.created_by::TEXT) as created_by,
		       ls.updated_at, COALESCE(u2.full_name, ls.updated_by::TEXT) as updated_by,
		       ls.deleted_at, COALESCE(u3.full_name, ls.deleted_by::TEXT) as deleted_by
		FROM crm.lead_sources ls
		LEFT JOIN core.users u1 ON ls.created_by = u1.id
		LEFT JOIN core.users u2 ON ls.updated_by = u2.id
		LEFT JOIN core.users u3 ON ls.deleted_by = u3.id
		WHERE ls.company_id = $1 AND ls.deleted_at IS NULL
	`

	args := []interface{}{companyID}
	argPos := 2

	// Add search filter
	if search != "" {
		query += fmt.Sprintf(` AND (ls.name ILIKE $%d OR ls.code ILIKE $%d OR ls.description ILIKE $%d)`, argPos, argPos, argPos)
		args = append(args, "%"+search+"%")
		argPos++
	}

	// Add isActive filter
	if isActive != nil {
		query += fmt.Sprintf(` AND ls.is_active = $%d`, argPos)
		args = append(args, *isActive)
		argPos++
	}

	// Add sorting and pagination
	query += fmt.Sprintf(` ORDER BY ls.sort_order ASC, ls.name ASC LIMIT $%d OFFSET $%d`, argPos, argPos+1)
	args = append(args, limit, offset)

	rows, err := r.db.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var leadSources []domain.LeadSource
	for rows.Next() {
		var ls domain.LeadSource
		err := rows.Scan(
			&ls.ID, &ls.CompanyID, &ls.Name, &ls.Code, &ls.Description, &ls.Color, &ls.Icon,
			&ls.IsActive, &ls.SortOrder, &ls.CreatedAt, &ls.CreatedBy, &ls.UpdatedAt,
			&ls.UpdatedBy, &ls.DeletedAt, &ls.DeletedBy,
		)
		if err != nil {
			return nil, err
		}
		leadSources = append(leadSources, ls)
	}

	return leadSources, nil
}

// Count counts total lead sources matching filters
func (r *LeadSourceRepository) Count(companyID, search string, isActive *bool) (int64, error) {
	ctx := context.Background()
	query := `
		SELECT COUNT(*)
		FROM crm.lead_sources ls
		WHERE ls.company_id = $1 AND ls.deleted_at IS NULL
	`

	args := []interface{}{companyID}
	argPos := 2

	// Add search filter
	if search != "" {
		query += fmt.Sprintf(` AND (ls.name ILIKE $%d OR ls.code ILIKE $%d OR ls.description ILIKE $%d)`, argPos, argPos, argPos)
		args = append(args, "%"+search+"%")
		argPos++
	}

	// Add isActive filter
	if isActive != nil {
		query += fmt.Sprintf(` AND ls.is_active = $%d`, argPos)
		args = append(args, *isActive)
		argPos++
	}

	var count int64
	err := r.db.QueryRow(ctx, query, args...).Scan(&count)
	return count, err
}

// Create creates a new lead source
func (r *LeadSourceRepository) Create(ls *domain.LeadSource) error {
	ctx := context.Background()
	query := `
		INSERT INTO crm.lead_sources (
			id, company_id, name, code, description, color, icon, is_active, sort_order,
			created_at, created_by, updated_at, updated_by
		) VALUES (
			$1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13
		)
	`

	_, err := r.db.Exec(ctx, query,
		ls.ID, ls.CompanyID, ls.Name, ls.Code, ls.Description, ls.Color, ls.Icon,
		ls.IsActive, ls.SortOrder, ls.CreatedAt, ls.CreatedBy, ls.UpdatedAt, ls.UpdatedBy,
	)

	return err
}

// Update updates an existing lead source
func (r *LeadSourceRepository) Update(ls *domain.LeadSource) error {
	ctx := context.Background()
	query := `
		UPDATE crm.lead_sources
		SET name = $1, code = $2, description = $3, color = $4, icon = $5,
		    is_active = $6, sort_order = $7, updated_at = $8, updated_by = $9
		WHERE id = $10 AND company_id = $11 AND deleted_at IS NULL
	`

	result, err := r.db.Exec(ctx, query,
		ls.Name, ls.Code, ls.Description, ls.Color, ls.Icon, ls.IsActive, ls.SortOrder,
		ls.UpdatedAt, ls.UpdatedBy, ls.ID, ls.CompanyID,
	)

	if err != nil {
		return err
	}

	if result.RowsAffected() == 0 {
		return errors.New("lead source not found or already deleted")
	}

	return nil
}

// Delete soft deletes a lead source
func (r *LeadSourceRepository) Delete(id, companyID, deletedBy string) error {
	ctx := context.Background()
	query := `
		UPDATE crm.lead_sources
		SET deleted_at = $1, deleted_by = $2
		WHERE id = $3 AND company_id = $4 AND deleted_at IS NULL
	`

	result, err := r.db.Exec(ctx, query, time.Now(), deletedBy, id, companyID)
	if err != nil {
		return err
	}

	if result.RowsAffected() == 0 {
		return errors.New("lead source not found or already deleted")
	}

	return nil
}

// Restore restores a soft-deleted lead source
func (r *LeadSourceRepository) Restore(id, companyID string) error {
	ctx := context.Background()
	query := `
		UPDATE crm.lead_sources
		SET deleted_at = NULL, deleted_by = NULL, updated_at = $1
		WHERE id = $2 AND company_id = $3 AND deleted_at IS NOT NULL
	`

	result, err := r.db.Exec(ctx, query, time.Now(), id, companyID)
	if err != nil {
		return err
	}

	if result.RowsAffected() == 0 {
		return errors.New("lead source not found or not deleted")
	}

	return nil
}
