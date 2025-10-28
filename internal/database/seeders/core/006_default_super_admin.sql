-- Insert default super admin user
-- Email: tantowilathif@gmail.com
-- Password: Bismillah1407*
-- Bcrypt hash generated with cost 10 (bcrypt.DefaultCost)

-- Insert user
INSERT INTO core.users (id, email, username, password_hash, full_name, is_active, is_email_verified) VALUES
(
    '850e8400-e29b-41d4-a716-446655440001',
    'tantowi@gmail.com',
    'tantowilathif',
    '$2a$10$sFtr9jQBR4chlbQrONKfReM28NZUbGNP0I2BG3LHgKWjd.tFS8pDy',
    'Super Administrator',
    TRUE,
    TRUE
)
ON CONFLICT (id) DO NOTHING;

-- Assign super_admin role to the user
INSERT INTO core.user_roles (user_id, role_id) VALUES
(
    '850e8400-e29b-41d4-a716-446655440001',
    '550e8400-e29b-41d4-a716-446655440001'
)
ON CONFLICT (user_id, role_id) DO NOTHING;
