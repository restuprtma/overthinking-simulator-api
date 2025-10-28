-- Add WAHA session metadata fields to sales_persons table
-- This migration adds fields to track WhatsApp connection status via WAHA

ALTER TABLE crm.sales_persons
ADD COLUMN IF NOT EXISTS waha_session_name VARCHAR(100),
ADD COLUMN IF NOT EXISTS waha_session_status VARCHAR(50) DEFAULT 'STOPPED',
ADD COLUMN IF NOT EXISTS waha_pairing_code VARCHAR(20),
ADD COLUMN IF NOT EXISTS waha_pairing_code_expires_at TIMESTAMP,
ADD COLUMN IF NOT EXISTS waha_last_seen_at TIMESTAMP,
ADD COLUMN IF NOT EXISTS waha_connected_at TIMESTAMP,
ADD COLUMN IF NOT EXISTS waha_disconnected_at TIMESTAMP;

-- Add indexes for WAHA session lookups
CREATE INDEX IF NOT EXISTS idx_sales_persons_waha_session_name
ON crm.sales_persons(waha_session_name)
WHERE waha_session_name IS NOT NULL;

CREATE INDEX IF NOT EXISTS idx_sales_persons_waha_session_status
ON crm.sales_persons(waha_session_status);

-- Add unique constraint on waha_session_name (one session per sales person)
CREATE UNIQUE INDEX IF NOT EXISTS uq_sales_persons_waha_session_name
ON crm.sales_persons(waha_session_name)
WHERE waha_session_name IS NOT NULL AND deleted_at IS NULL;

-- Add comments for documentation
COMMENT ON COLUMN crm.sales_persons.waha_session_name IS 'Unique session name in WAHA (format: sp_{id}_{timestamp})';
COMMENT ON COLUMN crm.sales_persons.waha_session_status IS 'WAHA session status: STOPPED, STARTING, SCAN_QR_CODE, WORKING, FAILED';
COMMENT ON COLUMN crm.sales_persons.waha_pairing_code IS 'Last generated pairing code for WhatsApp authentication';
COMMENT ON COLUMN crm.sales_persons.waha_pairing_code_expires_at IS 'Expiration timestamp for the pairing code';
COMMENT ON COLUMN crm.sales_persons.waha_last_seen_at IS 'Last activity timestamp from WAHA webhook';
COMMENT ON COLUMN crm.sales_persons.waha_connected_at IS 'Timestamp when WhatsApp was successfully connected';
COMMENT ON COLUMN crm.sales_persons.waha_disconnected_at IS 'Timestamp when WhatsApp was disconnected';
