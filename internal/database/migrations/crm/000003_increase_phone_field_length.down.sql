-- ============================================================================
-- Rollback: Revert phone field length back to VARCHAR(20)
-- ============================================================================

-- Revert phone field length in chats table
ALTER TABLE crm.chats
    ALTER COLUMN phone TYPE VARCHAR(20);

-- Revert phone field length in leads table
ALTER TABLE crm.leads
    ALTER COLUMN phone TYPE VARCHAR(20);

-- Revert phone field length in deals table
ALTER TABLE crm.deals
    ALTER COLUMN phone TYPE VARCHAR(20);

-- Revert customer_phone field length in auto_reply_logs table
ALTER TABLE crm.auto_reply_logs
    ALTER COLUMN customer_phone TYPE VARCHAR(20);
