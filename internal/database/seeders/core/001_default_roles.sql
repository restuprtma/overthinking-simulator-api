-- Insert default roles
INSERT INTO core.roles (id, name, description, is_system) VALUES
('550e8400-e29b-41d4-a716-446655440001', 'super_admin', 'Super Administrator with full access', TRUE),
('550e8400-e29b-41d4-a716-446655440002', 'admin', 'Administrator with limited access', TRUE),
('550e8400-e29b-41d4-a716-446655440003', 'user', 'Regular user with basic access', TRUE)
ON CONFLICT (id) DO NOTHING;
