# Authentication & Company Integration Guide

## Overview

Dokumentasi ini menjelaskan bagaimana sistem autentikasi terintegrasi dengan multi-company management di Lakukan Backend.

## Konsep Penting

### 1. Company ID di JWT Token

**Company ID disimpan di JWT token claims**, bukan di response body atau database session.

```json
{
  "user_id": "850e8400-e29b-41d4-a716-446655440001",
  "company_id": "750e8400-e29b-41d4-a716-446655440001",  // ← Company context
  "email": "tantowi@gmail.com",
  "username": "tantowilathif",
  "roles": ["super_admin"],
  "permissions": ["users:read", "users:create", ...],
  "exp": 1734700000,
  "iat": 1734613600,
  "iss": "lakukan-api"
}
```

### 2. Keuntungan Pendekatan Ini

✅ **Stateless**: Tidak perlu query database untuk mendapatkan company context
✅ **Performance**: Company ID langsung tersedia di setiap request
✅ **Security**: Company context terikat dengan token, tidak bisa dimanipulasi
✅ **Scalability**: Mudah di-scale karena tidak ada session di server

---

## Flow Diagram

### Login Flow

```
┌─────────┐                    ┌─────────┐                    ┌──────────┐
│ Client  │                    │  API    │                    │ Database │
└────┬────┘                    └────┬────┘                    └────┬─────┘
     │                              │                              │
     │ POST /auth/signin            │                              │
     │ { email, password }          │                              │
     ├─────────────────────────────>│                              │
     │                              │                              │
     │                              │ Validate credentials         │
     │                              ├─────────────────────────────>│
     │                              │<─────────────────────────────┤
     │                              │                              │
     │                              │ Get primary company_id       │
     │                              ├─────────────────────────────>│
     │                              │<─────────────────────────────┤
     │                              │                              │
     │                              │ Generate JWT with company_id │
     │                              │                              │
     │<─────────────────────────────┤                              │
     │ { access_token, user }       │                              │
     │                              │                              │
```

### Switch Company Flow

```
┌─────────┐                    ┌─────────┐                    ┌──────────┐
│ Client  │                    │  API    │                    │ Database │
└────┬────┘                    └────┬────┘                    └────┬─────┘
     │                              │                              │
     │ GET /auth/companies          │                              │
     │ (with current token)         │                              │
     ├─────────────────────────────>│                              │
     │                              │ Extract user_id from token   │
     │                              │                              │
     │                              │ Get user's companies         │
     │                              ├─────────────────────────────>│
     │                              │<─────────────────────────────┤
     │<─────────────────────────────┤                              │
     │ { companies: [...] }         │                              │
     │                              │                              │
     │ User selects company         │                              │
     │                              │                              │
     │ POST /auth/switch-company    │                              │
     │ { company_id }               │                              │
     ├─────────────────────────────>│                              │
     │                              │ Extract user_id from token   │
     │                              │                              │
     │                              │ Validate user is member      │
     │                              ├─────────────────────────────>│
     │                              │<─────────────────────────────┤
     │                              │                              │
     │                              │ Set as primary company       │
     │                              ├─────────────────────────────>│
     │                              │<─────────────────────────────┤
     │                              │                              │
     │                              │ Generate NEW token with      │
     │                              │ new company_id               │
     │                              │                              │
     │<─────────────────────────────┤                              │
     │ { access_token, company }    │                              │
     │                              │                              │
     │ Replace old token with       │                              │
     │ new token                    │                              │
     │                              │                              │
```

### Request with Company Context

```
┌─────────┐                    ┌─────────┐                    ┌──────────┐
│ Client  │                    │  API    │                    │ Database │
└────┬────┘                    └────┬────┘                    └────┬─────┘
     │                              │                              │
     │ GET /users?page=1            │                              │
     │ Authorization: Bearer TOKEN  │                              │
     ├─────────────────────────────>│                              │
     │                              │ Decode JWT                   │
     │                              │ Extract company_id from      │
     │                              │ token claims                 │
     │                              │                              │
     │                              │ SELECT * FROM users          │
     │                              │ WHERE company_id = ?         │
     │                              │ AND deleted_at IS NULL       │
     │                              ├─────────────────────────────>│
     │                              │<─────────────────────────────┤
     │<─────────────────────────────┤                              │
     │ { users: [...] }             │                              │
     │ (filtered by company)        │                              │
     │                              │                              │
```

---

## API Endpoints

### 1. Login - POST `/core/v1/auth/signin`

**Request:**
```json
{
  "email": "tantowi@gmail.com",
  "password": "Bismillah1407*"
}
```

**Response:**
```json
{
  "data": {
    "access_token": "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9...",
    "refresh_token": "8f4e7c3b9a2d1e5f6a7b8c9d0e1f2a3b...",
    "token_type": "Bearer",
    "expires_in": 86400,
    "user": {
      "id": "850e8400-e29b-41d4-a716-446655440001",
      "email": "tantowi@gmail.com",
      "username": "tantowilathif",
      "full_name": "Super Administrator",
      "is_active": true,
      "is_email_verified": true
    }
  },
  "message": "Sign in successful"
}
```

**JWT Token Decoded:**
```json
{
  "user_id": "850e8400-e29b-41d4-a716-446655440001",
  "company_id": "750e8400-e29b-41d4-a716-446655440001",  // ← Primary company
  "email": "tantowi@gmail.com",
  "username": "tantowilathif",
  "roles": ["super_admin"],
  "permissions": ["users:read", "users:create", ...],
  "exp": 1734700000
}
```

---

### 2. Get User Companies - GET `/core/v1/auth/companies`

**Headers:**
```
Authorization: Bearer eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9...
```

**Response:**
```json
{
  "data": {
    "companies": [
      {
        "id": "750e8400-e29b-41d4-a716-446655440001",
        "name": "PT Venturo Pro",
        "code": "VENTURO-PRO",
        "logo_url": null
      },
      {
        "id": "750e8400-e29b-41d4-a716-446655440002",
        "name": "PT ABC Indonesia",
        "code": "ABC",
        "logo_url": "https://example.com/logo.png"
      }
    ]
  },
  "message": "User companies retrieved successfully"
}
```

**Use Case:**
- Menampilkan dropdown company di frontend
- User memilih company mana yang ingin digunakan

---

### 3. Switch Company - POST `/core/v1/auth/switch-company`

**Headers:**
```
Authorization: Bearer eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9...
```

**Request:**
```json
{
  "company_id": "750e8400-e29b-41d4-a716-446655440002"
}
```

**Response:**
```json
{
  "data": {
    "access_token": "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9...",  // ← NEW TOKEN
    "refresh_token": "9a5b8c7d2e3f4a6b7c8d9e0f1a2b3c4d...",
    "token_type": "Bearer",
    "expires_in": 86400,
    "company": {
      "id": "750e8400-e29b-41d4-a716-446655440002",
      "name": "PT ABC Indonesia",
      "code": "ABC",
      "logo_url": "https://example.com/logo.png"
    }
  },
  "message": "Company switched successfully"
}
```

**New JWT Token Decoded:**
```json
{
  "user_id": "850e8400-e29b-41d4-a716-446655440001",
  "company_id": "750e8400-e29b-41d4-a716-446655440002",  // ← UPDATED
  "email": "tantowi@gmail.com",
  "username": "tantowilathif",
  "roles": ["super_admin"],
  "permissions": ["users:read", "users:create", ...],
  "exp": 1734700000
}
```

**Important:**
- Client **HARUS** replace token lama dengan token baru
- Token lama menjadi **invalid** karena company_id berbeda
- Semua request berikutnya menggunakan token baru

---

## Implementation Guide

### Backend Implementation

#### 1. Middleware - Extract Company ID from Token

```go
// internal/middleware/auth.go
func JWTAuth() gin.HandlerFunc {
    return func(c *gin.Context) {
        // Extract token from header
        tokenString := extractToken(c)

        // Parse and validate token
        claims, err := jwtpkg.ParseToken(tokenString)
        if err != nil {
            c.JSON(401, gin.H{"message": "Invalid token"})
            c.Abort()
            return
        }

        // Set context untuk digunakan di handler
        c.Set("user_id", claims.UserID)
        c.Set("company_id", claims.CompanyID)  // ← Company context
        c.Set("email", claims.Email)
        c.Set("roles", claims.Roles)
        c.Set("permissions", claims.Permissions)

        c.Next()
    }
}
```

#### 2. Repository - Filter by Company ID

```go
// internal/modules/core/user/repository/user.repository.go
func (r *UserRepository) FindAll(limit, offset int, search string) ([]domain.User, error) {
    // Get company_id from context (set by middleware)
    companyID := getCompanyIDFromContext()

    query := `
        SELECT * FROM core.users
        WHERE company_id = $1          -- ← Filter by company
          AND deleted_at IS NULL
          AND (
            email ILIKE $2 OR
            username ILIKE $2 OR
            full_name ILIKE $2
          )
        ORDER BY created_at DESC
        LIMIT $3 OFFSET $4
    `

    rows, err := r.db.Query(ctx, query, companyID, search, limit, offset)
    // ... rest of implementation
}
```

#### 3. Service - Generate Token with Company ID

```go
// internal/modules/core/auth/service/auth.service.go
func (s *AuthService) SignIn(req *dto.SignInRequest) (*dto.SignInResponse, error) {
    // ... validate credentials

    // Get primary company ID for user
    companyID, err := s.companyUserRepo.GetPrimaryCompanyID(user.ID)
    if err != nil {
        companyID = "" // User might not have company yet
    }

    // Generate JWT with company_id in claims
    accessToken, err := jwtpkg.GenerateToken(
        user.ID,
        companyID,      // ← Include company_id
        user.Email,
        user.Username,
        roles,
        permissions,
    )

    return &dto.SignInResponse{
        AccessToken:  accessToken,
        RefreshToken: refreshToken,
        TokenType:    "Bearer",
        ExpiresIn:    expiresIn,
        User:         user.ToPublic(),
    }, nil
}
```

---

### Frontend Implementation

#### 1. Login & Store Token

```typescript
// Login
const response = await fetch('http://localhost:8080/core/v1/auth/signin', {
  method: 'POST',
  headers: { 'Content-Type': 'application/json' },
  body: JSON.stringify({
    email: 'tantowi@gmail.com',
    password: 'Bismillah1407*'
  })
});

const data = await response.json();

// Store token
localStorage.setItem('access_token', data.data.access_token);
localStorage.setItem('refresh_token', data.data.refresh_token);

// Optional: Decode token untuk mendapatkan company_id
const decodedToken = jwt_decode(data.data.access_token);
console.log('Current company:', decodedToken.company_id);
```

#### 2. Get User Companies

```typescript
const response = await fetch('http://localhost:8080/core/v1/auth/companies', {
  headers: {
    'Authorization': `Bearer ${localStorage.getItem('access_token')}`
  }
});

const data = await response.json();
const companies = data.data.companies;

// Render dropdown
companies.forEach(company => {
  console.log(`${company.name} (${company.code})`);
});
```

#### 3. Switch Company

```typescript
const switchCompany = async (companyId: string) => {
  const response = await fetch('http://localhost:8080/core/v1/auth/switch-company', {
    method: 'POST',
    headers: {
      'Content-Type': 'application/json',
      'Authorization': `Bearer ${localStorage.getItem('access_token')}`
    },
    body: JSON.stringify({ company_id: companyId })
  });

  const data = await response.json();

  // IMPORTANT: Replace old token with new token
  localStorage.setItem('access_token', data.data.access_token);
  localStorage.setItem('refresh_token', data.data.refresh_token);

  // Reload page or update state
  window.location.reload(); // Or update React/Vue state
};
```

#### 4. Make Authenticated Request

```typescript
const getUsers = async () => {
  const response = await fetch('http://localhost:8080/core/v1/users', {
    headers: {
      'Authorization': `Bearer ${localStorage.getItem('access_token')}`
    }
  });

  // Backend automatically filters by company_id from token
  const data = await response.json();
  return data.data;
};
```

---

## Database Schema

### 1. companies table

```sql
CREATE TABLE core.companies (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    owner_id UUID NOT NULL,
    name VARCHAR(255) NOT NULL,
    code VARCHAR(50) UNIQUE NOT NULL,
    -- ... other fields
    is_active BOOLEAN DEFAULT TRUE,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    deleted_at TIMESTAMP
);
```

### 2. company_users table (junction table)

```sql
CREATE TABLE core.company_users (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    company_id UUID NOT NULL REFERENCES core.companies(id),
    user_id UUID NOT NULL REFERENCES core.users(id),
    role VARCHAR(50) NOT NULL DEFAULT 'MEMBER',  -- OWNER, ADMIN, MEMBER
    is_primary BOOLEAN DEFAULT FALSE,            -- Primary company for user
    is_active BOOLEAN DEFAULT TRUE,
    joined_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    deleted_at TIMESTAMP,
    UNIQUE (company_id, user_id)
);
```

**Key Points:**
- `is_primary`: Menandakan company mana yang aktif untuk user
- User bisa jadi member di multiple companies
- Saat switch company, `is_primary` flag dipindahkan ke company baru

---

## Security Considerations

### 1. Token Validation

✅ **Always validate token signature**
```go
claims, err := jwtpkg.ParseToken(tokenString)
if err != nil {
    return ErrInvalidToken
}
```

✅ **Check token expiration**
```go
if time.Now().After(claims.ExpiresAt.Time) {
    return ErrExpiredToken
}
```

✅ **Validate company membership**
```go
// Saat switch company, pastikan user adalah member
exists := checkUserCompanyMembership(userID, companyID)
if !exists {
    return ErrNotCompanyMember
}
```

### 2. Company Data Isolation

✅ **Always filter by company_id**
```sql
SELECT * FROM users WHERE company_id = $1 AND deleted_at IS NULL
```

❌ **Never allow cross-company access**
```sql
-- BAD: No company filter
SELECT * FROM users WHERE id = $1

-- GOOD: Always include company filter
SELECT * FROM users WHERE id = $1 AND company_id = $2
```

### 3. Permission Checks

```go
// 1. JWT Auth (validate token)
// 2. Permission Check (validate permission)
// 3. Company Filter (filter data by company_id)

func GetUser(c *gin.Context) {
    // From JWT middleware
    companyID := c.GetString("company_id")
    permissions := c.GetStringSlice("permissions")

    // Check permission
    if !hasPermission(permissions, "users:read") {
        return c.JSON(403, "Forbidden")
    }

    // Get user with company filter
    user := userRepo.FindByID(userID, companyID)
}
```

---

## Troubleshooting

### Issue: Token tidak berisi company_id

**Cause:** User belum tergabung di company manapun

**Solution:**
```sql
-- Check user's companies
SELECT * FROM core.company_users WHERE user_id = '...';

-- Add user to company
INSERT INTO core.company_users (company_id, user_id, is_primary, role)
VALUES ('company-uuid', 'user-uuid', true, 'MEMBER');
```

### Issue: Switch company gagal dengan error "not a member"

**Cause:** User bukan member dari company yang dipilih

**Solution:**
```sql
-- Verify membership
SELECT * FROM core.company_users
WHERE user_id = '...' AND company_id = '...' AND deleted_at IS NULL;
```

### Issue: Data masih menampilkan company lain

**Cause:** Backend tidak filter by company_id

**Solution:**
```go
// Pastikan semua query include company filter
query := `
    SELECT * FROM table_name
    WHERE company_id = $1  -- ← Add this
      AND deleted_at IS NULL
`
```

---

## References

- [OpenAPI Auth Documentation](./openapi/auth-api.yaml)
- [OpenAPI Company Documentation](./openapi/company-management-api.yaml)
- [JWT Token Implementation](../pkg/jwt/)
- [Company Repository](../internal/modules/core/company/repository/)
