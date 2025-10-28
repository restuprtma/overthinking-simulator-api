package repository

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"venturo-skeleton-go/internal/modules/crm/sales_persons/domain"
)

type SalesPersonRepository struct {
	db *pgxpool.Pool
}

func NewSalesPersonRepository(db *pgxpool.Pool) *SalesPersonRepository {
	return &SalesPersonRepository{db: db}
}

// FindByID retrieves a sales person by ID and company ID
func (r *SalesPersonRepository) FindByID(id, companyID string) (*domain.SalesPerson, error) {
	query := `
		SELECT
			sp.id, sp.company_user_id, sp.sales_code, sp.sales_name, sp.sales_area,
			sp.sales_target, sp.commission_rate, sp.whatsapp, sp.is_whatsapp_connected,
			sp.waha_session_name, sp.waha_session_status, sp.waha_pairing_code, sp.waha_pairing_code_expires_at,
			sp.waha_last_seen_at, sp.waha_connected_at, sp.waha_disconnected_at,
			sp.is_active, sp.notes,
			sp.created_at, sp.created_by, sp.updated_at, sp.updated_by, sp.deleted_at, sp.deleted_by,
			u.id as user_id, u.full_name as user_full_name, u.email as user_email,
			uc.full_name as created_by_name, uu.full_name as updated_by_name, ud.full_name as deleted_by_name
		FROM crm.sales_persons sp
		INNER JOIN core.company_users cu ON sp.company_user_id = cu.id
		INNER JOIN core.users u ON cu.user_id = u.id
		LEFT JOIN core.users uc ON sp.created_by = uc.id
		LEFT JOIN core.users uu ON sp.updated_by = uu.id
		LEFT JOIN core.users ud ON sp.deleted_by = ud.id
		WHERE sp.id = $1 AND cu.company_id = $2 AND sp.deleted_at IS NULL
	`

	var sp domain.SalesPerson
	err := r.db.QueryRow(context.Background(), query, id, companyID).Scan(
		&sp.ID, &sp.CompanyUserID, &sp.SalesCode, &sp.SalesName, &sp.SalesArea,
		&sp.SalesTarget, &sp.CommissionRate, &sp.Whatsapp, &sp.IsWhatsappConnected,
		&sp.WAHASessionName, &sp.WAHASessionStatus, &sp.WAHAPairingCode, &sp.WAHAPairingCodeExpiresAt,
		&sp.WAHALastSeenAt, &sp.WAHAConnectedAt, &sp.WAHADisconnectedAt,
		&sp.IsActive, &sp.Notes,
		&sp.CreatedAt, &sp.CreatedBy, &sp.UpdatedAt, &sp.UpdatedBy, &sp.DeletedAt, &sp.DeletedBy,
		&sp.UserID, &sp.UserFullName, &sp.UserEmail,
		&sp.CreatedByName, &sp.UpdatedByName, &sp.DeletedByName,
	)

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}

	return &sp, nil
}

// FindByCompanyUserID retrieves a sales person by company_user_id
func (r *SalesPersonRepository) FindByCompanyUserID(companyUserID, companyID string) (*domain.SalesPerson, error) {
	query := `
		SELECT
			sp.id, sp.company_user_id, sp.sales_code, sp.sales_name, sp.sales_area,
			sp.sales_target, sp.commission_rate, sp.whatsapp, sp.is_whatsapp_connected,
			sp.is_active, sp.notes,
			sp.created_at, sp.created_by, sp.updated_at, sp.updated_by, sp.deleted_at, sp.deleted_by,
			u.id as user_id, u.full_name as user_full_name, u.email as user_email,
			uc.full_name as created_by_name, uu.full_name as updated_by_name, ud.full_name as deleted_by_name
		FROM crm.sales_persons sp
		INNER JOIN core.company_users cu ON sp.company_user_id = cu.id
		INNER JOIN core.users u ON cu.user_id = u.id
		LEFT JOIN core.users uc ON sp.created_by = uc.id
		LEFT JOIN core.users uu ON sp.updated_by = uu.id
		LEFT JOIN core.users ud ON sp.deleted_by = ud.id
		WHERE sp.company_user_id = $1 AND cu.company_id = $2 AND sp.deleted_at IS NULL
	`

	var sp domain.SalesPerson
	err := r.db.QueryRow(context.Background(), query, companyUserID, companyID).Scan(
		&sp.ID, &sp.CompanyUserID, &sp.SalesCode, &sp.SalesName, &sp.SalesArea,
		&sp.SalesTarget, &sp.CommissionRate, &sp.Whatsapp, &sp.IsWhatsappConnected,
		&sp.IsActive, &sp.Notes,
		&sp.CreatedAt, &sp.CreatedBy, &sp.UpdatedAt, &sp.UpdatedBy, &sp.DeletedAt, &sp.DeletedBy,
		&sp.UserID, &sp.UserFullName, &sp.UserEmail,
		&sp.CreatedByName, &sp.UpdatedByName, &sp.DeletedByName,
	)

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}

	return &sp, nil
}

// FindBySalesCode retrieves a sales person by sales code and company ID
func (r *SalesPersonRepository) FindBySalesCode(salesCode, companyID string) (*domain.SalesPerson, error) {
	query := `
		SELECT
			sp.id, sp.company_user_id, sp.sales_code, sp.sales_name, sp.sales_area,
			sp.sales_target, sp.commission_rate, sp.whatsapp, sp.is_whatsapp_connected,
			sp.is_active, sp.notes,
			sp.created_at, sp.created_by, sp.updated_at, sp.updated_by, sp.deleted_at, sp.deleted_by,
			u.id as user_id, u.full_name as user_full_name, u.email as user_email,
			uc.full_name as created_by_name, uu.full_name as updated_by_name, ud.full_name as deleted_by_name
		FROM crm.sales_persons sp
		INNER JOIN core.company_users cu ON sp.company_user_id = cu.id
		INNER JOIN core.users u ON cu.user_id = u.id
		LEFT JOIN core.users uc ON sp.created_by = uc.id
		LEFT JOIN core.users uu ON sp.updated_by = uu.id
		LEFT JOIN core.users ud ON sp.deleted_by = ud.id
		WHERE sp.sales_code = $1 AND cu.company_id = $2 AND sp.deleted_at IS NULL
	`

	var sp domain.SalesPerson
	err := r.db.QueryRow(context.Background(), query, salesCode, companyID).Scan(
		&sp.ID, &sp.CompanyUserID, &sp.SalesCode, &sp.SalesName, &sp.SalesArea,
		&sp.SalesTarget, &sp.CommissionRate, &sp.Whatsapp, &sp.IsWhatsappConnected,
		&sp.IsActive, &sp.Notes,
		&sp.CreatedAt, &sp.CreatedBy, &sp.UpdatedAt, &sp.UpdatedBy, &sp.DeletedAt, &sp.DeletedBy,
		&sp.UserID, &sp.UserFullName, &sp.UserEmail,
		&sp.CreatedByName, &sp.UpdatedByName, &sp.DeletedByName,
	)

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}

	return &sp, nil
}

// FindAll retrieves all sales persons with pagination and filters
// Optimized for list view - only joins essential user data, no audit names
func (r *SalesPersonRepository) FindAll(companyID string, limit, offset int, search string, isActive, isWhatsappConnected *bool, salesArea *string) ([]*domain.SalesPerson, error) {
	query := `
		SELECT
			sp.id, sp.company_user_id, sp.sales_code, sp.sales_name, sp.sales_area,
			sp.sales_target, sp.commission_rate, sp.whatsapp, sp.is_whatsapp_connected,
			sp.waha_session_name, sp.waha_session_status, sp.waha_pairing_code, sp.waha_pairing_code_expires_at,
			sp.waha_last_seen_at, sp.waha_connected_at, sp.waha_disconnected_at,
			sp.is_active, sp.notes,
			sp.created_at, sp.created_by, sp.updated_at, sp.updated_by, sp.deleted_at, sp.deleted_by,
			u.id as user_id, u.full_name as user_full_name, u.email as user_email
		FROM crm.sales_persons sp
		INNER JOIN core.company_users cu ON sp.company_user_id = cu.id
		INNER JOIN core.users u ON cu.user_id = u.id
		WHERE cu.company_id = $1 AND sp.deleted_at IS NULL
	`

	args := []interface{}{companyID}
	argPos := 2

	// Add search filter
	if search != "" {
		query += fmt.Sprintf(` AND (sp.sales_code ILIKE $%d OR sp.sales_name ILIKE $%d OR u.full_name ILIKE $%d OR sp.sales_area ILIKE $%d)`, argPos, argPos, argPos, argPos)
		args = append(args, "%"+search+"%")
		argPos++
	}

	// Add is_active filter
	if isActive != nil {
		query += fmt.Sprintf(` AND sp.is_active = $%d`, argPos)
		args = append(args, *isActive)
		argPos++
	}

	// Add is_whatsapp_connected filter
	if isWhatsappConnected != nil {
		query += fmt.Sprintf(` AND sp.is_whatsapp_connected = $%d`, argPos)
		args = append(args, *isWhatsappConnected)
		argPos++
	}

	// Add sales_area filter
	if salesArea != nil && *salesArea != "" {
		query += fmt.Sprintf(` AND sp.sales_area = $%d`, argPos)
		args = append(args, *salesArea)
		argPos++
	}

	query += ` ORDER BY sp.created_at DESC`
	query += fmt.Sprintf(` LIMIT $%d OFFSET $%d`, argPos, argPos+1)
	args = append(args, limit, offset)

	rows, err := r.db.Query(context.Background(), query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var salesPersons []*domain.SalesPerson
	for rows.Next() {
		var sp domain.SalesPerson
		err := rows.Scan(
			&sp.ID, &sp.CompanyUserID, &sp.SalesCode, &sp.SalesName, &sp.SalesArea,
			&sp.SalesTarget, &sp.CommissionRate, &sp.Whatsapp, &sp.IsWhatsappConnected,
			&sp.WAHASessionName, &sp.WAHASessionStatus, &sp.WAHAPairingCode, &sp.WAHAPairingCodeExpiresAt,
			&sp.WAHALastSeenAt, &sp.WAHAConnectedAt, &sp.WAHADisconnectedAt,
			&sp.IsActive, &sp.Notes,
			&sp.CreatedAt, &sp.CreatedBy, &sp.UpdatedAt, &sp.UpdatedBy, &sp.DeletedAt, &sp.DeletedBy,
			&sp.UserID, &sp.UserFullName, &sp.UserEmail,
		)
		if err != nil {
			return nil, err
		}
		// Audit names are not loaded in list view for performance
		// Use FindByID if you need full audit trail information
		salesPersons = append(salesPersons, &sp)
	}

	if err = rows.Err(); err != nil {
		return nil, err
	}

	return salesPersons, nil
}

// Count returns the total count of sales persons with filters
// Optimized to match FindAll query structure
func (r *SalesPersonRepository) Count(companyID, search string, isActive, isWhatsappConnected *bool, salesArea *string) (int, error) {
	query := `
		SELECT COUNT(DISTINCT sp.id)
		FROM crm.sales_persons sp
		INNER JOIN core.company_users cu ON sp.company_user_id = cu.id
		INNER JOIN core.users u ON cu.user_id = u.id
		WHERE cu.company_id = $1 AND sp.deleted_at IS NULL
	`

	args := []interface{}{companyID}
	argPos := 2

	// Add search filter
	if search != "" {
		query += fmt.Sprintf(` AND (sp.sales_code ILIKE $%d OR sp.sales_name ILIKE $%d OR u.full_name ILIKE $%d OR sp.sales_area ILIKE $%d)`, argPos, argPos, argPos, argPos)
		args = append(args, "%"+search+"%")
		argPos++
	}

	// Add is_active filter
	if isActive != nil {
		query += fmt.Sprintf(` AND sp.is_active = $%d`, argPos)
		args = append(args, *isActive)
		argPos++
	}

	// Add is_whatsapp_connected filter
	if isWhatsappConnected != nil {
		query += fmt.Sprintf(` AND sp.is_whatsapp_connected = $%d`, argPos)
		args = append(args, *isWhatsappConnected)
		argPos++
	}

	// Add sales_area filter
	if salesArea != nil && *salesArea != "" {
		query += fmt.Sprintf(` AND sp.sales_area = $%d`, argPos)
		args = append(args, *salesArea)
		argPos++
	}

	var count int
	err := r.db.QueryRow(context.Background(), query, args...).Scan(&count)
	if err != nil {
		return 0, err
	}

	return count, nil
}

// Create inserts a new sales person
func (r *SalesPersonRepository) Create(sp *domain.SalesPerson) error {
	query := `
		INSERT INTO crm.sales_persons (
			id, company_user_id, sales_code, sales_name, sales_area,
			sales_target, commission_rate, whatsapp, is_whatsapp_connected,
			is_active, notes, created_at, created_by, updated_at, updated_by
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15)
	`

	_, err := r.db.Exec(context.Background(), query,
		sp.ID, sp.CompanyUserID, sp.SalesCode, sp.SalesName, sp.SalesArea,
		sp.SalesTarget, sp.CommissionRate, sp.Whatsapp, sp.IsWhatsappConnected,
		sp.IsActive, sp.Notes, sp.CreatedAt, sp.CreatedBy, sp.UpdatedAt, sp.UpdatedBy,
	)

	return err
}

// Update modifies an existing sales person
func (r *SalesPersonRepository) Update(sp *domain.SalesPerson) error {
	query := `
		UPDATE crm.sales_persons
		SET sales_code = $1, sales_name = $2, sales_area = $3, sales_target = $4,
		    commission_rate = $5, whatsapp = $6, is_whatsapp_connected = $7,
		    is_active = $8, notes = $9, updated_at = $10, updated_by = $11
		WHERE id = $12 AND deleted_at IS NULL
	`

	result, err := r.db.Exec(context.Background(), query,
		sp.SalesCode, sp.SalesName, sp.SalesArea, sp.SalesTarget,
		sp.CommissionRate, sp.Whatsapp, sp.IsWhatsappConnected,
		sp.IsActive, sp.Notes, sp.UpdatedAt, sp.UpdatedBy, sp.ID,
	)

	if err != nil {
		return err
	}

	if result.RowsAffected() == 0 {
		return pgx.ErrNoRows
	}

	return nil
}

// Delete soft deletes a sales person
func (r *SalesPersonRepository) Delete(id, companyID, deletedBy string) error {
	query := `
		UPDATE crm.sales_persons sp
		SET deleted_at = $1, deleted_by = $2
		FROM core.company_users cu
		WHERE sp.id = $3 AND sp.company_user_id = cu.id
		  AND cu.company_id = $4 AND sp.deleted_at IS NULL
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

// Restore restores a soft-deleted sales person
func (r *SalesPersonRepository) Restore(id, companyID string) error {
	query := `
		UPDATE crm.sales_persons sp
		SET deleted_at = NULL, deleted_by = NULL
		FROM core.company_users cu
		WHERE sp.id = $1 AND sp.company_user_id = cu.id
		  AND cu.company_id = $2 AND sp.deleted_at IS NOT NULL
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

// UpdateWAHASession updates WAHA session information
func (r *SalesPersonRepository) UpdateWAHASession(sp *domain.SalesPerson) error {
	query := `
		UPDATE crm.sales_persons
		SET waha_session_name = $1,
		    waha_session_status = $2,
		    waha_pairing_code = $3,
		    waha_pairing_code_expires_at = $4,
		    waha_last_seen_at = $5,
		    waha_connected_at = $6,
		    waha_disconnected_at = $7,
		    is_whatsapp_connected = $8,
		    updated_at = $9,
		    updated_by = $10
		WHERE id = $11
	`

	result, err := r.db.Exec(context.Background(), query,
		sp.WAHASessionName, sp.WAHASessionStatus, sp.WAHAPairingCode, sp.WAHAPairingCodeExpiresAt,
		sp.WAHALastSeenAt, sp.WAHAConnectedAt, sp.WAHADisconnectedAt,
		sp.IsWhatsappConnected, sp.UpdatedAt, sp.UpdatedBy, sp.ID,
	)

	if err != nil {
		return err
	}

	if result.RowsAffected() == 0 {
		return pgx.ErrNoRows
	}

	return nil
}

// UpdateWAHAConnectionStatus updates WhatsApp connection status
func (r *SalesPersonRepository) UpdateWAHAConnectionStatus(id string, status string, isConnected bool, timestamp time.Time) error {
	query := `
		UPDATE crm.sales_persons
		SET waha_session_status = $1,
		    is_whatsapp_connected = $2,
		    waha_last_seen_at = $3,
		    waha_connected_at = CASE WHEN $2 = true THEN $3 ELSE waha_connected_at END,
		    waha_disconnected_at = CASE WHEN $2 = false THEN $3 ELSE waha_disconnected_at END,
		    updated_at = $3
		WHERE id = $4
	`

	result, err := r.db.Exec(context.Background(), query, status, isConnected, timestamp, id)
	if err != nil {
		return err
	}

	if result.RowsAffected() == 0 {
		return pgx.ErrNoRows
	}

	return nil
}

// FindByWAHASessionName retrieves a sales person by WAHA session name
func (r *SalesPersonRepository) FindByWAHASessionName(sessionName string) (*domain.SalesPerson, error) {
	query := `
		SELECT
			sp.id, sp.company_user_id, sp.sales_code, sp.sales_name, sp.sales_area,
			sp.sales_target, sp.commission_rate, sp.whatsapp, sp.is_whatsapp_connected,
			sp.waha_session_name, sp.waha_session_status, sp.waha_pairing_code, sp.waha_pairing_code_expires_at,
			sp.waha_last_seen_at, sp.waha_connected_at, sp.waha_disconnected_at,
			sp.is_active, sp.notes,
			sp.created_at, sp.created_by, sp.updated_at, sp.updated_by, sp.deleted_at, sp.deleted_by,
			cu.company_id,
			u.id as user_id, u.full_name as user_full_name, u.email as user_email,
			uc.full_name as created_by_name, uu.full_name as updated_by_name, ud.full_name as deleted_by_name
		FROM crm.sales_persons sp
		INNER JOIN core.company_users cu ON sp.company_user_id = cu.id
		INNER JOIN core.users u ON cu.user_id = u.id
		LEFT JOIN core.users uc ON sp.created_by = uc.id
		LEFT JOIN core.users uu ON sp.updated_by = uu.id
		LEFT JOIN core.users ud ON sp.deleted_by = ud.id
		WHERE sp.waha_session_name = $1 AND sp.deleted_at IS NULL
	`

	var sp domain.SalesPerson
	var companyID string
	err := r.db.QueryRow(context.Background(), query, sessionName).Scan(
		&sp.ID, &sp.CompanyUserID, &sp.SalesCode, &sp.SalesName, &sp.SalesArea,
		&sp.SalesTarget, &sp.CommissionRate, &sp.Whatsapp, &sp.IsWhatsappConnected,
		&sp.WAHASessionName, &sp.WAHASessionStatus, &sp.WAHAPairingCode, &sp.WAHAPairingCodeExpiresAt,
		&sp.WAHALastSeenAt, &sp.WAHAConnectedAt, &sp.WAHADisconnectedAt,
		&sp.IsActive, &sp.Notes,
		&sp.CreatedAt, &sp.CreatedBy, &sp.UpdatedAt, &sp.UpdatedBy, &sp.DeletedAt, &sp.DeletedBy,
		&companyID,
		&sp.UserID, &sp.UserFullName, &sp.UserEmail,
		&sp.CreatedByName, &sp.UpdatedByName, &sp.DeletedByName,
	)

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}

	return &sp, nil
}

// GetCompanyIDBySessionName gets company ID for a WAHA session name
func (r *SalesPersonRepository) GetCompanyIDBySessionName(sessionName string) (string, error) {
	query := `
		SELECT cu.company_id
		FROM crm.sales_persons sp
		INNER JOIN core.company_users cu ON sp.company_user_id = cu.id
		WHERE sp.waha_session_name = $1 AND sp.deleted_at IS NULL
	`

	var companyID string
	err := r.db.QueryRow(context.Background(), query, sessionName).Scan(&companyID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return "", fmt.Errorf("session not found")
		}
		return "", err
	}

	return companyID, nil
}
