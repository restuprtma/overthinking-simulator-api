-- =====================================================
-- CORE SCHEMA - REFRESH TOKENS
-- =====================================================
-- Migration 003: JWT refresh tokens for session management
-- =====================================================

CREATE TABLE IF NOT EXISTS core.refresh_tokens (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES core.users(id) ON DELETE CASCADE,

    -- Token
    token_hash VARCHAR(255) NOT NULL,

    -- Device info (flexible JSON structure)
    device_info JSONB DEFAULT '{}',
    -- Example: {"name": "Chrome on Windows", "type": "browser", "os": "Windows 11"}

    ip_address VARCHAR(45),  -- Supports IPv6

    -- Lifecycle
    expires_at TIMESTAMPTZ NOT NULL,
    revoked_at TIMESTAMPTZ,
    last_used_at TIMESTAMPTZ,

    -- Audit
    created_at TIMESTAMPTZ DEFAULT NOW()
);

-- Unique constraint on token_hash
CREATE UNIQUE INDEX refresh_tokens_token_hash_key ON core.refresh_tokens(token_hash);

-- Performance indexes
CREATE INDEX idx_refresh_tokens_user_id ON core.refresh_tokens(user_id);
CREATE INDEX idx_refresh_tokens_expires_at ON core.refresh_tokens(expires_at) WHERE revoked_at IS NULL;
CREATE INDEX idx_refresh_tokens_revoked_at ON core.refresh_tokens(revoked_at) WHERE revoked_at IS NOT NULL;

-- =====================================================
-- COMMENTS
-- =====================================================
COMMENT ON TABLE core.refresh_tokens IS 'JWT refresh tokens for session management';
COMMENT ON COLUMN core.refresh_tokens.device_info IS 'JSON object containing device details: name, type, os, browser';
COMMENT ON COLUMN core.refresh_tokens.revoked_at IS 'Set when token is explicitly revoked (logout)';
