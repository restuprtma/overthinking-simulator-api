-- =====================================================
-- CORE SCHEMA - COMPANY USERS
-- =====================================================
-- Migration 007: User membership in companies (many-to-many)
-- =====================================================

CREATE TABLE IF NOT EXISTS core.company_users (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),

    -- Relations
    company_id UUID NOT NULL REFERENCES core.companies(id) ON DELETE CASCADE,
    user_id UUID NOT NULL REFERENCES core.users(id) ON DELETE CASCADE,
    role_id UUID REFERENCES core.roles(id) ON DELETE SET NULL,

    -- Status
    is_primary BOOLEAN DEFAULT FALSE,  -- User's primary/default company
    is_active BOOLEAN DEFAULT TRUE,

    -- Metadata
    invited_by UUID REFERENCES core.users(id),
    joined_at TIMESTAMPTZ DEFAULT NOW(),

    -- Audit
    created_at TIMESTAMPTZ DEFAULT NOW(),
    created_by UUID,
    updated_at TIMESTAMPTZ DEFAULT NOW(),
    updated_by UUID,
    deleted_at TIMESTAMPTZ,
    deleted_by UUID
);

-- Unique constraint: one active membership per user per company
CREATE UNIQUE INDEX company_users_company_user_active_key
    ON core.company_users(company_id, user_id) WHERE deleted_at IS NULL;

-- Ensure only one primary company per user
CREATE UNIQUE INDEX company_users_user_primary_key
    ON core.company_users(user_id) WHERE is_primary = TRUE AND deleted_at IS NULL;

-- Performance indexes
CREATE INDEX idx_company_users_company_id ON core.company_users(company_id) WHERE deleted_at IS NULL;
CREATE INDEX idx_company_users_user_id ON core.company_users(user_id) WHERE deleted_at IS NULL;
CREATE INDEX idx_company_users_role_id ON core.company_users(role_id) WHERE deleted_at IS NULL;
CREATE INDEX idx_company_users_is_primary ON core.company_users(is_primary) WHERE deleted_at IS NULL;
CREATE INDEX idx_company_users_is_active ON core.company_users(is_active) WHERE deleted_at IS NULL;

-- =====================================================
-- COMMENTS
-- =====================================================
COMMENT ON TABLE core.company_users IS 'User membership in companies (many-to-many)';
COMMENT ON COLUMN core.company_users.is_primary IS 'Marks user''s default company (only one per user)';
COMMENT ON COLUMN core.company_users.role_id IS 'User''s role within this specific company';

-- =====================================================
-- HELPER FUNCTIONS
-- =====================================================

-- Get user's companies with roles
CREATE OR REPLACE FUNCTION core.get_user_companies(p_user_id UUID)
RETURNS TABLE (
    company_id UUID,
    company_name VARCHAR,
    role_code VARCHAR,
    role_name VARCHAR,
    is_primary BOOLEAN
) AS $$
BEGIN
    RETURN QUERY
    SELECT
        c.id,
        c.name,
        r.code,
        r.name,
        cu.is_primary
    FROM core.company_users cu
    JOIN core.companies c ON c.id = cu.company_id
    LEFT JOIN core.roles r ON r.id = cu.role_id
    WHERE cu.user_id = p_user_id
      AND cu.is_active = TRUE
      AND cu.deleted_at IS NULL
      AND c.deleted_at IS NULL
    ORDER BY cu.is_primary DESC, c.name;
END;
$$ LANGUAGE plpgsql STABLE;

COMMENT ON FUNCTION core.get_user_companies IS 'Get all companies a user belongs to with their roles';
