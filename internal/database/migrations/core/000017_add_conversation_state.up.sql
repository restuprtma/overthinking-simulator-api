-- =====================================================
-- CORE SCHEMA - REFLECTIONS CONVERSATION STATE
-- =====================================================
-- Migration 017: Add conversation state tracking for 
-- interactive chat functionality in Overthinking Simulator
-- =====================================================

ALTER TABLE core.reflections ADD COLUMN IF NOT EXISTS conversation_state VARCHAR(20) NOT NULL DEFAULT 'initial';
ALTER TABLE core.reflections ADD COLUMN IF NOT EXISTS total_turns INTEGER NOT NULL DEFAULT 0;

-- Backfill rows created before this migration so total_turns is never a lie.
UPDATE core.reflections
SET total_turns = jsonb_array_length(dialog)
WHERE total_turns = 0 AND jsonb_array_length(dialog) > 0;

CREATE INDEX IF NOT EXISTS idx_reflections_conversation_state ON core.reflections(conversation_state);
CREATE INDEX IF NOT EXISTS idx_reflections_total_turns ON core.reflections(total_turns);
