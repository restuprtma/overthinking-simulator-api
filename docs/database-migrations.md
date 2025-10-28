# Database Migrations Documentation

## Overview
Dokumentasi ini menjelaskan struktur database untuk sistem autentikasi, otorisasi, dan manajemen perusahaan pada Lakukan Backend.

---

## Migration 000002: Authentication & Authorization Tables

Migration ini mencakup semua tabel yang diperlukan untuk sistem autentikasi, otorisasi, dan audit.

### Tables

#### 1. `core.users`
Tabel utama untuk menyimpan informasi pengguna.

**Columns:**
- `id` (UUID, PK): Primary key dengan auto-generate UUID
- `email` (VARCHAR(255), UNIQUE, NOT NULL): Email pengguna
- `username` (VARCHAR(100), UNIQUE, NOT NULL): Username unik
- `password_hash` (VARCHAR(255), NOT NULL): Hash password
- `full_name` (VARCHAR(255)): Nama lengkap pengguna
- `phone` (VARCHAR(20)): Nomor telepon
- `is_active` (BOOLEAN): Status aktif pengguna (default: TRUE)
- `is_email_verified` (BOOLEAN): Status verifikasi email (default: FALSE)
- `last_login_at` (TIMESTAMP): Waktu login terakhir
- `failed_login_count` (INT): Jumlah percobaan login gagal (default: 0)
- `locked_until` (TIMESTAMP): Waktu sampai akun terkunci
- `locked_at` (TIMESTAMP): Waktu akun dikunci
- `created_at`, `created_by`, `updated_at`, `updated_by`, `deleted_at`, `deleted_by`: Audit fields

**Indexes:**
- `idx_users_email`: Index pada email
- `idx_users_username`: Index pada username
- `idx_users_deleted_at`: Index untuk soft delete
- `idx_users_is_active`: Index pada status aktif
- `idx_users_locked_until`: Index untuk akun terkunci
- `idx_users_failed_login_count`: Index pada jumlah login gagal

**Use Cases:**
- User registration dan profile management
- Login authentication
- Account locking mechanism untuk security
- Email verification tracking

---

#### 2. `core.email_verification_tokens`
Tabel untuk menyimpan token verifikasi email.

**Columns:**
- `id` (UUID, PK): Primary key
- `user_id` (UUID, NOT NULL): Foreign key ke users
- `token` (VARCHAR(255), UNIQUE, NOT NULL): Token verifikasi
- `expires_at` (TIMESTAMP, NOT NULL): Waktu kadaluarsa token
- `verified_at` (TIMESTAMP): Waktu token diverifikasi
- `resent_count` (INT): Jumlah token dikirim ulang (default: 0)
- `last_resent_at` (TIMESTAMP): Waktu terakhir token dikirim ulang
- `created_at`, `created_by`, `updated_at`, `updated_by`, `deleted_at`, `deleted_by`: Audit fields

**Indexes:**
- `idx_email_verification_user_id`: Index pada user_id
- `idx_email_verification_token`: Index pada token
- `idx_email_verification_expires_at`: Index untuk cek kadaluarsa
- `idx_email_verification_verified_at`: Index pada status verifikasi

**Use Cases:**
- Email verification flow
- Resend verification email
- Token expiration management

---

#### 3. `core.password_reset_tokens`
Tabel untuk menyimpan token reset password.

**Columns:**
- `id` (UUID, PK): Primary key
- `user_id` (UUID, NOT NULL): Foreign key ke users
- `token` (VARCHAR(255), UNIQUE, NOT NULL): Token reset password
- `expires_at` (TIMESTAMP, NOT NULL): Waktu kadaluarsa
- `used_at` (TIMESTAMP): Waktu token digunakan
- `ip_address` (VARCHAR(45)): IP address yang request reset
- `user_agent` (TEXT): User agent browser
- `created_at` (TIMESTAMP): Waktu token dibuat

**Indexes:**
- `idx_password_reset_user_id`: Index pada user_id
- `idx_password_reset_token`: Index pada token
- `idx_password_reset_expires_at`: Index untuk cek kadaluarsa
- `idx_password_reset_used_at`: Index pada status penggunaan

**Use Cases:**
- Forgot password flow
- Password reset security tracking
- Token expiration dan one-time use validation

---

#### 4. `core.refresh_tokens`
Tabel untuk session management menggunakan refresh tokens.

**Columns:**
- `id` (UUID, PK): Primary key
- `user_id` (UUID, NOT NULL): Foreign key ke users
- `token_hash` (VARCHAR(255), UNIQUE, NOT NULL): Hash dari refresh token
- `device_name` (VARCHAR(255)): Nama device
- `device_id` (VARCHAR(255)): ID device unik
- `ip_address` (VARCHAR(45)): IP address
- `user_agent` (TEXT): User agent browser
- `expires_at` (TIMESTAMP, NOT NULL): Waktu kadaluarsa
- `last_used_at` (TIMESTAMP): Waktu terakhir digunakan
- `revoked_at` (TIMESTAMP): Waktu token di-revoke
- `created_at` (TIMESTAMP): Waktu token dibuat

**Indexes:**
- `idx_refresh_tokens_user_id`: Index pada user_id
- `idx_refresh_tokens_token_hash`: Index pada token hash
- `idx_refresh_tokens_expires_at`: Index untuk cek kadaluarsa
- `idx_refresh_tokens_revoked_at`: Index pada status revoke
- `idx_refresh_tokens_device_id`: Index pada device

**Use Cases:**
- JWT refresh token mechanism
- Multi-device session management
- Token revocation untuk logout
- Security monitoring per device

---

#### 5. `core.login_attempts`
Tabel untuk tracking percobaan login (berhasil/gagal).

**Columns:**
- `id` (UUID, PK): Primary key
- `user_id` (UUID): Foreign key ke users (nullable)
- `email` (VARCHAR(255)): Email yang digunakan untuk login
- `username` (VARCHAR(100)): Username yang digunakan
- `ip_address` (VARCHAR(45)): IP address
- `user_agent` (TEXT): User agent browser
- `status` (VARCHAR(20), NOT NULL): Status login (success/failed)
- `failure_reason` (VARCHAR(255)): Alasan kegagalan login
- `created_at` (TIMESTAMP): Waktu percobaan login

**Indexes:**
- `idx_login_attempts_user_id`: Index pada user_id
- `idx_login_attempts_email`: Index pada email
- `idx_login_attempts_ip_address`: Index pada IP address
- `idx_login_attempts_status`: Index pada status
- `idx_login_attempts_created_at`: Index untuk query berdasarkan waktu

**Use Cases:**
- Security monitoring dan analytics
- Brute force attack detection
- Login history tracking
- Account locking berdasarkan failed attempts

---

#### 6. `core.audit_logs`
Tabel untuk mencatat semua aktivitas penting dalam sistem.

**Columns:**
- `id` (UUID, PK): Primary key
- `user_id` (UUID): Foreign key ke users
- `action` (VARCHAR(100), NOT NULL): Jenis aksi (create, update, delete, etc.)
- `resource` (VARCHAR(100)): Resource yang diakses
- `resource_id` (UUID): ID resource yang diakses
- `ip_address` (VARCHAR(45)): IP address
- `user_agent` (TEXT): User agent browser
- `metadata` (JSONB): Data tambahan dalam format JSON
- `created_at` (TIMESTAMP): Waktu aktivitas

**Indexes:**
- `idx_audit_logs_user_id`: Index pada user_id
- `idx_audit_logs_action`: Index pada action
- `idx_audit_logs_resource`: Index pada resource
- `idx_audit_logs_created_at`: Index untuk query berdasarkan waktu

**Use Cases:**
- Compliance dan audit trail
- Security investigation
- Activity monitoring
- Data change history

---

#### 7. `core.roles`
Tabel untuk mendefinisikan peran/role dalam sistem.

**Columns:**
- `id` (UUID, PK): Primary key
- `name` (VARCHAR(100), UNIQUE, NOT NULL): Nama role
- `description` (TEXT): Deskripsi role
- `is_system` (BOOLEAN): Flag untuk role bawaan sistem (default: FALSE)
- `created_at`, `created_by`, `updated_at`, `updated_by`, `deleted_at`, `deleted_by`: Audit fields

**Indexes:**
- `idx_roles_name`: Index pada nama role
- `idx_roles_deleted_at`: Index untuk soft delete

**Use Cases:**
- Role-based access control (RBAC)
- Permission grouping
- System vs custom roles management

---

#### 8. `core.permissions`
Tabel untuk mendefinisikan izin/permission dalam sistem.

**Columns:**
- `id` (UUID, PK): Primary key
- `resource` (VARCHAR(100), NOT NULL): Resource yang diatur (users, companies, etc.)
- `action` (VARCHAR(50), NOT NULL): Aksi yang diizinkan (create, read, update, delete)
- `description` (TEXT): Deskripsi permission
- `created_at`, `created_by`, `updated_at`, `updated_by`, `deleted_at`, `deleted_by`: Audit fields
- **UNIQUE constraint**: (resource, action)

**Indexes:**
- `idx_permissions_resource`: Index pada resource
- `idx_permissions_action`: Index pada action

**Use Cases:**
- Granular permission management
- RBAC implementation
- API endpoint authorization

---

#### 9. `core.role_permissions`
Junction table untuk relasi many-to-many antara roles dan permissions.

**Columns:**
- `role_id` (UUID, NOT NULL): Foreign key ke roles
- `permission_id` (UUID, NOT NULL): Foreign key ke permissions
- `created_at`, `created_by`, `updated_at`, `updated_by`, `deleted_at`, `deleted_by`: Audit fields
- **PRIMARY KEY**: (role_id, permission_id)

**Indexes:**
- `idx_role_permissions_role_id`: Index pada role_id
- `idx_role_permissions_permission_id`: Index pada permission_id

**Use Cases:**
- Assign permissions to roles
- Permission inheritance
- Role capability management

---

#### 10. `core.user_roles`
Junction table untuk relasi many-to-many antara users dan roles.

**Columns:**
- `user_id` (UUID, NOT NULL): Foreign key ke users
- `role_id` (UUID, NOT NULL): Foreign key ke roles
- `created_at`, `created_by`, `updated_at`, `updated_by`, `deleted_at`, `deleted_by`: Audit fields
- **PRIMARY KEY**: (user_id, role_id)

**Indexes:**
- `idx_user_roles_user_id`: Index pada user_id
- `idx_user_roles_role_id`: Index pada role_id
- `idx_user_roles_deleted_at`: Index untuk soft delete

**Use Cases:**
- Assign roles to users
- Multi-role support per user
- Role-based authorization

---

## Migration 000004: Company Management Tables

Migration ini mencakup tabel untuk manajemen perusahaan dan relasi user-company.

### Tables

#### 1. `core.companies`
Tabel utama untuk menyimpan informasi perusahaan.

**Columns:**
- `id` (UUID, PK): Primary key dengan auto-generate UUID
- `owner_id` (UUID, NOT NULL): Foreign key ke users (pemilik perusahaan)
- `name` (VARCHAR(255), NOT NULL): Nama perusahaan
- `code` (VARCHAR(50), UNIQUE, NOT NULL): Kode unik perusahaan
- `tax_id` (VARCHAR(50)): NPWP atau tax ID
- `phone` (VARCHAR(20)): Nomor telepon perusahaan
- `email` (VARCHAR(255)): Email perusahaan
- `website` (VARCHAR(255)): Website perusahaan
- `logo_url` (VARCHAR(500)): URL logo perusahaan
- `address` (TEXT): Alamat lengkap
- `village_id` (UUID): Foreign key ke tabel desa/kelurahan
- `max_users` (INT): Batas maksimal user (default: 5)
- `max_branches` (INT): Batas maksimal cabang (default: 1)
- `is_active` (BOOLEAN): Status aktif perusahaan (default: TRUE)
- `created_at`, `created_by`, `updated_at`, `updated_by`, `deleted_at`, `deleted_by`: Audit fields

**Indexes:**
- `idx_companies_owner_id`: Index pada owner_id
- `idx_companies_code`: Index pada company code
- `idx_companies_deleted_at`: Index untuk soft delete
- `idx_companies_is_active`: Index pada status aktif
- `idx_companies_created_by`: Index pada creator
- `idx_companies_village_id`: Index pada village_id

**Use Cases:**
- Company registration dan profile management
- Multi-tenancy support
- Company ownership tracking
- Business limits management (users, branches)

---

#### 2. `core.company_users`
Junction table untuk relasi many-to-many antara companies dan users dengan metadata tambahan.

**Columns:**
- `id` (UUID, PK): Primary key
- `company_id` (UUID, NOT NULL): Foreign key ke companies
- `user_id` (UUID, NOT NULL): Foreign key ke users
- `role` (VARCHAR(50), NOT NULL): Role user di perusahaan (default: 'MEMBER')
- `is_primary` (BOOLEAN): Flag perusahaan utama user (default: FALSE)
- `is_active` (BOOLEAN): Status aktif membership (default: TRUE)
- `invited_by` (UUID): Foreign key ke users yang mengundang
- `joined_at` (TIMESTAMP): Waktu bergabung (default: CURRENT_TIMESTAMP)
- `created_at`, `created_by`, `updated_at`, `updated_by`, `deleted_at`, `deleted_by`: Audit fields
- **UNIQUE constraint**: (company_id, user_id)

**Indexes:**
- `idx_company_users_company_id`: Index pada company_id
- `idx_company_users_user_id`: Index pada user_id
- `idx_company_users_role`: Index pada role
- `idx_company_users_is_primary`: Index pada primary company flag
- `idx_company_users_is_active`: Index pada status aktif
- `idx_company_users_deleted_at`: Index untuk soft delete

**Use Cases:**
- Multi-company user management
- Company-specific roles (OWNER, ADMIN, MEMBER, etc.)
- User invitation tracking
- Primary company selection
- Company membership management

---

## Relationships & Foreign Keys

### Authentication Tables
- `email_verification_tokens.user_id` → `users.id`
- `password_reset_tokens.user_id` → `users.id`
- `refresh_tokens.user_id` → `users.id`
- `login_attempts.user_id` → `users.id`
- `audit_logs.user_id` → `users.id`
- `role_permissions.role_id` → `roles.id`
- `role_permissions.permission_id` → `permissions.id`
- `user_roles.user_id` → `users.id`
- `user_roles.role_id` → `roles.id`

### Company Tables
- `companies.owner_id` → `users.id`
- `companies.village_id` → `villages.id` (dari migration lain)
- `company_users.company_id` → `companies.id`
- `company_users.user_id` → `users.id`
- `company_users.invited_by` → `users.id`

---

## Security Features

### Authentication
1. **Password Security**: Password di-hash sebelum disimpan
2. **Account Locking**: Akun dikunci setelah beberapa percobaan login gagal
3. **Email Verification**: Validasi kepemilikan email
4. **Token Expiration**: Semua token memiliki masa kadaluarsa

### Session Management
1. **Refresh Tokens**: Token disimpan dalam bentuk hash
2. **Multi-Device Support**: Tracking session per device
3. **Token Revocation**: Logout mechanism dengan revoke token
4. **Device Tracking**: IP address dan user agent untuk security monitoring

### Audit & Monitoring
1. **Login Attempts**: Tracking semua percobaan login
2. **Audit Logs**: Mencatat semua aktivitas penting
3. **Soft Delete**: Data tidak dihapus permanen, menggunakan `deleted_at`
4. **Audit Fields**: Setiap perubahan data tercatat dengan timestamp dan user

### Authorization
1. **RBAC (Role-Based Access Control)**: Sistem role dan permission
2. **Granular Permissions**: Permission berbasis resource dan action
3. **Multi-Role Support**: User bisa memiliki multiple roles
4. **System Roles**: Role bawaan sistem yang protected

---

## Data Integrity

### Constraints
1. **UNIQUE Constraints**:
   - `users.email`, `users.username`
   - `roles.name`
   - `permissions.(resource, action)`
   - `companies.code`
   - `company_users.(company_id, user_id)`
   - Token fields untuk security

2. **NOT NULL Constraints**:
   - Critical fields seperti email, username, password_hash
   - Foreign keys yang wajib ada
   - Token dan expires_at fields

3. **DEFAULT Values**:
   - UUID generation menggunakan `gen_random_uuid()`
   - Boolean flags dengan default logical
   - Timestamps dengan `CURRENT_TIMESTAMP`

### Indexes Strategy
1. **Foreign Keys**: Semua foreign key di-index untuk JOIN performance
2. **Lookup Fields**: Email, username, token di-index untuk fast lookup
3. **Filter Fields**: Status, dates, dan flags di-index untuk filtering
4. **Soft Delete**: `deleted_at` di-index untuk query performance

---

## Best Practices

### Usage Guidelines
1. **Soft Delete**: Gunakan `deleted_at` instead of hard delete
2. **Audit Trail**: Selalu isi `created_by`, `updated_by` fields
3. **Token Security**: Token harus di-hash sebelum disimpan
4. **Index Usage**: Manfaatkan index untuk query optimization
5. **Batch Operations**: Gunakan transaction untuk operasi multiple tables

### Performance Optimization
1. **Index Coverage**: Query yang sering digunakan sudah ter-cover oleh index
2. **Pagination**: Gunakan limit dan offset untuk large datasets
3. **JSON Metadata**: Gunakan JSONB di audit_logs untuk flexible data
4. **Expire Old Data**: Periodic cleanup untuk expired tokens

### Security Guidelines
1. **Password Hashing**: Gunakan bcrypt atau argon2
2. **Token Generation**: Gunakan cryptographically secure random
3. **Rate Limiting**: Implement di application layer
4. **Input Validation**: Validate di application sebelum hit database
5. **SQL Injection**: Gunakan prepared statements/parameterized queries

---

## Rollback Instructions

Untuk rollback migration ini, jalankan:

```bash
# Rollback auth tables
migrate -path internal/database/migrations/core -database "postgresql://..." down 000002

# Rollback company tables
migrate -path internal/database/migrations/core -database "postgresql://..." down 000004
```

Atau gunakan file `down.sql` yang sesuai:
- `000002_auth_tables.down.sql`
- `000004_company_tables.down.sql`

---

## Notes

1. **UUID Extension**: Migration memastikan extension `uuid-ossp` enabled
2. **Schema**: Semua tabel menggunakan schema `core`
3. **Timestamps**: Menggunakan TIMESTAMP untuk semua date/time fields
4. **Varchar Lengths**: Disesuaikan dengan kebutuhan bisnis
5. **JSONB**: Digunakan di audit_logs untuk flexible metadata

---

## Version History

| Migration | Description | Date |
|-----------|-------------|------|
| 000002 | Authentication & Authorization Tables | - |
| 000004 | Company Management Tables | - |

---

## Related Files

- Migration Up:
  - [000002_auth_tables.up.sql](internal/database/migrations/core/000002_auth_tables.up.sql)
  - [000004_company_tables.up.sql](internal/database/migrations/core/000004_company_tables.up.sql)
- Migration Down:
  - [000002_auth_tables.down.sql](internal/database/migrations/core/000002_auth_tables.down.sql)
  - [000004_company_tables.down.sql](internal/database/migrations/core/000004_company_tables.down.sql)
