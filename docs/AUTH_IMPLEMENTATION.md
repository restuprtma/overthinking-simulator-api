# Auth System Implementation - Lakukan Backend

## 📋 Overview

Dokumen ini menjelaskan implementasi lengkap sistem autentikasi yang telah ditambahkan ke Lakukan Backend, termasuk registrasi, verifikasi email, dan fondasi untuk fitur-fitur security lainnya.

---

## ✅ Fitur Yang Sudah Diimplementasikan

### **Phase 1: Core Authentication (COMPLETED)**

#### 1. **User Registration (SignUp)** ✅
- Endpoint: `POST /api/v1/auth/signup`
- Features:
  - Email & username uniqueness validation
  - Password strength validation (min 8 chars, uppercase, lowercase, number)
  - Email format validation (RFC 5322 compliant)
  - Username format validation (alphanumeric, 3-20 chars)
  - Password hashing dengan bcrypt
  - Auto-generate verification token
  - Send verification email (async)
  - User created with `is_email_verified = false`

#### 2. **Email Verification** ✅
- Endpoint: `POST /api/v1/auth/verify-email`
- Features:
  - Token-based verification
  - Token expiration check (48 hours default)
  - Already verified check
  - Update `is_email_verified` status
  - Send welcome email after verification

#### 3. **Resend Verification Email** ✅
- Endpoint: `POST /api/v1/auth/resend-verification`
- Features:
  - Rate limiting (max 5 resend per user)
  - Resend count tracking
  - Generate new token
  - Security: don't reveal if email exists

#### 4. **User Sign In (Enhanced)** ✅
- Endpoint: `POST /api/v1/auth/signin`
- Existing features maintained
- Ready for integration dengan account lockout (future)

---

## 🗄️ Database Schema Changes

### **New Tables Created:**

#### 1. **password_reset_tokens**
```sql
CREATE TABLE core.password_reset_tokens (
    id CHAR(36) PRIMARY KEY,
    user_id CHAR(36) NOT NULL,
    token VARCHAR(255) UNIQUE NOT NULL,
    expires_at TIMESTAMP NOT NULL,
    used_at TIMESTAMP,
    ip_address VARCHAR(45),
    user_agent TEXT,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (user_id) REFERENCES core.users(id)
);
```

#### 2. **refresh_tokens**
```sql
CREATE TABLE core.refresh_tokens (
    id CHAR(36) PRIMARY KEY,
    user_id CHAR(36) NOT NULL,
    token_hash VARCHAR(255) UNIQUE NOT NULL,
    device_name VARCHAR(255),
    device_id VARCHAR(255),
    ip_address VARCHAR(45),
    user_agent TEXT,
    expires_at TIMESTAMP NOT NULL,
    last_used_at TIMESTAMP,
    revoked_at TIMESTAMP,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);
```

#### 3. **login_attempts**
```sql
CREATE TABLE core.login_attempts (
    id CHAR(36) PRIMARY KEY,
    user_id CHAR(36),
    email VARCHAR(255),
    username VARCHAR(100),
    ip_address VARCHAR(45),
    user_agent TEXT,
    status VARCHAR(20) NOT NULL, -- success, failed, locked
    failure_reason VARCHAR(255),
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);
```

#### 4. **audit_logs**
```sql
CREATE TABLE core.audit_logs (
    id CHAR(36) PRIMARY KEY,
    user_id CHAR(36),
    action VARCHAR(100) NOT NULL,
    resource VARCHAR(100),
    resource_id CHAR(36),
    ip_address VARCHAR(45),
    user_agent TEXT,
    metadata JSONB,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);
```

### **Table Modifications:**

#### email_verification_tokens (Enhanced)
```sql
ALTER TABLE core.email_verification_tokens
ADD COLUMN resent_count INT DEFAULT 0,
ADD COLUMN last_resent_at TIMESTAMP;
```

#### users (Enhanced for Security)
```sql
ALTER TABLE core.users
ADD COLUMN failed_login_count INT DEFAULT 0,
ADD COLUMN locked_until TIMESTAMP,
ADD COLUMN locked_at TIMESTAMP;
```

---

## 📁 New Files Structure

```
pkg/
├── email/                                  # ✅ Email Service
│   ├── email.go                           # Interface & NoOp implementation
│   ├── smtp.go                            # SMTP implementation
│   └── templates/
│       ├── verification.html              # Email verification template
│       ├── reset_password.html            # Password reset template
│       ├── welcome.html                   # Welcome email template
│       ├── account_locked.html            # Account locked notification
│       └── password_changed.html          # Password changed notification
│
├── validator/                             # ✅ Validation Utilities
│   ├── password.go                        # Password strength validation
│   ├── email.go                           # Email format validation
│   └── username.go                        # Username format validation
│
└── token/                                 # ✅ Token Generation
    └── generator.go                       # Secure token generation

internal/modules/core/auth/
├── domain/
│   └── token.go                           # ✅ Token domain models
│
├── dto/
│   ├── signin.dto.go                      # Existing
│   ├── signup.dto.go                      # ✅ New
│   └── verify_email.dto.go                # ✅ New
│
├── repository/
│   └── token.repository.go                # ✅ Token repository methods
│
├── service/
│   └── auth.service.go                    # ✅ Extended with new methods
│
├── handler/
│   └── auth.handler.go                    # ✅ Extended with new handlers
│
└── main.auth.go                           # ✅ Updated initialization & routes

internal/modules/core/user/repository/
└── user.repository.go                     # ✅ Added UpdateEmailVerified method
```

---

## 🔧 Configuration (.env)

### **New Environment Variables:**

```env
# JWT Configuration
JWT_REFRESH_EXPIRATION=168h

# Email Configuration (SMTP)
SMTP_HOST=smtp.mailtrap.io
SMTP_PORT=587
SMTP_USER=your-smtp-username
SMTP_PASSWORD=your-smtp-password
SMTP_FROM_EMAIL=noreply@lakukan.com
SMTP_FROM_NAME=Lakukan

# Frontend URLs (for email links)
FRONTEND_URL=http://localhost:3000
EMAIL_VERIFICATION_URL=http://localhost:3000/verify-email
RESET_PASSWORD_URL=http://localhost:3000/reset-password

# Support
SUPPORT_EMAIL=support@lakukan.com

# Security Settings
MAX_LOGIN_ATTEMPTS=5
ACCOUNT_LOCKOUT_DURATION=30m
PASSWORD_RESET_EXPIRATION=1h
EMAIL_VERIFICATION_EXPIRATION=48h
MAX_PASSWORD_RESET_REQUESTS=3
MAX_VERIFICATION_RESEND=5
```

---

## 🔌 API Endpoints

### **Authentication Endpoints:**

| Method | Endpoint | Description | Status |
|--------|----------|-------------|---------|
| POST | `/api/v1/auth/signin` | User sign in | ✅ Existing |
| POST | `/api/v1/auth/signup` | User registration | ✅ New |
| POST | `/api/v1/auth/verify-email` | Verify email with token | ✅ New |
| POST | `/api/v1/auth/resend-verification` | Resend verification email | ✅ New |
| POST | `/api/v1/auth/forgot-password` | Request password reset | ⏳ Planned |
| POST | `/api/v1/auth/reset-password` | Reset password with token | ⏳ Planned |
| POST | `/api/v1/auth/refresh` | Refresh access token | ⏳ Planned |
| POST | `/api/v1/auth/logout` | Logout (revoke token) | ⏳ Planned |

---

## 📝 Request/Response Examples

### 1. **SignUp Request**
```json
POST /api/v1/auth/signup
{
  "email": "user@example.com",
  "username": "john_doe",
  "password": "SecurePass123",
  "full_name": "John Doe"
}
```

**Response (201 Created):**
```json
{
  "success": true,
  "message": "Registration successful",
  "data": {
    "message": "Registration successful. Please check your email to verify your account.",
    "user": {
      "id": "uuid-here",
      "email": "user@example.com",
      "username": "john_doe",
      "full_name": "John Doe",
      "is_active": true,
      "is_email_verified": false
    }
  }
}
```

### 2. **Verify Email Request**
```json
POST /api/v1/auth/verify-email
{
  "token": "verification-token-here"
}
```

**Response (200 OK):**
```json
{
  "success": true,
  "message": "Email verified successfully",
  "data": {
    "message": "Email verified successfully. You can now sign in."
  }
}
```

### 3. **Resend Verification Request**
```json
POST /api/v1/auth/resend-verification
{
  "email": "user@example.com"
}
```

**Response (200 OK):**
```json
{
  "success": true,
  "message": "Verification email sent",
  "data": {
    "message": "Verification email has been sent. Please check your inbox."
  }
}
```

---

## 🔐 Security Features Implemented

### **Password Validation:**
- Minimum 8 characters
- At least one uppercase letter
- At least one lowercase letter
- At least one number
- Password cannot contain email address
- Check against common passwords

### **Email Validation:**
- RFC 5322 compliant
- Length check (max 254 chars)
- Format validation
- Normalization (lowercase, trim)
- Disposable email detection (basic)

### **Username Validation:**
- 3-20 characters
- Alphanumeric, underscores, hyphens
- Must start with letter or number

### **Token Security:**
- Cryptographically secure random tokens (64 chars hex)
- Token expiration (configurable)
- One-time use tokens
- Rate limiting on resend

---

## 🧪 Testing

### **Manual Testing Steps:**

#### 1. Test SignUp
```bash
curl -X POST http://localhost:8080/api/v1/auth/signup \
  -H "Content-Type: application/json" \
  -d '{
    "email": "test@example.com",
    "username": "testuser",
    "password": "TestPass123",
    "full_name": "Test User"
  }'
```

#### 2. Check Email/Logs for Verification Token
If using NoOpEmailService (development), token will be printed in console.

#### 3. Test Email Verification
```bash
curl -X POST http://localhost:8080/api/v1/auth/verify-email \
  -H "Content-Type: application/json" \
  -d '{
    "token": "TOKEN_FROM_EMAIL"
  }'
```

#### 4. Test SignIn After Verification
```bash
curl -X POST http://localhost:8080/api/v1/auth/signin \
  -H "Content-Type: application/json" \
  -d '{
    "login": "test@example.com",
    "password": "TestPass123"
  }'
```

#### 5. Test Resend Verification
```bash
curl -X POST http://localhost:8080/api/v1/auth/resend-verification \
  -H "Content-Type: application/json" \
  -d '{
    "email": "test@example.com"
  }'
```

---

## 📊 Migration Guide

### **Running Migrations:**

```bash
# Run new migrations
make migrate-up

# Or manually with specific version
migrate -path internal/database/migrations/core \
        -database "postgresql://user:pass@localhost:5432/lakukan?sslmode=disable" \
        up
```

### **Rollback if needed:**

```bash
make migrate-down

# Or manually
migrate -path internal/database/migrations/core \
        -database "postgresql://user:pass@localhost:5432/lakukan?sslmode=disable" \
        down 1
```

---

## 🚀 Next Steps (Phase 2 - Planned)

### **Forgot Password / Reset Password**
- [ ] `POST /auth/forgot-password` - Request reset token
- [ ] `POST /auth/reset-password` - Reset with token
- [ ] `GET /auth/validate-reset-token` - Validate token
- [ ] Rate limiting on reset requests
- [ ] Email notifications

### **Refresh Token & Session Management**
- [ ] `POST /auth/refresh` - Refresh access token
- [ ] `POST /auth/logout` - Revoke refresh token
- [ ] `GET /auth/sessions` - List active sessions
- [ ] `DELETE /auth/sessions/:id` - Revoke specific session
- [ ] Device tracking

### **Account Security**
- [ ] Account lockout after failed attempts
- [ ] Login attempts tracking
- [ ] Audit logging
- [ ] Rate limiting middleware
- [ ] Account unlock mechanism

---

## 🐛 Known Issues / Limitations

1. **Email Service:**
   - Currently using NoOpEmailService in development (prints to console)
   - Need to configure real SMTP in production
   - Email templates are basic (can be enhanced)

2. **Rate Limiting:**
   - Resend verification has simple counter-based limiting
   - Need proper distributed rate limiting for production

3. **Testing:**
   - No automated tests yet
   - Manual testing only

---

## 💡 Tips & Best Practices

### **For Development:**
```bash
# Copy example env
cp .env.example .env

# Update SMTP settings for testing with Mailtrap
SMTP_HOST=smtp.mailtrap.io
SMTP_PORT=2525
SMTP_USER=your-mailtrap-user
SMTP_PASSWORD=your-mailtrap-password
```

### **For Production:**
```bash
# Use strong JWT secret
JWT_SECRET=generate-strong-32plus-char-secret

# Use production email service (SendGrid, Mailgun, AWS SES)
SMTP_HOST=smtp.sendgrid.net
SMTP_PORT=587
SMTP_USER=apikey
SMTP_PASSWORD=your-sendgrid-api-key

# Set proper frontend URLs
FRONTEND_URL=https://your-domain.com
EMAIL_VERIFICATION_URL=https://your-domain.com/verify-email
```

---

## 📚 Dependencies Added

```go
// go.mod
require (
    gopkg.in/gomail.v2 v2.0.0-20160411212932-81ebce5c23df
    // ... existing dependencies
)
```

---

## 👥 Contributors

- Implementation: Claude Code
- Architecture: Based on industry best practices
- Date: January 2025

---

## 📞 Support

Jika ada pertanyaan atau issues:
1. Check logs di console
2. Verify database migrations ran successfully
3. Check SMTP configuration
4. Review error messages di API response

---

## ✨ Summary

**Total Files Created:** 20+
**Total Lines of Code:** ~2000+
**Database Tables:** 4 new tables + 2 modified
**API Endpoints:** 3 new endpoints
**Features:** Registration, Email Verification, Resend Verification
**Status:** ✅ Ready for testing and further development

**Next Priority:** Forgot Password / Reset Password implementation
