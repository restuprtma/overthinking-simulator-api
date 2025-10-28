-- Insert default company: PT Venturo Pro
-- This company will be assigned to the super admin user

-- Insert company
INSERT INTO core.companies (
    id,
    owner_id,
    name,
    code,
    tax_id,
    phone,
    email,
    website,
    address,
    max_users,
    max_branches,
    is_active
) VALUES (
    '750e8400-e29b-41d4-a716-446655440001',
    '850e8400-e29b-41d4-a716-446655440001', -- Super admin user ID
    'PT Venturo Pro',
    'VENTURO-PRO',
    NULL, -- Optional: Add NPWP if needed
    NULL, -- Optional: Add phone if needed
    NULL, -- Optional: Add email if needed
    NULL, -- Optional: Add website if needed
    NULL, -- Optional: Add address if needed
    50, -- Max users
    10, -- Max branches
    TRUE
)
ON CONFLICT (id) DO NOTHING;

-- Assign super admin as primary user of the company
INSERT INTO core.company_users (
    id,
    company_id,
    user_id,
    role,
    is_primary,
    is_active,
    invited_by
) VALUES (
    '650e8400-e29b-41d4-a716-446655440001',
    '750e8400-e29b-41d4-a716-446655440001', -- Company ID
    '850e8400-e29b-41d4-a716-446655440001', -- Super admin user ID
    'OWNER',
    TRUE,
    TRUE,
    '850e8400-e29b-41d4-a716-446655440001' -- Self-invited
)
ON CONFLICT (id) DO NOTHING;