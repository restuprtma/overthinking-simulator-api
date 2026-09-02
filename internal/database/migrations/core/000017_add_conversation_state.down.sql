-- =====================================================
-- CORE SCHEMA - REFLECTIONS CONVERSATION STATE (ROLLBACK)
-- =====================================================
-- Migration 017 Down: Remove conversation state tracking columns
-- =====================================================

DROP INDEX IF EXISTS core.idx_reflections_conversation_state;
DROP INDEX IF EXISTS core.idx_reflections_total_turns;

ALTER TABLE core.reflections DROP COLUMN IF EXISTS conversation_state;
ALTER TABLE core.reflections DROP COLUMN IF EXISTS total_turns;
