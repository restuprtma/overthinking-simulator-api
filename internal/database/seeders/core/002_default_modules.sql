-- =====================================================
-- Default Modules Seeder
-- =====================================================
-- This seeder creates default modules for grouping permissions
-- Modules are used to organize resources in the system
-- Supports hierarchical tree structure for dynamic menu
-- =====================================================

-- =====================================================
-- ROOT MODULES (MenuItem Level - Depth 0)
-- =====================================================
INSERT INTO core.modules (
    id, code, name, description, icon, color,
    parent_id, depth, heading, "to", url, permission, tooltip,
    is_menu_visible, sort_order, is_active
) VALUES
-- Core Module
(
    'a10e8400-e29b-41d4-a716-446655440001',
    'core',
    'Core Module',
    'Authentication, Users, Roles, Permissions, and Company Management',
    'shield',
    '#3B82F6',
    NULL, -- Root module
    0,    -- Depth 0
    'System Management',
    '/core',
    '/core',
    'core',
    'Core System Management',
    TRUE,
    1,
    TRUE
),
-- CRM Module
(
    'a10e8400-e29b-41d4-a716-446655440002',
    'crm',
    'CRM',
    'Customer Relationship Management - Leads, Deals, Sales Targets',
    'users',
    '#10B981',
    NULL, -- Root module
    0,    -- Depth 0
    'Customer Relationship',
    '/crm',
    '/crm',
    'crm',
    'Customer Relationship Management',
    TRUE,
    2,
    TRUE
),
-- Finance Module
(
    'a10e8400-e29b-41d4-a716-446655440003',
    'finance',
    'Finance',
    'Financial Management - Invoices, Expenses, COA',
    'dollar-sign',
    '#F59E0B',
    NULL, -- Root module
    0,    -- Depth 0
    'Financial Management',
    '/finance',
    '/finance',
    'finance',
    'Finance & Accounting',
    TRUE,
    3,
    TRUE
),
-- HR Module
(
    'a10e8400-e29b-41d4-a716-446655440004',
    'hr',
    'Human Resources',
    'HR Management - Employees, Attendance, Payroll',
    'user-check',
    '#8B5CF6',
    NULL, -- Root module
    0,    -- Depth 0
    'Human Resources',
    '/hr',
    '/hr',
    'hr',
    'HR Management',
    TRUE,
    4,
    TRUE
),
-- Inventory Module
(
    'a10e8400-e29b-41d4-a716-446655440005',
    'inventory',
    'Inventory',
    'Inventory Management - Products, Stock, Warehouses',
    'package',
    '#EF4444',
    NULL, -- Root module
    0,    -- Depth 0
    'Inventory Control',
    '/inventory',
    '/inventory',
    'inventory',
    'Inventory Management',
    TRUE,
    5,
    TRUE
)
ON CONFLICT (id) DO NOTHING;

-- =====================================================
-- CORE SUB-MODULES (ChildItem Level - Depth 1)
-- =====================================================
INSERT INTO core.modules (
    id, code, name, description, icon, color,
    parent_id, depth, "to", url, permission, tooltip,
    is_menu_visible, sort_order, is_active
) VALUES
-- Users Feature
(
    'a10e8400-e29b-41d4-a716-446655440006',
    'core.users',
    'Users',
    'User account management',
    'users',
    '#3B82F6',
    'a10e8400-e29b-41d4-a716-446655440001', -- Parent: Core
    1, -- Depth 1
    '/core/users',
    '/core/users',
    'core.users',
    'User Management',
    TRUE,
    1,
    TRUE
),
-- Roles Feature
(
    'a10e8400-e29b-41d4-a716-446655440007',
    'core.roles',
    'Roles',
    'Role and access level management',
    'shield',
    '#3B82F6',
    'a10e8400-e29b-41d4-a716-446655440001', -- Parent: Core
    1, -- Depth 1
    '/core/roles',
    '/core/roles',
    'core.roles',
    'Role Management',
    TRUE,
    2,
    TRUE
),
-- Permissions Feature
(
    'a10e8400-e29b-41d4-a716-446655440008',
    'core.permissions',
    'Permissions',
    'Permission configuration and management',
    'key',
    '#3B82F6',
    'a10e8400-e29b-41d4-a716-446655440001', -- Parent: Core
    1, -- Depth 1
    '/core/permissions',
    '/core/permissions',
    'core.permissions',
    'Permission Management',
    TRUE,
    3,
    TRUE
),
-- Companies Feature
(
    'a10e8400-e29b-41d4-a716-446655440009',
    'core.companies',
    'Companies',
    'Company and tenant management',
    'building',
    '#3B82F6',
    'a10e8400-e29b-41d4-a716-446655440001', -- Parent: Core
    1, -- Depth 1
    '/core/companies',
    '/core/companies',
    'core.companies',
    'Company Management',
    TRUE,
    4,
    TRUE
),
-- Permission Templates Feature
(
    'a10e8400-e29b-41d4-a716-446655440010',
    'core.permission_templates',
    'Permission Templates',
    'Permission template configuration for role management',
    'layers',
    '#3B82F6',
    'a10e8400-e29b-41d4-a716-446655440001', -- Parent: Core
    1, -- Depth 1
    '/core/permission-templates',
    '/core/permission-templates',
    'core.permission_templates',
    'Permission Templates',
    TRUE,
    5,
    TRUE
),
-- Profile Feature
(
    'a10e8400-e29b-41d4-a716-446655440011',
    'core.profile',
    'Profile',
    'User profile and account settings',
    'user',
    '#3B82F6',
    'a10e8400-e29b-41d4-a716-446655440001', -- Parent: Core
    1, -- Depth 1
    '/core/profile',
    '/core/profile',
    'core.profile',
    'My Profile',
    TRUE,
    6,
    TRUE
)
ON CONFLICT (id) DO NOTHING;

-- =====================================================
-- CRM SUB-MODULES (ChildItem Level - Depth 1)
-- =====================================================
INSERT INTO core.modules (
    id, code, name, description, icon, color,
    parent_id, depth, "to", url, permission, tooltip,
    is_menu_visible, sort_order, is_active
) VALUES
-- Chats Feature
(
    'a10e8400-e29b-41d4-a716-446655440012',
    'crm.chats',
    'Chats',
    'Multi-platform chat management (WhatsApp, Email, Phone)',
    'message-circle',
    '#10B981',
    'a10e8400-e29b-41d4-a716-446655440002', -- Parent: CRM
    1, -- Depth 1
    '/crm/chats',
    '/crm/chats',
    'crm.chats',
    'Chat Management',
    TRUE,
    1,
    TRUE
),
-- Leads Feature
(
    'a10e8400-e29b-41d4-a716-446655440013',
    'crm.leads',
    'Leads',
    'Lead and prospect management',
    'users',
    '#10B981',
    'a10e8400-e29b-41d4-a716-446655440002', -- Parent: CRM
    1, -- Depth 1
    '/crm/leads',
    '/crm/leads',
    'crm.leads',
    'Lead Management',
    TRUE,
    2,
    TRUE
),
-- Deals Feature
(
    'a10e8400-e29b-41d4-a716-446655440014',
    'crm.deals',
    'Deals',
    'Closed deals and revenue tracking',
    'dollar-sign',
    '#10B981',
    'a10e8400-e29b-41d4-a716-446655440002', -- Parent: CRM
    1, -- Depth 1
    '/crm/deals',
    '/crm/deals',
    'crm.deals',
    'Deal Management',
    TRUE,
    3,
    TRUE
),
-- Sales Persons Feature
(
    'a10e8400-e29b-41d4-a716-446655440015',
    'crm.sales_persons',
    'Sales Persons',
    'Sales team and representative management',
    'user-check',
    '#10B981',
    'a10e8400-e29b-41d4-a716-446655440002', -- Parent: CRM
    1, -- Depth 1
    '/crm/sales-persons',
    '/crm/sales-persons',
    'crm.sales_persons',
    'Sales Team',
    TRUE,
    4,
    TRUE
),
-- Sales Targets Feature
(
    'a10e8400-e29b-41d4-a716-446655440016',
    'crm.sales_targets',
    'Sales Targets',
    'Monthly and annual sales target management',
    'target',
    '#10B981',
    'a10e8400-e29b-41d4-a716-446655440002', -- Parent: CRM
    1, -- Depth 1
    '/crm/sales-targets',
    '/crm/sales-targets',
    'crm.sales_targets',
    'Sales Targets',
    TRUE,
    5,
    TRUE
),
-- Auto Reply Rules Feature
(
    'a10e8400-e29b-41d4-a716-446655440017',
    'crm.auto_reply_rules',
    'Auto Reply Rules',
    'AI-powered auto-reply configuration',
    'zap',
    '#10B981',
    'a10e8400-e29b-41d4-a716-446655440002', -- Parent: CRM
    1, -- Depth 1
    '/crm/auto-reply-rules',
    '/crm/auto-reply-rules',
    'crm.auto_reply_rules',
    'Auto Reply Settings',
    TRUE,
    6,
    TRUE
),
-- Activities Feature
(
    'a10e8400-e29b-41d4-a716-446655440018',
    'crm.activities',
    'Activities',
    'Sales activities timeline (calls, meetings, notes)',
    'activity',
    '#10B981',
    'a10e8400-e29b-41d4-a716-446655440002', -- Parent: CRM
    1, -- Depth 1
    '/crm/activities',
    '/crm/activities',
    'crm.activities',
    'Activity Timeline',
    TRUE,
    7,
    TRUE
),
-- Company Settings Feature
(
    'a10e8400-e29b-41d4-a716-446655440019',
    'crm.company_settings',
    'Company Settings',
    'CRM configuration and settings',
    'settings',
    '#10B981',
    'a10e8400-e29b-41d4-a716-446655440002', -- Parent: CRM
    1, -- Depth 1
    '/crm/settings',
    '/crm/settings',
    'crm.company_settings',
    'CRM Settings',
    TRUE,
    8,
    TRUE
),
-- Reports Feature
(
    'a10e8400-e29b-41d4-a716-446655440020',
    'crm.reports',
    'Reports',
    'CRM analytics and reports',
    'bar-chart',
    '#10B981',
    'a10e8400-e29b-41d4-a716-446655440002', -- Parent: CRM
    1, -- Depth 1
    '/crm/reports',
    '/crm/reports',
    'crm.reports',
    'CRM Reports & Analytics',
    TRUE,
    9,
    TRUE
)
ON CONFLICT (id) DO NOTHING;
