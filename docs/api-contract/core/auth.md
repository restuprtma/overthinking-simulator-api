# Authentication API Contract

**Version:** v1
**Base URL:** `/core/v1/auth`
**Module:** Core Authentication
**Last Updated:** 2026-02-11

---

## Table of Contents

- [Overview](#overview)
- [Response Format](#response-format)
- [Authentication Endpoints](#authentication-endpoints)
  - [Sign In](#1-sign-in)

---

## Overview

The Authentication API provides secure user login capabilities. All endpoints use JSON for request and response bodies.

**Key Features:**
- JWT-based authentication with refresh tokens
- Support login with email or username
- User active status validation

---

## Response Format

### Success Response Structure

All successful responses follow this format:

```json
{
  "data": {
    // Response data object
  },
  "message": "Success message",
  "meta": null,
  "errors": null
}
```

### Error Response Structure

All error responses follow this format:

```json
{
  "data": null,
  "message": "Error message",
  "meta": null,
  "errors": {
    "detail": ["Error detail message"]
  }
}
```

### HTTP Status Codes

| Status Code | Description |
|------------|-------------|
| 200 OK | Request successful |
| 400 Bad Request | Invalid request payload or validation error |
| 401 Unauthorized | Authentication failed or token invalid |
| 500 Internal Server Error | Server error |

---

## Authentication Endpoints

### 1. Sign In

Authenticate user with email/username and password.

**Endpoint:** `POST /core/v1/auth/signin`
**Authentication:** Not required
**Permission:** None

#### Request

**Headers:**
```
Content-Type: application/json
```

**Body:**
```json
{
  "email": "user@example.com",
  "password": "SecurePass123"
}
```

**Parameters:**

| Field | Type | Required | Validation | Description |
|-------|------|----------|------------|-------------|
| email | string | Yes | - | Email address or username |
| password | string | Yes | Min 8 characters | User password |

#### Response

**Success (200 OK):**
```json
{
  "data": {
    "access_token": "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9...",
    "refresh_token": "a1b2c3d4e5f6g7h8i9j0",
    "token_type": "Bearer",
    "expires_in": 86400,
    "user": {
      "id": "550e8400-e29b-41d4-a716-446655440000",
      "email": "user@example.com",
      "username": "johndoe",
      "full_name": "John Doe"
    }
  },
  "message": "Sign in successful",
  "meta": null,
  "errors": null
}
```

**Error Responses:**

**400 Bad Request** - Invalid request payload:
```json
{
  "data": null,
  "message": "Invalid request payload",
  "meta": null,
  "errors": {
    "detail": ["Key: 'SignInRequest.Email' Error:Field validation for 'Email' failed on the 'required' tag"]
  }
}
```

**401 Unauthorized** - Invalid credentials:
```json
{
  "data": null,
  "message": "Invalid credentials",
  "meta": null,
  "errors": null
}
```

**401 Unauthorized** - User not active:
```json
{
  "data": null,
  "message": "User account is not active",
  "meta": null,
  "errors": null
}
```

**500 Internal Server Error:**
```json
{
  "data": null,
  "message": "Failed to sign in",
  "meta": null,
  "errors": {
    "detail": ["Error description"]
  }
}
```

---

## Authentication Flow

### Standard Login Flow

```
1. User Login
   POST /core/v1/auth/signin
   Body: { "email": "user@example.com", "password": "password" }
   |
   v
2. Server validates credentials
   - Find user by email or username
   - Verify password hash
   - Check user is active
   |
   v
3. Generate tokens
   - Create JWT access token
   - Create refresh token
   |
   v
4. Return tokens and user info
   Response: { access_token, refresh_token, user }
   |
   v
5. Client stores tokens
   - Access token for API requests
   - Refresh token for token renewal
   |
   v
6. Access Protected Resources
   Include: Authorization: Bearer <access_token>
```

---

## Common Headers

### For Public Endpoints (Sign In)
```
Content-Type: application/json
```

### For Protected Endpoints
```
Content-Type: application/json
Authorization: Bearer <access_token>
```

---

## Testing API

**Health Check:**

```bash
curl http://localhost:8080/health
```

**Sign In:**

```bash
curl -X POST http://localhost:8080/core/v1/auth/signin \
  -H "Content-Type: application/json" \
  -d '{
    "email": "tantowi@gmail.com",
    "password": "Bismillah1407*"
  }'
```

---

## Changelog

### Version 1.0 (2026-02-11)
- Initial API contract documentation
- Sign In endpoint documented

---

**Last Updated:** 2026-02-11
**API Version:** v1
**Document Version:** 1.0
