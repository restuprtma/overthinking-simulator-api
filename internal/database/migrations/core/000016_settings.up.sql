-- =====================================================
-- CORE SCHEMA - SETTINGS (key-value)
-- =====================================================
-- Migration 016: A generic key-value settings table used to persist
-- runtime-configurable values (e.g. gemini_credentials) that can be
-- changed from the web without a redeploy.
-- =====================================================

CREATE TABLE IF NOT EXISTS core.settings (
    key VARCHAR(100) PRIMARY KEY,
    value TEXT NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

COMMENT ON TABLE core.settings IS
    'Generic key-value settings store for runtime-configurable values.';
COMMENT ON COLUMN core.settings.value IS
    'Serialized value (often JSON). For gemini_credentials this is a JSON array of {key, model} pairs.';
