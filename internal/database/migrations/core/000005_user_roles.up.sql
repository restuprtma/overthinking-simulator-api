-- =====================================================
-- CORE SCHEMA - USER ROLES
-- =====================================================
-- Migration 005: Junction table linking users to roles with optional company scope
-- =====================================================

CREATE TABLE IF NOT EXISTS core.user_roles (
    user_id UUID NOT NULL REFERENCES core.users(id) ON DELETE CASCADE,
    role_id UUID NOT NULL REFERENCES core.roles(id) ON DELETE CASCADE,

    -- Scope: which company this role assignment applies to
    -- NULL = global assignment (user has this role everywhere)
    -- UUID = role only applies within specific company (FK added in 000006_companies)
    company_id UUID,

    -- Audit
    created_at TIMESTAMPTZ DEFAULT NOW(),
    created_by UUID,

    PRIMARY KEY (user_id, role_id)
);

-- Partial unique index to prevent duplicate role assignments per company
CREATE UNIQUE INDEX user_roles_user_role_company_key
    ON core.user_roles(user_id, role_id, company_id)
    WHERE company_id IS NOT NULL;
CREATE UNIQUE INDEX user_roles_user_role_global_key
    ON core.user_roles(user_id, role_id)
    WHERE company_id IS NULL;

-- Performance indexes
CREATE INDEX idx_user_roles_user_id ON core.user_roles(user_id);
CREATE INDEX idx_user_roles_role_id ON core.user_roles(role_id);
CREATE INDEX idx_user_roles_company_id ON core.user_roles(company_id) WHERE company_id IS NOT NULL;

-- =====================================================
-- COMMENTS
-- =====================================================
COMMENT ON TABLE core.user_roles IS 'Junction table linking users to roles with optional company scope';
COMMENT ON COLUMN core.user_roles.company_id IS 'NULL = global assignment, UUID = company-scoped assignment';
