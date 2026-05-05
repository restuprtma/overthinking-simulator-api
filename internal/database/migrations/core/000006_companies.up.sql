-- =====================================================
-- CORE SCHEMA - COMPANIES
-- =====================================================
-- Migration 006: Hierarchical company structure
--   Also adds the deferred FKs from roles & user_roles to companies
--   (those columns were created in 000004_roles / 000005_user_roles).
-- =====================================================

-- Company type enum
CREATE TYPE core.company_type AS ENUM (
    'holding',      -- Top-level parent company
    'subsidiary'    -- Child company (owned by holding)
);

CREATE TABLE IF NOT EXISTS core.companies (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),

    -- Hierarchy
    parent_id UUID REFERENCES core.companies(id) ON DELETE RESTRICT,

    -- Identity
    name VARCHAR(255) NOT NULL,
    type core.company_type DEFAULT 'subsidiary',

    -- Details
    logo_url VARCHAR(500),

    -- Ownership
    owner_id UUID NOT NULL REFERENCES core.users(id),

    -- Display
    sort INT DEFAULT 0,

    -- Status
    is_active BOOLEAN DEFAULT TRUE,

    -- Audit
    created_at TIMESTAMPTZ DEFAULT NOW(),
    created_by UUID,
    updated_at TIMESTAMPTZ DEFAULT NOW(),
    updated_by UUID,
    deleted_at TIMESTAMPTZ,
    deleted_by UUID
);

-- Performance indexes
CREATE INDEX idx_companies_parent_id ON core.companies(parent_id) WHERE deleted_at IS NULL;
CREATE INDEX idx_companies_type ON core.companies(type) WHERE deleted_at IS NULL;
CREATE INDEX idx_companies_owner_id ON core.companies(owner_id) WHERE deleted_at IS NULL;
CREATE INDEX idx_companies_is_active ON core.companies(is_active) WHERE deleted_at IS NULL;
CREATE INDEX idx_companies_sort ON core.companies(sort) WHERE deleted_at IS NULL;

-- =====================================================
-- ADD FOREIGN KEY TO ROLES & USER_ROLES (companies now exists)
-- =====================================================
ALTER TABLE core.roles
    ADD CONSTRAINT fk_roles_company_id
    FOREIGN KEY (company_id) REFERENCES core.companies(id) ON DELETE CASCADE;

ALTER TABLE core.user_roles
    ADD CONSTRAINT fk_user_roles_company_id
    FOREIGN KEY (company_id) REFERENCES core.companies(id) ON DELETE CASCADE;

-- =====================================================
-- COMMENTS
-- =====================================================
COMMENT ON TABLE core.companies IS 'Hierarchical company structure';
COMMENT ON COLUMN core.companies.parent_id IS 'Reference to parent company (NULL for root/holding)';
COMMENT ON COLUMN core.companies.type IS 'Company type: holding, subsidiary';

-- =====================================================
-- HELPER FUNCTIONS
-- =====================================================

-- Get all ancestors of a company (using recursive CTE)
CREATE OR REPLACE FUNCTION core.get_company_ancestors(p_company_id UUID)
RETURNS TABLE (
    id UUID,
    name VARCHAR,
    level INT
) AS $$
BEGIN
    RETURN QUERY
    WITH RECURSIVE ancestors AS (
        SELECT c.id, c.name, c.parent_id, 0 AS lvl
        FROM core.companies c
        WHERE c.id = (SELECT parent_id FROM core.companies WHERE id = p_company_id)
          AND c.deleted_at IS NULL
        UNION ALL
        SELECT c.id, c.name, c.parent_id, a.lvl + 1
        FROM core.companies c
        JOIN ancestors a ON c.id = a.parent_id
        WHERE c.deleted_at IS NULL
    )
    SELECT ancestors.id, ancestors.name, ancestors.lvl
    FROM ancestors
    ORDER BY ancestors.lvl DESC;
END;
$$ LANGUAGE plpgsql STABLE;

-- Get all descendants of a company (using recursive CTE)
CREATE OR REPLACE FUNCTION core.get_company_descendants(p_company_id UUID)
RETURNS TABLE (
    id UUID,
    name VARCHAR,
    level INT
) AS $$
BEGIN
    RETURN QUERY
    WITH RECURSIVE descendants AS (
        SELECT c.id, c.name, c.parent_id, 1 AS lvl
        FROM core.companies c
        WHERE c.parent_id = p_company_id
          AND c.deleted_at IS NULL
        UNION ALL
        SELECT c.id, c.name, c.parent_id, d.lvl + 1
        FROM core.companies c
        JOIN descendants d ON c.parent_id = d.id
        WHERE c.deleted_at IS NULL
    )
    SELECT descendants.id, descendants.name, descendants.lvl
    FROM descendants
    ORDER BY descendants.lvl, descendants.name;
END;
$$ LANGUAGE plpgsql STABLE;

COMMENT ON FUNCTION core.get_company_ancestors IS 'Get all parent companies up to root';
COMMENT ON FUNCTION core.get_company_descendants IS 'Get all child companies recursively';
