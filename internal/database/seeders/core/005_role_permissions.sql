-- =====================================================
-- Role Permissions Assignment
-- =====================================================
-- This file assigns permissions to default roles
--
-- Roles:
-- 1. super_admin (ID: 550e8400-e29b-41d4-a716-446655440001)
--    - Full access to all features
-- 2. admin (ID: 550e8400-e29b-41d4-a716-446655440002)
--    - Full user/company management
--    - Full CRM access
--    - Read-only access to roles/permissions
-- 3. user (ID: 550e8400-e29b-41d4-a716-446655440003)
--    - Profile management
--    - Limited read access to CRM
-- =====================================================

-- Assign ALL permissions to super_admin role dynamically
INSERT INTO core.role_permissions (role_id, permission_id)
SELECT '550e8400-e29b-41d4-a716-446655440001', id
FROM core.permissions
WHERE deleted_at IS NULL
ON CONFLICT (role_id, permission_id) DO NOTHING;

-- =====================================================
-- Admin Role Permissions
-- =====================================================
-- Admin has full access to:
-- - User management (CRUD + Restore)
-- - Company management (CRUD + User management)
-- - CRM modules (Full CRUD)
-- - Read-only access to Roles and Permissions
-- =====================================================

INSERT INTO core.role_permissions (role_id, permission_id) VALUES
-- =====================================================
-- CORE MODULE PERMISSIONS
-- =====================================================

-- User Management (Full CRUD + Restore)
('550e8400-e29b-41d4-a716-446655440002', '650e8400-e29b-41d4-a716-446655440001'), -- users:create
('550e8400-e29b-41d4-a716-446655440002', '650e8400-e29b-41d4-a716-446655440002'), -- users:read
('550e8400-e29b-41d4-a716-446655440002', '650e8400-e29b-41d4-a716-446655440003'), -- users:update
('550e8400-e29b-41d4-a716-446655440002', '650e8400-e29b-41d4-a716-446655440004'), -- users:delete
('550e8400-e29b-41d4-a716-446655440002', '650e8400-e29b-41d4-a716-446655440005'), -- users:restore

-- Role Management (Read only - cannot create/modify roles)
('550e8400-e29b-41d4-a716-446655440002', '650e8400-e29b-41d4-a716-446655440007'), -- roles:read

-- Permission Management (Read only - cannot create/modify permissions)
('550e8400-e29b-41d4-a716-446655440002', '650e8400-e29b-41d4-a716-446655440010'), -- permissions:read

-- Profile Management
('550e8400-e29b-41d4-a716-446655440002', '650e8400-e29b-41d4-a716-446655440011'), -- profile:read
('550e8400-e29b-41d4-a716-446655440002', '650e8400-e29b-41d4-a716-446655440012'), -- profile:update

-- Company Management (Full CRUD + User management)
('550e8400-e29b-41d4-a716-446655440002', '650e8400-e29b-41d4-a716-446655440013'), -- companies:create
('550e8400-e29b-41d4-a716-446655440002', '650e8400-e29b-41d4-a716-446655440014'), -- companies:read
('550e8400-e29b-41d4-a716-446655440002', '650e8400-e29b-41d4-a716-446655440015'), -- companies:update
('550e8400-e29b-41d4-a716-446655440002', '650e8400-e29b-41d4-a716-446655440016'), -- companies:delete
('550e8400-e29b-41d4-a716-446655440002', '650e8400-e29b-41d4-a716-446655440017'), -- companies:manage_users

-- =====================================================
-- CRM MODULE PERMISSIONS
-- =====================================================

-- Lead Sources (Full CRUD + Restore)
('550e8400-e29b-41d4-a716-446655440002', '650e8400-e29b-41d4-a716-446655440101'), -- lead_sources:create
('550e8400-e29b-41d4-a716-446655440002', '650e8400-e29b-41d4-a716-446655440102'), -- lead_sources:read
('550e8400-e29b-41d4-a716-446655440002', '650e8400-e29b-41d4-a716-446655440103'), -- lead_sources:update
('550e8400-e29b-41d4-a716-446655440002', '650e8400-e29b-41d4-a716-446655440104'), -- lead_sources:delete
('550e8400-e29b-41d4-a716-446655440002', '650e8400-e29b-41d4-a716-446655440105'), -- lead_sources:restore

-- Leads (Full CRUD + Special actions)
('550e8400-e29b-41d4-a716-446655440002', '650e8400-e29b-41d4-a716-446655440111'), -- leads:create
('550e8400-e29b-41d4-a716-446655440002', '650e8400-e29b-41d4-a716-446655440112'), -- leads:read
('550e8400-e29b-41d4-a716-446655440002', '650e8400-e29b-41d4-a716-446655440113'), -- leads:update
('550e8400-e29b-41d4-a716-446655440002', '650e8400-e29b-41d4-a716-446655440114'), -- leads:delete
('550e8400-e29b-41d4-a716-446655440002', '650e8400-e29b-41d4-a716-446655440115'), -- leads:assign
('550e8400-e29b-41d4-a716-446655440002', '650e8400-e29b-41d4-a716-446655440116'), -- leads:convert

-- Deals (Full CRUD + Close action)
('550e8400-e29b-41d4-a716-446655440002', '650e8400-e29b-41d4-a716-446655440121'), -- deals:create
('550e8400-e29b-41d4-a716-446655440002', '650e8400-e29b-41d4-a716-446655440122'), -- deals:read
('550e8400-e29b-41d4-a716-446655440002', '650e8400-e29b-41d4-a716-446655440123'), -- deals:update
('550e8400-e29b-41d4-a716-446655440002', '650e8400-e29b-41d4-a716-446655440124'), -- deals:delete
('550e8400-e29b-41d4-a716-446655440002', '650e8400-e29b-41d4-a716-446655440125'), -- deals:close

-- Sales Targets (Full CRUD)
('550e8400-e29b-41d4-a716-446655440002', '650e8400-e29b-41d4-a716-446655440131'), -- sales_targets:create
('550e8400-e29b-41d4-a716-446655440002', '650e8400-e29b-41d4-a716-446655440132'), -- sales_targets:read
('550e8400-e29b-41d4-a716-446655440002', '650e8400-e29b-41d4-a716-446655440133'), -- sales_targets:update
('550e8400-e29b-41d4-a716-446655440002', '650e8400-e29b-41d4-a716-446655440134'), -- sales_targets:delete

-- Activities (Full CRUD)
('550e8400-e29b-41d4-a716-446655440002', '650e8400-e29b-41d4-a716-446655440141'), -- activities:create
('550e8400-e29b-41d4-a716-446655440002', '650e8400-e29b-41d4-a716-446655440142'), -- activities:read
('550e8400-e29b-41d4-a716-446655440002', '650e8400-e29b-41d4-a716-446655440143'), -- activities:update
('550e8400-e29b-41d4-a716-446655440002', '650e8400-e29b-41d4-a716-446655440144'), -- activities:delete

-- CRM Reports (Read + Export)
('550e8400-e29b-41d4-a716-446655440002', '650e8400-e29b-41d4-a716-446655440151'), -- crm_reports:read
('550e8400-e29b-41d4-a716-446655440002', '650e8400-e29b-41d4-a716-446655440152'), -- crm_reports:export

-- Sales Persons (Full CRUD + Restore)
('550e8400-e29b-41d4-a716-446655440002', '650e8400-e29b-41d4-a716-446655440171'), -- sales_persons:create
('550e8400-e29b-41d4-a716-446655440002', '650e8400-e29b-41d4-a716-446655440172'), -- sales_persons:read
('550e8400-e29b-41d4-a716-446655440002', '650e8400-e29b-41d4-a716-446655440173'), -- sales_persons:update
('550e8400-e29b-41d4-a716-446655440002', '650e8400-e29b-41d4-a716-446655440174'), -- sales_persons:delete
('550e8400-e29b-41d4-a716-446655440002', '650e8400-e29b-41d4-a716-446655440175'), -- sales_persons:restore

-- Auto-Reply Rules (Full CRUD + Restore)
('550e8400-e29b-41d4-a716-446655440002', '650e8400-e29b-41d4-a716-446655440181'), -- auto_reply_rules:create
('550e8400-e29b-41d4-a716-446655440002', '650e8400-e29b-41d4-a716-446655440182'), -- auto_reply_rules:read
('550e8400-e29b-41d4-a716-446655440002', '650e8400-e29b-41d4-a716-446655440183'), -- auto_reply_rules:update
('550e8400-e29b-41d4-a716-446655440002', '650e8400-e29b-41d4-a716-446655440184'), -- auto_reply_rules:delete
('550e8400-e29b-41d4-a716-446655440002', '650e8400-e29b-41d4-a716-446655440185'), -- auto_reply_rules:restore

-- Chats (Full CRUD + Restore)
('550e8400-e29b-41d4-a716-446655440002', '650e8400-e29b-41d4-a716-446655440191'), -- chats:create
('550e8400-e29b-41d4-a716-446655440002', '650e8400-e29b-41d4-a716-446655440192'), -- chats:read
('550e8400-e29b-41d4-a716-446655440002', '650e8400-e29b-41d4-a716-446655440193'), -- chats:update
('550e8400-e29b-41d4-a716-446655440002', '650e8400-e29b-41d4-a716-446655440194'), -- chats:delete
('550e8400-e29b-41d4-a716-446655440002', '650e8400-e29b-41d4-a716-446655440195'), -- chats:restore

-- Company Settings (Read + Update)
('550e8400-e29b-41d4-a716-446655440002', '650e8400-e29b-41d4-a716-446655440201'), -- company_settings:read
('550e8400-e29b-41d4-a716-446655440002', '650e8400-e29b-41d4-a716-446655440202'), -- company_settings:update

-- Permission Templates (Full CRUD + Restore)
('550e8400-e29b-41d4-a716-446655440002', '650e8400-e29b-41d4-a716-446655440161'), -- permission_templates:create
('550e8400-e29b-41d4-a716-446655440002', '650e8400-e29b-41d4-a716-446655440162'), -- permission_templates:read
('550e8400-e29b-41d4-a716-446655440002', '650e8400-e29b-41d4-a716-446655440163'), -- permission_templates:update
('550e8400-e29b-41d4-a716-446655440002', '650e8400-e29b-41d4-a716-446655440164'), -- permission_templates:delete
('550e8400-e29b-41d4-a716-446655440002', '650e8400-e29b-41d4-a716-446655440165')  -- permission_templates:restore
ON CONFLICT (role_id, permission_id) DO NOTHING;

-- =====================================================
-- User Role Permissions
-- =====================================================
-- Basic user has access to:
-- - Profile management only
-- - Read-only access to assigned CRM data
-- =====================================================

INSERT INTO core.role_permissions (role_id, permission_id) VALUES
-- Profile Management
('550e8400-e29b-41d4-a716-446655440003', '650e8400-e29b-41d4-a716-446655440011'), -- profile:read
('550e8400-e29b-41d4-a716-446655440003', '650e8400-e29b-41d4-a716-446655440012'), -- profile:update

-- Limited CRM Read Access (can view data assigned to them)
('550e8400-e29b-41d4-a716-446655440003', '650e8400-e29b-41d4-a716-446655440102'), -- lead_sources:read
('550e8400-e29b-41d4-a716-446655440003', '650e8400-e29b-41d4-a716-446655440112'), -- leads:read
('550e8400-e29b-41d4-a716-446655440003', '650e8400-e29b-41d4-a716-446655440122'), -- deals:read
('550e8400-e29b-41d4-a716-446655440003', '650e8400-e29b-41d4-a716-446655440142'), -- activities:read
('550e8400-e29b-41d4-a716-446655440003', '650e8400-e29b-41d4-a716-446655440172'), -- sales_persons:read
('550e8400-e29b-41d4-a716-446655440003', '650e8400-e29b-41d4-a716-446655440182'), -- auto_reply_rules:read
('550e8400-e29b-41d4-a716-446655440003', '650e8400-e29b-41d4-a716-446655440192'), -- chats:read
('550e8400-e29b-41d4-a716-446655440003', '650e8400-e29b-41d4-a716-446655440201')  -- company_settings:read
ON CONFLICT (role_id, permission_id) DO NOTHING;

-- =====================================================
-- Role Module Templates Assignment
-- =====================================================
-- This section maps default roles to their module templates
-- This helps the UI understand which template was used for each module
-- Note: Only sub-modules (depth > 0) are assigned templates
-- =====================================================

-- =====================================================
-- Admin Role Module Templates
-- =====================================================
-- Admin role template assignments for sub-modules:
-- - core.users: Admin template (CRUD + Restore)
-- - core.roles: Viewer template (Read only)
-- - core.permissions: Viewer template (Read only)
-- - core.profile: Editor template (CRUD)
-- - core.companies: Manager template (CRUD + manage_users)
-- - core.permission_templates: Admin template (CRUD + Restore)
-- - crm.leads: Manager template (CRUD + assign + convert)
-- - crm.deals: Manager template (CRUD + close)
-- - crm.sales_targets: Editor template (CRUD)
-- - crm.activities: Editor template (CRUD)
-- - crm.reports: Viewer template (Read + Export handled separately)
-- - crm.sales_persons: Admin template (CRUD + Restore)
-- - crm.auto_reply_rules: Admin template (CRUD + Restore)
-- - crm.chats: Admin template (CRUD + Restore)
-- - crm.company_settings: Editor template (Read + Update)
-- =====================================================

INSERT INTO core.role_module_templates (role_id, module_id, permission_template_id) VALUES
-- Core sub-modules (from 002_default_modules.sql)
('550e8400-e29b-41d4-a716-446655440002', 'a10e8400-e29b-41d4-a716-446655440006', '750e8400-e29b-41d4-a716-446655440003'),  -- core.users → Admin
('550e8400-e29b-41d4-a716-446655440002', 'a10e8400-e29b-41d4-a716-446655440007', '750e8400-e29b-41d4-a716-446655440001'),  -- core.roles → Viewer
('550e8400-e29b-41d4-a716-446655440002', 'a10e8400-e29b-41d4-a716-446655440008', '750e8400-e29b-41d4-a716-446655440001'),  -- core.permissions → Viewer
('550e8400-e29b-41d4-a716-446655440002', 'a10e8400-e29b-41d4-a716-446655440009', '750e8400-e29b-41d4-a716-446655440004'),  -- core.companies → Manager
('550e8400-e29b-41d4-a716-446655440002', 'a10e8400-e29b-41d4-a716-446655440010', '750e8400-e29b-41d4-a716-446655440003'),  -- core.permission_templates → Admin
('550e8400-e29b-41d4-a716-446655440002', 'a10e8400-e29b-41d4-a716-446655440011', '750e8400-e29b-41d4-a716-446655440002'),  -- core.profile → Editor

-- CRM sub-modules (from 002_default_modules.sql)
('550e8400-e29b-41d4-a716-446655440002', 'a10e8400-e29b-41d4-a716-446655440012', '750e8400-e29b-41d4-a716-446655440003'),  -- crm.chats → Admin
('550e8400-e29b-41d4-a716-446655440002', 'a10e8400-e29b-41d4-a716-446655440013', '750e8400-e29b-41d4-a716-446655440004'),  -- crm.leads → Manager
('550e8400-e29b-41d4-a716-446655440002', 'a10e8400-e29b-41d4-a716-446655440014', '750e8400-e29b-41d4-a716-446655440004'),  -- crm.deals → Manager
('550e8400-e29b-41d4-a716-446655440002', 'a10e8400-e29b-41d4-a716-446655440015', '750e8400-e29b-41d4-a716-446655440003'),  -- crm.sales_persons → Admin
('550e8400-e29b-41d4-a716-446655440002', 'a10e8400-e29b-41d4-a716-446655440016', '750e8400-e29b-41d4-a716-446655440002'),  -- crm.sales_targets → Editor
('550e8400-e29b-41d4-a716-446655440002', 'a10e8400-e29b-41d4-a716-446655440017', '750e8400-e29b-41d4-a716-446655440003'),  -- crm.auto_reply_rules → Admin
('550e8400-e29b-41d4-a716-446655440002', 'a10e8400-e29b-41d4-a716-446655440018', '750e8400-e29b-41d4-a716-446655440002'),  -- crm.activities → Editor
('550e8400-e29b-41d4-a716-446655440002', 'a10e8400-e29b-41d4-a716-446655440019', '750e8400-e29b-41d4-a716-446655440002'),  -- crm.company_settings → Editor
('550e8400-e29b-41d4-a716-446655440002', 'a10e8400-e29b-41d4-a716-446655440020', '750e8400-e29b-41d4-a716-446655440001')   -- crm.reports → Viewer
ON CONFLICT (role_id, module_id) DO NOTHING;

-- =====================================================
-- User Role Module Templates
-- =====================================================
-- Basic user role template assignments for sub-modules:
-- - core.profile: Editor template (CRUD)
-- - All CRM sub-modules: Viewer template (Read only)
-- =====================================================

INSERT INTO core.role_module_templates (role_id, module_id, permission_template_id) VALUES
-- Core sub-modules
('550e8400-e29b-41d4-a716-446655440003', 'a10e8400-e29b-41d4-a716-446655440011', '750e8400-e29b-41d4-a716-446655440002'),  -- core.profile → Editor

-- CRM sub-modules (all read-only for basic user)
('550e8400-e29b-41d4-a716-446655440003', 'a10e8400-e29b-41d4-a716-446655440012', '750e8400-e29b-41d4-a716-446655440001'),  -- crm.chats → Viewer
('550e8400-e29b-41d4-a716-446655440003', 'a10e8400-e29b-41d4-a716-446655440013', '750e8400-e29b-41d4-a716-446655440001'),  -- crm.leads → Viewer
('550e8400-e29b-41d4-a716-446655440003', 'a10e8400-e29b-41d4-a716-446655440014', '750e8400-e29b-41d4-a716-446655440001'),  -- crm.deals → Viewer
('550e8400-e29b-41d4-a716-446655440003', 'a10e8400-e29b-41d4-a716-446655440015', '750e8400-e29b-41d4-a716-446655440001'),  -- crm.sales_persons → Viewer
('550e8400-e29b-41d4-a716-446655440003', 'a10e8400-e29b-41d4-a716-446655440017', '750e8400-e29b-41d4-a716-446655440001'),  -- crm.auto_reply_rules → Viewer
('550e8400-e29b-41d4-a716-446655440003', 'a10e8400-e29b-41d4-a716-446655440018', '750e8400-e29b-41d4-a716-446655440001'),  -- crm.activities → Viewer
('550e8400-e29b-41d4-a716-446655440003', 'a10e8400-e29b-41d4-a716-446655440019', '750e8400-e29b-41d4-a716-446655440001')   -- crm.company_settings → Viewer
ON CONFLICT (role_id, module_id) DO NOTHING;
