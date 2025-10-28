-- =====================================================
-- Default Permission Templates
-- =====================================================
-- This seeder creates default permission templates
-- Templates group common action patterns for easier role management
-- =====================================================

-- Insert default permission templates
INSERT INTO core.permission_templates (id, name, description, is_system) VALUES
('750e8400-e29b-41d4-a716-446655440001', 'Viewer', 'Read-only access to resources', TRUE),
('750e8400-e29b-41d4-a716-446655440002', 'Editor', 'Create, read, update, delete, and restore resources', TRUE)
ON CONFLICT (id) DO NOTHING;

-- =====================================================
-- Template: Viewer (Read-only)
-- =====================================================
INSERT INTO core.permission_template_actions (permission_template_id, action) VALUES
('750e8400-e29b-41d4-a716-446655440001', 'read')
ON CONFLICT (permission_template_id, action) DO NOTHING;

-- =====================================================
-- Template: Editor (CRUD + Restore)
-- =====================================================
INSERT INTO core.permission_template_actions (permission_template_id, action) VALUES
('750e8400-e29b-41d4-a716-446655440002', 'create'),
('750e8400-e29b-41d4-a716-446655440002', 'read'),
('750e8400-e29b-41d4-a716-446655440002', 'update'),
('750e8400-e29b-41d4-a716-446655440002', 'delete'),
('750e8400-e29b-41d4-a716-446655440002', 'restore')
ON CONFLICT (permission_template_id, action) DO NOTHING;
