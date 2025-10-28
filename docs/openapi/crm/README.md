# CRM Module - OpenAPI Documentation

This directory contains OpenAPI 3.0 specifications for the CRM module endpoints.

## Available Specifications

### Sales Persons API
**File:** `sales_persons.yaml`

Complete API documentation for Sales Persons management including:
- List with pagination and filters
- Create, Read, Update, Delete operations
- Soft delete with restore capability
- WhatsApp integration tracking
- Audit trail information

## How to Use

### 1. View in Swagger UI

#### Online Viewers:
- **Swagger Editor**: https://editor.swagger.io/
  - Copy and paste the YAML content
  - Or use "File > Import File"

- **Redoc**: https://redocly.github.io/redoc/
  - Provides beautiful, responsive documentation

#### Local Swagger UI (if available in your app):
```bash
# Navigate to your app's swagger endpoint
http://localhost:8080/swagger
```

### 2. Generate Client Code

Using Swagger Codegen:
```bash
# Install swagger-codegen-cli
brew install swagger-codegen

# Generate TypeScript client
swagger-codegen generate \
  -i docs/openapi/crm/sales_persons.yaml \
  -l typescript-axios \
  -o ./generated/typescript-client

# Generate Go client
swagger-codegen generate \
  -i docs/openapi/crm/sales_persons.yaml \
  -l go \
  -o ./generated/go-client
```

Using OpenAPI Generator (recommended):
```bash
# Install openapi-generator-cli
npm install -g @openapitools/openapi-generator-cli

# Generate TypeScript Axios client
openapi-generator-cli generate \
  -i docs/openapi/crm/sales_persons.yaml \
  -g typescript-axios \
  -o ./generated/typescript-client

# Generate Python client
openapi-generator-cli generate \
  -i docs/openapi/crm/sales_persons.yaml \
  -g python \
  -o ./generated/python-client
```

### 3. Validate API Specification

```bash
# Using Swagger CLI
npm install -g @apidevtools/swagger-cli
swagger-cli validate docs/openapi/crm/sales_persons.yaml

# Using OpenAPI CLI
npm install -g @redocly/cli
redocly lint docs/openapi/crm/sales_persons.yaml
```

### 4. Test API Endpoints

Using the OpenAPI spec with tools like:

#### Postman:
1. Open Postman
2. Click "Import"
3. Select the YAML file
4. Postman will create a collection with all endpoints

#### Insomnia:
1. Open Insomnia
2. Create new Document
3. Import from File
4. Select the YAML file

#### HTTPie + OpenAPI plugin:
```bash
# Install httpie-openapi
pip install httpie-openapi

# Test endpoint
http --openapi docs/openapi/crm/sales_persons.yaml \
  GET /sales-persons \
  page==1 \
  page_size==10
```

## API Authentication

All endpoints require JWT Bearer token authentication:

```bash
# Get token from login endpoint first
curl -X POST http://localhost:8080/core/v1/auth/login \
  -H "Content-Type: application/json" \
  -d '{
    "email": "user@example.com",
    "password": "password123"
  }'

# Use token in subsequent requests
curl -X GET http://localhost:8080/crm/v1/sales-persons \
  -H "Authorization: Bearer YOUR_JWT_TOKEN"
```

## Example Requests

### List Sales Persons
```bash
curl -X GET "http://localhost:8080/crm/v1/sales-persons?page=1&page_size=10&is_active=true" \
  -H "Authorization: Bearer YOUR_JWT_TOKEN"
```

### Create Sales Person
```bash
curl -X POST http://localhost:8080/crm/v1/sales-persons \
  -H "Authorization: Bearer YOUR_JWT_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "company_user_id": "660e8400-e29b-41d4-a716-446655440001",
    "sales_code": "SP001",
    "sales_name": "John Sales",
    "sales_area": "Jakarta Selatan",
    "sales_target": 50000000.00,
    "commission_rate": 5.00,
    "whatsapp": "+628123456789",
    "is_active": true
  }'
```

### Update Sales Person
```bash
curl -X PUT http://localhost:8080/crm/v1/sales-persons/550e8400-e29b-41d4-a716-446655440000 \
  -H "Authorization: Bearer YOUR_JWT_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "sales_target": 75000000.00,
    "is_whatsapp_connected": true
  }'
```

### Delete Sales Person
```bash
curl -X DELETE http://localhost:8080/crm/v1/sales-persons/550e8400-e29b-41d4-a716-446655440000 \
  -H "Authorization: Bearer YOUR_JWT_TOKEN"
```

### Restore Deleted Sales Person
```bash
curl -X POST http://localhost:8080/crm/v1/sales-persons/550e8400-e29b-41d4-a716-446655440000/restore \
  -H "Authorization: Bearer YOUR_JWT_TOKEN"
```

## Integration Examples

### TypeScript/React Example
```typescript
import axios from 'axios';

// Configure axios instance
const api = axios.create({
  baseURL: 'http://localhost:8080/crm/v1',
  headers: {
    'Authorization': `Bearer ${token}`,
    'Content-Type': 'application/json'
  }
});

// List sales persons
const listSalesPersons = async (page = 1, pageSize = 10) => {
  const response = await api.get('/sales-persons', {
    params: { page, page_size: pageSize, is_active: true }
  });
  return response.data;
};

// Create sales person
const createSalesPerson = async (data) => {
  const response = await api.post('/sales-persons', data);
  return response.data;
};

// Update sales person
const updateSalesPerson = async (id, data) => {
  const response = await api.put(`/sales-persons/${id}`, data);
  return response.data;
};
```

### Python Example
```python
import requests

BASE_URL = "http://localhost:8080/crm/v1"
headers = {
    "Authorization": f"Bearer {token}",
    "Content-Type": "application/json"
}

# List sales persons
def list_sales_persons(page=1, page_size=10):
    response = requests.get(
        f"{BASE_URL}/sales-persons",
        headers=headers,
        params={"page": page, "page_size": page_size, "is_active": True}
    )
    return response.json()

# Create sales person
def create_sales_person(data):
    response = requests.post(
        f"{BASE_URL}/sales-persons",
        headers=headers,
        json=data
    )
    return response.json()
```

## Response Structure

All responses follow a consistent structure:

### Success Response (Single Item)
```json
{
  "data": {
    "id": "uuid",
    "sales_code": "SP001",
    "user": {
      "id": "uuid",
      "full_name": "John Doe",
      "email": "john@example.com"
    },
    "audit": {
      "created_at": "2024-01-15T10:30:00Z",
      "created_by": "uuid"
    }
  },
  "message": "Sales person retrieved successfully"
}
```

### Success Response (List with Pagination)
```json
{
  "data": [...],
  "message": "Sales persons retrieved successfully",
  "meta": {
    "page": 1,
    "page_size": 10,
    "total": 25,
    "total_pages": 3
  }
}
```

### Error Response
```json
{
  "message": "Error description",
  "errors": "Detailed error information"
}
```

## Performance Notes

### List Endpoint Optimization
The list endpoint (`GET /sales-persons`) is optimized for performance:
- Only essential user information is joined
- Audit trail names (created_by_name, updated_by_name) are **not included** in list view
- Use the detail endpoint (`GET /sales-persons/{id}`) to get complete audit information

### Recommended Practices
1. **Pagination**: Always use pagination for large datasets
2. **Filters**: Use filters to reduce result set size
3. **Caching**: Implement client-side caching for frequently accessed data
4. **Rate Limiting**: Respect rate limits if implemented

## Support

For issues or questions:
- **API Issues**: Create an issue in the project repository
- **Documentation**: Check the main API documentation
- **Email**: support@lakukan.id

## Changelog

### Version 1.0.0 (2024-01-25)
- Initial release
- Complete CRUD operations for Sales Persons
- Pagination and filtering support
- Soft delete with restore
- Audit trail tracking
- WhatsApp integration fields
