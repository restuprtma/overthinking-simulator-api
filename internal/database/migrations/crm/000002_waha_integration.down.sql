-- Rollback WAHA integration changes

-- Drop indexes
DROP INDEX IF EXISTS crm.uq_sales_persons_waha_session_name;
DROP INDEX IF EXISTS crm.idx_sales_persons_waha_session_status;
DROP INDEX IF EXISTS crm.idx_sales_persons_waha_session_name;

-- Drop columns
ALTER TABLE crm.sales_persons
DROP COLUMN IF EXISTS waha_disconnected_at,
DROP COLUMN IF EXISTS waha_connected_at,
DROP COLUMN IF EXISTS waha_last_seen_at,
DROP COLUMN IF EXISTS waha_pairing_code_expires_at,
DROP COLUMN IF EXISTS waha_pairing_code,
DROP COLUMN IF EXISTS waha_session_status,
DROP COLUMN IF EXISTS waha_session_name;
