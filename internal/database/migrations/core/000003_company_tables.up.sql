-- Company Management Tables

-- Companies table
CREATE TABLE IF NOT EXISTS core.companies (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    owner_id UUID NOT NULL,
    name VARCHAR(255) NOT NULL,
    code VARCHAR(50) UNIQUE NOT NULL,
    tax_id VARCHAR(50),
    phone VARCHAR(20),
    email VARCHAR(255),
    website VARCHAR(255),
    logo_url VARCHAR(500),
    address TEXT,
    village_id UUID,
    max_users INT DEFAULT 5,
    max_branches INT DEFAULT 1,
    is_active BOOLEAN DEFAULT TRUE,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    created_by UUID,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_by UUID,
    deleted_at TIMESTAMP,
    deleted_by UUID
);

-- Company Users table (junction table for user-company relationship)
-- Supports hierarchical roles: company-specific roles override global roles
CREATE TABLE IF NOT EXISTS core.company_users (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    company_id UUID NOT NULL,
    user_id UUID NOT NULL,
    role_id UUID, -- Company-specific role. If NULL, inherits global role (no foreign key constraint)
    role VARCHAR(50) NOT NULL DEFAULT 'MEMBER', -- DEPRECATED: Use role_id instead. Kept for backward compatibility
    is_primary BOOLEAN DEFAULT FALSE,
    is_active BOOLEAN DEFAULT TRUE,
    invited_by UUID,
    joined_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    created_by UUID,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_by UUID,
    deleted_at TIMESTAMP,
    deleted_by UUID
    -- Note: UNIQUE constraint handled separately with partial index (see migration 000004)
);

-- Create indexes for company tables
CREATE INDEX idx_companies_owner_id ON core.companies(owner_id);
CREATE INDEX idx_companies_code ON core.companies(code);
CREATE INDEX idx_companies_deleted_at ON core.companies(deleted_at);
CREATE INDEX idx_companies_is_active ON core.companies(is_active);
CREATE INDEX idx_companies_created_by ON core.companies(created_by);
CREATE INDEX idx_companies_village_id ON core.companies(village_id);

CREATE INDEX idx_company_users_company_id ON core.company_users(company_id);
CREATE INDEX idx_company_users_user_id ON core.company_users(user_id);
CREATE INDEX idx_company_users_role_id ON core.company_users(role_id); -- For hierarchical role lookups
CREATE INDEX idx_company_users_role ON core.company_users(role); -- DEPRECATED: For backward compatibility
CREATE INDEX idx_company_users_is_primary ON core.company_users(is_primary);
CREATE INDEX idx_company_users_is_active ON core.company_users(is_active);
CREATE INDEX idx_company_users_deleted_at ON core.company_users(deleted_at);

-- Partial unique index: ensures only one active (non-deleted) record per (company_id, user_id)
-- This allows users to be re-added to a company after being removed (soft deleted)
CREATE UNIQUE INDEX company_users_company_id_user_id_active_key
ON core.company_users (company_id, user_id)
WHERE deleted_at IS NULL;
