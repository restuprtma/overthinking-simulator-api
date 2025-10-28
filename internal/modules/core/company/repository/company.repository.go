package repository

import (
	"context"
	"errors"
	"time"

	"venturo-skeleton-go/internal/modules/core/company/domain"
	"venturo-skeleton-go/internal/shared/query"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type CompanyRepository struct {
	db *pgxpool.Pool
}

func NewCompanyRepository(db *pgxpool.Pool) *CompanyRepository {
	return &CompanyRepository{db: db}
}

// Create creates a new company
func (r *CompanyRepository) Create(company *domain.Company) error {
	ctx := context.Background()
	query := `
		INSERT INTO core.companies (
			id, owner_id, name, code, tax_id, phone, email, website, logo_url,
			address, village_id, max_users, max_branches, is_active,
			created_at, created_by, updated_at, updated_by
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17, $18)
	`

	_, err := r.db.Exec(ctx, query,
		company.ID, company.OwnerID, company.Name, company.Code,
		company.TaxID, company.Phone, company.Email, company.Website, company.LogoURL,
		company.Address, company.VillageID, company.MaxUsers, company.MaxBranches,
		company.IsActive, company.CreatedAt, company.CreatedBy,
		company.UpdatedAt, company.UpdatedBy,
	)

	return err
}

// FindByID finds a company by ID
func (r *CompanyRepository) FindByID(id string) (*domain.Company, error) {
	ctx := context.Background()

	baseQuery := `
		SELECT c.id, c.owner_id, c.name, c.code, c.tax_id, c.phone, c.email, c.website, c.logo_url,
		       c.address, c.village_id, c.max_users, c.max_branches, c.is_active,
		       c.created_at, COALESCE(u1.full_name, c.created_by::TEXT) as created_by,
		       c.updated_at, COALESCE(u2.full_name, c.updated_by::TEXT) as updated_by,
		       c.deleted_at, COALESCE(u3.full_name, c.deleted_by::TEXT) as deleted_by
		FROM core.companies c
		LEFT JOIN core.users u1 ON c.created_by = u1.id
		LEFT JOIN core.users u2 ON c.updated_by = u2.id
		LEFT JOIN core.users u3 ON c.deleted_by = u3.id`

	qb := query.NewQueryBuilder(baseQuery)
	sql, args := qb.
		AddFilter("c.id", "=", id).
		AddCustomCondition("c.deleted_at IS NULL").
		Build()

	var company domain.Company
	err := r.db.QueryRow(ctx, sql, args...).Scan(
		&company.ID, &company.OwnerID, &company.Name, &company.Code,
		&company.TaxID, &company.Phone, &company.Email, &company.Website, &company.LogoURL,
		&company.Address, &company.VillageID, &company.MaxUsers, &company.MaxBranches,
		&company.IsActive, &company.CreatedAt, &company.CreatedBy,
		&company.UpdatedAt, &company.UpdatedBy, &company.DeletedAt, &company.DeletedBy,
	)

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}

	return &company, nil
}

// FindByCode finds a company by code
func (r *CompanyRepository) FindByCode(code string) (*domain.Company, error) {
	ctx := context.Background()

	baseQuery := `
		SELECT c.id, c.owner_id, c.name, c.code, c.tax_id, c.phone, c.email, c.website, c.logo_url,
		       c.address, c.village_id, c.max_users, c.max_branches, c.is_active,
		       c.created_at, COALESCE(u1.full_name, c.created_by::TEXT) as created_by,
		       c.updated_at, COALESCE(u2.full_name, c.updated_by::TEXT) as updated_by,
		       c.deleted_at, COALESCE(u3.full_name, c.deleted_by::TEXT) as deleted_by
		FROM core.companies c
		LEFT JOIN core.users u1 ON c.created_by = u1.id
		LEFT JOIN core.users u2 ON c.updated_by = u2.id
		LEFT JOIN core.users u3 ON c.deleted_by = u3.id`

	qb := query.NewQueryBuilder(baseQuery)
	sql, args := qb.
		AddFilter("c.code", "=", code).
		AddCustomCondition("c.deleted_at IS NULL").
		Build()

	var company domain.Company
	err := r.db.QueryRow(ctx, sql, args...).Scan(
		&company.ID, &company.OwnerID, &company.Name, &company.Code,
		&company.TaxID, &company.Phone, &company.Email, &company.Website, &company.LogoURL,
		&company.Address, &company.VillageID, &company.MaxUsers, &company.MaxBranches,
		&company.IsActive, &company.CreatedAt, &company.CreatedBy,
		&company.UpdatedAt, &company.UpdatedBy, &company.DeletedAt, &company.DeletedBy,
	)

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}

	return &company, nil
}

// FindAll finds all companies with pagination and filters
func (r *CompanyRepository) FindAll(limit, offset int, search string, isActive *bool, ownerID string) ([]domain.Company, error) {
	ctx := context.Background()

	baseQuery := `
		SELECT c.id, c.owner_id, c.name, c.code, c.tax_id, c.phone, c.email, c.website, c.logo_url,
		       c.address, c.village_id, c.max_users, c.max_branches, c.is_active,
		       c.created_at, COALESCE(u1.full_name, c.created_by::TEXT) as created_by,
		       c.updated_at, COALESCE(u2.full_name, c.updated_by::TEXT) as updated_by,
		       c.deleted_at, COALESCE(u3.full_name, c.deleted_by::TEXT) as deleted_by
		FROM core.companies c
		LEFT JOIN core.users u1 ON c.created_by = u1.id
		LEFT JOIN core.users u2 ON c.updated_by = u2.id
		LEFT JOIN core.users u3 ON c.deleted_by = u3.id`

	qb := query.NewQueryBuilder(baseQuery)
	qb.AddCustomCondition("c.deleted_at IS NULL").
		AddSearchFilter(search, "c.name", "c.code", "c.email").
		AddIsActiveFilter(isActive, "c.is_active")

	if ownerID != "" {
		qb.AddFilter("c.owner_id", "=", ownerID)
	}

	sql, args := qb.
		AddOrderBy("c.created_at", "DESC").
		AddPagination(limit, offset).
		Build()

	rows, err := r.db.Query(ctx, sql, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var companies []domain.Company
	for rows.Next() {
		var company domain.Company
		err := rows.Scan(
			&company.ID, &company.OwnerID, &company.Name, &company.Code,
			&company.TaxID, &company.Phone, &company.Email, &company.Website, &company.LogoURL,
			&company.Address, &company.VillageID, &company.MaxUsers, &company.MaxBranches,
			&company.IsActive, &company.CreatedAt, &company.CreatedBy,
			&company.UpdatedAt, &company.UpdatedBy, &company.DeletedAt, &company.DeletedBy,
		)
		if err != nil {
			return nil, err
		}
		companies = append(companies, company)
	}

	return companies, nil
}

// Count counts total companies with filters
func (r *CompanyRepository) Count(search string, isActive *bool, ownerID string) (int64, error) {
	ctx := context.Background()

	baseQuery := "SELECT COUNT(*) FROM core.companies c"

	qb := query.NewQueryBuilder(baseQuery)
	qb.AddCustomCondition("c.deleted_at IS NULL").
		AddSearchFilter(search, "c.name", "c.code", "c.email").
		AddIsActiveFilter(isActive, "c.is_active")

	if ownerID != "" {
		qb.AddFilter("c.owner_id", "=", ownerID)
	}

	sql, args := qb.Build()

	var count int64
	err := r.db.QueryRow(ctx, sql, args...).Scan(&count)
	return count, err
}

// Update updates a company
func (r *CompanyRepository) Update(company *domain.Company) error {
	ctx := context.Background()
	query := `
		UPDATE core.companies
		SET name = $1, tax_id = $2, phone = $3, email = $4, website = $5,
		    logo_url = $6, address = $7, village_id = $8, max_users = $9,
		    max_branches = $10, is_active = $11, updated_at = $12, updated_by = $13
		WHERE id = $14 AND deleted_at IS NULL
	`

	_, err := r.db.Exec(ctx, query,
		company.Name, company.TaxID, company.Phone, company.Email, company.Website,
		company.LogoURL, company.Address, company.VillageID, company.MaxUsers,
		company.MaxBranches, company.IsActive, company.UpdatedAt, company.UpdatedBy,
		company.ID,
	)

	return err
}

// Delete soft deletes a company
func (r *CompanyRepository) Delete(companyID string, deletedBy string) error {
	ctx := context.Background()
	query := `
		UPDATE core.companies
		SET deleted_at = $1, deleted_by = $2
		WHERE id = $3 AND deleted_at IS NULL
	`

	_, err := r.db.Exec(ctx, query, time.Now(), deletedBy, companyID)
	return err
}

// CountCompaniesByOwner counts companies owned by a specific user
func (r *CompanyRepository) CountCompaniesByOwner(ownerID string) (int64, error) {
	ctx := context.Background()

	baseQuery := "SELECT COUNT(*) FROM core.companies c"

	qb := query.NewQueryBuilder(baseQuery)
	sql, args := qb.
		AddFilter("c.owner_id", "=", ownerID).
		AddCustomCondition("c.deleted_at IS NULL").
		Build()

	var count int64
	err := r.db.QueryRow(ctx, sql, args...).Scan(&count)
	return count, err
}
