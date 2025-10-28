# Refresh Token Implementation

## Overview

Mekanisme refresh token telah diimplementasikan untuk meningkatkan keamanan dan user experience dalam Lakukan Backend API. Refresh token memungkinkan user untuk mendapatkan access token baru tanpa harus login ulang.

## Konsep

### Token Types

1. **Access Token (JWT)**
   - Durasi pendek (default: 24 jam)
   - Digunakan untuk autentikasi API requests
   - Berisi user info, roles, dan permissions
   - Tidak dapat di-revoke

2. **Refresh Token**
   - Durasi panjang (default: 7 hari)
   - Digunakan untuk mendapatkan access token baru
   - Disimpan di database (hashed)
   - Dapat di-revoke
   - One-time use (rotasi otomatis)

### Security Features

1. **Token Hashing**: Refresh token di-hash menggunakan SHA256 sebelum disimpan di database
2. **Token Rotation**: Setiap kali refresh token digunakan, token lama di-revoke dan token baru dibuat
3. **Expiration**: Refresh token memiliki masa berlaku yang dapat dikonfigurasi
4. **Revocation**: Refresh token dapat di-revoke untuk logout atau security purposes
5. **Device Tracking**: Optional tracking untuk device info, IP address, dan user agent

## Architecture

### Database Schema

Table `core.refresh_tokens`:
```sql
CREATE TABLE core.refresh_tokens (
    id UUID PRIMARY KEY,
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
```

### Components

1. **Domain** ([internal/modules/core/auth/domain/token.go](../internal/modules/core/auth/domain/token.go))
   - `RefreshToken` struct

2. **Repository** ([internal/modules/core/auth/repository/token.repository.go](../internal/modules/core/auth/repository/token.repository.go))
   - `CreateRefreshToken()` - Simpan refresh token baru
   - `FindRefreshToken()` - Cari refresh token berdasarkan hash
   - `UpdateRefreshTokenLastUsed()` - Update timestamp penggunaan
   - `RevokeRefreshToken()` - Revoke token tertentu
   - `RevokeAllUserRefreshTokens()` - Revoke semua token user
   - `GetUserRefreshTokens()` - List semua active tokens user

3. **Service** ([internal/modules/core/auth/service/auth.service.go](../internal/modules/core/auth/service/auth.service.go))
   - `SignIn()` - Generate access & refresh tokens saat login
   - `RefreshToken()` - Exchange refresh token untuk tokens baru
   - `RevokeToken()` - Revoke refresh token

4. **Handler** ([internal/modules/core/auth/handler/auth.handler.go](../internal/modules/core/auth/handler/auth.handler.go))
   - `RefreshToken()` - POST /auth/refresh
   - `RevokeToken()` - POST /auth/revoke

5. **Helper** ([pkg/token/generator.go](../pkg/token/generator.go))
   - `GenerateRefreshToken()` - Generate random token
   - `HashRefreshToken()` - Hash token dengan SHA256

## API Endpoints

### 1. Sign In (Updated)

Generate access dan refresh tokens saat login.

**Endpoint:** `POST /core/v1/auth/signin`

**Request:**
```json
{
  "email": "user@example.com",
  "password": "password123"
}
```

**Response:**
```json
{
  "success": true,
  "message": "Sign in successful",
  "data": {
    "access_token": "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9...",
    "refresh_token": "dGhpc19pc19hX3JlZnJlc2hfdG9rZW5fZXhhbXBsZQ==",
    "token_type": "Bearer",
    "expires_in": 86400,
    "user": {
      "id": "uuid",
      "email": "user@example.com",
      "username": "username",
      "full_name": "User Name"
    }
  }
}
```

### 2. Refresh Token

Exchange refresh token untuk mendapatkan access dan refresh token baru.

**Endpoint:** `POST /core/v1/auth/refresh`

**Request:**
```json
{
  "refresh_token": "dGhpc19pc19hX3JlZnJlc2hfdG9rZW5fZXhhbXBsZQ=="
}
```

**Response:**
```json
{
  "success": true,
  "message": "Token refreshed successfully",
  "data": {
    "access_token": "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9...",
    "refresh_token": "bmV3X3JlZnJlc2hfdG9rZW5fZXhhbXBsZQ==",
    "token_type": "Bearer",
    "expires_in": 86400
  }
}
```

**Error Responses:**
- `401 Unauthorized` - Invalid or expired refresh token
- `401 Unauthorized` - Refresh token has been revoked
- `401 Unauthorized` - User account is not active

### 3. Revoke Token

Revoke refresh token (logout dari device tertentu).

**Endpoint:** `POST /core/v1/auth/revoke`

**Request:**
```json
{
  "refresh_token": "dGhpc19pc19hX3JlZnJlc2hfdG9rZW5fZXhhbXBsZQ=="
}
```

**Response:**
```json
{
  "success": true,
  "message": "Token revoked successfully",
  "data": {
    "message": "Token revoked successfully"
  }
}
```

## Configuration

Tambahkan environment variables berikut di `.env`:

```bash
# JWT Access Token Configuration
JWT_SECRET=your-secret-key-change-in-production
JWT_EXPIRATION=24h

# Refresh Token Configuration
REFRESH_TOKEN_EXPIRATION=168h  # 7 days (7 * 24h)
```

### Expiration Format

Gunakan format duration Go:
- `h` - hours (e.g., `24h` = 24 jam)
- `m` - minutes (e.g., `30m` = 30 menit)
- `s` - seconds (e.g., `3600s` = 1 jam)

Kombinasi: `72h30m` = 72 jam 30 menit

## Usage Examples

### Frontend Implementation (JavaScript/TypeScript)

#### 1. Store Tokens

```javascript
// Setelah login, simpan tokens
const loginResponse = await fetch('/core/v1/auth/signin', {
  method: 'POST',
  headers: { 'Content-Type': 'application/json' },
  body: JSON.stringify({ email, password })
});

const { data } = await loginResponse.json();

// Simpan di localStorage atau secure storage
localStorage.setItem('access_token', data.access_token);
localStorage.setItem('refresh_token', data.refresh_token);
```

#### 2. API Request dengan Auto-Refresh

```javascript
async function apiRequest(url, options = {}) {
  // Tambahkan access token ke header
  const accessToken = localStorage.getItem('access_token');
  const headers = {
    ...options.headers,
    'Authorization': `Bearer ${accessToken}`
  };

  let response = await fetch(url, { ...options, headers });

  // Jika access token expired, refresh dan retry
  if (response.status === 401) {
    const refreshed = await refreshAccessToken();
    if (refreshed) {
      // Retry request dengan token baru
      const newAccessToken = localStorage.getItem('access_token');
      headers.Authorization = `Bearer ${newAccessToken}`;
      response = await fetch(url, { ...options, headers });
    }
  }

  return response;
}

async function refreshAccessToken() {
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
    console.error('Failed to refresh token:', error);
  }

  // Jika refresh gagal, logout user
  localStorage.removeItem('access_token');
  localStorage.removeItem('refresh_token');
  window.location.href = '/login';
  return false;
}
```

#### 3. Logout

```javascript
async function logout() {
  const refreshToken = localStorage.getItem('refresh_token');

  // Revoke refresh token di server
  await fetch('/core/v1/auth/revoke', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ refresh_token: refreshToken })
  });

  // Clear local storage
  localStorage.removeItem('access_token');
  localStorage.removeItem('refresh_token');

  // Redirect to login
  window.location.href = '/login';
}
```

### Mobile Implementation (Example)

#### React Native / Expo

```javascript
import * as SecureStore from 'expo-secure-store';

// Simpan tokens securely
await SecureStore.setItemAsync('access_token', data.access_token);
await SecureStore.setItemAsync('refresh_token', data.refresh_token);

// Retrieve tokens
const accessToken = await SecureStore.getItemAsync('access_token');
const refreshToken = await SecureStore.getItemAsync('refresh_token');
```

## Flow Diagram

### Login Flow
```
User → Frontend → POST /auth/signin
                ↓
              Backend
                ↓
    Generate Access Token (JWT)
                ↓
    Generate Refresh Token
                ↓
    Hash & Store in Database
                ↓
    Return Both Tokens
                ↓
              Frontend
                ↓
    Store Tokens Securely
```

### Refresh Flow
```
Frontend → Check Access Token Expired
              ↓
    POST /auth/refresh with Refresh Token
              ↓
           Backend
              ↓
    Validate Refresh Token
              ↓
    Check Expiration & Revocation
              ↓
    Generate New Access Token
              ↓
    Generate New Refresh Token
              ↓
    Revoke Old Refresh Token
              ↓
    Save New Refresh Token
              ↓
    Return New Tokens
              ↓
          Frontend
              ↓
    Update Stored Tokens
              ↓
    Retry Original Request
```

### Logout Flow
```
Frontend → POST /auth/revoke with Refresh Token
              ↓
           Backend
              ↓
    Find Refresh Token
              ↓
    Mark as Revoked
              ↓
    Return Success
              ↓
          Frontend
              ↓
    Clear Stored Tokens
              ↓
    Redirect to Login
```

## Security Best Practices

### Backend

1. **Always hash refresh tokens** sebelum menyimpan ke database
2. **Rotate tokens** setiap kali digunakan
3. **Set appropriate expiration times**:
   - Access token: pendek (15min - 24h)
   - Refresh token: panjang (7 days - 30 days)
4. **Log all token operations** untuk audit trail
5. **Rate limit** refresh endpoint untuk mencegah abuse
6. **Monitor suspicious activities** (multiple failed refresh attempts)

### Frontend

1. **Store tokens securely**:
   - Web: HttpOnly cookies (preferred) atau localStorage
   - Mobile: Secure storage (Keychain/Keystore)
2. **Never log or expose** refresh tokens
3. **Clear tokens** saat logout
4. **Handle token refresh** sebelum access token expired
5. **Implement automatic retry** dengan exponential backoff

## Troubleshooting

### Common Issues

**1. "Invalid or expired refresh token"**
- Refresh token sudah expired
- Refresh token sudah di-revoke
- Refresh token tidak valid
- **Solution**: User harus login ulang

**2. "User account is not active"**
- User account telah di-disable
- **Solution**: Contact admin atau aktivasi kembali account

**3. "Failed to generate refresh token"**
- Error saat generate random token
- **Solution**: Check crypto/rand availability di system

**4. "Failed to save refresh token"**
- Database connection error
- **Solution**: Check database connectivity

## Monitoring & Maintenance

### Database Cleanup

Refresh tokens yang expired atau revoked harus dibersihkan secara berkala:

```sql
-- Delete expired tokens (older than 30 days)
DELETE FROM core.refresh_tokens
WHERE expires_at < NOW() - INTERVAL '30 days';

-- Delete revoked tokens (older than 30 days)
DELETE FROM core.refresh_tokens
WHERE revoked_at IS NOT NULL
  AND revoked_at < NOW() - INTERVAL '30 days';
```

### Monitoring Queries

```sql
-- Count active refresh tokens per user
SELECT user_id, COUNT(*) as token_count
FROM core.refresh_tokens
WHERE revoked_at IS NULL
  AND expires_at > NOW()
GROUP BY user_id
ORDER BY token_count DESC;

-- Find users with many devices
SELECT user_id, COUNT(DISTINCT device_id) as device_count
FROM core.refresh_tokens
WHERE revoked_at IS NULL
  AND expires_at > NOW()
  AND device_id IS NOT NULL
GROUP BY user_id
HAVING COUNT(DISTINCT device_id) > 5;

-- Recent refresh token activity
SELECT user_id, last_used_at, created_at, revoked_at
FROM core.refresh_tokens
ORDER BY last_used_at DESC
LIMIT 100;
```

## Testing

### Manual Testing dengan curl

#### 1. Login
```bash
curl -X POST http://localhost:8080/core/v1/auth/signin \
  -H "Content-Type: application/json" \
  -d '{
    "email": "user@example.com",
    "password": "password123"
  }'
```

#### 2. Refresh Token
```bash
curl -X POST http://localhost:8080/core/v1/auth/refresh \
  -H "Content-Type: application/json" \
  -d '{
    "refresh_token": "YOUR_REFRESH_TOKEN_HERE"
  }'
```

#### 3. Revoke Token
```bash
curl -X POST http://localhost:8080/core/v1/auth/revoke \
  -H "Content-Type: application/json" \
  -d '{
    "refresh_token": "YOUR_REFRESH_TOKEN_HERE"
  }'
```

## Future Enhancements

1. **Device Management**: Endpoint untuk list dan revoke tokens per device
2. **Refresh Token Families**: Detect token theft dengan token family tracking
3. **Sliding Sessions**: Extend refresh token expiration on use
4. **Multi-factor Authentication**: Require MFA untuk sensitive operations
5. **Geographic Restrictions**: Block refresh from unusual locations
6. **Automatic Cleanup**: Background job untuk hapus expired tokens

## References

- [OAuth 2.0 RFC 6749](https://datatracker.ietf.org/doc/html/rfc6749)
- [JWT Best Practices](https://datatracker.ietf.org/doc/html/rfc8725)
- [OWASP Authentication Cheat Sheet](https://cheatsheetseries.owasp.org/cheatsheets/Authentication_Cheat_Sheet.html)

## Support

Jika ada pertanyaan atau menemukan bug, silakan buat issue di repository atau hubungi team development.
