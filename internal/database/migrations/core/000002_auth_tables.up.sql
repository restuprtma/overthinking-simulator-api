-- Authentication & Authorization Tables

-- Enable UUID extension if not already enabled
CREATE EXTENSION IF NOT EXISTS "uuid-ossp";

-- Users table
CREATE TABLE IF NOT EXISTS core.users (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    email VARCHAR(255) UNIQUE NOT NULL,
    username VARCHAR(100) UNIQUE NOT NULL,
    password_hash VARCHAR(255) NOT NULL,
    full_name VARCHAR(255),
    phone VARCHAR(20),
    company_id UUID DEFAULT NULL,
    is_active BOOLEAN DEFAULT TRUE,
    is_email_verified BOOLEAN DEFAULT FALSE,
    last_login_at TIMESTAMP,
    failed_login_count INT DEFAULT 0,
    locked_until TIMESTAMP,
    locked_at TIMESTAMP,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    created_by UUID,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_by UUID,
    deleted_at TIMESTAMP,
    deleted_by UUID
);

-- Email verification tokens
CREATE TABLE IF NOT EXISTS core.email_verification_tokens (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL,
    token VARCHAR(255) UNIQUE NOT NULL,
    expires_at TIMESTAMP NOT NULL,
    verified_at TIMESTAMP,
    resent_count INT DEFAULT 0,
    last_resent_at TIMESTAMP,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    created_by UUID,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_by UUID,
    deleted_at TIMESTAMP,
    deleted_by UUID
);

-- Password reset tokens
CREATE TABLE IF NOT EXISTS core.password_reset_tokens (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL,
    token VARCHAR(255) UNIQUE NOT NULL,
    expires_at TIMESTAMP NOT NULL,
    used_at TIMESTAMP,
    ip_address VARCHAR(45),
    user_agent TEXT,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- Refresh tokens for session management
CREATE TABLE IF NOT EXISTS core.refresh_tokens (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL,
    token_hash VARCHAR(255) UNIQUE NOT NULL,
    device_name VARCHAR(255),
    device_id VARCHAR(255),
    ip_address VARCHAR(45),
    user_agent TEXT,
    expires_at TIMESTAMP NOT NULL,
    last_used_at TIMESTAMP,
    revoked_at TIMESTAMP,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- Login attempts tracking
CREATE TABLE IF NOT EXISTS core.login_attempts (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID,
    email VARCHAR(255),
    username VARCHAR(100),
    ip_address VARCHAR(45),
    user_agent TEXT,
    status VARCHAR(20) NOT NULL,
    failure_reason VARCHAR(255),
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- Audit logs
CREATE TABLE IF NOT EXISTS core.audit_logs (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID,
    action VARCHAR(100) NOT NULL,
    resource VARCHAR(100),
    resource_id UUID,
    ip_address VARCHAR(45),
    user_agent TEXT,
    metadata JSONB,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- Roles table
CREATE TABLE IF NOT EXISTS core.roles (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name VARCHAR(100) UNIQUE NOT NULL,
    description TEXT,
    company_id UUID DEFAULT NULL,
    is_system BOOLEAN DEFAULT FALSE,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    created_by UUID,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_by UUID,
    deleted_at TIMESTAMP,
    deleted_by UUID
);

-- Modules table for grouping permissions and menu tree structure
CREATE TABLE IF NOT EXISTS core.modules (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    code VARCHAR(50) UNIQUE NOT NULL,
    name VARCHAR(100) NOT NULL,
    description TEXT,
    icon VARCHAR(50),
    color VARCHAR(20),
    sort_order INT DEFAULT 0,
    is_active BOOLEAN DEFAULT TRUE,

    -- Tree structure fields (for hierarchical menu)
    parent_id UUID DEFAULT NULL,
    depth INT DEFAULT 0 CHECK (depth >= 0),

    -- Frontend-matching fields (MenuItem/ChildItem interface)
    heading TEXT,
    "to" VARCHAR(255),
    url VARCHAR(255),
    permission VARCHAR(100),
    tooltip VARCHAR(255),
    is_menu_visible BOOLEAN DEFAULT TRUE,

    -- Audit fields
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    created_by UUID,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_by UUID,
    deleted_at TIMESTAMP,
    deleted_by UUID
);

-- Permissions table
CREATE TABLE IF NOT EXISTS core.permissions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    module_id UUID NOT NULL,
    resource VARCHAR(100) NOT NULL,
    action VARCHAR(50) NOT NULL,
    description TEXT,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    created_by UUID,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_by UUID,
    deleted_at TIMESTAMP,
    deleted_by UUID
);

-- Role permissions junction table
CREATE TABLE IF NOT EXISTS core.role_permissions (
    role_id UUID NOT NULL,
    permission_id UUID NOT NULL,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    created_by UUID,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_by UUID,
    deleted_at TIMESTAMP,
    deleted_by UUID,
    PRIMARY KEY (role_id, permission_id)
);

-- User Roles Junction Table (Many-to-Many relationship between users and roles)

-- User roles junction table
CREATE TABLE IF NOT EXISTS core.user_roles (
    user_id UUID NOT NULL,
    role_id UUID NOT NULL,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    created_by UUID,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_by UUID,
    deleted_at TIMESTAMP,
    deleted_by UUID,
    PRIMARY KEY (user_id, role_id)
);

-- Create indexes for authentication tables

-- Users indexes
CREATE INDEX idx_users_email ON core.users(email);
CREATE INDEX idx_users_username ON core.users(username);
CREATE INDEX idx_users_deleted_at ON core.users(deleted_at);
CREATE INDEX idx_users_is_active ON core.users(is_active);
CREATE INDEX idx_users_locked_until ON core.users(locked_until);
CREATE INDEX idx_users_failed_login_count ON core.users(failed_login_count);
CREATE INDEX idx_users_company_id ON core.users(company_id) WHERE company_id IS NOT NULL;

-- Email verification tokens indexes
CREATE INDEX idx_email_verification_user_id ON core.email_verification_tokens(user_id);
CREATE INDEX idx_email_verification_token ON core.email_verification_tokens(token);
CREATE INDEX idx_email_verification_expires_at ON core.email_verification_tokens(expires_at);
CREATE INDEX idx_email_verification_verified_at ON core.email_verification_tokens(verified_at);

-- Password reset tokens indexes
CREATE INDEX idx_password_reset_user_id ON core.password_reset_tokens(user_id);
CREATE INDEX idx_password_reset_token ON core.password_reset_tokens(token);
CREATE INDEX idx_password_reset_expires_at ON core.password_reset_tokens(expires_at);
CREATE INDEX idx_password_reset_used_at ON core.password_reset_tokens(used_at);

-- Refresh tokens indexes
CREATE INDEX idx_refresh_tokens_user_id ON core.refresh_tokens(user_id);
CREATE INDEX idx_refresh_tokens_token_hash ON core.refresh_tokens(token_hash);
CREATE INDEX idx_refresh_tokens_expires_at ON core.refresh_tokens(expires_at);
CREATE INDEX idx_refresh_tokens_revoked_at ON core.refresh_tokens(revoked_at);
CREATE INDEX idx_refresh_tokens_device_id ON core.refresh_tokens(device_id);

-- Login attempts indexes
CREATE INDEX idx_login_attempts_user_id ON core.login_attempts(user_id);
CREATE INDEX idx_login_attempts_email ON core.login_attempts(email);
CREATE INDEX idx_login_attempts_ip_address ON core.login_attempts(ip_address);
CREATE INDEX idx_login_attempts_status ON core.login_attempts(status);
CREATE INDEX idx_login_attempts_created_at ON core.login_attempts(created_at);

-- Audit logs indexes
CREATE INDEX idx_audit_logs_user_id ON core.audit_logs(user_id);
CREATE INDEX idx_audit_logs_action ON core.audit_logs(action);
CREATE INDEX idx_audit_logs_resource ON core.audit_logs(resource);
CREATE INDEX idx_audit_logs_created_at ON core.audit_logs(created_at);

-- Roles indexes
CREATE INDEX idx_roles_name ON core.roles(name);
CREATE INDEX idx_roles_deleted_at ON core.roles(deleted_at);
CREATE INDEX idx_roles_company_id ON core.roles(company_id) WHERE company_id IS NOT NULL;

-- Modules indexes
CREATE INDEX idx_modules_code ON core.modules(code) WHERE deleted_at IS NULL;
CREATE INDEX idx_modules_sort_order ON core.modules(sort_order, is_active);
CREATE INDEX idx_modules_is_active ON core.modules(is_active) WHERE deleted_at IS NULL;
CREATE INDEX idx_modules_parent_id ON core.modules(parent_id) WHERE parent_id IS NOT NULL;
CREATE INDEX idx_modules_depth ON core.modules(depth);
CREATE INDEX idx_modules_permission ON core.modules(permission) WHERE permission IS NOT NULL;
CREATE INDEX idx_modules_menu_query ON core.modules(parent_id, sort_order, is_active, is_menu_visible) WHERE deleted_at IS NULL;

-- Permissions indexes
-- Partial unique index: ensures (module_id, resource, action) uniqueness only for non-deleted records
-- This allows the same permission to be recreated after soft deletion
CREATE UNIQUE INDEX idx_permissions_module_resource_action_unique ON core.permissions(module_id, resource, action) WHERE deleted_at IS NULL;

CREATE INDEX idx_permissions_module_id ON core.permissions(module_id) WHERE deleted_at IS NULL;
CREATE INDEX idx_permissions_module_resource ON core.permissions(module_id, resource) WHERE deleted_at IS NULL;
CREATE INDEX idx_permissions_resource ON core.permissions(resource);
CREATE INDEX idx_permissions_action ON core.permissions(action);

-- Role permissions indexes
CREATE INDEX idx_role_permissions_role_id ON core.role_permissions(role_id);
CREATE INDEX idx_role_permissions_permission_id ON core.role_permissions(permission_id);

-- User roles indexes
CREATE INDEX idx_user_roles_user_id ON core.user_roles(user_id);
CREATE INDEX idx_user_roles_role_id ON core.user_roles(role_id);
CREATE INDEX idx_user_roles_deleted_at ON core.user_roles(deleted_at);

-- =====================================================
-- PERMISSION TEMPLATE SYSTEM
-- =====================================================
-- This system provides UI helpers for role management
-- Templates simplify permission assignment by grouping common action patterns

-- Permission Templates - Predefined sets of actions (e.g., Viewer, Editor, Admin)
CREATE TABLE IF NOT EXISTS core.permission_templates (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name VARCHAR(100) UNIQUE NOT NULL,
    description TEXT,
    company_id UUID DEFAULT NULL,
    is_system BOOLEAN DEFAULT FALSE,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    created_by UUID,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_by UUID,
    deleted_at TIMESTAMP,
    deleted_by UUID
);

-- Permission Template Actions - Defines which actions are included in each template
CREATE TABLE IF NOT EXISTS core.permission_template_actions (
    permission_template_id UUID NOT NULL,
    action VARCHAR(50) NOT NULL,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    created_by UUID,
    PRIMARY KEY (permission_template_id, action)
);

-- Role Module Templates - Maps roles to modules with their selected templates
-- This is a UI helper table to remember which template was selected for each module
-- The actual permissions are still stored in role_permissions for authorization
-- Note: Only sub-modules (depth > 0) should have templates assigned
CREATE TABLE IF NOT EXISTS core.role_module_templates (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    role_id UUID NOT NULL,
    module_id UUID NOT NULL,
    permission_template_id UUID NOT NULL,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    created_by UUID,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_by UUID,
    deleted_at TIMESTAMP,
    deleted_by UUID,
    UNIQUE (role_id, module_id)
);

-- Permission template indexes
CREATE INDEX idx_permission_templates_name ON core.permission_templates(name);
CREATE INDEX idx_permission_templates_is_system ON core.permission_templates(is_system);
CREATE INDEX idx_permission_templates_deleted_at ON core.permission_templates(deleted_at);
CREATE INDEX idx_permission_templates_company_id ON core.permission_templates(company_id) WHERE company_id IS NOT NULL;

-- Permission template actions indexes
CREATE INDEX idx_permission_template_actions_template_id ON core.permission_template_actions(permission_template_id);
CREATE INDEX idx_permission_template_actions_action ON core.permission_template_actions(action);

-- Role module templates indexes
CREATE INDEX idx_role_module_templates_role_id ON core.role_module_templates(role_id);
CREATE INDEX idx_role_module_templates_module_id ON core.role_module_templates(module_id);
CREATE INDEX idx_role_module_templates_permission_template_id ON core.role_module_templates(permission_template_id);
CREATE INDEX idx_role_module_templates_deleted_at ON core.role_module_templates(deleted_at);