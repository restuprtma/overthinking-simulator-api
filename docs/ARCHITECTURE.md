# Architecture Documentation

## Project Structure

```
lakukan-be/
├── cmd/api/                           # Application entry point
│   └── main.go                        # Main server initialization
├── internal/
│   ├── config/                        # Configuration management
│   ├── database/                      # Database connection & migrations
│   │   ├── migrations/                # SQL migration files
│   │   │   ├── core/                  # Core module migrations
│   │   │   └── finance/               # Finance module migrations
│   │   └── seeders/                   # Database seeders
│   ├── modules/                       # Business modules
│   │   ├── core/                      # Core business domain
│   │   │   ├── auth/                  # Authentication feature
│   │   │   │   ├── domain/            # Domain models
│   │   │   │   ├── dto/               # Data Transfer Objects
│   │   │   │   ├── handler/           # HTTP handlers
│   │   │   │   ├── service/           # Business logic
│   │   │   │   └── main.auth.go       # Module initialization & routes
│   │   │   ├── user/                  # User management feature
│   │   │   │   ├── domain/
│   │   │   │   ├── dto/
│   │   │   │   ├── handler/
│   │   │   │   ├── repository/
│   │   │   │   ├── service/
│   │   │   │   └── main.user.go
│   │   │   └── role/                  # Role management feature
│   │   │       ├── domain/
│   │   │       ├── dto/
│   │   │       ├── handler/
│   │   │       ├── repository/
│   │   │       ├── service/
│   │   │       └── main.role.go
│   │   └── finance/                   # Finance business domain
│   │       ├── coa/
│   │       ├── invoice/
│   │       ├── journal/
│   │       └── ...
│   ├── router/                        # Route configuration
│   │   └── router.go                  # Central router setup
│   └── shared/                        # Shared utilities
│       └── response/                  # API response helpers
└── pkg/                               # Public packages
    ├── crypto/                        # Cryptography utilities
    ├── eventbus/                      # Event bus
    └── validator/                     # Validation utilities
```

## Module Pattern

Setiap feature module mengikuti pattern yang sama:

### 1. Module Structure

```
module_name/
├── domain/          # Domain entities & business rules
├── dto/             # Request/Response data structures
├── handler/         # HTTP request handlers
├── repository/      # Data access layer
├── service/         # Business logic layer
└── main.{module}.go # Module initialization & route setup
```

### 2. Layer Responsibilities

#### Domain Layer
- Mendefinisikan core business entities
- Berisi business rules dan validations
- Tidak bergantung pada layer lain

#### DTO Layer
- Request payload structures
- Response structures
- Input validation tags

#### Repository Layer
- Database operations (CRUD)
- Query builders
- Data mapping

#### Service Layer
- Business logic implementation
- Orchestration between repositories
- Transaction management
- Error handling

#### Handler Layer
- HTTP request/response handling
- Input validation
- Calling service layer
- Response formatting

### 3. Module Initialization Pattern

Setiap module memiliki file `main.{module}.go` yang berisi:

```go
package modulename

import (
    "github.com/gin-gonic/gin"
    "github.com/jackc/pgx/v5/pgxpool"
)

type ModuleNameModule struct {
    Handler *handler.ModuleNameHandler
}

// Initialize initializes the module with all dependencies
func Initialize(db *pgxpool.Pool) *ModuleNameModule {
    // 1. Initialize repositories
    repo := repository.NewRepository(db)

    // 2. Initialize services
    service := service.NewService(repo)

    // 3. Initialize handlers
    handler := handler.NewHandler(service)

    return &ModuleNameModule{
        Handler: handler,
    }
}

// SetupRoutes sets up module routes
func (m *ModuleNameModule) SetupRoutes(router *gin.RouterGroup) {
    group := router.Group("/endpoint")
    {
        group.GET("", m.Handler.GetAll)
        group.GET("/:id", m.Handler.GetByID)
        group.POST("", m.Handler.Create)
        group.PUT("/:id", m.Handler.Update)
        group.DELETE("/:id", m.Handler.Delete)
    }
}
```

### 4. Router Setup

Router di `internal/router/router.go` menginisialisasi semua modules:

```go
func Setup(router *gin.Engine, db *pgxpool.Pool) {
    v1 := router.Group("/api/v1")
    {
        // Initialize and setup each module
        authModule := auth.Initialize(db)
        authModule.SetupRoutes(v1)

        userModule := user.Initialize(db)
        userModule.SetupRoutes(v1)
    }
}
```

## Dependency Flow

```
main.go
  └─> router.Setup()
       └─> module.Initialize()
            ├─> repository.New()
            ├─> service.New(repo)
            └─> handler.New(service)
       └─> module.SetupRoutes()
```

## Adding New Module

1. Buat folder module baru di `internal/modules/{domain}/{feature}/`
2. Buat layers: domain, dto, handler, repository, service
3. Buat file `main.{feature}.go` dengan pattern di atas
4. Register module di `internal/router/router.go`

Contoh:
```bash
# Buat struktur folder
mkdir -p internal/modules/core/company/{domain,dto,handler,repository,service}

# Buat main.company.go
touch internal/modules/core/company/main.company.go

# Tambahkan di router.go:
companyModule := company.Initialize(db)
companyModule.SetupRoutes(v1)
```

## Benefits

- ✅ **Modular**: Setiap feature independent
- ✅ **Scalable**: Mudah add/remove features
- ✅ **Testable**: Clear separation of concerns
- ✅ **Maintainable**: Consistent structure across modules
- ✅ **Clean**: Dependencies injection yang jelas