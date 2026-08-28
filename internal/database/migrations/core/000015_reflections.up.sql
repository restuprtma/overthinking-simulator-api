-- =====================================================
-- CORE SCHEMA - REFLECTIONS
-- =====================================================
-- Migration 015: Persisted reflection sessions for the Overthinking
-- Simulator MVP. Each row is scoped to a single user.
-- =====================================================

CREATE TABLE IF NOT EXISTS core.reflections (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES core.users(id) ON DELETE CASCADE,
    thought TEXT NOT NULL,
    detected_distortions JSONB NOT NULL DEFAULT '[]',
    core_fear TEXT NOT NULL,
    dialog JSONB NOT NULL DEFAULT '[]',
    actionable_suggestion TEXT NOT NULL,
    safety_triggered BOOLEAN NOT NULL DEFAULT FALSE,
    safety_response TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_reflections_user_id_created ON core.reflections(user_id, created_at DESC);
