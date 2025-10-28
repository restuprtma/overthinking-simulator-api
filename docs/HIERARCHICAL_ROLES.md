# Hierarchical Role System

## Overview

Sistem **Hierarchical Role** memungkinkan user memiliki:
- **Global roles** - Berlaku di semua company (via `core.user_roles`)
- **Company-specific roles** - Override global role per company (via `core.company_users.role_id`)

## Konsep

### Global Role
- Assigned melalui tabel `core.user_roles`
- Berlaku sebagai default role di semua company
- Contoh: User memiliki role "Viewer" secara global

### Company-Specific Role
- Assigned melalui kolom `core.company_users.role_id`
- Override global role untuk company tertentu
- Jika `role_id = NULL` → pakai global role
- Jika `role_id = UUID` → pakai company-specific role
- Contoh: User yang global rolenya "Viewer" bisa jadi "Admin" di Company X

## Database Schema

### Tabel: `core.company_users`

```sql
CREATE TABLE core.company_users (
    id UUID PRIMARY KEY,
    company_id UUID NOT NULL,
    user_id UUID NOT NULL,
    role_id UUID REFERENCES core.roles(id), -- ✨ Company-specific role
    role VARCHAR(50),                        -- DEPRECATED
    is_primary BOOLEAN,
    is_active BOOLEAN,
    ...
);

CREATE INDEX idx_company_users_role_id ON core.company_users(role_id);
```

### Field Explanation

| Field | Type | Purpose |
|-------|------|---------|
| `role_id` | UUID | Company-specific role. NULL = inherit global role |
| `role` | VARCHAR | DEPRECATED. Kept for backward compatibility |

## Authentication Flow

```
1. User Login
   ↓
2. Get global roles from core.user_roles
   ↓
3. Get primary company ID
   ↓
4. Query effective roles:
   SELECT COALESCE(cu.role_id, ur.role_id) as effective_role
   FROM company_users cu
   LEFT JOIN user_roles ur ON ur.user_id = cu.user_id
   WHERE cu.company_id = ? AND cu.user_id = ?
   ↓
5. Generate JWT with effective roles
   ↓
6. Return token to client
```

## Example Scenarios

### Scenario 1: User dengan Global Role Saja

**Setup:**
```sql
user_roles:     { user_id: U1, role_id: R_VIEWER }
company_users:  { user_id: U1, company_id: C1, role_id: NULL }
```

**Result:**
- Login ke Company C1 → JWT roles: `["Viewer"]` ✓ (pakai global)

---

### Scenario 2: User dengan Company-Specific Role

**Setup:**
```sql
user_roles:     { user_id: U1, role_id: R_VIEWER }
company_users:  { user_id: U1, company_id: C1, role_id: R_ADMIN }
```

**Result:**
- Login ke Company C1 → JWT roles: `["Admin"]` ✨ (override global)

---

### Scenario 3: User di Multiple Companies

**Setup:**
```sql
user_roles:     { user_id: U1, role_id: R_VIEWER }
company_users:
  - { user_id: U1, company_id: C1, role_id: R_ADMIN }
  - { user_id: U1, company_id: C2, role_id: NULL }
  - { user_id: U1, company_id: C3, role_id: R_MANAGER }
```

**Result:**
- Company C1 → `["Admin"]` ✨
- Company C2 → `["Viewer"]` ✓ (global)
- Company C3 → `["Manager"]` ✨

## API Usage

### Add User to Company (Inherit Global Role)

```json
POST /core/v1/companies/{id}/users

{
  "user_id": "uuid-here",
  "role_id": null
}
```
User akan inherit global rolenya.

---

### Add User to Company (Company-Specific Role)

```json
POST /core/v1/companies/{id}/users

{
  "user_id": "uuid-here",
  "role_id": "admin-role-uuid"
}
```
User akan punya role "Admin" khusus di company ini.

---

### Update User Role (Override Global)

```json
PUT /core/v1/companies/{id}/users/{user_id}

{
  "role_id": "manager-role-uuid"
}
```
Update role khusus company.

---

### Remove Company-Specific Role (Back to Global)

```json
PUT /core/v1/companies/{id}/users/{user_id}

{
  "role_id": null
}
```
User kembali pakai global role.

## Code Implementation

### Repository: `GetUserRolesAndPermissionsInCompany()`

```go
func (r *CompanyUserRepository) GetUserRolesAndPermissionsInCompany(
    userID, companyID string,
) ([]string, []string, error) {
    query := `
        SELECT
            COALESCE(
                json_agg(DISTINCT r.name) FILTER (WHERE r.name IS NOT NULL),
                '[]'
            ) as roles,
            COALESCE(
                json_agg(DISTINCT (p.resource || ':' || p.action))
                FILTER (WHERE p.resource IS NOT NULL),
                '[]'
            ) as permissions
        FROM core.company_users cu
        LEFT JOIN core.user_roles ur ON ur.user_id = cu.user_id
        LEFT JOIN core.roles r ON r.id = COALESCE(cu.role_id, ur.role_id)
        LEFT JOIN core.role_permissions rp ON rp.role_id = r.id
        LEFT JOIN core.permissions p ON p.id = rp.permission_id
        WHERE cu.user_id = $1 AND cu.company_id = $2
    `
    // ... execute query
}
```

**Key Logic:**
- `COALESCE(cu.role_id, ur.role_id)` → Company role prioritas, fallback ke global

### Auth Service: `SignIn()`

```go
func (s *AuthService) SignIn(req *dto.SignInRequest) (*dto.SignInResponse, error) {
    // 1. Get user with global roles
    user, roles, permissions, err := s.userRepo.FindByEmailWithRoles(email)

    // 2. Get primary company ID
    companyID, err := s.companyUserRepo.GetPrimaryCompanyID(user.ID)

    // 3. Get company-specific roles (if exists)
    if companyID != "" {
        companyRoles, companyPerms, err :=
            s.companyUserRepo.GetUserRolesAndPermissionsInCompany(user.ID, companyID)

        if err == nil && len(companyRoles) > 0 {
            roles = companyRoles           // ✨ Override
            permissions = companyPerms
        }
    }

    // 4. Generate JWT with effective roles
    accessToken, err := jwtpkg.GenerateToken(
        user.ID, companyID, email, username, roles, permissions,
    )

    return &dto.SignInResponse{AccessToken: accessToken, ...}
}
```

## Testing

### Test Case 1: User with Global Role
```bash
# Setup: User has global "Viewer" role, no company-specific role
# Expected: JWT contains ["Viewer"]

curl -X POST /core/v1/auth/signin \
  -H "Content-Type: application/json" \
  -d '{"email":"user@test.com","password":"pass"}'
```

### Test Case 2: User with Company-Specific Role
```bash
# Setup: User has global "Viewer", company role "Admin"
# Expected: JWT contains ["Admin"]

# Same login request as above
# Decode JWT at jwt.io → should show "Admin" in roles
```

### Test Case 3: Switch Company
```bash
# User in Company A (Admin) and Company B (Viewer)

# Switch to Company B
curl -X POST /core/v1/auth/switch-company \
  -H "Authorization: Bearer {token}" \
  -d '{"company_id":"company-b-uuid"}'

# Expected: New JWT contains ["Viewer"]
```

## Benefits

✅ **Flexible** - Different roles per company
✅ **Simple** - Transparent to middleware
✅ **Backward Compatible** - Old `role` field still works
✅ **Zero Breaking Changes** - Existing API unchanged
✅ **Clean** - Logic isolated in auth service

## Migration Notes

### Database Migration

Migration sudah terintegrasi di `000003_company_tables.up.sql`:
- Tabel `company_users` sudah include kolom `role_id`
- Index `idx_company_users_role_id` sudah dibuat
- Field lama `role` dipertahankan untuk backward compatibility

### No Separate Migration Required

Berbeda dengan rencana awal, hierarchical role sudah built-in di schema awal, sehingga:
- ✅ Fresh install langsung support hierarchical role
- ✅ Tidak perlu migration terpisah
- ✅ Backward compatible dari awal

## Troubleshooting

### User punya permission yang salah

**Check JWT token:**
```bash
# Decode JWT at jwt.io
# Lihat field "roles" dan "permissions"
```

**Check database:**
```sql
SELECT
    cu.role_id,
    COALESCE(cu.role_id, ur.role_id) as effective_role_id,
    r.name as role_name
FROM core.company_users cu
LEFT JOIN core.user_roles ur ON ur.user_id = cu.user_id
LEFT JOIN core.roles r ON r.id = COALESCE(cu.role_id, ur.role_id)
WHERE cu.user_id = 'user-uuid' AND cu.company_id = 'company-uuid';
```

### Role update tidak apply

**Solution:** User perlu:
1. Refresh token, atau
2. Logout dan login lagi, atau
3. Switch company (generate token baru)

## Summary

Hierarchical Role System menyediakan fleksibilitas untuk:
- Set default role via global `user_roles`
- Override per-company via `company_users.role_id`
- Seamless integration dengan existing auth flow
- Zero impact ke middleware/API consumers
