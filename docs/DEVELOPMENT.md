# Development Guide

## 🔥 Hot Reload Development

Project ini menggunakan **Air** untuk hot reload development, jadi setiap perubahan code akan otomatis rebuild dan restart server.

### Prerequisites

Install Air (sudah otomatis jika run `make dev`):
```bash
go install github.com/air-verse/air@latest
```

### Running Development Server

```bash
# Start dev server with hot reload
make dev
```

Command ini akan:
1. ✅ Generate Swagger docs
2. ✅ Start Air hot reload
3. ✅ Watch for file changes
4. ✅ Auto rebuild & restart on changes

### Available Make Commands

#### Development Commands

| Command | Description |
|---------|-------------|
| `make dev` | Run with hot reload (Air) + Swagger generation |
| `make swagger-gen` | Generate Swagger documentation only |
| `make build` | Build application binary to `bin/lakukan-api` |
| `make run` | Run application without hot reload |

#### Database Commands

| Command | Description |
|---------|-------------|
| `make db-setup` | Fresh database setup (migrations + seeders) |
| `make db-reset` | Drop, recreate, and seed database |
| `make migrate-up` | Run all pending migrations |
| `make migrate-down` | Rollback last migration |
| `make seed` | Run all seeders |

#### Help

```bash
make help  # Show all available commands
```

---

## 📁 Air Configuration

Configuration di [.air.toml](.air.toml):

```toml
[build]
  cmd = "go build -o ./tmp/main ./cmd/api/main.go"
  bin = "./tmp/main"
  exclude_dir = ["assets", "tmp", "vendor", "testdata", "docs/swagger", "bin"]
  include_ext = ["go", "tpl", "tmpl", "html"]
```

### Excluded Directories

Air **TIDAK** watch changes di:
- `tmp/` - Temporary build files
- `docs/swagger/` - Generated swagger files
- `bin/` - Binary files
- `vendor/` - Dependencies
- `.git/`, `.vscode/` - IDE & version control

### Included Extensions

Air **WATCH** changes di file:
- `.go` - Go source files
- `.tpl`, `.tmpl` - Template files
- `.html` - HTML files

---

## 🛠️ Development Workflow

### 1. First Time Setup

```bash
# Clone repository
git clone <repo-url>
cd lakukan-be

# Copy environment file
cp .env.example .env

# Edit .env dengan database credentials
nano .env

# Setup database (migrations + seeders)
make db-setup
```

### 2. Daily Development

```bash
# Start development server
make dev

# Server will run on http://localhost:8080
# Swagger UI: http://localhost:8080/swagger/index.html
# Health check: http://localhost:8080/health
```

### 3. Code Changes

Saat Anda edit file `.go`:
1. Air detect perubahan
2. Auto rebuild binary ke `tmp/main`
3. Auto restart server
4. Logger akan show restart message

**Terminal output example:**
```
building...
running...
2025/10/09 20:00:00 INFO    Starting Lakukan API
2025/10/09 20:00:00 INFO    Database connected successfully
2025/10/09 20:00:00 INFO    Starting server on :8080 (Environment: development)
```

### 4. Swagger Changes

Jika Anda ubah:
- API annotations di handler
- Swagger comments di main.go
- Request/Response DTOs

Run:
```bash
make swagger-gen
```

Atau restart `make dev` (otomatis generate swagger).

---

## 🐛 Debugging Tips

### Check Logs

Air logs ada di:
- Console output (stdout)
- `build-errors.log` - Build errors only

### Force Rebuild

Jika Air tidak detect changes:
```bash
# Stop Air (Ctrl+C)
# Clean temporary files
rm -rf tmp/

# Restart
make dev
```

### Check Air Status

```bash
# Verify Air installed
~/go/bin/air -v

# Run Air directly (without make)
~/go/bin/air
```

---

## 📦 Building for Production

```bash
# Build binary
make build

# Run binary
./bin/lakukan-api

# Or use go run
make run
```

---

## 🔧 Customizing Air

Edit [.air.toml](.air.toml) untuk customize behavior:

```toml
[build]
  # Delay before rebuild (ms)
  delay = 1000

  # Stop on build error
  stop_on_error = false

  # Pre-build command
  pre_cmd = []

  # Post-build command
  post_cmd = []
```

---

## 🚀 Performance Tips

### 1. Exclude Large Directories

Tambahkan ke `exclude_dir` di `.air.toml`:
```toml
exclude_dir = ["node_modules", "public", "assets"]
```

### 2. Watch Only What You Need

Konfigurasi `include_ext`:
```toml
include_ext = ["go"]  # Only .go files
```

### 3. Adjust Delay

Kurangi delay untuk faster reload:
```toml
delay = 500  # 500ms (default: 1000ms)
```

---

## 📝 Common Issues

### Issue: "air: command not found"

**Solution:**
```bash
# Install Air
go install github.com/air-verse/air@latest

# Add Go bin to PATH (add to ~/.zshrc or ~/.bashrc)
export PATH=$PATH:~/go/bin
```

### Issue: Changes not detected

**Solution:**
1. Check file is NOT in `exclude_dir`
2. Check file extension in `include_ext`
3. Force rebuild: `rm -rf tmp/ && make dev`

### Issue: Port already in use

**Solution:**
```bash
# Find process using port 8080
lsof -ti:8080

# Kill process
kill -9 $(lsof -ti:8080)

# Or change port in .env
SERVER_PORT=8081
```

---

## 🔗 References

- **Air**: https://github.com/air-verse/air
- **Go Modules**: https://go.dev/doc/modules/
- **Makefile Tutorial**: https://makefiletutorial.com/