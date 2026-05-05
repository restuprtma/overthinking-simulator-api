# Authentication API Contract

**Version:** v1
**Base URL:** `/core/v1/auth`
**Module:** Core Authentication
**Last Updated:** 2026-04-20

---

## ⚠️ Breaking Change (2026-04-20) — JWT no longer carries `permissions`

Sebelumnya JWT access token berisi array `permissions` yang ~3.8 KB di kasus user dengan banyak permission → sering kena block ingress (HTTP 400 Bad Request).

Mulai sekarang:

- **JWT tidak lagi berisi field `permissions`.** Permissions disimpan di Redis dan di-lookup backend saat setiap request masuk.
- **Response body TETAP return `permissions`** di `/signin` dan `/switch-company` — FE ambil dari sana.
- **`/refresh` TIDAK mengubah permissions** (user & company sama). FE cukup pakai permissions yang sudah disimpan dari signin/switch-company sebelumnya.

### Yang perlu FE lakukan

| Skenario lama | Skenario baru |
|---|---|
| FE decode JWT → ambil `claims.permissions` | FE baca `response.data.permissions` dari signin / switch-company |
| Setelah `switch-company`, FE decode JWT baru buat update UI | FE baca `response.data.permissions` dari switch-company (field ini **baru ditambah**) |
| FE decode JWT setelah `refresh` | Tidak perlu — permissions tidak berubah saat refresh |

**Cara check permission di FE:** `response.data.permissions.includes("finance.contacts:read")` (tidak berubah — polanya sama, cuma sumbernya beda).

**Apa yang masih bisa di-decode dari JWT:** `user_id`, `company_id`, `company_name`, `client_id`, `client_slug`, `email`, `username`, `full_name`, `is_super_admin`, `roles`, `exp`, `iat`.

---

## Overview

Authentication API untuk registrasi, login, token management, dan multi-tenant company switching. Menggunakan JWT + refresh token.

**Key Features:**
- JWT-based authentication dengan refresh token rotation
- Multi-device session management
- Multi-tenant company switching
- Account lockout protection (5x gagal login = lock 15 menit)
- Server-side permission cache di Redis (ganti JWT-embedded permissions)

---

## Response Format

Semua response mengikuti format:

```json
{
  "data": { ... },
  "message": "Success message",
  "meta": null,
  "errors": null
}
```

Error response:

```json
{
  "data": null,
  "message": "Error message",
  "meta": null,
  "errors": "detail error string"
}
```

---

## JWT Claims Structure

Access token berisi **identity claims saja** — permissions tidak ada di sini (lihat Breaking Change di atas).

```json
{
  "user_id": "550e8400-...",
  "email": "user@example.com",
  "username": "johndoe",
  "full_name": "John Doe",
  "company_id": "660e8400-...",
  "company_name": "PT Wizhub Indonesia",
  "client_id": "770e8400-...",
  "client_slug": "wizhub",
  "is_super_admin": false,
  "roles": ["administrator"],
  "exp": 1735689600,
  "iat": 1735603200,
  "nbf": 1735603200,
  "iss": "wizhub-api",
  "sub": "550e8400-..."
}
```

### Penjelasan Claims

| Claim | Type | Keterangan |
|-------|------|------------|
| `user_id` | string (UUID) | ID user (juga duplikat di `sub`) |
| `email` | string | Email user |
| `username` | string | Username |
| `full_name` | string | Nama lengkap |
| `company_id` | string (UUID) | Company aktif. `""` jika user belum switch company |
| `company_name` | string | Nama company aktif. `""` jika belum switch |
| `client_id` | string (UUID) | Registration-level tenant ID (parent dari company) |
| `client_slug` | string | DNS-safe client slug — dipakai FE untuk bootstrap translation overrides |
| `is_super_admin` | bool | `true` = bypass semua permission check di backend |
| `roles` | string[] | Role codes aktif user (mis. `["administrator"]`) |
| `exp` | int64 | Expiry (Unix timestamp, default 24 jam) |
| `iat` | int64 | Issued-at (Unix timestamp) |
| `nbf` | int64 | Not-before (Unix timestamp) |
| `iss` | string | Selalu `"wizhub-api"` |
| `sub` | string | Subject = `user_id` |

> **Catatan:** Jangan bergantung pada urutan field. Field yang tidak kamu pakai boleh diabaikan.

### Permissions (di response body, bukan di JWT)

Permissions sekarang dikirim lewat **response body** pada endpoint berikut:
- `POST /signin` → `data.permissions`
- `POST /switch-company` → `data.permissions`

Format string tidak berubah: flat array `"resource:action"`. Contoh:
```json
[
  "finance.contacts:read",
  "finance.contacts:create",
  "user-management:read"
]
```

**FE check:** `response.data.permissions.includes("finance.contacts:create")`

### Cara permissions di-derive

Di database, role menyimpan permissions format **level-based**:
```json
{"dashboard": "viewer", "user-management": "admin"}
```

Backend expand level ke actions:
- `viewer` → `["read"]`
- `editor` → `["create", "read", "update", "delete"]`
- `admin` → `["create", "read", "update", "delete", "export", "import", "restore", "approve"]`

Sehingga response-nya flat: `["dashboard:read", "user-management:create", "user-management:read", ...]`. FE tidak perlu tahu level — cukup pakai flat array.

### Invalidation / kapan permissions berubah

Setelah signin/switch-company, permissions **tetap valid sampai** salah satu dari:
1. Admin edit permissions role yang dipegang user
2. Admin assign/remove role user
3. User logout

Kalau FE dapet **403 Forbidden** untuk aksi yang sebelumnya diizinkan → kemungkinan permissions telah berubah. Rekomendasi handling: logout user dan minta login ulang (agar dapet list permissions terbaru).

---

## Endpoints

### 1. Sign In

Autentikasi user dengan email/username dan password.

```
POST /core/v1/auth/signin
```

**Auth:** Tidak diperlukan

**Request:**
```json
{
  "login": "admin@wizhub.com",
  "password": "admin123"
}
```

| Field | Type | Required | Keterangan |
|-------|------|----------|------------|
| `login` | string | Ya | Email atau username |
| `password` | string | Ya | Min 8 karakter |

**Response (200):**
```json
{
  "data": {
    "access_token": "eyJhbGci...",
    "refresh_token": "a1b2c3d4e5...",
    "token_type": "Bearer",
    "expires_in": 86400,
    "user": {
      "id": "550e8400-...",
      "email": "admin@wizhub.com",
      "username": "admin",
      "full_name": "Admin Wizhub"
    },
    "company": {
      "id": "660e8400-...",
      "name": "PT Wizhub Indonesia"
    },
    "client": {
      "id": "770e8400-...",
      "slug": "wizhub",
      "name": "Wizhub"
    },
    "roles": ["administrator"],
    "permissions": [
      "finance.contacts:read",
      "finance.contacts:create",
      "user-management:read"
    ]
  },
  "message": "Sign in successful"
}
```

**Response fields:**

| Field | Type | Keterangan |
|-------|------|------------|
| `access_token` | string | JWT. Pakai di header `Authorization: Bearer <token>` |
| `refresh_token` | string | Opaque token 32 byte. Simpan aman (HttpOnly cookie / secure storage) |
| `token_type` | string | Selalu `"Bearer"` |
| `expires_in` | int | Lifetime access_token dalam detik (default 86400 = 24 jam) |
| `user` | object | Info user signed-in |
| `company` | object \| `null` | Primary company user. `null` jika user belum punya company |
| `client` | object \| `null` | Registration-level tenant (parent dari company). `null` jika belum ada company |
| `roles` | string[] | Role codes user di primary company (mis. `["administrator"]`) |
| `permissions` | string[] | **Flat permission list** — FE pakai untuk gating UI |

> `company`, `client` bisa `null` jika user belum punya primary company. Dalam case ini, `roles` dan `permissions` akan kosong `[]` atau berisi role global saja.

**Errors:**

| Status | Message | Kapan |
|--------|---------|-------|
| 400 | Invalid request payload | Payload tidak valid |
| 401 | Invalid credentials | Email/password salah |
| 401 | User account is not active | `is_active = false` |
| 401 | User account is locked | Terkunci setelah 5x gagal |

---

### 2. Sign Up

Registrasi user baru. Otomatis membuat company dan cabang default "Cabang Pusat".

```
POST /core/v1/auth/signup
```

**Auth:** Tidak diperlukan

**Request:**
```json
{
  "email": "newuser@example.com",
  "username": "johndoe",
  "password": "SecurePass123",
  "full_name": "John Doe",
  "phone": "+6281234567890",
  "company_name": "PT Contoh Sejahtera"
}
```

| Field | Type | Required | Validasi |
|-------|------|----------|----------|
| `email` | string | Ya | Format email valid |
| `username` | string | Ya | 3-100 karakter |
| `password` | string | Ya | Min 8 karakter |
| `full_name` | string | Tidak | Max 255 karakter |
| `phone` | string | Tidak | Max 20 karakter |
| `company_name` | string | Ya | 2-255 karakter |

**Response (201):**
```json
{
  "data": {
    "message": "User registered successfully. Please verify your email.",
    "user": {
      "id": "550e8400-...",
      "email": "newuser@example.com",
      "username": "johndoe",
      "full_name": "John Doe"
    },
    "company": {
      "id": "660e8400-...",
      "name": "PT Contoh Sejahtera"
    }
  },
  "message": "User registered successfully. Please verify your email."
}
```

> **Auto-provisioning saat registrasi:**
> 1. User dibuat dengan `is_active=true`, `is_email_verified=false`
> 2. Company dibuat dengan type `holding`, owner = user baru
> 3. User di-link ke company sebagai primary member (`is_primary=true`)
> 4. Branch "Cabang Pusat" dibuat sebagai default branch (`is_default=true`)

**Errors:**

| Status | Message |
|--------|---------|
| 400 | Invalid request payload |
| 409 | Email already exists |
| 409 | Username already exists |

---

### 3. Refresh Token

Generate access token baru menggunakan refresh token.

```
POST /core/v1/auth/refresh
```

**Auth:** Tidak diperlukan

**Request:**
```json
{
  "refresh_token": "a1b2c3d4e5..."
}
```

**Response (200):**
```json
{
  "data": {
    "access_token": "eyJhbGci...",
    "refresh_token": "z9y8x7w6...",
    "token_type": "Bearer",
    "expires_in": 86400
  },
  "message": "Token refreshed successfully"
}
```

> **Token Rotation:** Setiap refresh menghasilkan refresh token BARU. Token lama otomatis di-revoke.

**Errors:**

| Status | Message |
|--------|---------|
| 401 | Invalid refresh token |
| 401 | Refresh token expired |
| 401 | Refresh token revoked |

---

### 4. Logout (Single Device)

Revoke satu refresh token.

```
POST /core/v1/auth/logout
```

**Auth:** `Authorization: Bearer <access_token>`

**Request:**
```json
{
  "refresh_token": "a1b2c3d4e5..."
}
```

**Response (200):**
```json
{
  "data": null,
  "message": "Logged out successfully"
}
```

---

### 5. Logout All Devices

Revoke semua refresh token milik user.

```
POST /core/v1/auth/logout-all
```

**Auth:** `Authorization: Bearer <access_token>`

**Request:** Body kosong

**Response (200):**
```json
{
  "data": null,
  "message": "Logged out from all devices successfully"
}
```

---

### 6. Switch Company

Pindah konteks company aktif. Menghasilkan JWT baru dengan `company_id` dan `roles` sesuai company target. Permissions dikirim di response body (tidak lagi di JWT).

```
POST /core/v1/auth/switch-company
```

**Auth:** `Authorization: Bearer <access_token>`

**Request:**
```json
{
  "company_id": "660e8400-..."
}
```

**Response (200):**
```json
{
  "data": {
    "access_token": "eyJhbGci...",
    "refresh_token": "z9y8x7w6...",
    "token_type": "Bearer",
    "expires_in": 86400,
    "company": {
      "id": "660e8400-...",
      "name": "PT Wizhub Indonesia"
    },
    "roles": ["manager"],
    "permissions": [
      "finance.contacts:read",
      "finance.cash_transactions:read",
      "finance.cash_transactions:create"
    ]
  },
  "message": "Company switched successfully"
}
```

> **Baru sejak 2026-04-20:** field `roles` dan `permissions` kini dikirim di response body. FE wajib **replace** permissions yang disimpan sebelumnya dengan nilai dari response ini — tidak lagi bisa didapat dari decode JWT.

**Response fields:**

| Field | Type | Keterangan |
|-------|------|------------|
| `access_token` | string | JWT baru dengan `company_id` target |
| `refresh_token` | string | Refresh token baru (token rotation) |
| `token_type` | string | Selalu `"Bearer"` |
| `expires_in` | int | Lifetime access_token (detik) |
| `company` | object | Company target |
| `roles` | string[] | Role codes user di company target. Bisa beda dari primary company |
| `permissions` | string[] | Flat permissions di company target. **Wajib FE refresh state permissions** dengan nilai ini |

**Errors:**

| Status | Message | Kapan |
|--------|---------|-------|
| 403 | You are not a member of this company | User bukan member |
| 404 | Company not found | Company tidak ada |

---

### 7. Get Me (Profile + Effective Permissions)

Ambil profile user yang sedang login beserta **effective permissions** untuk company aktif. Permissions dibaca dari Redis (authz cache), bukan dari decode JWT.

```
GET /core/v1/auth/me
```

**Auth:** `Authorization: Bearer <access_token>`

**Response (200):**
```json
{
  "data": {
    "user": {
      "id": "550e8400-...",
      "email": "admin@wizhub.com",
      "username": "admin",
      "full_name": "Admin Wizhub"
    },
    "company": {
      "id": "660e8400-...",
      "name": "PT Wizhub Indonesia"
    },
    "client": {
      "id": "770e8400-...",
      "slug": "wizhub",
      "name": "Wizhub"
    },
    "roles": ["administrator"],
    "permissions": [
      "finance.contacts:read",
      "finance.contacts:create",
      "user-management:read"
    ],
    "is_super_admin": false
  },
  "message": "Profile retrieved successfully"
}
```

> `company` dan `client` bisa `null` jika user belum switch company.

**Kapan FE memanggil endpoint ini:**

1. **Setelah page reload** — untuk re-hydrate permissions state tanpa simpan di localStorage (data sensitif)
2. **Setelah dapet 403 tak terduga** — untuk cek apakah permissions user telah berubah (admin revoke role misal). Kalau `permissions` turun, update state & tampilkan notif. Kalau tetap sama, berarti memang user tidak punya akses.
3. **Periodik (opsional)** — kalau app-mu butuh real-time awareness terhadap perubahan permissions, poll `/me` tiap N menit. Tapi untuk kebanyakan app, cara (1) dan (2) sudah cukup.

**Perbedaan dari signin/switch-company:**

| | `/signin` | `/switch-company` | `/me` |
|---|---|---|---|
| Return tokens? | ✅ ya | ✅ ya | ❌ tidak |
| Mengubah server state? | ✅ ya | ✅ ya | ❌ read-only |
| Bisa dipanggil berulang? | ❌ (butuh password) | ❌ (butuh company_id baru) | ✅ unlimited |
| Source permissions? | DB + isi cache | DB + isi cache | Redis cache (fallback DB) |

**Errors:**

| Status | Message | Kapan |
|--------|---------|-------|
| 401 | User not active | User di-deactivate setelah JWT di-issue |
| 401 | Authentication required | Token missing/invalid/expired |

---

### 8. Get My Companies

Ambil daftar company yang user login punya akses. Digunakan FE untuk menampilkan branch tree, company switcher, dll.

```
GET /core/v1/auth/companies
```

**Auth:** `Authorization: Bearer <access_token>`

**Response (200):**
```json
{
  "data": [
    {
      "id": "20000000-...",
      "name": "PT Wizhub Indonesia",
      "type": "holding",
      "logo_url": "https://cdn.example.com/logos/wizhub-id.png",
      "parent_id": null,
      "is_primary": true,
      "is_owner": true,
      "role_name": "Super Admin",
      "role_code": "super_admin"
    },
    {
      "id": "20000001-...",
      "name": "Wizhub Jakarta",
      "type": "subsidiary",
      "logo_url": null,
      "parent_id": "20000000-...",
      "is_primary": false,
      "is_owner": false,
      "role_name": "Manager",
      "role_code": "manager"
    }
  ],
  "message": "Companies retrieved successfully"
}
```

**Field notes:**
- `type` — `holding` atau `subsidiary` (lihat enum `core.company_type`).
- `parent_id` — `null` untuk holding/root company.
- `is_primary` — menandai company default user (hanya satu per user). FE bisa auto-select ini setelah login.
- `is_owner` — `true` jika user adalah `owner_id` dari company tersebut.
- `role_name` / `role_code` — peran user di company ini (bisa `null` jika membership belum di-assign role).

> Hanya company dimana user terdaftar sebagai member (via `company_users`). Super admin yang terdaftar di 2 company hanya lihat 2 company tersebut — bukan semua company di sistem.
> Urutan: `is_primary DESC`, lalu `name ASC`.

---

## Authentication Flow

```
1. POST /auth/signup           → Registrasi + auto-create company & branch "Cabang Pusat"
2. POST /auth/signin           → Login, dapat token + company + roles + permissions
3. Semua request pakai header: Authorization: Bearer <access_token>
4. GET  /auth/me               → (Opsional) Rehydrate profile+permissions setelah reload
5. GET  /auth/companies        → Ambil daftar company yang bisa diakses
6. POST /auth/switch-company   → (Opsional) Pindah company, dapat token + permissions baru
7. POST /auth/refresh          → Saat access_token expired, tukar refresh_token
8. POST /auth/logout           → Logout dari device ini
```

### Token Expiry

| Token | Default Expiry |
|-------|---------------|
| Access token | 24 jam |
| Refresh token | 7 hari |

---

## FE Integration Checklist (migrasi dari JWT-embedded permissions)

Kalau FE kamu sebelumnya pakai permissions dari JWT, berikut langkah migrasinya:

- [ ] Stop decode JWT untuk ambil `permissions`. Hapus semua `jwtDecode(token).permissions`.
- [ ] Simpan `response.data.permissions` dari `/signin` ke state/store FE (Pinia, Redux, dst).
- [ ] Pada `/switch-company`, **update** state permissions dengan `response.data.permissions` (field ini baru ditambahkan).
- [ ] Pada `/refresh`, **tidak perlu update** permissions — user & company sama, permissions tidak berubah.
- [ ] Pada **page reload**, call `GET /auth/me` untuk rehydrate permissions (kalau tidak disimpan di persistent storage). Kalau user sudah punya token di memory, `/me` lebih aman daripada simpan permissions di localStorage.
- [ ] Kalau dapet **403 tak terduga**, call `GET /auth/me` dulu untuk cek apakah permissions telah berubah. Kalau berubah (admin revoke), update state + show toast "Your permissions have changed". Kalau tidak berubah, artinya user memang tidak punya akses — biarkan error handler normal jalan.
- [ ] Cek ukuran request header setelah deploy — harusnya drop dari ~4 KB ke < 1 KB, gak akan kena block ingress lagi.

**Rough size comparison:**

| | Access token size |
|---|---|
| Sebelum (permissions di JWT) | ~3.8 KB (untuk user dengan 160 permissions) |
| Sesudah | ~550 bytes |

### Rekomendasi state management FE

```
┌─────────────┐
│   Signin    │──┐
└─────────────┘  │
                 ├──► permissions state (Pinia/Redux/Zustand)
┌─────────────┐  │
│Switch Company──┤
└─────────────┘  │
                 │
┌─────────────┐  │
│  GET /me    │──┘  (dipanggil saat reload atau setelah 403)
└─────────────┘
```

Jangan simpan permissions di `localStorage` / `sessionStorage` — sensitif dan bisa stale. Cukup state in-memory + panggil `/me` saat reload.

---

**Last Updated:** 2026-04-20 (JWT permissions extraction → Redis cache)
