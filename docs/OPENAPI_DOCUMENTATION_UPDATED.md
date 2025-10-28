# OpenAPI Documentation Update - WhatsApp Integration

**Date**: 2025-01-26
**Task**: Update OpenAPI documentation for WAHA (WhatsApp HTTP API) integration
**Files Modified**: 2

---

## Summary

Updated OpenAPI 3.0 documentation files to document the new WhatsApp integration endpoints for sales persons and WAHA webhook metadata structure for chat messages.

---

## Files Updated

### 1. `docs/openapi/crm/sales_persons.yaml`

**Previous Lines**: 792
**Updated Lines**: 1172
**Lines Added**: +380

#### Changes Made:

##### New Permission Added
- `sales_persons:manage` - Manage WhatsApp integration (connect, disconnect, restart)

##### New Tag Added
- **Sales Persons - WhatsApp** - WhatsApp integration for sales persons

##### New Endpoints (4 total)

1. **POST /sales-persons/{id}/whatsapp/connect**
   - **Summary**: Connect WhatsApp to sales person
   - **Description**: Initiate WhatsApp connection using pairing code authentication
   - **Flow Documentation**:
     - Creates WAHA session and requests pairing code
     - Returns 8-digit pairing code (90-second expiration)
     - User enters code in WhatsApp mobile app
     - Backend receives webhook when connected
     - Frontend polls status endpoint for updates
   - **Response**: Returns `ConnectWhatsAppResponse` with pairing code
   - **Permissions**: `sales_persons:manage`

2. **GET /sales-persons/{id}/whatsapp/status**
   - **Summary**: Get WhatsApp connection status
   - **Description**: Get current WhatsApp connection status for polling
   - **Status Values**: STARTING, SCAN_QR_CODE, WORKING, FAILED, STOPPED, null
   - **Use Case**: Poll every 3 seconds after connection initiation
   - **Response**: Returns `WhatsAppStatusResponse` with current status
   - **Multiple Examples**:
     - WhatsApp connected (WORKING)
     - Connection in progress (STARTING)
     - No session (null)
   - **Permissions**: `sales_persons:read`

3. **POST /sales-persons/{id}/whatsapp/disconnect**
   - **Summary**: Disconnect WhatsApp from sales person
   - **Description**: Stop session, logout from WhatsApp, clear metadata
   - **Effect**: Removes device from WhatsApp linked devices
   - **Response**: Success message
   - **Permissions**: `sales_persons:manage`

4. **POST /sales-persons/{id}/whatsapp/restart**
   - **Summary**: Restart WhatsApp session
   - **Description**: Restart session without logging out (keeps authentication)
   - **Use Case**: Fix unstable connections or FAILED state
   - **Response**: Success message
   - **Permissions**: `sales_persons:manage`

##### New Schemas (4 total)

1. **ConnectWhatsAppData**
   - Properties:
     - `sales_person_id` (uuid)
     - `session_name` (string) - WAHA session identifier
     - `whatsapp_number` (string)
     - `pairing_code` (string) - 8-digit code
     - `status` (enum) - Session status
     - `expires_at` (date-time) - Code expiration

2. **ConnectWhatsAppResponse**
   - Properties:
     - `data` (ConnectWhatsAppData)
     - `message` (string)

3. **WhatsAppStatusData**
   - Properties:
     - `sales_person_id` (uuid)
     - `session_name` (string, nullable)
     - `whatsapp_number` (string)
     - `status` (enum, nullable) - Current status
     - `is_connected` (boolean)
     - `pairing_code` (string, nullable) - During connection only
     - `pairing_code_expires_at` (date-time, nullable)
     - `connected_at` (date-time, nullable)
     - `disconnected_at` (date-time, nullable)
     - `last_seen_at` (date-time, nullable)

4. **WhatsAppStatusResponse**
   - Properties:
     - `data` (WhatsAppStatusData)
     - `message` (string)

---

### 2. `docs/openapi/crm/chats.yaml`

**Previous Lines**: 795
**Updated Lines**: 806
**Lines Added**: +11

#### Changes Made:

##### Updated Overview Section
- Added **WAHA Integration** subsection
- Documented automatic chat/message creation from WAHA webhooks
- Noted WAHA metadata stored in JSONB `metadata` field

##### Enhanced Schema Documentation

**Updated**: `CreateChatMessageRequest.metadata` property
- Added comprehensive documentation for WAHA metadata structure
- Documented metadata fields:
  - `waha_message_id` - WhatsApp message ID for tracking
  - `waha_timestamp` - Original message timestamp from WhatsApp
  - `waha_from` - WhatsApp sender ID (e.g., 628123456789@c.us)
  - `waha_to` - WhatsApp recipient ID
  - `waha_chat_id` - WhatsApp chat ID
  - `waha_ack` - Acknowledgment status (0-5 mapping documented)
- Added realistic example with WAHA metadata

---

## Technical Details

### Pairing Code Authentication Flow

```
1. User clicks "Connect WhatsApp" in frontend
   ↓
2. Frontend: POST /sales-persons/{id}/whatsapp/connect
   ↓
3. Backend: Creates WAHA session + requests pairing code
   ↓
4. Backend: Returns 8-digit code (expires in 90s)
   ↓
5. Frontend: Displays code + starts polling status endpoint
   ↓
6. User: Opens WhatsApp → Settings → Linked Devices → Link with phone number
   ↓
7. User: Enters pairing code
   ↓
8. WhatsApp: Validates code → connects session
   ↓
9. WAHA: Sends webhook to backend (session.status = WORKING)
   ↓
10. Backend: Updates is_whatsapp_connected = true
   ↓
11. Frontend: Polling detects WORKING status → stops polling → shows success
```

### Webhook Integration (Automatic)

When messages are received from WhatsApp:

```
1. WAHA sends webhook: message.any event
   ↓
2. Backend receives at: POST /crm/v1/webhooks/waha
   ↓
3. Webhook service extracts phone number and company_id
   ↓
4. Chat service finds or creates chat session
   ↓
5. Chat message created with WAHA metadata (JSONB):
   {
     "waha_message_id": "true_628xxx@c.us_ABC123",
     "waha_timestamp": 1706257800000,
     "waha_from": "628123456789@c.us",
     "waha_to": "628999999999@c.us",
     "waha_chat_id": "628123456789@c.us",
     "waha_ack": 3
   }
   ↓
6. Message appears in chat inbox (GET /chats)
   ↓
7. Message appears in chat detail (GET /chats/{id})
```

### ACK Status Values

WAHA acknowledgment statuses stored in metadata:
- `0` - ERROR
- `1` - PENDING
- `2` - SERVER (sent to WhatsApp server)
- `3` - DELIVERY (delivered to recipient)
- `4` - READ (read by recipient)
- `5` - PLAYED (audio/video played by recipient)

---

## OpenAPI Specification Compliance

Both files follow OpenAPI 3.0.3 specification:
- ✅ Valid YAML syntax
- ✅ Proper schema references ($ref)
- ✅ Complete request/response examples
- ✅ Comprehensive descriptions
- ✅ Multiple response examples where applicable
- ✅ Proper HTTP status codes
- ✅ Permission requirements documented
- ✅ Business rules documented
- ✅ Use cases documented

---

## File Locations

```
docs/openapi/crm/
├── sales_persons.yaml    (1172 lines)  ← Updated with WhatsApp endpoints
└── chats.yaml            (806 lines)   ← Updated with WAHA metadata docs
```

---

## Integration with Existing Code

The documentation matches the implementation in:

- **Handlers**: `internal/modules/crm/sales_persons/handler/sales_person.handler.go`
- **Service**: `internal/modules/crm/sales_persons/service/whatsapp_session.service.go`
- **DTOs**: `internal/modules/crm/sales_persons/dto/whatsapp.dto.go`
- **Routes**: `internal/modules/crm/sales_persons/main.sales_persons.go`
- **Webhooks**: `internal/modules/crm/webhooks/handler/waha_webhook.handler.go`
- **Chat Integration**: `internal/modules/crm/chats/service/chat.service.go`

---

## Usage Examples

### 1. Connect WhatsApp

**Request**:
```bash
curl -X POST http://localhost:8080/crm/v1/sales-persons/{id}/whatsapp/connect \
  -H "Authorization: Bearer {token}" \
  -H "X-Company-ID: {company_id}"
```

**Response**:
```json
{
  "data": {
    "sales_person_id": "550e8400-e29b-41d4-a716-446655440000",
    "session_name": "sp_550e8400_e29b_41d4_a716_446655440000",
    "whatsapp_number": "+628123456789",
    "pairing_code": "12345678",
    "status": "STARTING",
    "expires_at": "2025-01-26T10:31:30Z"
  },
  "message": "WhatsApp connection initiated. Please enter the pairing code in WhatsApp mobile app within 90 seconds."
}
```

### 2. Poll Status

**Request**:
```bash
curl -X GET http://localhost:8080/crm/v1/sales-persons/{id}/whatsapp/status \
  -H "Authorization: Bearer {token}" \
  -H "X-Company-ID: {company_id}"
```

**Response (Connected)**:
```json
{
  "data": {
    "sales_person_id": "550e8400-e29b-41d4-a716-446655440000",
    "session_name": "sp_550e8400_e29b_41d4_a716_446655440000",
    "whatsapp_number": "+628123456789",
    "status": "WORKING",
    "is_connected": true,
    "connected_at": "2025-01-26T10:30:45Z",
    "last_seen_at": "2025-01-26T10:35:00Z"
  },
  "message": "WhatsApp status retrieved successfully"
}
```

---

## Next Steps

1. ✅ OpenAPI documentation updated
2. ⏳ **TODO**: Generate Swagger UI from OpenAPI files
3. ⏳ **TODO**: Update frontend to use new endpoints
4. ⏳ **TODO**: Run database migration (if not done)
5. ⏳ **TODO**: Test end-to-end flow

---

## Related Documentation

- [WAHA_IMPLEMENTATION_COMPLETE.md](WAHA_IMPLEMENTATION_COMPLETE.md) - Full implementation details
- [WAHA_QUICK_START.md](WAHA_QUICK_START.md) - 5-minute setup guide
- [WAHA_IMPLEMENTATION_STATUS.md](WAHA_IMPLEMENTATION_STATUS.md) - Current progress tracker
- [BUILD_SUCCESS.md](BUILD_SUCCESS.md) - Build verification

---

**Status**: ✅ Documentation Complete
**Build Status**: ✅ Go build successful
**OpenAPI Status**: ✅ Valid YAML syntax
**Total Endpoints Documented**: 4 new WhatsApp endpoints
**Total Schemas Added**: 4 new schemas
