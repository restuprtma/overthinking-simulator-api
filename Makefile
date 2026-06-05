.PHONY: help migrate-up migrate-down migrate-create migrate-force migrate-version seed seed-core db-setup db-reset dev build run build-iqplus-publisher build-iqplus-publisher-freebsd build-running-trade-consumer build-running-trade-consumer-freebsd build-quote-consumer build-quote-consumer-freebsd build-meta-consumer build-meta-consumer-freebsd build-news-consumer build-news-consumer-freebsd build-tradedone-consumer build-tradedone-consumer-freebsd build-nbs-consumer build-nbs-consumer-freebsd build-resend-trade-consumer build-resend-trade-consumer-freebsd build-resend-order-consumer build-resend-order-consumer-freebsd build-running-order-consumer build-running-order-consumer-freebsd build-orderbook-consumer build-orderbook-consumer-freebsd run-ws-gateway run-orderbook-consumer

help:
	@echo "Available commands:"
	@echo ""
	@echo "Development:"
	@echo "  make dev                     - Run with hot reload (Air)"
	@echo "  make build                   - Build application binary"
	@echo "  make run                     - Run application"
	@echo ""
	@echo "Database migrations:"
	@echo "  make migrate-up              - Run all pending migrations"
	@echo "  make migrate-down            - Rollback last migration"
	@echo "  make migrate-create NAME=xxx - Create new migration file"
	@echo "  make migrate-force V=xxx     - Force migration to specific version"
	@echo "  make migrate-version         - Show current migration version"
	@echo ""
	@echo "Seeding commands:"
	@echo "  make seed                    - Run all seeders"
	@echo "  make seed-core               - Run core module seeders only"
	@echo ""
	@echo "Setup commands:"
	@echo "  make db-setup                - Run migrations + seeders (fresh setup)"
	@echo "  make db-reset                - Drop, migrate, and seed database"
	@echo ""
	@echo "IQPlus publisher:"
	@echo "  make build-iqplus-publisher          - Build for the current host"
	@echo "  make build-iqplus-publisher-freebsd  - Cross-compile for FreeBSD/amd64"
	@echo ""
	@echo "Running trade consumer:"
	@echo "  make build-running-trade-consumer            - Build for the current host"
	@echo "  make build-running-trade-consumer-freebsd    - Cross-compile for FreeBSD/amd64"
	@echo ""
	@echo "Quote consumer:"
	@echo "  make build-quote-consumer            - Build for the current host"
	@echo "  make build-quote-consumer-freebsd    - Cross-compile for FreeBSD/amd64"
	@echo ""
	@echo "Meta consumer (status/activity/summary/top20):"
	@echo "  make build-meta-consumer             - Build for the current host"
	@echo "  make build-meta-consumer-freebsd     - Cross-compile for FreeBSD/amd64"
	@echo ""
	@echo "News consumer (idx.news.> → MongoDB):"
	@echo "  make build-news-consumer             - Build for the current host"
	@echo "  make build-news-consumer-freebsd     - Cross-compile for FreeBSD/amd64"
	@echo ""
	@echo "Trade Done consumer (idx.tradedone.> → Redis volume profile):"
	@echo "  make build-tradedone-consumer        - Build for the current host"
	@echo "  make build-tradedone-consumer-freebsd - Cross-compile for FreeBSD/amd64"
	@echo ""
	@echo "NBS consumer (idx.nbs.> → Redis dual-view bandar/foreign flow):"
	@echo "  make build-nbs-consumer              - Build for the current host"
	@echo "  make build-nbs-consumer-freebsd      - Cross-compile for FreeBSD/amd64"
	@echo ""
	@echo "Resend trade handler (idx.resend.trade.> → QuestDB trades w/ broker codes):"
	@echo "  make build-resend-trade-consumer          - Build for the current host"
	@echo "  make build-resend-trade-consumer-freebsd  - Cross-compile for FreeBSD/amd64"
	@echo ""
	@echo "Resend order handler (idx.resend.order.> → QuestDB orders w/ broker codes):"
	@echo "  make build-resend-order-consumer          - Build for the current host"
	@echo "  make build-resend-order-consumer-freebsd  - Cross-compile for FreeBSD/amd64"
	@echo ""
	@echo "Running order consumer (idx.order.> → QuestDB running_orders, broker masked):"
	@echo "  make build-running-order-consumer        - Build for the current host"
	@echo "  make build-running-order-consumer-freebsd - Cross-compile for FreeBSD/amd64"
	@echo ""
	@echo "Order Book consumer (idx.bestquote.> → Redis bid/ask depth):"
	@echo "  make build-orderbook-consumer        - Build for the current host"
	@echo "  make build-orderbook-consumer-freebsd - Cross-compile for FreeBSD/amd64"
	@echo ""
	@echo "Local dev run (preflight: .env + NATS + Redis, then go run):"
	@echo "  make run-ws-gateway                  - WebSocket gateway on :8081"
	@echo "  make run-orderbook-consumer          - Order book engine + Redis/NATS sinks"

# Database migrations
DB_URL=postgresql://$(DB_USER):$(DB_PASSWORD)@$(DB_HOST):$(DB_PORT)/$(DB_NAME)?sslmode=$(DB_SSLMODE)
MIGRATIONS_PATH=internal/database/migrations
SEEDERS_PATH=internal/database/seeders

# Default values from .env
include .env
export

migrate-up:
	@echo "Running migrations..."
	@if [ -d "$(MIGRATIONS_PATH)/core" ]; then \
		echo "Running core migrations..."; \
		migrate -path $(MIGRATIONS_PATH)/core -database "$(DB_URL)&x-migrations-table=schema_migrations_core" up; \
	fi
	@if [ -d "$(MIGRATIONS_PATH)/stock" ]; then \
		echo "Running stock migrations..."; \
		migrate -path $(MIGRATIONS_PATH)/stock -database "$(DB_URL)&x-migrations-table=schema_migrations_stock" up; \
	fi

migrate-down:
	@if [ -z "$(MODULE)" ]; then echo "Error: MODULE is required. Usage: make migrate-down MODULE=core|stock"; exit 1; fi
	@echo "Rolling back last migration for module $(MODULE)..."
	migrate -path $(MIGRATIONS_PATH)/$(MODULE) -database "$(DB_URL)&x-migrations-table=schema_migrations_$(MODULE)" down 1

migrate-create:
	@if [ -z "$(NAME)" ]; then echo "Error: NAME is required. Usage: make migrate-create NAME=your_migration_name MODULE=core|stock"; exit 1; fi
	@if [ -z "$(MODULE)" ]; then echo "Error: MODULE is required. Available: core, stock"; exit 1; fi
	@echo "Creating migration: $(NAME) in module $(MODULE)"
	migrate create -ext sql -dir $(MIGRATIONS_PATH)/$(MODULE) -seq $(NAME)

migrate-force:
	@if [ -z "$(V)" ]; then echo "Error: V is required. Usage: make migrate-force V=1 MODULE=core|stock"; exit 1; fi
	@if [ -z "$(MODULE)" ]; then echo "Error: MODULE is required. Available: core, stock"; exit 1; fi
	migrate -path $(MIGRATIONS_PATH)/$(MODULE) -database "$(DB_URL)&x-migrations-table=schema_migrations_$(MODULE)" force $(V)

migrate-version:
	@echo "Core module version:"
	@migrate -path $(MIGRATIONS_PATH)/core -database "$(DB_URL)&x-migrations-table=schema_migrations_core" version
	@echo "Stock module version:"
	@migrate -path $(MIGRATIONS_PATH)/stock -database "$(DB_URL)&x-migrations-table=schema_migrations_stock" version

# Database seeders
seed-core:
	@echo "Running core seeders..."
	@if [ -d "$(SEEDERS_PATH)/core" ]; then \
		for file in $(SEEDERS_PATH)/core/*.sql; do \
			if [ -f "$$file" ]; then \
				echo "Running seeder: $$(basename $$file)"; \
				PGPASSWORD=$(DB_PASSWORD) psql -h $(DB_HOST) -p $(DB_PORT) -U $(DB_USER) -d $(DB_NAME) -f $$file; \
			fi; \
		done; \
	fi
	@echo "✅ Core seeders completed!"

seed: seed-core
	@echo "✅ All seeders completed!"

# Database setup commands
db-setup:
	@echo "🚀 Setting up database..."
	@make migrate-up
	@echo ""
	@make seed
	@echo "✅ Database setup completed!"

db-reset:
	@echo "⚠️  Resetting database (this will drop all data)..."
	@PGPASSWORD=$(DB_PASSWORD) psql -h $(DB_HOST) -p $(DB_PORT) -U $(DB_USER) -d postgres -c "DROP DATABASE IF EXISTS $(DB_NAME);"
	@PGPASSWORD=$(DB_PASSWORD) psql -h $(DB_HOST) -p $(DB_PORT) -U $(DB_USER) -d postgres -c "CREATE DATABASE $(DB_NAME);"
	@echo "Database dropped and recreated!"
	@make db-setup

# Development commands
build:
	@echo "🔨 Building application..."
	@go build -o bin/tuai-api cmd/api/main.go
	@echo "✅ Build completed: bin/tuai-api"

run:
	@echo "🚀 Running application..."
	@go run cmd/api/main.go

dev:
	@echo "🔥 Starting development server with hot reload..."
	@~/go/bin/air

# IQPlus → JetStream publisher binary (host build)
build-iqplus-publisher:
	@echo "🔨 Building iqplus-publisher..."
	@go build -o bin/iqplus-publisher ./cmd/iqplus-publisher
	@echo "✅ Build completed: bin/iqplus-publisher"

# Cross-compile for the FreeBSD deployment target.
# Override GOARCH if your server is not amd64 (e.g. GOARCH=arm64).
build-iqplus-publisher-freebsd:
	@echo "🔨 Cross-compiling iqplus-publisher for FreeBSD/amd64..."
	@GOOS=freebsd GOARCH=amd64 CGO_ENABLED=0 \
		go build -trimpath -ldflags="-s -w" \
		-o bin/iqplus-publisher-freebsd-amd64 ./cmd/iqplus-publisher
	@echo "✅ Build completed: bin/iqplus-publisher-freebsd-amd64"

# Mock IQPlus TCP server for load-testing the publisher's burst handling.
# See cmd/iqplus-mock-server/main.go for protocol & flags.
build-iqplus-mock-server:
	@echo "🔨 Building iqplus-mock-server..."
	@go build -o bin/iqplus-mock-server ./cmd/iqplus-mock-server
	@echo "✅ Build completed: bin/iqplus-mock-server"

build-iqplus-mock-server-linux:
	@echo "🔨 Cross-compiling iqplus-mock-server for Linux/amd64..."
	@GOOS=linux GOARCH=amd64 CGO_ENABLED=0 \
		go build -trimpath -ldflags="-s -w" \
		-o bin/iqplus-mock-server-linux-amd64 ./cmd/iqplus-mock-server
	@echo "✅ Build completed: bin/iqplus-mock-server-linux-amd64"

build-iqplus-mock-server-freebsd:
	@echo "🔨 Cross-compiling iqplus-mock-server for FreeBSD/amd64..."
	@GOOS=freebsd GOARCH=amd64 CGO_ENABLED=0 \
		go build -trimpath -ldflags="-s -w" \
		-o bin/iqplus-mock-server-freebsd-amd64 ./cmd/iqplus-mock-server
	@echo "✅ Build completed: bin/iqplus-mock-server-freebsd-amd64"

# Loadtest receiver — exercises iqplus client.go TCP path in isolation
# (no NATS publish), so we can measure pure kernel-TCP loss without
# polluting the live IDX_TICK stream.
build-iqplus-loadtest-receiver-freebsd:
	@echo "🔨 Cross-compiling iqplus-loadtest-receiver for FreeBSD/amd64..."
	@GOOS=freebsd GOARCH=amd64 CGO_ENABLED=0 \
		go build -trimpath -ldflags="-s -w" \
		-o bin/iqplus-loadtest-receiver-freebsd-amd64 ./cmd/iqplus-loadtest-receiver
	@echo "✅ Build completed: bin/iqplus-loadtest-receiver-freebsd-amd64"

# Replay-mock: feeds the publisher with REAL trade rows pulled from QuestDB
# (NDJSON format) so load tests use realistic record shape & broker codes.
build-iqplus-replay-mock-linux:
	@echo "🔨 Cross-compiling iqplus-replay-mock for Linux/amd64..."
	@GOOS=linux GOARCH=amd64 CGO_ENABLED=0 \
		go build -trimpath -ldflags="-s -w" \
		-o bin/iqplus-replay-mock-linux-amd64 ./cmd/iqplus-replay-mock
	@echo "✅ Build completed: bin/iqplus-replay-mock-linux-amd64"

# Running trade consumer (idx.trade.> → QuestDB tick + Redis OHLCV bars)
build-running-trade-consumer:
	@echo "🔨 Building running-trade-consumer..."
	@go build -o bin/running-trade-consumer ./cmd/running-trade-consumer
	@echo "✅ Build completed: bin/running-trade-consumer"

build-running-trade-consumer-freebsd:
	@echo "🔨 Cross-compiling running-trade-consumer for FreeBSD/amd64..."
	@GOOS=freebsd GOARCH=amd64 CGO_ENABLED=0 \
		go build -trimpath -ldflags="-s -w" \
		-o bin/running-trade-consumer-freebsd-amd64 ./cmd/running-trade-consumer
	@echo "✅ Build completed: bin/running-trade-consumer-freebsd-amd64"

# Quote state consumer (idx.quote.> → Redis hash quote:<stock>)
build-quote-consumer:
	@echo "🔨 Building quote-consumer..."
	@go build -o bin/quote-consumer ./cmd/quote-consumer
	@echo "✅ Build completed: bin/quote-consumer"

build-quote-consumer-freebsd:
	@echo "🔨 Cross-compiling quote-consumer for FreeBSD/amd64..."
	@GOOS=freebsd GOARCH=amd64 CGO_ENABLED=0 \
		go build -trimpath -ldflags="-s -w" \
		-o bin/quote-consumer-freebsd-amd64 ./cmd/quote-consumer
	@echo "✅ Build completed: bin/quote-consumer-freebsd-amd64"

# Meta consumer (idx.{status,activity,summary,top20}.> → Redis hashes)
build-meta-consumer:
	@echo "🔨 Building meta-consumer..."
	@go build -o bin/meta-consumer ./cmd/meta-consumer
	@echo "✅ Build completed: bin/meta-consumer"

build-meta-consumer-freebsd:
	@echo "🔨 Cross-compiling meta-consumer for FreeBSD/amd64..."
	@GOOS=freebsd GOARCH=amd64 CGO_ENABLED=0 \
		go build -trimpath -ldflags="-s -w" \
		-o bin/meta-consumer-freebsd-amd64 ./cmd/meta-consumer
	@echo "✅ Build completed: bin/meta-consumer-freebsd-amd64"

# News consumer (idx.news.> → MongoDB collection `news`)
build-news-consumer:
	@echo "🔨 Building news-consumer..."
	@go build -o bin/news-consumer ./cmd/news-consumer
	@echo "✅ Build completed: bin/news-consumer"

build-news-consumer-freebsd:
	@echo "🔨 Cross-compiling news-consumer for FreeBSD/amd64..."
	@GOOS=freebsd GOARCH=amd64 CGO_ENABLED=0 \
		go build -trimpath -ldflags="-s -w" \
		-o bin/news-consumer-freebsd-amd64 ./cmd/news-consumer
	@echo "✅ Build completed: bin/news-consumer-freebsd-amd64"

# Trade Done consumer (idx.tradedone.> → Redis volume profile per stock)
build-tradedone-consumer:
	@echo "🔨 Building tradedone-consumer..."
	@go build -o bin/tradedone-consumer ./cmd/tradedone-consumer
	@echo "✅ Build completed: bin/tradedone-consumer"

build-tradedone-consumer-freebsd:
	@echo "🔨 Cross-compiling tradedone-consumer for FreeBSD/amd64..."
	@GOOS=freebsd GOARCH=amd64 CGO_ENABLED=0 \
		go build -trimpath -ldflags="-s -w" \
		-o bin/tradedone-consumer-freebsd-amd64 ./cmd/tradedone-consumer
	@echo "✅ Build completed: bin/tradedone-consumer-freebsd-amd64"

# NBS consumer (idx.nbs.> → Redis dual-view: nbs:stock:* + nbs:broker:*)
build-nbs-consumer:
	@echo "🔨 Building nbs-consumer..."
	@go build -o bin/nbs-consumer ./cmd/nbs-consumer
	@echo "✅ Build completed: bin/nbs-consumer"

build-nbs-consumer-freebsd:
	@echo "🔨 Cross-compiling nbs-consumer for FreeBSD/amd64..."
	@GOOS=freebsd GOARCH=amd64 CGO_ENABLED=0 \
		go build -trimpath -ldflags="-s -w" \
		-o bin/nbs-consumer-freebsd-amd64 ./cmd/nbs-consumer
	@echo "✅ Build completed: bin/nbs-consumer-freebsd-amd64"

# Resend trade handler (idx.resend.trade.> → QuestDB trades w/ broker codes)
build-resend-trade-consumer:
	@echo "🔨 Building resend-trade-consumer..."
	@go build -o bin/resend-trade-consumer ./cmd/resend-trade-consumer
	@echo "✅ Build completed: bin/resend-trade-consumer"

build-resend-trade-consumer-freebsd:
	@echo "🔨 Cross-compiling resend-trade-consumer for FreeBSD/amd64..."
	@GOOS=freebsd GOARCH=amd64 CGO_ENABLED=0 \
		go build -trimpath -ldflags="-s -w" \
		-o bin/resend-trade-consumer-freebsd-amd64 ./cmd/resend-trade-consumer
	@echo "✅ Build completed: bin/resend-trade-consumer-freebsd-amd64"

# Resend order handler (idx.resend.order.> → QuestDB orders w/ broker codes)
build-resend-order-consumer:
	@echo "🔨 Building resend-order-consumer..."
	@go build -o bin/resend-order-consumer ./cmd/resend-order-consumer
	@echo "✅ Build completed: bin/resend-order-consumer"

build-resend-order-consumer-freebsd:
	@echo "🔨 Cross-compiling resend-order-consumer for FreeBSD/amd64..."
	@GOOS=freebsd GOARCH=amd64 CGO_ENABLED=0 \
		go build -trimpath -ldflags="-s -w" \
		-o bin/resend-order-consumer-freebsd-amd64 ./cmd/resend-order-consumer
	@echo "✅ Build completed: bin/resend-order-consumer-freebsd-amd64"

# Running order consumer (idx.order.> → QuestDB running_orders, broker masked)
build-running-order-consumer:
	@echo "🔨 Building running-order-consumer..."
	@go build -o bin/running-order-consumer ./cmd/running-order-consumer
	@echo "✅ Build completed: bin/running-order-consumer"

build-running-order-consumer-freebsd:
	@echo "🔨 Cross-compiling running-order-consumer for FreeBSD/amd64..."
	@GOOS=freebsd GOARCH=amd64 CGO_ENABLED=0 \
		go build -trimpath -ldflags="-s -w" \
		-o bin/running-order-consumer-freebsd-amd64 ./cmd/running-order-consumer
	@echo "✅ Build completed: bin/running-order-consumer-freebsd-amd64"

# Order Book consumer (idx.bestquote.> → Redis bid/ask depth)
build-orderbook-consumer:
	@echo "🔨 Building orderbook-consumer..."
	@go build -o bin/orderbook-consumer ./cmd/orderbook-consumer
	@echo "✅ Build completed: bin/orderbook-consumer"

build-orderbook-consumer-freebsd:
	@echo "🔨 Cross-compiling orderbook-consumer for FreeBSD/amd64..."
	@GOOS=freebsd GOARCH=amd64 CGO_ENABLED=0 \
		go build -trimpath -ldflags="-s -w" \
		-o bin/orderbook-consumer-freebsd-amd64 ./cmd/orderbook-consumer
	@echo "✅ Build completed: bin/orderbook-consumer-freebsd-amd64"

# -----------------------------------------------------------------------------
# Local dev runners — wrap `go run` with preflight checks (env file, required
# vars, NATS reachability, Redis ping + DBSIZE). Use these instead of plain
# `go run ./cmd/<service>` when you want fail-fast misconfig detection.
#
# Both delegate to scripts/run-service.sh which carries the per-service
# preflight profile.
# -----------------------------------------------------------------------------
run-ws-gateway:
	@./scripts/run-service.sh ws-gateway

run-orderbook-consumer:
	@./scripts/run-service.sh orderbook-consumer
