# Lakukan Backend API Documentation

Dokumentasi OpenAPI untuk semua endpoint Lakukan Backend API.

## 📚 Available API Documentation

### 🔐 Authentication & Authorization
- **[Authentication API](./auth-api.yaml)**
  - Sign In/Sign Up
  - Email Verification
  - Token Management (Refresh, Revoke)
  - Company Switching
  - Get User Companies

### 🏢 Company Management
- **[Company Management API](./company-management-api.yaml)**
  - CRUD Companies
  - Company Users Management
  - Multi-tenant Support

### 👥 User & Access Control
- **[Role Management API](./role-api.yaml)**
  - CRUD Roles
  - Role-Permission Assignment
  - RBAC Implementation

- **[Permission Management API](./permission-api.yaml)**
  - CRUD Permissions
  - Resource-Action Based Permissions
  - Fine-grained Access Control

- **[Permission Templates API](./permission-templates-and-actions.yaml)**
  - Permission Templates
  - Permission Actions
  - Template Management

---

## 🚀 Quick Start

### 1. View Documentation

#### Option A: Swagger UI (Recommended)

```bash
# Run the application
make run

# Open browser
http://localhost:8080/swagger
```

#### Option B: Swagger Editor

1. Go to [https://editor.swagger.io/](https://editor.swagger.io/)
2. File > Import File
3. Select salah satu YAML file dari folder ini

#### Option C: VS Code

Install extension:
- **OpenAPI (Swagger) Editor** by 42Crunch
- **Swagger Viewer** by Arjun G

Kemudian buka file `.yaml` dan klik "Preview Swagger"

---

### 2. Test API

#### Using Swagger UI

1. Buka http://localhost:8080/swagger
2. Pilih endpoint yang ingin ditest
3. Klik "Try it out"
4. Isi parameter/body
5. Klik "Execute"

#### Using curl

```bash
# Login
curl -X POST http://localhost:8080/core/v1/auth/signin \
  -H "Content-Type: application/json" \
  -d '{
    "email": "tantowi@gmail.com",
    "password": "Bismillah1407*"
  }'

# Get companies (with token)
curl -X GET http://localhost:8080/core/v1/auth/companies \
  -H "Authorization: Bearer YOUR_TOKEN_HERE"
```

#### Using Postman

1. Import file `.yaml` ke Postman
2. Postman akan auto-generate collection
3. Set environment variable untuk token

---

## 🔑 Authentication Flow

### Standard Flow

```
1. POST /auth/signin
   → Get access_token & refresh_token

2. Use access_token for all requests
   Header: Authorization: Bearer <access_token>

3. When token expired:
   POST /auth/refresh
   → Get new access_token & refresh_token
```

### Multi-Company Flow

```
1. POST /auth/signin
   → Get token (includes company_id in JWT claims)

2. GET /auth/companies
   → Get list of companies user belongs to

3. POST /auth/switch-company
   → Get NEW token with different company_id

4. Use new token for company-specific data
```

**Important:**
- Company ID ada di JWT token claims, bukan di response body
- Setiap switch company akan generate token baru
- Token lama menjadi invalid setelah switch

---

## 📖 Detailed Documentation

### Integration Guides

- **[Auth & Company Integration Guide](../AUTH_COMPANY_INTEGRATION.md)**
  - JWT Token Structure
  - Multi-Company Architecture
  - Switch Company Flow
  - Frontend Implementation
  - Backend Implementation
  - Security Best Practices

### Development Guides

- **[Development Guide](../DEVELOPMENT.md)**
  - Setup Development Environment
  - Running the Application
  - Database Migrations
  - Testing

- **[Architecture Guide](../ARCHITECTURE.md)**
  - Project Structure
  - Design Patterns
  - Module Organization

---

## 🌐 Base URLs

| Environment | Base URL |
|------------|----------|
| Development | `http://localhost:8080/core/v1` |
| Production | `https://api.lakukan.id/core/v1` |

---

## 🔒 Authentication

Semua endpoint (kecuali public endpoints) memerlukan Bearer token di header:

```
Authorization: Bearer <your-jwt-token>
```

### Public Endpoints (No Auth Required)

- `POST /auth/signin`
- `POST /auth/signup`
- `POST /auth/verify-email`
- `POST /auth/resend-verification`
- `POST /auth/refresh`
- `POST /auth/revoke`

### Protected Endpoints (Auth Required)

- `POST /auth/switch-company` - Switch company context
- `GET /auth/companies` - Get user companies
- All other endpoints (users, roles, permissions, companies, etc.)

---

## 📝 Response Format

Semua response mengikuti format standard:

### Success Response

```json
{
  "data": {
    // Response data here
  },
  "message": "Success message",
  "meta": {
    "pagination": {
      "page": 1,
      "page_size": 10,
      "total": 100,
      "total_pages": 10
    }
  }
}
```

### Error Response

```json
{
  "message": "Error message",
  "errors": "Error details (optional)"
}
```

---

## 🔍 Common Query Parameters

### Pagination

- `page` - Page number (default: 1)
- `page_size` - Items per page (default: 10, max: 100)

### Filtering

- `search` - Search query (varies by endpoint)
- `is_active` - Filter by active status (true/false)

### Sorting

- `sort_by` - Field to sort by
- `sort_order` - Sort direction (asc/desc)

---

## 📊 Status Codes

| Code | Description |
|------|-------------|
| 200 | OK - Request successful |
| 201 | Created - Resource created |
| 400 | Bad Request - Invalid input |
| 401 | Unauthorized - Missing or invalid token |
| 403 | Forbidden - Insufficient permissions |
| 404 | Not Found - Resource not found |
| 409 | Conflict - Resource already exists |
| 500 | Internal Server Error |

---

## 🧪 Testing Examples

### 1. Complete Authentication Flow

```bash
# 1. Sign up
curl -X POST http://localhost:8080/core/v1/auth/signup \
  -H "Content-Type: application/json" \
  -d '{
    "email": "test@example.com",
    "username": "testuser",
    "password": "TestPass123!",
    "full_name": "Test User"
  }'

# 2. Sign in
TOKEN=$(curl -X POST http://localhost:8080/core/v1/auth/signin \
  -H "Content-Type: application/json" \
  -d '{
    "email": "test@example.com",
    "password": "TestPass123!"
  }' | jq -r '.data.access_token')

# 3. Get companies
curl -X GET http://localhost:8080/core/v1/auth/companies \
  -H "Authorization: Bearer $TOKEN"

# 4. Switch company
curl -X POST http://localhost:8080/core/v1/auth/switch-company \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "company_id": "750e8400-e29b-41d4-a716-446655440001"
  }'
```

### 2. Company Management

```bash
# Get all companies
curl -X GET http://localhost:8080/core/v1/companies \
  -H "Authorization: Bearer $TOKEN"

# Create company
curl -X POST http://localhost:8080/core/v1/companies \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "PT Example",
    "code": "EXAMPLE"
  }'
```

---

## 🛠️ Tools & Resources

### API Testing Tools

- [Postman](https://www.postman.com/) - API testing & documentation
- [Insomnia](https://insomnia.rest/) - REST client
- [Thunder Client](https://www.thunderclient.com/) - VS Code extension
- [Swagger UI](http://localhost:8080/swagger) - Built-in API tester

### Documentation Tools

- [Swagger Editor](https://editor.swagger.io/) - Online OpenAPI editor
- [Redoc](https://redocly.com/) - Beautiful API documentation
- [Stoplight](https://stoplight.io/) - API design platform

### Development Tools

- [jwt.io](https://jwt.io/) - Decode JWT tokens
- [JSON Formatter](https://jsonformatter.org/) - Format JSON
- [cURL Converter](https://curlconverter.com/) - Convert cURL to code

---

## 📞 Support

Jika ada pertanyaan atau issue:

1. Check dokumentasi di folder `docs/`
2. Check issue di repository
3. Contact: support@lakukan.id

---

## 📄 License

Proprietary - © 2024 Lakukan
