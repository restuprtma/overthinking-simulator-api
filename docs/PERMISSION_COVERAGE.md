# Permission Coverage - Endpoint & Role Mapping

Dokumen ini menjelaskan mapping lengkap antara endpoint API, permission yang diperlukan, dan role assignment.

## Overview

Sistem menggunakan **Permission-Based Access Control (PBAC)** dengan format `resource:action`.

### Role Hierarchy

1. **super_admin** - Full access ke semua permission
2. **admin** - Full user/company management + Full CRM access
3. **user** - Profile management + Limited CRM read access

---

## Core Module Permissions

### User Management (`/core/v1/users`)

| Endpoint | Method | Permission | Super Admin | Admin | User |
|----------|--------|------------|-------------|-------|------|
| `/users` | GET | `users:read` | ✅ | ✅ | ❌ |
| `/users/:id` | GET | `users:read` | ✅ | ✅ | ❌ |
| `/users` | POST | `users:create` | ✅ | ✅ | ❌ |
| `/users/:id` | PUT | `users:update` | ✅ | ✅ | ❌ |
| `/users/:id` | DELETE | `users:delete` | ✅ | ✅ | ❌ |
| `/users/:id/restore` | POST | `users:restore` | ✅ | ✅ | ❌ |

**Admin Notes**: Full CRUD access untuk user management

---

### Role Management (`/core/v1/roles`)

| Endpoint | Method | Permission | Super Admin | Admin | User |
|----------|--------|------------|-------------|-------|------|
| `/roles` | GET | `roles:read` | ✅ | ✅ | ❌ |
| `/roles/:id` | GET | `roles:read` | ✅ | ✅ | ❌ |
| `/roles` | POST | `roles:create` | ✅ | ❌ | ❌ |
| `/roles/:id` | PUT | `roles:update` | ✅ | ❌ | ❌ |
| `/roles/:id` | DELETE | `roles:delete` | ✅ | ❌ | ❌ |
| `/roles/:id/restore` | POST | `roles:restore` | ✅ | ❌ | ❌ |
| `/roles/:id/permissions` | GET | `roles:read` | ✅ | ✅ | ❌ |
| `/roles/:id/permissions` | PUT | `roles:update` | ✅ | ❌ | ❌ |
| `/roles/:id/permissions/:permission_id` | POST | `roles:update` | ✅ | ❌ | ❌ |
| `/roles/:id/permissions/:permission_id` | DELETE | `roles:update` | ✅ | ❌ | ❌ |

**Admin Notes**: Read-only access - tidak dapat membuat/mengubah role

---

### Permission Management (`/core/v1/permissions`)

| Endpoint | Method | Permission | Super Admin | Admin | User |
|----------|--------|------------|-------------|-------|------|
| `/permissions` | GET | `permissions:read` | ✅ | ✅ | ❌ |
| `/permissions/:id` | GET | `permissions:read` | ✅ | ✅ | ❌ |
| `/permissions` | POST | `permissions:create` | ✅ | ❌ | ❌ |
| `/permissions/:id` | PUT | `permissions:update` | ✅ | ❌ | ❌ |
| `/permissions/:id` | DELETE | `permissions:delete` | ✅ | ❌ | ❌ |

**Admin Notes**: Read-only access - tidak dapat membuat/mengubah permission

---

### Company Management (`/core/v1/companies`)

| Endpoint | Method | Permission | Super Admin | Admin | User |
|----------|--------|------------|-------------|-------|------|
| `/companies` | GET | `companies:read` | ✅ | ✅ | ❌ |
| `/companies/:id` | GET | `companies:read` | ✅ | ✅ | ❌ |
| `/companies` | POST | `companies:create` | ✅ | ✅ | ❌ |
| `/companies/:id` | PUT | `companies:update` | ✅ | ✅ | ❌ |
| `/companies/:id` | DELETE | `companies:delete` | ✅ | ✅ | ❌ |
| `/companies/:id/users` | GET | `companies:read` | ✅ | ✅ | ❌ |
| `/companies/:id/users` | POST | `companies:manage_users` | ✅ | ✅ | ❌ |
| `/companies/:id/users/:user_id` | PUT | `companies:manage_users` | ✅ | ✅ | ❌ |
| `/companies/:id/users/:user_id` | DELETE | `companies:manage_users` | ✅ | ✅ | ❌ |

**Admin Notes**: Full access termasuk user management dalam company

---

### Profile Management (`/core/v1/profile`)

| Endpoint | Method | Permission | Super Admin | Admin | User |
|----------|--------|------------|-------------|-------|------|
| `/profile` | GET | `profile:read` | ✅ | ✅ | ✅ |
| `/profile` | PUT | `profile:update` | ✅ | ✅ | ✅ |

**User Notes**: Semua user dapat mengelola profile mereka sendiri

---

## CRM Module Permissions

### Lead Sources (`/crm/v1/lead-sources`)

| Endpoint | Method | Permission | Super Admin | Admin | User |
|----------|--------|------------|-------------|-------|------|
| `/lead-sources` | GET | `lead_sources:read` | ✅ | ✅ | ✅ |
| `/lead-sources/:id` | GET | `lead_sources:read` | ✅ | ✅ | ✅ |
| `/lead-sources` | POST | `lead_sources:create` | ✅ | ✅ | ❌ |
| `/lead-sources/:id` | PUT | `lead_sources:update` | ✅ | ✅ | ❌ |
| `/lead-sources/:id` | DELETE | `lead_sources:delete` | ✅ | ✅ | ❌ |
| `/lead-sources/:id/restore` | POST | `lead_sources:restore` | ✅ | ✅ | ❌ |

**User Notes**: Read-only access untuk melihat master data lead sources

---

### Leads Management (`/crm/v1/leads`)

| Endpoint | Method | Permission | Super Admin | Admin | User |
|----------|--------|------------|-------------|-------|------|
| `/leads` | GET | `leads:read` | ✅ | ✅ | ✅ |
| `/leads/:id` | GET | `leads:read` | ✅ | ✅ | ✅ |
| `/leads` | POST | `leads:create` | ✅ | ✅ | ❌ |
| `/leads/:id` | PUT | `leads:update` | ✅ | ✅ | ❌ |
| `/leads/:id` | DELETE | `leads:delete` | ✅ | ✅ | ❌ |
| `/leads/:id/assign` | POST | `leads:assign` | ✅ | ✅ | ❌ |
| `/leads/:id/convert` | POST | `leads:convert` | ✅ | ✅ | ❌ |

**User Notes**: Read-only - dapat melihat leads yang di-assign ke mereka

---

### Deals Management (`/crm/v1/deals`)

| Endpoint | Method | Permission | Super Admin | Admin | User |
|----------|--------|------------|-------------|-------|------|
| `/deals` | GET | `deals:read` | ✅ | ✅ | ✅ |
| `/deals/:id` | GET | `deals:read` | ✅ | ✅ | ✅ |
| `/deals` | POST | `deals:create` | ✅ | ✅ | ❌ |
| `/deals/:id` | PUT | `deals:update` | ✅ | ✅ | ❌ |
| `/deals/:id` | DELETE | `deals:delete` | ✅ | ✅ | ❌ |
| `/deals/:id/close` | POST | `deals:close` | ✅ | ✅ | ❌ |

**User Notes**: Read-only - dapat melihat deals yang di-assign ke mereka

---

### Sales Targets (`/crm/v1/sales-targets`)

| Endpoint | Method | Permission | Super Admin | Admin | User |
|----------|--------|------------|-------------|-------|------|
| `/sales-targets` | GET | `sales_targets:read` | ✅ | ✅ | ❌ |
| `/sales-targets/:id` | GET | `sales_targets:read` | ✅ | ✅ | ❌ |
| `/sales-targets` | POST | `sales_targets:create` | ✅ | ✅ | ❌ |
| `/sales-targets/:id` | PUT | `sales_targets:update` | ✅ | ✅ | ❌ |
| `/sales-targets/:id` | DELETE | `sales_targets:delete` | ✅ | ✅ | ❌ |

**User Notes**: No access - hanya admin yang dapat manage targets

---

### Activities (`/crm/v1/activities`)

| Endpoint | Method | Permission | Super Admin | Admin | User |
|----------|--------|------------|-------------|-------|------|
| `/activities` | GET | `activities:read` | ✅ | ✅ | ✅ |
| `/activities/:id` | GET | `activities:read` | ✅ | ✅ | ✅ |
| `/activities` | POST | `activities:create` | ✅ | ✅ | ❌ |
| `/activities/:id` | PUT | `activities:update` | ✅ | ✅ | ❌ |
| `/activities/:id` | DELETE | `activities:delete` | ✅ | ✅ | ❌ |

**User Notes**: Read-only - dapat melihat activity log

---

### CRM Reports (`/crm/v1/reports`)

| Endpoint | Method | Permission | Super Admin | Admin | User |
|----------|--------|------------|-------------|-------|------|
| `/reports` | GET | `crm_reports:read` | ✅ | ✅ | ❌ |
| `/reports/:id` | GET | `crm_reports:read` | ✅ | ✅ | ❌ |
| `/reports/export` | POST | `crm_reports:export` | ✅ | ✅ | ❌ |

**User Notes**: No access ke reports dan analytics

---

## Permission Summary by Role

### Super Admin (ID: 550e8400-e29b-41d4-a716-446655440001)
- **Total Permissions**: ALL (Dynamically assigned)
- **Access Level**: Full system access
- **Can manage**: Everything including roles and permissions

### Admin (ID: 550e8400-e29b-41d4-a716-446655440002)
- **Total Permissions**: 42 permissions
- **Access Level**: Full operational access
- **Can manage**:
  - ✅ Users (CRUD + Restore)
  - ✅ Companies (CRUD + User management)
  - ✅ CRM (Full CRUD for all modules)
  - ✅ Reports (View + Export)
  - 👁️ Roles (Read only)
  - 👁️ Permissions (Read only)
- **Cannot manage**:
  - ❌ Create/Update/Delete Roles
  - ❌ Create/Update/Delete Permissions

### User (ID: 550e8400-e29b-41d4-a716-446655440003)
- **Total Permissions**: 6 permissions
- **Access Level**: Limited read access
- **Can manage**:
  - ✅ Own Profile (View + Update)
  - 👁️ Lead Sources (Read only)
  - 👁️ Leads (Read only - assigned)
  - 👁️ Deals (Read only - assigned)
  - 👁️ Activities (Read only)
- **Cannot manage**:
  - ❌ Users, Roles, Permissions
  - ❌ Companies
  - ❌ Create/Update/Delete CRM data
  - ❌ Reports

---

## Database Seeder Files

### 1. `002_default_permissions.sql`
**Updated permissions**:
- ✅ Added `companies:manage_users` (ID: 650e8400-e29b-41d4-a716-446655440017)
- ✅ Added `lead_sources:restore` (ID: 650e8400-e29b-41d4-a716-446655440105)

**Total Permissions**: 36 permissions across:
- Core Module: 16 permissions
- CRM Module: 20 permissions

### 2. `003_role_permissions.sql`
**Complete rewrite** with:
- ✅ Detailed comments explaining each role's access
- ✅ Organized by modules (Core, CRM)
- ✅ All endpoint permissions covered
- ✅ Admin role now has 42 permissions (was 15)
- ✅ User role now has 6 permissions (was 2)

---

## Testing Checklist

### Super Admin Testing
- [ ] Can access all endpoints
- [ ] Can create/update/delete roles
- [ ] Can create/update/delete permissions
- [ ] Can manage all CRM modules

### Admin Testing
- [ ] Can CRUD users
- [ ] Can CRUD companies
- [ ] Can manage company users
- [ ] Can CRUD all CRM data
- [ ] Can view roles (read-only)
- [ ] Can view permissions (read-only)
- [ ] Cannot create/update roles
- [ ] Cannot create/update permissions

### User Testing
- [ ] Can view/update own profile
- [ ] Can view lead sources
- [ ] Can view assigned leads
- [ ] Can view assigned deals
- [ ] Can view activities
- [ ] Cannot access user management
- [ ] Cannot access company management
- [ ] Cannot create/update CRM data
- [ ] Cannot access reports

---

## Migration Instructions

### Fresh Installation
```bash
# Run migrations
make migrate-up

# Run seeders (in order)
psql -U your_user -d your_db -f internal/database/seeders/core/001_default_roles.sql
psql -U your_user -d your_db -f internal/database/seeders/core/002_default_permissions.sql
psql -U your_user -d your_db -f internal/database/seeders/core/003_role_permissions.sql
psql -U your_user -d your_db -f internal/database/seeders/core/004_default_admin.sql
```

### Existing Database Update
```bash
# 1. Backup existing role_permissions
psql -U your_user -d your_db -c "CREATE TABLE core.role_permissions_backup AS SELECT * FROM core.role_permissions;"

# 2. Run updated seeders
psql -U your_user -d your_db -f internal/database/seeders/core/002_default_permissions.sql
psql -U your_user -d your_db -f internal/database/seeders/core/003_role_permissions.sql

# 3. Verify changes
psql -U your_user -d your_db -c "SELECT r.name, COUNT(rp.permission_id) as permission_count FROM core.roles r LEFT JOIN core.role_permissions rp ON r.id = rp.role_id GROUP BY r.name;"
```

Expected output:
```
    name     | permission_count
-------------+-----------------
 super_admin |              36
 admin       |              42
 user        |               6
```

---

## Notes

1. **Dynamic Super Admin**: Super admin mendapat ALL permissions secara dinamis via SELECT query
2. **Conflict Handling**: Semua INSERT menggunakan `ON CONFLICT DO NOTHING` untuk idempotency
3. **Future Modules**: Permissions untuk Leads, Deals, Sales Targets, Activities sudah disiapkan meskipun module belum diimplementasi
4. **Multi-tenancy**: Lead Sources menggunakan `CompanyContext()` middleware untuk isolasi data per company

---

**Last Updated**: 2025-10-19
**Version**: 1.0.0
