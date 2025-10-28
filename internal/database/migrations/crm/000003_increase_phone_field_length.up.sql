-- ============================================================================
-- Migration: Increase phone field length to support WhatsApp Group IDs
-- ============================================================================
-- WhatsApp Group IDs can be very long (e.g., "12345678901234567890_1234567890@g.us")
-- Individual phone numbers are typically 15 chars max (E.164 format)
-- But Group IDs can be 50+ chars
-- ============================================================================

-- Increase phone field length in chats table
ALTER TABLE crm.chats
    ALTER COLUMN phone TYPE VARCHAR(100);

-- Increase phone field length in leads table
ALTER TABLE crm.leads
    ALTER COLUMN phone TYPE VARCHAR(100);

-- Increase phone field length in deals table
ALTER TABLE crm.deals
    ALTER COLUMN phone TYPE VARCHAR(100);

-- Increase customer_phone field length in auto_reply_logs table
ALTER TABLE crm.auto_reply_logs
    ALTER COLUMN customer_phone TYPE VARCHAR(100);

-- Note: VARCHAR(100) is sufficient for:
-- - E.164 phone numbers (max 15 chars): +1234567890123
-- - WhatsApp individual IDs (max 20 chars): 1234567890123@c.us
-- - WhatsApp group IDs (max 60 chars): 12345678901234567890_1234567890@g.us
-- - Leaving room for future formats
