# Migration Consolidation - Auth Tables

## 📋 Summary

Semua auth-related database changes telah digabungkan ke dalam satu migration file untuk kemudahan maintenance.

---

## ✅ Changes Made

### **Before (3 separate migrations):**
- `000002_auth_tables.up.sql` - Users, roles, permissions (original)
- `000006_password_reset_tokens.up.sql` - Password reset & email verification enhancements
- `000007_security_tables.up.sql` - Refresh tokens, login attempts, audit logs

### **After (1 consolidated migration):**
- `000002_auth_tables.up.sql` - ALL auth tables in one file

---

## 📊 Tables in Consolidated Migration

### **000002_auth_tables.up.sql** now includes:

#### Core Authentication Tables:
1. **users** - User accounts
   - ✅ Original fields (id, email, username, password_hash, etc.)
   - ✅ **New:** `failed_login_count`, `locked_until`, `locked_at`

2. **email_verification_tokens** - Email verification
   - ✅ Original fields (id, user_id, token, expires_at, verified_at)
   - ✅ **New:** `resent_count`, `last_resent_at`

3. **password_reset_tokens** - Password reset (NEW)
   - id, user_id, token, expires_at, used_at
   - ip_address, user_agent, created_at

4. **refresh_tokens** - Session management (NEW)
   - id, user_id, token_hash, device_name, device_id
   - ip_address, user_agent, expires_at, last_used_at, revoked_at

5. **login_attempts** - Security tracking (NEW)
   - id, user_id, email, username
   - ip_address, user_agent, status, failure_reason, created_at

6. **audit_logs** - Audit trail (NEW)
   - id, user_id, action, resource, resource_id
   - ip_address, user_agent, metadata (JSONB), created_at

#### Authorization Tables (unchanged):
7. **roles** - User roles
8. **permissions** - System permissions
9. **user_roles** - User-role mapping
10. **role_permissions** - Role-permission mapping

---

## 🔧 What This Means For You

### **If you're starting fresh:**
```bash
# Just run migrations normally
make migrate-up
```
Everything will be created in one go ✅

### **If you already ran migrations before:**

**Option 1: Fresh Start (Development Only - Will Delete Data!)**
```bash
# Drop everything and start fresh
make migrate-down  # or migrate down to version 1
make migrate-up
```

**Option 2: Manual ALTER (Production Safe)**

If your database already has users table from old 000002, you need to add new columns:

```sql
-- Add to users table
ALTER TABLE core.users
ADD COLUMN IF NOT EXISTS failed_login_count INT DEFAULT 0,
ADD COLUMN IF NOT EXISTS locked_until TIMESTAMP,
ADD COLUMN IF NOT EXISTS locked_at TIMESTAMP;

-- Add to email_verification_tokens table
ALTER TABLE core.email_verification_tokens
ADD COLUMN IF NOT EXISTS resent_count INT DEFAULT 0,
ADD COLUMN IF NOT EXISTS last_resent_at TIMESTAMP;

-- Then run migrate up to create new tables
```

**Option 3: Force Migration Version (Advanced)**
```bash
# Mark as version 1, then migrate up
make migrate-force V=1 MODULE=core
make migrate-up
```

---

## 🗑️ Deleted Files

The following migration files have been removed (consolidated into 000002):
- ❌ `000006_password_reset_tokens.up.sql`
- ❌ `000006_password_reset_tokens.down.sql`
- ❌ `000007_security_tables.up.sql`
- ❌ `000007_security_tables.down.sql`

---

## 📦 Current Migration Files

```
internal/database/migrations/core/
├── 000001_init_schema.up.sql           # Schema creation
├── 000002_auth_tables.up.sql           # ALL auth tables (consolidated)
├── 000003_geographic_tables.up.sql     # Geographic data
├── 000004_company_tables.up.sql        # Company management
└── 000005_subscription_tables.up.sql   # Subscriptions
```

---

## ✨ Benefits

1. **Simpler** - One file for all auth tables
2. **Cleaner** - No scattered auth migrations
3. **Easier rollback** - Down migration handles all auth tables
4. **Better organization** - Related tables together
5. **Atomic** - All auth tables created in one transaction

---

## ⚠️ Important Notes

1. **Indexes:** All indexes consolidated and organized by table
2. **Foreign Keys:** Proper CASCADE deletes configured
3. **IF NOT EXISTS:** Safe to re-run if needed
4. **Backward Compatible:** Uses same structure, just consolidated

---

## 🧪 Testing Migration

```bash
# Test migration up
make migrate-up

# Verify tables
psql -U postgres -d lakukan -c "
SELECT table_name
FROM information_schema.tables
WHERE table_schema = 'core'
ORDER BY table_name;
"

# Should show all 10 tables

# Test migration down
make migrate-down

# Verify tables dropped
psql -U postgres -d lakukan -c "
SELECT table_name
FROM information_schema.tables
WHERE table_schema = 'core'
ORDER BY table_name;
"

# Should show empty or minimal tables

# Migrate up again
make migrate-up
```

---

## 📞 Questions?

**Q: Will this break my existing database?**
A: If you haven't run migrations 000006 or 000007, you're fine. Just run `make migrate-up`.

**Q: I already have some users in my database. What should I do?**
A: Use Option 2 (Manual ALTER) above to add new columns without dropping data.

**Q: Can I still rollback?**
A: Yes! Migration down file handles all tables properly.

**Q: What if I get "relation already exists" error?**
A: The migration uses `IF NOT EXISTS`, so it should be safe. If you get errors, check migration version with `make migrate-version`.

---

## ✅ Checklist

After consolidation, verify:
- [ ] Migration 000002 contains all auth tables
- [ ] Files 000006 and 000007 deleted
- [ ] Migration up works without errors
- [ ] All 10 tables created (6 auth + 4 authorization)
- [ ] Indexes created properly
- [ ] Foreign keys working
- [ ] Migration down works (test environment only!)

---

**Date:** October 10, 2025
**Reason:** Code consolidation and better organization
**Impact:** None if fresh install, minimal if existing database
