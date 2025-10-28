# Logging & Swagger Documentation

## 🔍 Logging dengan Ginzap + Zap

### Overview

Project ini menggunakan **Uber's Zap** untuk logging dengan **Ginzap** middleware untuk HTTP request logging.

### Features

- ✅ **High Performance** - Zero allocation logging
- ✅ **Structured Logging** - JSON format untuk production
- ✅ **Pretty Console** - Colored output untuk development
- ✅ **HTTP Request Logging** - Automatic via Ginzap middleware
- ✅ **Panic Recovery** - Automatic recovery dengan stack trace

### Logger Package

Logger sudah di-wrap di `pkg/logger/logger.go` sehingga mudah diganti implementasinya.

**PENTING**: Package lain **TIDAK BOLEH** import `go.uber.org/zap` secara langsung. Gunakan wrapper di `pkg/logger`.

```go
package logger

// Log level functions
func Info(msg string, fields ...Field)
func Debug(msg string, fields ...Field)
func Error(msg string, fields ...Field)
func Warn(msg string, fields ...Field)
func Fatal(msg string, fields ...Field)

// Field type constructors (NO need to import zap!)
func String(key string, val string) Field
func Int(key string, val int) Field
func Float64(key string, val float64) Field
func Bool(key string, val bool) Field
func Err(err error) Field
func Any(key string, val interface{}) Field
```

### Usage Examples

#### 1. Basic Logging

```go
import "lakukan-be/pkg/logger"

// Simple message
logger.Info("User logged in")
logger.Error("Failed to connect to database")
```

#### 2. Structured Logging (WITH Fields) ✅

```go
import "lakukan-be/pkg/logger"

// ✅ CORRECT - No need to import zap!
logger.Info("User signed in",
    logger.String("user_id", user.ID),
    logger.String("email", user.Email),
)

logger.Error("Database query failed",
    logger.Err(err),
    logger.String("query", "SELECT * FROM users"),
    logger.Int("rows", 100),
)
```

#### ❌ WRONG Way (DON'T DO THIS)

```go
import (
    "lakukan-be/pkg/logger"
    "go.uber.org/zap"  // ❌ Don't import zap in service/handler!
)

logger.Info("User signed in",
    zap.String("user_id", user.ID),  // ❌ Don't use zap directly
)
```

#### 3. Logging di Service Layer

```go
import "lakukan-be/pkg/logger"

func (s *AuthService) SignIn(req *dto.SignInRequest) (*dto.SignInResponse, error) {
    logger.Info("SignIn attempt", logger.String("login", req.Login))

    user, err := s.userRepo.FindByEmail(req.Login)
    if err != nil {
        logger.Error("Database error", logger.Err(err))
        return nil, err
    }

    logger.Info("SignIn successful",
        logger.String("user_id", user.ID),
        logger.String("email", user.Email),
    )
    return response, nil
}
```

### Log Levels

- **Debug**: Detailed information for debugging (hanya di development)
- **Info**: General information about app flow
- **Warn**: Warning messages (tidak error tapi perlu perhatian)
- **Error**: Error yang terjadi tapi app masih jalan
- **Fatal**: Critical error, app akan terminate

### Environment-based Configuration

**Development Mode** (`env=development`):
- Console output dengan warna
- Log level: Debug
- Pretty format

**Production Mode** (`env=production`):
- JSON output
- Log level: Info
- Optimized untuk log aggregation (ELK, Splunk, etc)

---

## 📚 Swagger API Documentation

### Overview

Project ini menggunakan **Swaggo** untuk auto-generate Swagger/OpenAPI documentation dari Go comments.

### Access Swagger UI

Setelah server running:

```
http://localhost:8080/swagger/index.html
```

### Generate Swagger Docs

Setiap kali ada perubahan pada API (handler, DTO), run:

```bash
# Generate swagger docs
~/go/bin/swag init -g cmd/api/main.go -o docs/swagger --parseDependency --parseInternal

# Or add to Makefile
make swagger-gen
```

### Swagger Annotations

#### 1. Main API Info (di main.go)

```go
// @title Lakukan API
// @version 1.0
// @description Backend API untuk Lakukan
// @host localhost:8080
// @BasePath /api/v1
// @securityDefinitions.apikey BearerAuth
// @in header
// @name Authorization
func main() { ... }
```

#### 2. Handler Annotations

```go
// SignIn handles user signin request
// @Summary User signin
// @Description Authenticate user with email/username and password
// @Tags Authentication
// @Accept json
// @Produce json
// @Param request body dto.SignInRequest true "Signin credentials"
// @Success 200 {object} response.Response{data=dto.SignInResponse}
// @Failure 400 {object} response.Response
// @Failure 401 {object} response.Response
// @Router /auth/signin [post]
func (h *AuthHandler) SignIn(c *gin.Context) { ... }
```

#### 3. Secured Endpoints (dengan JWT)

```go
// @Summary Get user profile
// @Security BearerAuth
// @Tags User
// @Produce json
// @Success 200 {object} response.Response
// @Router /users/me [get]
```

### Swagger Tags Reference

| Tag | Description |
|-----|-------------|
| `@Summary` | Short description (1 line) |
| `@Description` | Detailed description |
| `@Tags` | Group endpoints (e.g., Authentication, User) |
| `@Accept` | Request content type |
| `@Produce` | Response content type |
| `@Param` | Request parameters (body, query, path, header) |
| `@Success` | Success response |
| `@Failure` | Error responses |
| `@Router` | Endpoint path and method |
| `@Security` | Authentication scheme |

### Example: Complete Endpoint Documentation

```go
// CreateUser creates a new user
// @Summary Create new user
// @Description Create a new user with the provided details
// @Tags User Management
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param request body dto.CreateUserRequest true "User details"
// @Success 201 {object} response.Response{data=dto.UserResponse} "User created"
// @Failure 400 {object} response.Response "Invalid request"
// @Failure 401 {object} response.Response "Unauthorized"
// @Failure 409 {object} response.Response "User already exists"
// @Failure 500 {object} response.Response "Internal server error"
// @Router /users [post]
func (h *UserHandler) CreateUser(c *gin.Context) { ... }
```

---

## 🚀 Best Practices

### Logging

1. **Jangan log password atau sensitive data**
   ```go
   // ❌ Bad
   logger.Info("User login", logger.String("password", req.Password))

   // ✅ Good
   logger.Info("User login", logger.String("email", req.Email))
   ```

2. **Gunakan appropriate log level**
   ```go
   // User action = Info
   logger.Info("User created account")

   // Expected errors = Warn
   logger.Warn("Invalid credentials")

   // Unexpected errors = Error
   logger.Error("Database connection failed", logger.Err(err))
   ```

3. **Add context dengan fields**
   ```go
   logger.Error("Payment failed",
       logger.String("user_id", userID),
       logger.String("order_id", orderID),
       logger.Float64("amount", amount),
       logger.Err(err),
   )
   ```

4. **ALWAYS use logger wrapper, NOT zap directly**
   ```go
   // ❌ WRONG - Don't import zap
   import "go.uber.org/zap"
   logger.Info("msg", zap.String("key", "val"))

   // ✅ CORRECT - Use logger wrapper
   import "lakukan-be/pkg/logger"
   logger.Info("msg", logger.String("key", "val"))
   ```

### Swagger

1. **Keep annotations up-to-date** - Run `swag init` setiap ada perubahan
2. **Use meaningful tags** - Group related endpoints
3. **Document all status codes** - Include semua possible responses
4. **Use examples** - Tambahkan example values di DTO structs

---

## 📝 Makefile Commands (Optional)

Tambahkan ke `Makefile` untuk kemudahan:

```makefile
# Generate swagger docs
swagger-gen:
	@echo "Generating swagger documentation..."
	~/go/bin/swag init -g cmd/api/main.go -o docs/swagger --parseDependency --parseInternal
	@echo "✅ Swagger docs generated!"

# Run with swagger auto-reload (development)
dev:
	@make swagger-gen
	@go run cmd/api/main.go
```

Usage:
```bash
make swagger-gen  # Generate swagger docs
make dev          # Run with auto swagger generation
```

---

## 🔗 References

- **Zap**: https://github.com/uber-go/zap
- **Ginzap**: https://github.com/gin-contrib/zap
- **Swaggo**: https://github.com/swaggo/swag
- **Swagger Spec**: https://swagger.io/specification/