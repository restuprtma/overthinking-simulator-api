# Signin API Documentation

## POST /api/v1/auth/signin

Endpoint untuk melakukan autentikasi user dengan email/username dan password.

### Request

**Headers:**
```
Content-Type: application/json
```

**Body:**
```json
{
  "login": "user@example.com",  // atau bisa menggunakan username
  "password": "yourpassword123"
}
```

### Response

**Success (200 OK):**
```json
{
  "success": true,
  "message": "Sign in successful",
  "data": {
    "access_token": "temp_access_token_<user_id>",
    "refresh_token": "",
    "token_type": "Bearer",
    "expires_in": 3600,
    "user": {
      "id": "550e8400-e29b-41d4-a716-446655440000",
      "email": "user@example.com",
      "username": "username",
      "full_name": "John Doe",
      "phone": "+6281234567890",
      "is_active": true,
      "is_email_verified": true,
      "last_login_at": "2025-01-15T10:30:00Z"
    }
  }
}
```

**Error Responses:**

**400 Bad Request** - Invalid request payload:
```json
{
  "success": false,
  "message": "Invalid request payload",
  "error": "validation error details"
}
```

**401 Unauthorized** - Invalid credentials:
```json
{
  "success": false,
  "message": "Invalid credentials",
  "error": ""
}
```

**401 Unauthorized** - User not active:
```json
{
  "success": false,
  "message": "User account is not active",
  "error": ""
}
```

**500 Internal Server Error** - Server error:
```json
{
  "success": false,
  "message": "Failed to sign in",
  "error": "error details"
}
```

### Example cURL

```bash
curl -X POST http://localhost:8080/api/v1/auth/signin \
  -H "Content-Type: application/json" \
  -d '{
    "login": "admin@lakukan.com",
    "password": "admin123"
  }'
```

### Notes

- Field `login` dapat berupa email atau username
- Token JWT saat ini masih menggunakan implementasi sementara (TODO: implement proper JWT)
- Password di-hash menggunakan bcrypt
- Last login timestamp akan di-update setelah signin berhasil