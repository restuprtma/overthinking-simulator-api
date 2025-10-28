-- =====================================================
-- Default Permissions Seeder
-- =====================================================
-- This seeder creates default permissions with module grouping
-- Permissions follow the pattern: module:resource:action
-- =====================================================

-- Insert default permissions
INSERT INTO core.permissions (id, module_id, resource, action, description) VALUES
-- =====================================================
-- CORE MODULE PERMISSIONS
-- =====================================================

-- User management permissions (Module: core.users - a10e8400-e29b-41d4-a716-446655440006)
('650e8400-e29b-41d4-a716-446655440001', 'a10e8400-e29b-41d4-a716-446655440006', 'core.users', 'create', 'Create new users'),
('650e8400-e29b-41d4-a716-446655440002', 'a10e8400-e29b-41d4-a716-446655440006', 'core.users', 'read', 'View users'),
('650e8400-e29b-41d4-a716-446655440003', 'a10e8400-e29b-41d4-a716-446655440006', 'core.users', 'update', 'Update users'),
('650e8400-e29b-41d4-a716-446655440004', 'a10e8400-e29b-41d4-a716-446655440006', 'core.users', 'delete', 'Delete users'),
('650e8400-e29b-41d4-a716-446655440005', 'a10e8400-e29b-41d4-a716-446655440006', 'core.users', 'restore', 'Restore deleted users'),

-- Role management permissions (Module: core.roles - a10e8400-e29b-41d4-a716-446655440007)
('650e8400-e29b-41d4-a716-446655440006', 'a10e8400-e29b-41d4-a716-446655440007', 'core.roles', 'create', 'Create new roles'),
('650e8400-e29b-41d4-a716-446655440007', 'a10e8400-e29b-41d4-a716-446655440007', 'core.roles', 'read', 'View roles'),
('650e8400-e29b-41d4-a716-446655440008', 'a10e8400-e29b-41d4-a716-446655440007', 'core.roles', 'update', 'Update roles'),
('650e8400-e29b-41d4-a716-446655440009', 'a10e8400-e29b-41d4-a716-446655440007', 'core.roles', 'delete', 'Delete roles'),
('650e8400-e29b-41d4-a716-446655440037', 'a10e8400-e29b-41d4-a716-446655440007', 'core.roles', 'restore', 'Restore deleted roles'),

-- Permission management permissions (Module: core.permissions - a10e8400-e29b-41d4-a716-446655440008)
('650e8400-e29b-41d4-a716-446655440010', 'a10e8400-e29b-41d4-a716-446655440008', 'core.permissions', 'read', 'View permissions'),
('650e8400-e29b-41d4-a716-446655440038', 'a10e8400-e29b-41d4-a716-446655440008', 'core.permissions', 'create', 'Create new permissions'),
('650e8400-e29b-41d4-a716-446655440039', 'a10e8400-e29b-41d4-a716-446655440008', 'core.permissions', 'update', 'Update permissions'),
('650e8400-e29b-41d4-a716-446655440040', 'a10e8400-e29b-41d4-a716-446655440008', 'core.permissions', 'delete', 'Delete permissions'),

-- Profile management permissions (Module: core.profile - a10e8400-e29b-41d4-a716-446655440011)
('650e8400-e29b-41d4-a716-446655440011', 'a10e8400-e29b-41d4-a716-446655440011', 'core.profile', 'read', 'View own profile'),
('650e8400-e29b-41d4-a716-446655440012', 'a10e8400-e29b-41d4-a716-446655440011', 'core.profile', 'update', 'Update own profile'),

-- Company management permissions (Module: core.companies - a10e8400-e29b-41d4-a716-446655440009)
('650e8400-e29b-41d4-a716-446655440013', 'a10e8400-e29b-41d4-a716-446655440009', 'core.companies', 'create', 'Create new companies'),
('650e8400-e29b-41d4-a716-446655440014', 'a10e8400-e29b-41d4-a716-446655440009', 'core.companies', 'read', 'View companies'),
('650e8400-e29b-41d4-a716-446655440015', 'a10e8400-e29b-41d4-a716-446655440009', 'core.companies', 'update', 'Update companies'),
('650e8400-e29b-41d4-a716-446655440016', 'a10e8400-e29b-41d4-a716-446655440009', 'core.companies', 'delete', 'Delete companies'),
('650e8400-e29b-41d4-a716-446655440017', 'a10e8400-e29b-41d4-a716-446655440009', 'core.companies', 'manage_users', 'Manage users in companies'),

-- Permission Templates management permissions (Module: core.permission_templates - a10e8400-e29b-41d4-a716-446655440010)
('650e8400-e29b-41d4-a716-446655440161', 'a10e8400-e29b-41d4-a716-446655440010', 'core.permission_templates', 'create', 'Create permission templates'),
('650e8400-e29b-41d4-a716-446655440162', 'a10e8400-e29b-41d4-a716-446655440010', 'core.permission_templates', 'read', 'View permission templates'),
('650e8400-e29b-41d4-a716-446655440163', 'a10e8400-e29b-41d4-a716-446655440010', 'core.permission_templates', 'update', 'Update permission templates'),
('650e8400-e29b-41d4-a716-446655440164', 'a10e8400-e29b-41d4-a716-446655440010', 'core.permission_templates', 'delete', 'Delete permission templates'),
('650e8400-e29b-41d4-a716-446655440165', 'a10e8400-e29b-41d4-a716-446655440010', 'core.permission_templates', 'restore', 'Restore deleted permission templates'),

-- =====================================================
-- CRM MODULE PERMISSIONS
-- =====================================================

-- CRM: Lead Sources permissions (Note: No dedicated sub-module, using CRM root - a10e8400-e29b-41d4-a716-446655440002)
('650e8400-e29b-41d4-a716-446655440101', 'a10e8400-e29b-41d4-a716-446655440002', 'crm.lead_sources', 'create', 'Create new lead sources'),
('650e8400-e29b-41d4-a716-446655440102', 'a10e8400-e29b-41d4-a716-446655440002', 'crm.lead_sources', 'read', 'View lead sources'),
('650e8400-e29b-41d4-a716-446655440103', 'a10e8400-e29b-41d4-a716-446655440002', 'crm.lead_sources', 'update', 'Update lead sources'),
('650e8400-e29b-41d4-a716-446655440104', 'a10e8400-e29b-41d4-a716-446655440002', 'crm.lead_sources', 'delete', 'Delete lead sources'),
('650e8400-e29b-41d4-a716-446655440105', 'a10e8400-e29b-41d4-a716-446655440002', 'crm.lead_sources', 'restore', 'Restore deleted lead sources'),

-- CRM: Leads permissions (Module: crm.leads - a10e8400-e29b-41d4-a716-446655440013)
('650e8400-e29b-41d4-a716-446655440111', 'a10e8400-e29b-41d4-a716-446655440013', 'crm.leads', 'create', 'Create new leads'),
('650e8400-e29b-41d4-a716-446655440112', 'a10e8400-e29b-41d4-a716-446655440013', 'crm.leads', 'read', 'View leads'),
('650e8400-e29b-41d4-a716-446655440113', 'a10e8400-e29b-41d4-a716-446655440013', 'crm.leads', 'update', 'Update leads'),
('650e8400-e29b-41d4-a716-446655440114', 'a10e8400-e29b-41d4-a716-446655440013', 'crm.leads', 'delete', 'Delete leads'),
('650e8400-e29b-41d4-a716-446655440115', 'a10e8400-e29b-41d4-a716-446655440013', 'crm.leads', 'assign', 'Assign leads to users'),
('650e8400-e29b-41d4-a716-446655440116', 'a10e8400-e29b-41d4-a716-446655440013', 'crm.leads', 'convert', 'Convert leads to deals'),

-- CRM: Deals permissions (Module: crm.deals - a10e8400-e29b-41d4-a716-446655440014)
('650e8400-e29b-41d4-a716-446655440121', 'a10e8400-e29b-41d4-a716-446655440014', 'crm.deals', 'create', 'Create new deals'),
('650e8400-e29b-41d4-a716-446655440122', 'a10e8400-e29b-41d4-a716-446655440014', 'crm.deals', 'read', 'View deals'),
('650e8400-e29b-41d4-a716-446655440123', 'a10e8400-e29b-41d4-a716-446655440014', 'crm.deals', 'update', 'Update deals'),
('650e8400-e29b-41d4-a716-446655440124', 'a10e8400-e29b-41d4-a716-446655440014', 'crm.deals', 'delete', 'Delete deals'),
('650e8400-e29b-41d4-a716-446655440125', 'a10e8400-e29b-41d4-a716-446655440014', 'crm.deals', 'close', 'Close deals (won/lost)'),

-- CRM: Sales Targets permissions (Module: crm.sales_targets - a10e8400-e29b-41d4-a716-446655440016)
('650e8400-e29b-41d4-a716-446655440131', 'a10e8400-e29b-41d4-a716-446655440016', 'crm.sales_targets', 'create', 'Create sales targets'),
('650e8400-e29b-41d4-a716-446655440132', 'a10e8400-e29b-41d4-a716-446655440016', 'crm.sales_targets', 'read', 'View sales targets'),
('650e8400-e29b-41d4-a716-446655440133', 'a10e8400-e29b-41d4-a716-446655440016', 'crm.sales_targets', 'update', 'Update sales targets'),
('650e8400-e29b-41d4-a716-446655440134', 'a10e8400-e29b-41d4-a716-446655440016', 'crm.sales_targets', 'delete', 'Delete sales targets'),

-- CRM: Activities permissions (Module: crm.activities - a10e8400-e29b-41d4-a716-446655440018)
('650e8400-e29b-41d4-a716-446655440141', 'a10e8400-e29b-41d4-a716-446655440018', 'crm.activities', 'create', 'Create activities (calls, meetings, notes)'),
('650e8400-e29b-41d4-a716-446655440142', 'a10e8400-e29b-41d4-a716-446655440018', 'crm.activities', 'read', 'View activities'),
('650e8400-e29b-41d4-a716-446655440143', 'a10e8400-e29b-41d4-a716-446655440018', 'crm.activities', 'update', 'Update activities'),
('650e8400-e29b-41d4-a716-446655440144', 'a10e8400-e29b-41d4-a716-446655440018', 'crm.activities', 'delete', 'Delete activities'),

-- CRM: Reports permissions (Module: crm.reports - a10e8400-e29b-41d4-a716-446655440020)
('650e8400-e29b-41d4-a716-446655440151', 'a10e8400-e29b-41d4-a716-446655440020', 'crm.reports', 'read', 'View CRM reports and analytics'),
('650e8400-e29b-41d4-a716-446655440152', 'a10e8400-e29b-41d4-a716-446655440020', 'crm.reports', 'export', 'Export CRM reports'),

-- CRM: Sales Persons permissions (Module: crm.sales_persons - a10e8400-e29b-41d4-a716-446655440015)
('650e8400-e29b-41d4-a716-446655440171', 'a10e8400-e29b-41d4-a716-446655440015', 'crm.sales_persons', 'create', 'Create new sales persons'),
('650e8400-e29b-41d4-a716-446655440172', 'a10e8400-e29b-41d4-a716-446655440015', 'crm.sales_persons', 'read', 'View sales persons'),
('650e8400-e29b-41d4-a716-446655440173', 'a10e8400-e29b-41d4-a716-446655440015', 'crm.sales_persons', 'update', 'Update sales persons'),
('650e8400-e29b-41d4-a716-446655440174', 'a10e8400-e29b-41d4-a716-446655440015', 'crm.sales_persons', 'delete', 'Delete sales persons'),
('650e8400-e29b-41d4-a716-446655440175', 'a10e8400-e29b-41d4-a716-446655440015', 'crm.sales_persons', 'restore', 'Restore deleted sales persons'),
('650e8400-e29b-41d4-a716-446655440176', 'a10e8400-e29b-41d4-a716-446655440015', 'crm.sales_persons', 'manage', 'Manage sales persons WhatsApp sessions and settings'),

-- CRM: Auto-Reply Rules permissions (Module: crm.auto_reply_rules - a10e8400-e29b-41d4-a716-446655440017)
('650e8400-e29b-41d4-a716-446655440181', 'a10e8400-e29b-41d4-a716-446655440017', 'crm.auto_reply_rules', 'create', 'Create auto-reply rules'),
('650e8400-e29b-41d4-a716-446655440182', 'a10e8400-e29b-41d4-a716-446655440017', 'crm.auto_reply_rules', 'read', 'View auto-reply rules'),
('650e8400-e29b-41d4-a716-446655440183', 'a10e8400-e29b-41d4-a716-446655440017', 'crm.auto_reply_rules', 'update', 'Update auto-reply rules'),
('650e8400-e29b-41d4-a716-446655440184', 'a10e8400-e29b-41d4-a716-446655440017', 'crm.auto_reply_rules', 'delete', 'Delete auto-reply rules'),
('650e8400-e29b-41d4-a716-446655440185', 'a10e8400-e29b-41d4-a716-446655440017', 'crm.auto_reply_rules', 'restore', 'Restore deleted auto-reply rules'),

-- CRM: Chats permissions (Module: crm.chats - a10e8400-e29b-41d4-a716-446655440012)
('650e8400-e29b-41d4-a716-446655440191', 'a10e8400-e29b-41d4-a716-446655440012', 'crm.chats', 'create', 'Create new chat sessions'),
('650e8400-e29b-41d4-a716-446655440192', 'a10e8400-e29b-41d4-a716-446655440012', 'crm.chats', 'read', 'View chats and messages'),
('650e8400-e29b-41d4-a716-446655440193', 'a10e8400-e29b-41d4-a716-446655440012', 'crm.chats', 'update', 'Update chats'),
('650e8400-e29b-41d4-a716-446655440194', 'a10e8400-e29b-41d4-a716-446655440012', 'crm.chats', 'delete', 'Delete chats'),
('650e8400-e29b-41d4-a716-446655440195', 'a10e8400-e29b-41d4-a716-446655440012', 'crm.chats', 'restore', 'Restore deleted chats'),

-- CRM: Company Settings permissions (Module: crm.company_settings - a10e8400-e29b-41d4-a716-446655440019)
('650e8400-e29b-41d4-a716-446655440201', 'a10e8400-e29b-41d4-a716-446655440019', 'crm.company_settings', 'read', 'View company CRM settings'),
('650e8400-e29b-41d4-a716-446655440202', 'a10e8400-e29b-41d4-a716-446655440019', 'crm.company_settings', 'update', 'Update company CRM settings')
ON CONFLICT (id) DO NOTHING;
