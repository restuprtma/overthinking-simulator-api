-- =====================================================
-- Sales Persons Seeder
-- =====================================================
-- Creates sales persons from existing users and links to PT Venturo Pro
-- Uses users from core.company_users (user01-user20)
-- Company: PT Venturo Pro (ID: 750e8400-e29b-41d4-a716-446655440001)
-- =====================================================

-- Insert 10 sales persons from the first 10 users
-- Each sales person will have:
-- - Unique sales code
-- - Sales area
-- - Monthly target
-- - Commission rate
-- - WhatsApp number

-- Note: is_whatsapp_connected is set to FALSE for all by default
-- WAHA session fields (waha_session_name, waha_session_status, etc.) are NULL until actual connection
INSERT INTO crm.sales_persons (
    id,
    company_user_id,
    sales_code,
    sales_name,
    sales_area,
    sales_target,
    commission_rate,
    whatsapp,
    is_active,
    notes,
    created_by
) VALUES
-- Sales Person 01 - Jakarta Area
(
    'c50e8400-e29b-41d4-a716-446655440001',
    'b50e8400-e29b-41d4-a716-446655440001', -- company_user_id from user01
    'SP-001',
    'Budi Santoso',
    'Jakarta Pusat',
    50000000.00, -- 50 juta target per bulan
    5.00, -- 5% commission
    '+6281234567801',
    TRUE,
    'Sales untuk area Jakarta Pusat',
    '850e8400-e29b-41d4-a716-446655440001' -- Super admin
),

-- Sales Person 02 - Jakarta Area
(
    'c50e8400-e29b-41d4-a716-446655440002',
    'b50e8400-e29b-41d4-a716-446655440002',
    'SP-002',
    'Siti Nurhaliza',
    'Jakarta Selatan',
    45000000.00,
    5.00,
    '+6281234567802',
    TRUE,
    'Sales untuk area Jakarta Selatan',
    '850e8400-e29b-41d4-a716-446655440001'
),

-- Sales Person 03 - Jakarta Area
(
    'c50e8400-e29b-41d4-a716-446655440003',
    'b50e8400-e29b-41d4-a716-446655440003',
    'SP-003',
    'Agus Setiawan',
    'Jakarta Barat',
    45000000.00,
    5.00,
    '+6281234567803',
    TRUE,
    'Sales untuk area Jakarta Barat',
    '850e8400-e29b-41d4-a716-446655440001'
),

-- Sales Person 04 - Jakarta Area
(
    'c50e8400-e29b-41d4-a716-446655440004',
    'b50e8400-e29b-41d4-a716-446655440004',
    'SP-004',
    'Dewi Lestari',
    'Jakarta Timur',
    40000000.00,
    5.00,
    '+6281234567804',
    TRUE,
    'Sales untuk area Jakarta Timur',
    '850e8400-e29b-41d4-a716-446655440001'
),

-- Sales Person 05 - Jakarta Area
(
    'c50e8400-e29b-41d4-a716-446655440005',
    'b50e8400-e29b-41d4-a716-446655440005',
    'SP-005',
    'Rudi Hartono',
    'Jakarta Utara',
    40000000.00,
    5.00,
    '+6281234567805',
    TRUE,
    'Sales untuk area Jakarta Utara',
    '850e8400-e29b-41d4-a716-446655440001'
),

-- Sales Person 06 - Jabodetabek Area
(
    'c50e8400-e29b-41d4-a716-446655440006',
    'b50e8400-e29b-41d4-a716-446655440006',
    'SP-006',
    'Rina Wijaya',
    'Tangerang',
    35000000.00,
    5.50,
    '+6281234567806',
    TRUE,
    'Sales untuk area Tangerang dan sekitarnya',
    '850e8400-e29b-41d4-a716-446655440001'
),

-- Sales Person 07 - Jabodetabek Area
(
    'c50e8400-e29b-41d4-a716-446655440007',
    'b50e8400-e29b-41d4-a716-446655440007',
    'SP-007',
    'Andi Prasetyo',
    'Bekasi',
    35000000.00,
    5.50,
    '+6281234567807',
    TRUE,
    'Sales untuk area Bekasi dan sekitarnya',
    '850e8400-e29b-41d4-a716-446655440001'
),

-- Sales Person 08 - Jabodetabek Area
(
    'c50e8400-e29b-41d4-a716-446655440008',
    'b50e8400-e29b-41d4-a716-446655440008',
    'SP-008',
    'Fitri Rahayu',
    'Depok',
    30000000.00,
    5.50,
    '+6281234567808',
    TRUE,
    'Sales untuk area Depok dan sekitarnya',
    '850e8400-e29b-41d4-a716-446655440001'
),

-- Sales Person 09 - Jabodetabek Area
(
    'c50e8400-e29b-41d4-a716-446655440009',
    'b50e8400-e29b-41d4-a716-446655440009',
    'SP-009',
    'Joko Widodo',
    'Bogor',
    30000000.00,
    5.50,
    '+6281234567809',
    TRUE,
    'Sales untuk area Bogor dan sekitarnya',
    '850e8400-e29b-41d4-a716-446655440001'
),

-- Sales Person 10 - Jawa Barat Area
(
    'c50e8400-e29b-41d4-a716-446655440010',
    'b50e8400-e29b-41d4-a716-446655440010',
    'SP-010',
    'Maya Sari',
    'Bandung',
    40000000.00,
    6.00,
    '+6281234567810',
    TRUE,
    'Sales untuk area Bandung dan Jawa Barat',
    '850e8400-e29b-41d4-a716-446655440001'
)
ON CONFLICT (id) DO NOTHING;

-- =====================================================
-- Summary
-- =====================================================
-- Total Sales Persons: 10
-- Company: PT Venturo Pro
-- Sales Areas:
--   - Jakarta Pusat, Selatan, Barat, Timur, Utara (5 sales)
--   - Tangerang, Bekasi, Depok, Bogor (4 sales)
--   - Bandung (1 sales)
-- Total Monthly Target: 390 juta
-- Commission Rate: 5% - 6%
-- WhatsApp Numbers: All 10 have WhatsApp numbers configured
-- WhatsApp Connected: 0 out of 10 (need to connect via WAHA)
--
-- Note: is_whatsapp_connected defaults to FALSE
-- To connect WhatsApp, use POST /sales-persons/{id}/whatsapp/connect
-- =====================================================
