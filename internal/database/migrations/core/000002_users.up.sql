-- =====================================================
-- CORE SCHEMA - USERS
-- =====================================================
-- Migration 002: User accounts with authentication fields
-- =====================================================

CREATE TABLE IF NOT EXISTS core.users (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),

    -- Identity
    email VARCHAR(255) NOT NULL,
    username VARCHAR(100) NOT NULL,
    password_hash VARCHAR(255) NOT NULL,
    full_name VARCHAR(255),
    phone VARCHAR(20),
    avatar_url VARCHAR(500),

    -- Status
    is_active BOOLEAN DEFAULT TRUE,
    is_email_verified BOOLEAN DEFAULT FALSE,
    email_verified_at TIMESTAMPTZ,

    -- Security
    failed_login_count INT DEFAULT 0,
    locked_until TIMESTAMPTZ,
    last_login_at TIMESTAMPTZ,

    -- Audit
    created_at TIMESTAMPTZ DEFAULT NOW(),
    created_by UUID,
    updated_at TIMESTAMPTZ DEFAULT NOW(),
    updated_by UUID,
    deleted_at TIMESTAMPTZ,
    deleted_by UUID
);

-- Partial unique indexes (allow re-registration after soft delete)
CREATE UNIQUE INDEX users_email_active_key ON core.users(email) WHERE deleted_at IS NULL;
CREATE UNIQUE INDEX users_username_active_key ON core.users(username) WHERE deleted_at IS NULL;

-- Performance indexes
CREATE INDEX idx_users_email ON core.users(email);
CREATE INDEX idx_users_username ON core.users(username);
CREATE INDEX idx_users_is_active ON core.users(is_active) WHERE deleted_at IS NULL;
CREATE INDEX idx_users_deleted_at ON core.users(deleted_at) WHERE deleted_at IS NOT NULL;

-- =====================================================
-- COMMENTS
-- =====================================================
COMMENT ON TABLE core.users IS 'User accounts with authentication fields';
COMMENT ON COLUMN core.users.email_verified_at IS 'Timestamp when email was verified (NULL = not verified)';
COMMENT ON COLUMN core.users.failed_login_count IS 'Counter for failed login attempts, reset on successful login';
COMMENT ON COLUMN core.users.locked_until IS 'Account locked until this timestamp after max failed attempts';
