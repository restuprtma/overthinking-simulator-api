# Refresh Token - Quick Start Guide

## Setup (5 Menit)

### 1. Update Environment Variables

Tambahkan ke file `.env`:

```bash
# JWT Configuration
JWT_SECRET=your-secure-secret-key-change-in-production
JWT_EXPIRATION=24h

# Refresh Token Configuration
REFRESH_TOKEN_EXPIRATION=168h  # 7 hari
```

### 2. Run Migration

Migration untuk refresh tokens table sudah ada, jalankan:

```bash
# Migration sudah termasuk dalam 000002_auth_tables.up.sql
# Table core.refresh_tokens sudah otomatis dibuat
```

### 3. Restart Server

```bash
go run cmd/api/main.go
```

## API Endpoints

### 1. Login (Updated)
```bash
POST /core/v1/auth/signin
Content-Type: application/json

{
  "email": "user@example.com",
  "password": "password123"
}

# Response sekarang include refresh_token
{
  "success": true,
  "data": {
    "access_token": "eyJhbGci...",
    "refresh_token": "dGhpc19pc...",  # 👈 NEW!
    "token_type": "Bearer",
    "expires_in": 86400
  }
}
```

### 2. Refresh Token (NEW)
```bash
POST /core/v1/auth/refresh
Content-Type: application/json

{
  "refresh_token": "dGhpc19pc..."
}

# Response
{
  "success": true,
  "data": {
    "access_token": "eyJhbGci...",  # 👈 Access token baru
    "refresh_token": "bmV3X3Jl...",  # 👈 Refresh token baru
    "token_type": "Bearer",
    "expires_in": 86400
  }
}
```

### 3. Revoke Token (NEW)
```bash
POST /core/v1/auth/revoke
Content-Type: application/json

{
  "refresh_token": "dGhpc19pc..."
}

# Response
{
  "success": true,
  "data": {
    "message": "Token revoked successfully"
  }
}
```

## Frontend Integration

### JavaScript/TypeScript Example

```javascript
// 1. Login dan simpan tokens
async function login(email, password) {
  const response = await fetch('/core/v1/auth/signin', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ email, password })
  });

  const { data } = await response.json();
  localStorage.setItem('access_token', data.access_token);
  localStorage.setItem('refresh_token', data.refresh_token);
}

// 2. API call dengan auto-refresh
async function apiCall(url, options = {}) {
  const accessToken = localStorage.getItem('access_token');

  let response = await fetch(url, {
    ...options,
    headers: {
      ...options.headers,
      'Authorization': `Bearer ${accessToken}`
    }
  });

  // Jika 401, refresh dan retry
  if (response.status === 401) {
    const refreshed = await refreshToken();
    if (refreshed) {
      const newAccessToken = localStorage.getItem('access_token');
      response = await fetch(url, {
        ...options,
        headers: {
          ...options.headers,
          'Authorization': `Bearer ${newAccessToken}`
        }
      });
    }
  }

  return response;
}

// 3. Refresh token
async function refreshToken() {
  const refreshToken = localStorage.getItem('refresh_token');

  try {
    const response = await fetch('/core/v1/auth/refresh', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ refresh_token: refreshToken })
    });

    if (response.ok) {
      const { data } = await response.json();
      localStorage.setItem('access_token', data.access_token);
      localStorage.setItem('refresh_token', data.refresh_token);
      return true;
    }
  } catch (error) {
    console.error('Refresh failed:', error);
  }

  // Jika gagal, logout
  logout();
  return false;
}

// 4. Logout
async function logout() {
  const refreshToken = localStorage.getItem('refresh_token');

  await fetch('/core/v1/auth/revoke', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ refresh_token: refreshToken })
  });

  localStorage.removeItem('access_token');
  localStorage.removeItem('refresh_token');
  window.location.href = '/login';
}
```

## How It Works

### Token Lifecycle

```
1. LOGIN
   ↓
   User mendapat: Access Token (24h) + Refresh Token (7d)
   ↓
2. API CALLS
   ↓
   Gunakan Access Token di Authorization header
   ↓
3. ACCESS TOKEN EXPIRED (setelah 24h)
   ↓
   Frontend deteksi 401 error
   ↓
4. REFRESH
   ↓
   Kirim Refresh Token ke /auth/refresh
   ↓
   Dapat Access Token baru + Refresh Token baru
   ↓
   Retry API call yang gagal
   ↓
5. REFRESH TOKEN EXPIRED (setelah 7d)
   ↓
   User harus login ulang
```

### Security Features

✅ **Token Rotation**: Refresh token otomatis diganti setiap digunakan
✅ **Hashing**: Refresh token di-hash (SHA256) di database
✅ **Revocation**: Logout = revoke refresh token
✅ **Expiration**: Kedua token punya waktu expired
✅ **One-time Use**: Refresh token lama di-revoke saat digunakan

## Testing

### Test dengan curl

```bash
# 1. Login
curl -X POST http://localhost:8080/core/v1/auth/signin \
  -H "Content-Type: application/json" \
  -d '{"email":"test@example.com","password":"password123"}'

# Copy refresh_token dari response

# 2. Refresh token
curl -X POST http://localhost:8080/core/v1/auth/refresh \
  -H "Content-Type: application/json" \
  -d '{"refresh_token":"PASTE_REFRESH_TOKEN_HERE"}'

# 3. Revoke token
curl -X POST http://localhost:8080/core/v1/auth/revoke \
  -H "Content-Type: application/json" \
  -d '{"refresh_token":"PASTE_REFRESH_TOKEN_HERE"}'
```

## Troubleshooting

| Error | Penyebab | Solusi |
|-------|----------|--------|
| "Invalid or expired refresh token" | Token sudah expired/invalid | User harus login ulang |
| "Refresh token has been revoked" | Token sudah di-revoke (logout) | User harus login ulang |
| "User account is not active" | Account di-disable | Aktivasi account |

## Configuration Options

| Environment Variable | Default | Deskripsi |
|---------------------|---------|-----------|
| `JWT_EXPIRATION` | `24h` | Durasi access token |
| `REFRESH_TOKEN_EXPIRATION` | `168h` (7 hari) | Durasi refresh token |
| `JWT_SECRET` | - | Secret key untuk JWT (WAJIB di production) |

## What Changed

### Files Modified:
1. ✅ [auth.service.go](../internal/modules/core/auth/service/auth.service.go) - Added refresh token generation in SignIn
2. ✅ [auth.service.go](../internal/modules/core/auth/service/auth.service.go) - Added RefreshToken() and RevokeToken() methods
3. ✅ [auth.handler.go](../internal/modules/core/auth/handler/auth.handler.go) - Added handlers
4. ✅ [main.auth.go](../internal/modules/core/auth/main.auth.go) - Registered routes
5. ✅ [token/generator.go](../pkg/token/generator.go) - Added HashRefreshToken() function

### Files Created:
1. ✅ [refresh.dto.go](../internal/modules/core/auth/dto/refresh.dto.go) - DTOs untuk refresh token

### Already Exists (No Changes Needed):
- ✅ Database migration (000002_auth_tables.up.sql)
- ✅ RefreshToken domain model
- ✅ Token repository methods
- ✅ JWT utilities

## Next Steps

1. Update frontend untuk handle refresh tokens
2. Implement automatic token refresh sebelum expired
3. Add refresh token rotation detection (optional)
4. Monitor token usage di database

## Full Documentation

Lihat [REFRESH_TOKEN.md](./REFRESH_TOKEN.md) untuk dokumentasi lengkap termasuk:
- Architecture details
- Security best practices
- Database queries
- Monitoring guide
- Advanced features

---

**Status:** ✅ IMPLEMENTED & READY TO USE

**Last Updated:** 2025-10-12
