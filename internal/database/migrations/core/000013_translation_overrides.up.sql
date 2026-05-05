-- =====================================================
-- CORE SCHEMA - TRANSLATION OVERRIDES (per-client)
-- =====================================================
-- Migration 013: Admin-editable overrides for FE i18n strings,
-- scoped per-client. Two clients can have different values for the
-- same translation_key.
--
-- Design notes:
--   - Per-client (FK to core.clients). Mutations restricted to
--     super_admin / client admin; the list endpoint is public so the
--     FE can bootstrap translations before login (?slug=acme).
--   - Key format permits i18next namespaces ("contacts:title") and
--     dotted nested keys ("contacts:nested.key").
--   - created_by / updated_by reference core.users for audit; ON DELETE
--     SET NULL so removing a user does not cascade-delete overrides.
-- =====================================================

CREATE TABLE IF NOT EXISTS core.translation_overrides (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    client_id       UUID NOT NULL REFERENCES core.clients(id) ON DELETE CASCADE,
    translation_key VARCHAR(128) NOT NULL,
    value           TEXT NOT NULL,
    notes           TEXT,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    created_by      UUID REFERENCES core.users(id) ON DELETE SET NULL,
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_by      UUID REFERENCES core.users(id) ON DELETE SET NULL,

    CONSTRAINT chk_translation_overrides_value_not_empty
        CHECK (LENGTH(TRIM(value)) > 0),
    CONSTRAINT chk_translation_overrides_key_format
        CHECK (translation_key ~ '^[a-z0-9._:-]+$'),
    CONSTRAINT uq_translation_overrides_client_key
        UNIQUE (client_id, translation_key)
);

CREATE INDEX idx_translation_overrides_client
    ON core.translation_overrides (client_id);

CREATE INDEX idx_translation_overrides_updated_at
    ON core.translation_overrides (updated_at DESC);

COMMENT ON TABLE core.translation_overrides IS
    'Admin-editable i18n string overrides scoped per-client. Public list endpoint used by FE bootstrap; mutations restricted to super_admin / client admin.';
COMMENT ON COLUMN core.translation_overrides.client_id IS
    'Client that owns this override. Overrides are scoped per-client; two clients can have different values for the same translation_key. CASCADE on client delete cleans up orphans.';
COMMENT ON COLUMN core.translation_overrides.translation_key IS
    'Stable i18n key. Format: ^[a-z0-9._:-]+$ (lowercase, digits, hyphen, dot, colon). Colon separates the i18next namespace; dot separates nested keys.';
COMMENT ON COLUMN core.translation_overrides.value IS
    'Override text rendered by the FE in place of the built-in default.';
COMMENT ON COLUMN core.translation_overrides.notes IS
    'Optional admin-facing note — not shown to end users.';
