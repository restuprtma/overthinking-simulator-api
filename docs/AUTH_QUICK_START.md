# 🚀 Auth System - Quick Start Guide

## Setup & Testing dalam 5 Menit

### 1. Update Environment Variables

```bash
# Copy .env.example jika belum
cp .env.example .env

# Edit .env dan tambahkan (atau sudah otomatis ada):
```

Minimal config untuk development:
```env
# JWT Configuration
JWT_SECRET=your-super-secret-key-change-in-production-min-32-characters
JWT_EXPIRATION=24h
JWT_REFRESH_EXPIRATION=168h

# Email - Development Mode (NoOp - prints to console)
# Biarkan kosong atau gunakan Mailtrap untuk testing
SMTP_HOST=smtp.mailtrap.io
SMTP_PORT=2525
SMTP_USER=
SMTP_PASSWORD=
```

### 2. Run Database Migrations

**IMPORTANT:** Semua auth tables sudah digabung dalam migration 000002_auth_tables.up.sql

```bash
# Jika fresh install (belum pernah migrate):
make migrate-up

# Jika sudah pernah run migration 000002 sebelumnya:
# Option 1: Fresh start (HATI-HATI: akan drop data!)
make migrate-down
make migrate-up

# Option 2: Force version (lebih aman)
make migrate-force V=2 MODULE=core
make migrate-up

# Atau manual:
migrate -path internal/database/migrations/core \
        -database "postgresql://postgres:postgres@localhost:5432/lakukan?sslmode=disable" \
        up
```

**Check if migration successful:**
```bash
# Verify all tables created
psql -U postgres -d lakukan -c "\dt core.*"

# Should see:
# - users (with lockout fields)
# - email_verification_tokens (with resend tracking)
# - password_reset_tokens (new)
# - refresh_tokens (new)
# - login_attempts (new)
# - audit_logs (new)
# - roles, permissions, user_roles, role_permissions
```

### 3. Start Server

```bash
# Development mode dengan hot reload
make dev

# Atau run biasa
make run
```

### 4. Test API Endpoints

#### ✅ Test 1: Register New User

```bash
curl -X POST http://localhost:8080/api/v1/auth/signup \
  -H "Content-Type: application/json" \
  -d '{
    "email": "john@example.com",
    "username": "johndoe",
    "password": "SecurePass123",
    "full_name": "John Doe"
  }'
```

**Expected Response:**
```json
{
  "success": true,
  "message": "Registration successful",
  "data": {
    "message": "Registration successful. Please check your email to verify your account.",
    "user": {
      "id": "...",
      "email": "john@example.com",
      "username": "johndoe",
      "full_name": "John Doe",
      "is_active": true,
      "is_email_verified": false
    }
  }
}
```

**⚠️ IMPORTANT:** Check console output untuk verification token!

Output akan seperti:
```
[NoOpEmail] Verification email to john@example.com (token: abc123def456...)
```

Copy token tersebut!

---

#### ✅ Test 2: Verify Email

```bash
curl -X POST http://localhost:8080/api/v1/auth/verify-email \
  -H "Content-Type: application/json" \
  -d '{
    "token": "PASTE_TOKEN_FROM_CONSOLE_HERE"
  }'
```

**Expected Response:**
```json
{
  "success": true,
  "message": "Email verified successfully",
  "data": {
    "message": "Email verified successfully. You can now sign in."
  }
}
```

---

#### ✅ Test 3: Sign In

```bash
curl -X POST http://localhost:8080/api/v1/auth/signin \
  -H "Content-Type: application/json" \
  -d '{
    "login": "john@example.com",
    "password": "SecurePass123"
  }'
```

**Expected Response:**
```json
{
  "success": true,
  "message": "Sign in successful",
  "data": {
    "access_token": "eyJhbGciOiJIUzI1NiIs...",
    "token_type": "Bearer",
    "expires_in": 86400,
    "user": {
      "id": "...",
      "email": "john@example.com",
      "username": "johndoe",
      "is_email_verified": true
    }
  }
}
```

---

#### ✅ Test 4: Resend Verification (Optional)

Before verifying, test resend:

```bash
curl -X POST http://localhost:8080/api/v1/auth/resend-verification \
  -H "Content-Type: application/json" \
  -d '{
    "email": "john@example.com"
  }'
```

---

## 🧪 Test Cases

### Test Case 1: Duplicate Email
```bash
# Try to register with same email
curl -X POST http://localhost:8080/api/v1/auth/signup \
  -H "Content-Type: application/json" \
  -d '{
    "email": "john@example.com",
    "username": "different_username",
    "password": "SecurePass123",
    "full_name": "Different User"
  }'

# Expected: 409 Conflict - Email already exists
```

### Test Case 2: Weak Password
```bash
curl -X POST http://localhost:8080/api/v1/auth/signup \
  -H "Content-Type: application/json" \
  -d '{
    "email": "test@example.com",
    "username": "testuser",
    "password": "weak",
    "full_name": "Test User"
  }'

# Expected: 400 Bad Request - Password validation error
```

### Test Case 3: Invalid Email Format
```bash
curl -X POST http://localhost:8080/api/v1/auth/signup \
  -H "Content-Type: application/json" \
  -d '{
    "email": "not-an-email",
    "username": "testuser",
    "password": "SecurePass123",
    "full_name": "Test User"
  }'

# Expected: 400 Bad Request - Invalid email format
```

### Test Case 4: Invalid Username
```bash
curl -X POST http://localhost:8080/api/v1/auth/signup \
  -H "Content-Type: application/json" \
  -d '{
    "email": "test@example.com",
    "username": "ab",
    "password": "SecurePass123",
    "full_name": "Test User"
  }'

# Expected: 400 Bad Request - Username too short
```

### Test Case 5: Expired/Invalid Token
```bash
curl -X POST http://localhost:8080/api/v1/auth/verify-email \
  -H "Content-Type: application/json" \
  -d '{
    "token": "invalid-token-12345"
  }'

# Expected: 400 Bad Request - Invalid or expired token
```

---

## 📱 Swagger UI

Buka browser dan akses:
```
http://localhost:8080/swagger/index.html
```

Cari section **Authentication** dan test semua endpoints via UI.

---

## 🔍 Troubleshooting

### Issue: Migration Failed

```bash
# Check current migration version
make migrate-version

# Force to specific version if needed
make migrate-force V=7 MODULE=core
```

### Issue: SMTP Error (Email Service)

Jika ada error terkait email service, aplikasi akan fallback ke NoOpEmailService yang print ke console. Ini normal untuk development.

### Issue: Database Connection

```bash
# Test database connection
psql -U postgres -d lakukan -c "SELECT 1;"

# Check if tables exist
psql -U postgres -d lakukan -c "\dt core.*"
```

### Issue: Token Not Found in Console

Search log output untuk `[NoOpEmail]`:
```bash
# Jika running dengan make dev
tail -f tmp/main.log | grep NoOpEmail

# Or check console directly
```

---

## 📊 Check Database

```bash
# Connect to database
psql -U postgres -d lakukan

# Check users
SELECT id, email, username, is_email_verified FROM core.users;

# Check verification tokens
SELECT user_id, token, expires_at, verified_at FROM core.email_verification_tokens;

# Check recent login attempts (will be populated later)
SELECT * FROM core.login_attempts ORDER BY created_at DESC LIMIT 10;
```

---

## 🎯 Next Steps

Setelah semua test berhasil:

1. **Configure Real Email Service (Production):**
   - Setup Mailtrap untuk development testing
   - Setup SendGrid/Mailgun untuk production

2. **Test Email Templates:**
   - Configure SMTP credentials
   - Test actual email delivery

3. **Implement Remaining Features:**
   - Forgot Password
   - Reset Password
   - Refresh Token
   - Logout

4. **Add Frontend Integration:**
   - Update `FRONTEND_URL` in .env
   - Create frontend pages untuk verification

---

## ✅ Checklist

- [ ] .env configured
- [ ] Database migrations run successfully
- [ ] Server started without errors
- [ ] SignUp endpoint tested
- [ ] Email verification tested
- [ ] SignIn after verification tested
- [ ] Swagger documentation accessible
- [ ] Database contains test data

---

## 🎉 Success!

Jika semua checklist di atas ✅, authentication system Anda sudah siap!

**Status:** Phase 1 Complete - Ready for Phase 2 (Forgot/Reset Password)

---

## 📞 Need Help?

Check:
1. Server logs: `tail -f tmp/main.log`
2. Database logs: Check PostgreSQL logs
3. Auth implementation doc: `docs/AUTH_IMPLEMENTATION.md`
