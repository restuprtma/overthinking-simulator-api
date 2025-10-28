package repository

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"venturo-skeleton-go/internal/modules/core/company/domain"
	"venturo-skeleton-go/internal/shared/query"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type CompanyUserRepository struct {
	db *pgxpool.Pool
}

func NewCompanyUserRepository(db *pgxpool.Pool) *CompanyUserRepository {
	return &CompanyUserRepository{db: db}
}

// Create adds a user to a company
func (r *CompanyUserRepository) Create(companyUser *domain.CompanyUser) error {
	ctx := context.Background()
	query := `
		INSERT INTO core.company_users (
			id, company_id, user_id, role_id, role, is_primary, is_active,
			invited_by, joined_at, created_at, created_by, updated_at, updated_by
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13)
	`

	_, err := r.db.Exec(ctx, query,
		companyUser.ID, companyUser.CompanyID, companyUser.UserID,
		companyUser.RoleID, companyUser.Role, companyUser.IsPrimary, companyUser.IsActive,
		companyUser.InvitedBy, companyUser.JoinedAt,
		companyUser.CreatedAt, companyUser.CreatedBy,
		companyUser.UpdatedAt, companyUser.UpdatedBy,
	)

	return err
}

// FindByCompanyAndUser finds a company user relationship
func (r *CompanyUserRepository) FindByCompanyAndUser(companyID, userID string) (*domain.CompanyUser, error) {
	ctx := context.Background()

	baseQuery := `
		SELECT cu.id, cu.company_id, cu.user_id, cu.role_id, cu.role, cu.is_primary, cu.is_active,
		       cu.invited_by, cu.joined_at,
		       u.email, u.username, u.full_name,
		       cu.created_at, COALESCE(u1.full_name, cu.created_by::TEXT) as created_by,
		       cu.updated_at, COALESCE(u2.full_name, cu.updated_by::TEXT) as updated_by,
		       cu.deleted_at, COALESCE(u3.full_name, cu.deleted_by::TEXT) as deleted_by
		FROM core.company_users cu
		LEFT JOIN core.users u ON cu.user_id = u.id
		LEFT JOIN core.users u1 ON cu.created_by = u1.id
		LEFT JOIN core.users u2 ON cu.updated_by = u2.id
		LEFT JOIN core.users u3 ON cu.deleted_by = u3.id`

	qb := query.NewQueryBuilder(baseQuery)
	sql, args := qb.
		AddFilter("cu.company_id", "=", companyID).
		AddFilter("cu.user_id", "=", userID).
		AddCustomCondition("cu.deleted_at IS NULL").
		Build()

	var cu domain.CompanyUser
	err := r.db.QueryRow(ctx, sql, args...).Scan(
		&cu.ID, &cu.CompanyID, &cu.UserID, &cu.RoleID, &cu.Role,
		&cu.IsPrimary, &cu.IsActive, &cu.InvitedBy, &cu.JoinedAt,
		&cu.UserEmail, &cu.UserUsername, &cu.UserFullName,
		&cu.CreatedAt, &cu.CreatedBy, &cu.UpdatedAt, &cu.UpdatedBy,
		&cu.DeletedAt, &cu.DeletedBy,
	)

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}

	return &cu, nil
}

// FindByID finds a company user by ID
func (r *CompanyUserRepository) FindByID(id string) (*domain.CompanyUser, error) {
	ctx := context.Background()

	baseQuery := `
		SELECT cu.id, cu.company_id, cu.user_id, cu.role_id, cu.role, cu.is_primary, cu.is_active,
		       cu.invited_by, cu.joined_at,
		       u.email, u.username, u.full_name,
		       cu.created_at, COALESCE(u1.full_name, cu.created_by::TEXT) as created_by,
		       cu.updated_at, COALESCE(u2.full_name, cu.updated_by::TEXT) as updated_by,
		       cu.deleted_at, COALESCE(u3.full_name, cu.deleted_by::TEXT) as deleted_by
		FROM core.company_users cu
		LEFT JOIN core.users u ON cu.user_id = u.id
		LEFT JOIN core.users u1 ON cu.created_by = u1.id
		LEFT JOIN core.users u2 ON cu.updated_by = u2.id
		LEFT JOIN core.users u3 ON cu.deleted_by = u3.id`

	qb := query.NewQueryBuilder(baseQuery)
	sql, args := qb.
		AddFilter("cu.id", "=", id).
		AddCustomCondition("cu.deleted_at IS NULL").
		Build()

	var cu domain.CompanyUser
	err := r.db.QueryRow(ctx, sql, args...).Scan(
		&cu.ID, &cu.CompanyID, &cu.UserID, &cu.RoleID, &cu.Role,
		&cu.IsPrimary, &cu.IsActive, &cu.InvitedBy, &cu.JoinedAt,
		&cu.UserEmail, &cu.UserUsername, &cu.UserFullName,
		&cu.CreatedAt, &cu.CreatedBy, &cu.UpdatedAt, &cu.UpdatedBy,
		&cu.DeletedAt, &cu.DeletedBy,
	)

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}

	return &cu, nil
}

// FindByCompany finds all users in a company with pagination
func (r *CompanyUserRepository) FindByCompany(companyID string, limit, offset int, role string, isActive *bool) ([]domain.CompanyUser, error) {
	ctx := context.Background()

	baseQuery := `
		SELECT cu.id, cu.company_id, cu.user_id, cu.role_id, cu.role, cu.is_primary, cu.is_active,
		       cu.invited_by, cu.joined_at,
		       u.email, u.username, u.full_name,
		       cu.created_at, COALESCE(u1.full_name, cu.created_by::TEXT) as created_by,
		       cu.updated_at, COALESCE(u2.full_name, cu.updated_by::TEXT) as updated_by,
		       cu.deleted_at, COALESCE(u3.full_name, cu.deleted_by::TEXT) as deleted_by
		FROM core.company_users cu
		LEFT JOIN core.users u ON cu.user_id = u.id
		LEFT JOIN core.users u1 ON cu.created_by = u1.id
		LEFT JOIN core.users u2 ON cu.updated_by = u2.id
		LEFT JOIN core.users u3 ON cu.deleted_by = u3.id`

	qb := query.NewQueryBuilder(baseQuery)
	qb.AddFilter("cu.company_id", "=", companyID).
		AddCustomCondition("cu.deleted_at IS NULL").
		AddIsActiveFilter(isActive, "cu.is_active")

	if role != "" {
		qb.AddFilter("cu.role", "=", role)
	}

	sql, args := qb.
		AddOrderBy("cu.is_primary", "DESC").
		AddOrderBy("cu.joined_at", "ASC").
		AddPagination(limit, offset).
		Build()

	rows, err := r.db.Query(ctx, sql, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var companyUsers []domain.CompanyUser
	for rows.Next() {
		var cu domain.CompanyUser
		err := rows.Scan(
			&cu.ID, &cu.CompanyID, &cu.UserID, &cu.RoleID, &cu.Role,
			&cu.IsPrimary, &cu.IsActive, &cu.InvitedBy, &cu.JoinedAt,
			&cu.UserEmail, &cu.UserUsername, &cu.UserFullName,
			&cu.CreatedAt, &cu.CreatedBy, &cu.UpdatedAt, &cu.UpdatedBy,
			&cu.DeletedAt, &cu.DeletedBy,
		)
		if err != nil {
			return nil, err
		}
		companyUsers = append(companyUsers, cu)
	}

	return companyUsers, nil
}

// CountByCompany counts users in a company
func (r *CompanyUserRepository) CountByCompany(companyID string, role string, isActive *bool) (int64, error) {
	ctx := context.Background()

	baseQuery := "SELECT COUNT(*) FROM core.company_users cu"

	qb := query.NewQueryBuilder(baseQuery)
	qb.AddFilter("cu.company_id", "=", companyID).
		AddCustomCondition("cu.deleted_at IS NULL").
		AddIsActiveFilter(isActive, "cu.is_active")

	if role != "" {
		qb.AddFilter("cu.role", "=", role)
	}

	sql, args := qb.Build()

	var count int64
	err := r.db.QueryRow(ctx, sql, args...).Scan(&count)
	return count, err
}

// CountActiveUsersByCompany counts active users in a company
func (r *CompanyUserRepository) CountActiveUsersByCompany(companyID string) (int64, error) {
	ctx := context.Background()

	baseQuery := "SELECT COUNT(*) FROM core.company_users cu"
	isActive := true

	qb := query.NewQueryBuilder(baseQuery)
	sql, args := qb.
		AddFilter("cu.company_id", "=", companyID).
		AddIsActiveFilter(&isActive, "cu.is_active").
		AddCustomCondition("cu.deleted_at IS NULL").
		Build()

	var count int64
	err := r.db.QueryRow(ctx, sql, args...).Scan(&count)
	return count, err
}

// Update updates a company user
func (r *CompanyUserRepository) Update(companyUser *domain.CompanyUser) error {
	ctx := context.Background()
	query := `
		UPDATE core.company_users
		SET role_id = $1, role = $2, is_active = $3, updated_at = $4, updated_by = $5
		WHERE id = $6 AND deleted_at IS NULL
	`

	_, err := r.db.Exec(ctx, query,
		companyUser.RoleID, companyUser.Role, companyUser.IsActive,
		companyUser.UpdatedAt, companyUser.UpdatedBy,
		companyUser.ID,
	)

	return err
}

// Delete soft deletes a company user (removes user from company)
func (r *CompanyUserRepository) Delete(id string, deletedBy string) error {
	ctx := context.Background()
	query := `
		UPDATE core.company_users
		SET deleted_at = $1, deleted_by = $2
		WHERE id = $3 AND deleted_at IS NULL
	`

	_, err := r.db.Exec(ctx, query, time.Now(), deletedBy, id)
	return err
}

// FindUserCompanies finds all companies a user belongs to
func (r *CompanyUserRepository) FindUserCompanies(userID string, isActive *bool) ([]string, error) {
	ctx := context.Background()

	baseQuery := "SELECT cu.company_id FROM core.company_users cu"

	qb := query.NewQueryBuilder(baseQuery)
	qb.AddFilter("cu.user_id", "=", userID).
		AddCustomCondition("cu.deleted_at IS NULL").
		AddIsActiveFilter(isActive, "cu.is_active")

	sql, args := qb.Build()

	rows, err := r.db.Query(ctx, sql, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var companyIDs []string
	for rows.Next() {
		var companyID string
		if err := rows.Scan(&companyID); err != nil {
			return nil, err
		}
		companyIDs = append(companyIDs, companyID)
	}

	return companyIDs, nil
}

// GetPrimaryCompanyID gets the primary company ID for a user
// Returns empty string if user has no primary company
func (r *CompanyUserRepository) GetPrimaryCompanyID(userID string) (string, error) {
	ctx := context.Background()
	query := `
		SELECT company_id
		FROM core.company_users
		WHERE user_id = $1 AND is_primary = true AND is_active = true AND deleted_at IS NULL
		LIMIT 1
	`

	var companyID string
	err := r.db.QueryRow(ctx, query, userID).Scan(&companyID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			// No primary company found, try to get any active company
			query = `
				SELECT company_id
				FROM core.company_users
				WHERE user_id = $1 AND is_active = true AND deleted_at IS NULL
				ORDER BY joined_at ASC
				LIMIT 1
			`
			err = r.db.QueryRow(ctx, query, userID).Scan(&companyID)
			if err != nil {
				if errors.Is(err, pgx.ErrNoRows) {
					return "", nil // No company found
				}
				return "", err
			}
		} else {
			return "", err
		}
	}

	return companyID, nil
}

// GetPrimaryCompany gets the primary company details for a user
// Returns nil if user has no primary company
func (r *CompanyUserRepository) GetPrimaryCompany(userID string) (*domain.CompanyBasic, error) {
	ctx := context.Background()
	query := `
		SELECT c.id, c.name, c.code, c.logo_url
		FROM core.companies c
		INNER JOIN core.company_users cu ON c.id = cu.company_id
		WHERE cu.user_id = $1 AND cu.is_primary = true AND cu.is_active = true
		  AND cu.deleted_at IS NULL AND c.deleted_at IS NULL AND c.is_active = true
		LIMIT 1
	`

	var company domain.CompanyBasic
	err := r.db.QueryRow(ctx, query, userID).Scan(
		&company.ID, &company.Name, &company.Code, &company.LogoURL,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			// No primary company found, try to get any active company
			query = `
				SELECT c.id, c.name, c.code, c.logo_url
				FROM core.companies c
				INNER JOIN core.company_users cu ON c.id = cu.company_id
				WHERE cu.user_id = $1 AND cu.is_active = true
				  AND cu.deleted_at IS NULL AND c.deleted_at IS NULL AND c.is_active = true
				ORDER BY cu.joined_at ASC
				LIMIT 1
			`
			err = r.db.QueryRow(ctx, query, userID).Scan(
				&company.ID, &company.Name, &company.Code, &company.LogoURL,
			)
			if err != nil {
				if errors.Is(err, pgx.ErrNoRows) {
					return nil, nil // No company found
				}
				return nil, err
			}
		} else {
			return nil, err
		}
	}

	return &company, nil
}

// GetUserCompanies gets all companies a user belongs to
func (r *CompanyUserRepository) GetUserCompanies(userID string) ([]domain.CompanyBasic, error) {
	ctx := context.Background()
	query := `
		SELECT c.id, c.name, c.code, c.logo_url
		FROM core.companies c
		INNER JOIN core.company_users cu ON c.id = cu.company_id
		WHERE cu.user_id = $1 AND cu.is_active = true
		  AND cu.deleted_at IS NULL AND c.deleted_at IS NULL AND c.is_active = true
		ORDER BY cu.is_primary DESC, cu.joined_at ASC
	`

	rows, err := r.db.Query(ctx, query, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var companies []domain.CompanyBasic
	for rows.Next() {
		var company domain.CompanyBasic
		err := rows.Scan(
			&company.ID, &company.Name, &company.Code, &company.LogoURL,
		)
		if err != nil {
			return nil, err
		}
		companies = append(companies, company)
	}

	return companies, nil
}

// SetPrimaryCompany sets a company as primary for a user
// Returns error if user is not a member of the company
func (r *CompanyUserRepository) SetPrimaryCompany(userID, companyID string) error {
	ctx := context.Background()

	// Start a transaction
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	// First, verify user is a member of the company
	var exists bool
	checkQuery := `
		SELECT EXISTS(
			SELECT 1 FROM core.company_users
			WHERE user_id = $1 AND company_id = $2 AND is_active = true AND deleted_at IS NULL
		)
	`
	err = tx.QueryRow(ctx, checkQuery, userID, companyID).Scan(&exists)
	if err != nil {
		return err
	}
	if !exists {
		return errors.New("user is not a member of this company")
	}

	// Remove primary flag from all user's companies
	updateQuery := `
		UPDATE core.company_users
		SET is_primary = false, updated_at = $1
		WHERE user_id = $2 AND deleted_at IS NULL
	`
	_, err = tx.Exec(ctx, updateQuery, time.Now(), userID)
	if err != nil {
		return err
	}

	// Set the new primary company
	setPrimaryQuery := `
		UPDATE core.company_users
		SET is_primary = true, updated_at = $1
		WHERE user_id = $2 AND company_id = $3 AND deleted_at IS NULL
	`
	_, err = tx.Exec(ctx, setPrimaryQuery, time.Now(), userID, companyID)
	if err != nil {
		return err
	}

	return tx.Commit(ctx)
}

// GetUserRolesAndPermissionsInCompany gets effective roles and permissions for user in company context
// Returns company-specific roles if set, otherwise returns global roles
// This implements the hierarchical role system where company roles override global roles
func (r *CompanyUserRepository) GetUserRolesAndPermissionsInCompany(userID, companyID string) ([]string, []string, error) {
	ctx := context.Background()
	query := `
		SELECT
			COALESCE(
				json_agg(DISTINCT r.name) FILTER (WHERE r.name IS NOT NULL),
				'[]'
			) as roles,
			COALESCE(
				json_agg(DISTINCT (p.resource || ':' || p.action)) FILTER (WHERE p.resource IS NOT NULL),
				'[]'
			) as permissions
		FROM core.company_users cu
		LEFT JOIN core.user_roles ur ON ur.user_id = cu.user_id AND ur.deleted_at IS NULL
		LEFT JOIN core.roles r ON r.id = COALESCE(cu.role_id, ur.role_id) AND r.deleted_at IS NULL
		LEFT JOIN core.role_permissions rp ON rp.role_id = r.id AND rp.deleted_at IS NULL
		LEFT JOIN core.permissions p ON p.id = rp.permission_id AND p.deleted_at IS NULL
		WHERE cu.user_id = $1
		  AND cu.company_id = $2
		  AND cu.deleted_at IS NULL
		  AND cu.is_active = true
	`

	var rolesJSON, permissionsJSON []byte
	err := r.db.QueryRow(ctx, query, userID, companyID).Scan(&rolesJSON, &permissionsJSON)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return []string{}, []string{}, nil
		}
		return nil, nil, err
	}

	var roles, permissions []string
	if err := json.Unmarshal(rolesJSON, &roles); err != nil {
		return nil, nil, fmt.Errorf("failed to unmarshal roles: %w", err)
	}
	if err := json.Unmarshal(permissionsJSON, &permissions); err != nil {
		return nil, nil, fmt.Errorf("failed to unmarshal permissions: %w", err)
	}

	return roles, permissions, nil
}
