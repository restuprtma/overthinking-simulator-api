# Wizhub API Contract Documentation

**Last Updated:** 2026-05-05
**Total Modules:** 1 (Core)

---

## 📚 Documentation Structure

```
docs/api-contract/
├── README.md (this file)
└── core/
    ├── auth.md
    ├── user.md
    ├── role.md
    ├── permission.md (planned)
    ├── permission-template.md (planned)
    └── company.md (planned)
```

---

## 🎯 Core Module

Authentication, user management, and role-based access control.

**Base Path:** `/core/v1`

### ✅ Available Documentation

| File | Endpoints | Description | Status |
|------|-----------|-------------|--------|
| [auth.md](core/auth.md) | 14 | Authentication, email verification, password reset, company switching | ✅ Complete |
| [user.md](core/user.md) | 10 | User CRUD, role assignment, password management | ✅ Complete |
| [role.md](core/role.md) | 10 | Role CRUD, permission assignment, system vs company roles | ✅ Complete |

### 🚧 Planned Documentation

| File | Endpoints | Description | Status |
|------|-----------|-------------|--------|
| permission.md | 15 | Permission & Module management, CRUD templates | 🚧 Planned |
| permission-template.md | 6 | Permission template management | 🚧 Planned |
| company.md | 9 | Company CRUD, user management | 🚧 Planned |

---

## 📖 Documentation Standards

All API contract documentation follows a consistent format:

### Structure
- ✅ **Overview**: Module description and key features
- ✅ **Response Format**: Success and error response structures
- ✅ **Authentication & Authorization**: Required headers and permissions
- ✅ **Endpoints**: Detailed endpoint documentation
- ✅ **Best Practices**: Implementation guidelines
- ✅ **Changelog**: Version history

### Each Endpoint Includes
- ✅ HTTP Method and Path
- ✅ Authentication requirements
- ✅ Permission requirements
- ✅ Query/Path parameters
- ✅ Request body with validation rules
- ✅ Success response with examples
- ✅ Error responses with HTTP status codes
- ✅ Business rules and workflows

---

## 🔐 Authentication

All endpoints (except public auth endpoints) require JWT authentication:

```
Authorization: Bearer <access_token>
Content-Type: application/json
```

**Getting Access Token:**
1. Sign in: `POST /core/v1/auth/signin`
2. Include token in all subsequent requests
3. Refresh when expired: `POST /core/v1/auth/refresh`

---

## 🏢 Multi-Tenancy

All endpoints are company-aware:

- JWT token contains `company_id` claim
- All data is automatically filtered by company
- Users can switch companies via `POST /core/v1/auth/switch-company`

---

## 🎭 Permission System

Permissions follow the format: `{module}.{resource}:{action}`

**Examples:**
- `core.users:read` - View users
- `core.roles:create` - Create roles

---

## 📊 Response Format

### Success Response
```json
{
  "data": { /* response data */ },
  "message": "Success message",
  "meta": null,
  "errors": null
}
```

### Error Response
```json
{
  "data": null,
  "message": "Error message",
  "meta": null,
  "errors": {
    "field_name": ["Error detail message"]
  }
}
```

### Paginated Response
```json
{
  "data": [ /* array of items */ ],
  "message": "Success message",
  "meta": {
    "pagination": {
      "page": 1,
      "limit": 20,
      "total": 100,
      "total_pages": 5
    }
  }
}
```

**Note:** Pagination metadata is in `meta.pagination`, not inside `data`. Query parameter is `limit` (not `page_size`).

---

## 🔢 HTTP Status Codes

| Code | Description | When Used |
|------|-------------|-----------|
| 200 OK | Success | Successful GET, PUT, DELETE requests |
| 201 Created | Resource created | Successful POST requests |
| 400 Bad Request | Invalid request | Validation errors, invalid data |
| 401 Unauthorized | Not authenticated | Missing or invalid token |
| 403 Forbidden | No permission | User lacks required permission |
| 404 Not Found | Resource not found | Invalid ID or resource deleted |
| 409 Conflict | Resource conflict | Duplicate email, username, code |
| 429 Too Many Requests | Rate limited | Too many attempts |
| 500 Internal Server Error | Server error | Unexpected server errors |

---

## 🚀 Getting Started

### 1. Authentication
Start with the [auth.md](core/auth.md) documentation to:
- Register a new account
- Verify email
- Sign in and get access token
- Understand company switching

### 2. User & Role Management
Use [user.md](core/user.md) and [role.md](core/role.md) to:
- Create and manage users
- Set up roles and permissions
- Assign roles to users

---

## 🛠️ For Developers

### Frontend Integration
1. Use TypeScript/JavaScript fetch or axios
2. Include JWT token in Authorization header
3. Handle error responses appropriately
4. Implement token refresh logic

### Example Request (JavaScript)
```javascript
const response = await fetch('/core/v1/users', {
  method: 'GET',
  headers: {
    'Authorization': `Bearer ${accessToken}`,
    'Content-Type': 'application/json'
  }
});

const data = await response.json();

if (!response.ok) {
  console.error(data.message, data.errors);
} else {
  console.log(data.data);
}
```
