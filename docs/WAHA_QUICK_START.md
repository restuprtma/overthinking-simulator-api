# ⚡ WAHA Integration - Quick Start Guide

## 🚀 5-Minute Setup

### 1. Configure Environment (1 min)
```bash
# Add to .env file:
WAHA_BASE_URL=https://wapi.venturo.id
WAHA_API_KEY=venturojaya
WAHA_WEBHOOK_URL=https://your-backend.com/crm/v1/webhooks/waha
WAHA_WEBHOOK_SECRET=your-secret-key-here
```

### 2. Run Migration (1 min)
```bash
cd /Users/venturo/Venturo/lakukan/lakukan-be

# Run migration
migrate -path internal/database/migrations/crm \
        -database "postgresql://user:pass@localhost:5432/lakukan?sslmode=disable" \
        up
```

### 3. Start Server (1 min)
```bash
make dev
# Server starts on http://localhost:8080
```

### 4. Connect WhatsApp (2 min)
```bash
# Get pairing code
curl -X POST http://localhost:8080/crm/v1/sales-persons/{id}/whatsapp/connect \
  -H "Authorization: Bearer YOUR_TOKEN" \
  -H "X-Company-ID: YOUR_COMPANY_ID"

# Enter code in WhatsApp app
# Settings > Linked Devices > Link with Phone Number

# Check status
curl -X GET http://localhost:8080/crm/v1/sales-persons/{id}/whatsapp/status \
  -H "Authorization: Bearer YOUR_TOKEN" \
  -H "X-Company-ID: YOUR_COMPANY_ID"
```

## 📱 WhatsApp Connection Flow

```
1. POST /sales-persons/:id/whatsapp/connect
   └─> Returns: { "pairing_code": "ABCD-1234" }

2. Enter code in WhatsApp mobile app
   └─> WhatsApp → Settings → Linked Devices → Link with Phone Number

3. Poll: GET /sales-persons/:id/whatsapp/status (every 3s)
   └─> Until: { "is_connected": true, "status": "WORKING" }

4. Done! Messages will now be received automatically via webhook
```

## 🔄 Message Flow

```
Customer sends WhatsApp message
    ↓
WAHA receives message
    ↓
WAHA sends webhook → POST /crm/v1/webhooks/waha
    ↓
Backend processes webhook
    ↓
Creates/Updates chat in crm.chats
    ↓
Stores message in crm.chat_messages
```

## 🎯 Key Endpoints

| Endpoint | Method | Description |
|----------|--------|-------------|
| `/sales-persons/:id/whatsapp/connect` | POST | Get pairing code |
| `/sales-persons/:id/whatsapp/status` | GET | Check connection |
| `/sales-persons/:id/whatsapp/disconnect` | POST | Disconnect |
| `/sales-persons/:id/whatsapp/restart` | POST | Restart session |
| `/webhooks/waha` | POST | Receive webhooks |

## 🔍 Verification

### Check Database:
```sql
-- Check WAHA session
SELECT id, sales_name, whatsapp, is_whatsapp_connected,
       waha_session_name, waha_session_status
FROM crm.sales_persons;

-- Check chats created
SELECT id, customer_name, phone, platform, status, message_count
FROM crm.chats
WHERE platform = 'whatsapp';

-- Check messages
SELECT id, chat_id, sender_type, content, sent_at
FROM crm.chat_messages
ORDER BY sent_at DESC
LIMIT 10;
```

### Check Logs:
```bash
# Watch logs for webhook events
tail -f logs/app.log | grep -i waha
```

## ⚠️ Troubleshooting

| Issue | Solution |
|-------|----------|
| Migration fails | Check DB connection in .env |
| API key invalid | Verify WAHA_API_KEY=venturojaya |
| Webhook not working | Ensure webhook URL is publicly accessible |
| Messages not storing | Check logs for errors in webhook processing |
| Connection fails | Verify sales person has `whatsapp` field set |

## 📊 Success Criteria

- ✅ Migration runs without errors
- ✅ Server starts successfully
- ✅ Can create WhatsApp session (get pairing code)
- ✅ Can enter code and connect WhatsApp
- ✅ Status endpoint shows "WORKING"
- ✅ Sending message creates chat + message in database
- ✅ Webhook events appear in logs

## 🎉 Done!

Your WAHA integration is ready. Sales persons can now connect their WhatsApp and receive messages automatically!

---

**Need Help?** Check `WAHA_IMPLEMENTATION_COMPLETE.md` for full documentation.
