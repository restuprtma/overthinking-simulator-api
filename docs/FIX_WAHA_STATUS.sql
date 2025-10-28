-- Fix WAHA Status Issue
-- Problem: Some sales persons have waha_session_status = 'STOPPED' even though they never connected
-- Solution: Clear WAHA fields for sales persons that have status but no actual session name

-- Show affected records before fix
SELECT
    id,
    sales_name,
    whatsapp,
    waha_session_name,
    waha_session_status,
    is_whatsapp_connected,
    waha_connected_at
FROM crm.sales_persons
WHERE waha_session_status IS NOT NULL
  AND waha_session_name IS NULL;

-- Uncomment below to apply the fix:
-- UPDATE crm.sales_persons
-- SET
--     waha_session_status = NULL,
--     waha_pairing_code = NULL,
--     waha_pairing_code_expires_at = NULL,
--     waha_last_seen_at = NULL,
--     waha_connected_at = NULL,
--     waha_disconnected_at = NULL,
--     is_whatsapp_connected = false,
--     updated_at = NOW()
-- WHERE waha_session_status IS NOT NULL
--   AND waha_session_name IS NULL;

-- Note: This clears WAHA fields only for records that have a status but no session name
-- If a session name exists, it means there was a real connection attempt
