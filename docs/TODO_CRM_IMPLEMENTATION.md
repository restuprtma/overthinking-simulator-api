# TODO: CRM Module Implementation Progress

**Last Updated:** 2025-10-13
**Session:** Chat implementation - CRM Module Development

---

## 🎯 Overview

Implementasi CRM module untuk Lakukan Backend dengan multi-tenancy support menggunakan JWT token yang sudah include CompanyID.

---

## ✅ COMPLETED

### 1. Infrastructure & Multi-Tenancy Setup ✅

#### JWT & Authentication
- [x] Update JWT Claims - Added `CompanyID` field ([pkg/jwt/claims.go](pkg/jwt/claims.go))
- [x] Update `GenerateToken()` to accept `companyID` parameter ([pkg/jwt/jwt.go](pkg/jwt/jwt.go))
- [x] Update `RefreshToken()` to pass `CompanyID` ([pkg/jwt/jwt.go](pkg/jwt/jwt.go))

#### Middleware
- [x] Add `GetCompanyID()` function ([internal/middleware/context.go](internal/middleware/context.go#L86-L93))
- [x] Add `MustGetUserID()` helper function ([internal/middleware/context.go](internal/middleware/context.go#L95-L100))
- [x] Create `CompanyContext()` middleware ([internal/middleware/company.go](internal/middleware/company.go))

#### Repository
- [x] Add `GetPrimaryCompanyID()` in CompanyUserRepository ([internal/modules/core/company/repository/company_user.repository.go](internal/modules/core/company/repository/company_user.repository.go#L265-L301))

#### Auth Service
- [x] Inject `CompanyUserRepository` to auth module
- [x] Update SignIn to fetch and include `companyID` in token
- [x] Update RefreshToken to fetch and include `companyID` in token

### 2. CRM Database Migration ✅
- [x] Create CRM schema migration ([internal/database/migrations/crm/000006_crm_tables.up.sql](internal/database/migrations/crm/000006_crm_tables.up.sql))
  - 9 tables (lead_sources, sales_targets, auto_reply_rules, leads, deals, chats, chat_messages, activities, sales_performance)
  - 2 materialized views (mv_dashboard_stats, mv_revenue_by_source)
  - 99+ database objects (indexes, constraints, etc.)
- [x] Create rollback migration ([internal/database/migrations/crm/000006_crm_tables.down.sql](internal/database/migrations/crm/000006_crm_tables.down.sql))

### 3. FASE 1: Lead Sources Module ✅

**Files Created (6 files):**
- [x] Domain model ([internal/modules/crm/lead_sources/domain/lead_source.go](internal/modules/crm/lead_sources/domain/lead_source.go))
- [x] DTOs ([internal/modules/crm/lead_sources/dto/lead_source.dto.go](internal/modules/crm/lead_sources/dto/lead_source.dto.go))
  - CreateLeadSourceRequest
  - UpdateLeadSourceRequest
  - LeadSourceResponse
  - LeadSourceListResponse
  - LeadSourceQueryParams
- [x] Repository ([internal/modules/crm/lead_sources/repository/lead_source.repository.go](internal/modules/crm/lead_sources/repository/lead_source.repository.go))
  - FindByID, FindByCode, FindAll, Count
  - Create, Update, Delete, Restore
- [x] Service ([internal/modules/crm/lead_sources/service/lead_source.service.go](internal/modules/crm/lead_sources/service/lead_source.service.go))
  - Business logic with validation
  - Error handling
- [x] Handler ([internal/modules/crm/lead_sources/handler/lead_source.handler.go](internal/modules/crm/lead_sources/handler/lead_source.handler.go))
  - 6 endpoints with Swagger docs
- [x] Main module ([internal/modules/crm/lead_sources/main.lead_sources.go](internal/modules/crm/lead_sources/main.lead_sources.go))
  - Module initialization
  - Route setup with middleware

**Endpoints (6 total):**
- [x] `GET /crm/lead-sources` - List with pagination
- [x] `GET /crm/lead-sources/:id` - Get by ID
- [x] `POST /crm/lead-sources` - Create
- [x] `PUT /crm/lead-sources/:id` - Update
- [x] `DELETE /crm/lead-sources/:id` - Soft delete
- [x] `POST /crm/lead-sources/:id/restore` - Restore

**Status:** ✅ 100% Complete, Tested, Swagger Generated

---

## ⏳ IN PROGRESS / TODO

### 4. FASE 2: Sales Targets & Auto Reply Rules

#### Sales Targets Module (5% complete)
**Files:**
- [x] Domain model ([internal/modules/crm/sales_targets/domain/sales_target.go](internal/modules/crm/sales_targets/domain/sales_target.go))
- [ ] DTOs - `internal/modules/crm/sales_targets/dto/sales_target.dto.go`
  - [ ] CreateSalesTargetRequest
  - [ ] UpdateSalesTargetRequest
  - [ ] SalesTargetResponse
  - [ ] SalesTargetListResponse
  - [ ] SalesTargetQueryParams
- [ ] Repository - `internal/modules/crm/sales_targets/repository/sales_target.repository.go`
  - [ ] FindByID, FindAll, Count
  - [ ] FindByPeriod (year, month, target_type, user_id)
  - [ ] Create, Update, Delete, Restore
  - [ ] UpdateAchievement (for automatic updates)
- [ ] Service - `internal/modules/crm/sales_targets/service/sales_target.service.go`
  - [ ] Business logic
  - [ ] Validation: target_type='team' → user_id must be NULL
  - [ ] Validation: target_type='individual' → user_id must be filled
  - [ ] Calculate achievement_percentage
- [ ] Handler - `internal/modules/crm/sales_targets/handler/sales_target.handler.go`
  - [ ] GetAll, GetByID, Create, Update, Delete, Restore
  - [ ] Special: `GET /sales-targets/achievement` - Achievement report
- [ ] Main module - `internal/modules/crm/sales_targets/main.sales_targets.go`

**Endpoints Needed (7 total):**
- [ ] `GET /crm/sales-targets` - List with pagination
- [ ] `GET /crm/sales-targets/:id` - Get by ID
- [ ] `GET /crm/sales-targets/achievement` - Achievement report
- [ ] `POST /crm/sales-targets` - Create
- [ ] `PUT /crm/sales-targets/:id` - Update
- [ ] `DELETE /crm/sales-targets/:id` - Soft delete
- [ ] `POST /crm/sales-targets/:id/restore` - Restore

#### Auto Reply Rules Module (0% complete)
**Files Needed (6 files):**
- [ ] Domain model - `internal/modules/crm/auto_reply_rules/domain/auto_reply_rule.go`
- [ ] DTOs - `internal/modules/crm/auto_reply_rules/dto/auto_reply_rule.dto.go`
  - [ ] CreateAutoReplyRuleRequest
  - [ ] UpdateAutoReplyRuleRequest
  - [ ] AutoReplyRuleResponse
  - [ ] AutoReplyRuleListResponse
  - [ ] AutoReplyRuleQueryParams
- [ ] Repository - `internal/modules/crm/auto_reply_rules/repository/auto_reply_rule.repository.go`
  - [ ] FindByID, FindAll, Count
  - [ ] FindActive (is_enabled=true, order by priority)
  - [ ] Create, Update, Delete, Restore
  - [ ] UpdateUsageCount
- [ ] Service - `internal/modules/crm/auto_reply_rules/service/auto_reply_rule.service.go`
  - [ ] Business logic
  - [ ] JSONB handling for `conditions` field
  - [ ] Rule matching logic
- [ ] Handler - `internal/modules/crm/auto_reply_rules/handler/auto_reply_rule.handler.go`
  - [ ] GetAll, GetByID, Create, Update, Delete, Restore
  - [ ] Special: `POST /:id/enable`, `POST /:id/disable`
  - [ ] Special: `GET /active` - Get active rules
- [ ] Main module - `internal/modules/crm/auto_reply_rules/main.auto_reply_rules.go`

**Endpoints Needed (9 total):**
- [ ] `GET /crm/auto-reply-rules` - List with pagination
- [ ] `GET /crm/auto-reply-rules/:id` - Get by ID
- [ ] `GET /crm/auto-reply-rules/active` - Get active rules
- [ ] `POST /crm/auto-reply-rules` - Create
- [ ] `PUT /crm/auto-reply-rules/:id` - Update
- [ ] `POST /crm/auto-reply-rules/:id/enable` - Enable rule
- [ ] `POST /crm/auto-reply-rules/:id/disable` - Disable rule
- [ ] `DELETE /crm/auto-reply-rules/:id` - Soft delete
- [ ] `POST /crm/auto-reply-rules/:id/restore` - Restore

---

### 5. FASE 3: Leads Module (Core CRM)

#### Basic CRUD
**Files Needed (6 files):**
- [ ] Domain - `internal/modules/crm/leads/domain/lead.go`
- [ ] DTOs - `internal/modules/crm/leads/dto/lead.dto.go`
- [ ] Repository - `internal/modules/crm/leads/repository/lead.repository.go`
- [ ] Service - `internal/modules/crm/leads/service/lead.service.go`
- [ ] Handler - `internal/modules/crm/leads/handler/lead.handler.go`
- [ ] Main module - `internal/modules/crm/leads/main.leads.go`

**Standard Endpoints (6 total):**
- [ ] `GET /crm/leads` - List with filters
- [ ] `GET /crm/leads/:id` - Get by ID
- [ ] `POST /crm/leads` - Create
- [ ] `PUT /crm/leads/:id` - Update
- [ ] `DELETE /crm/leads/:id` - Soft delete
- [ ] `POST /crm/leads/:id/restore` - Restore

**Advanced Endpoints (6 total):**
- [ ] `PUT /crm/leads/:id/assign` - Assign to sales person
- [ ] `PUT /crm/leads/:id/category` - Change category (hot/warm/cold)
- [ ] `PUT /crm/leads/:id/status` - Change status
- [ ] `POST /crm/leads/:id/convert` - Convert to deal
- [ ] `GET /crm/leads/my-leads` - My assigned leads
- [ ] `GET /crm/leads/follow-ups-today` - Today's follow-ups

**Special Features:**
- [ ] JSONB handling for `ai_highlights`
- [ ] Query filters: category, status, assigned_to, source_id, next_follow_up

---

### 6. FASE 4: Deals Module (Revenue Tracking)

**Files Needed (6 files):**
- [ ] Domain, DTO, Repository, Service, Handler, Main module

**Standard Endpoints (6 total):**
- [ ] CRUD endpoints (GetAll, GetByID, Create, Update, Delete, Restore)

**Report Endpoints (4 total):**
- [ ] `GET /crm/deals/revenue` - Revenue summary
- [ ] `GET /crm/deals/leaderboard` - Top performers
- [ ] `GET /crm/deals/won` - Won deals only
- [ ] `GET /crm/deals/lost` - Lost deals with reasons

**Business Logic:**
- [ ] Auto-calculate commission: `commission = deal_value * commission_percentage / 100`
- [ ] Validate status: only 'won' or 'lost'

---

### 7. FASE 5: Chats & Messages Module

#### Chats Module
**Files Needed (6 files):**
- [ ] Domain, DTO, Repository, Service, Handler, Main module

**Endpoints (9 total):**
- [ ] Standard CRUD (6 endpoints)
- [ ] `GET /crm/chats/inbox` - Sales inbox with unread count
- [ ] `PUT /crm/chats/:id/assign` - Assign to sales
- [ ] `POST /crm/chats/:id/mark-read` - Mark all as read

**Special Features:**
- [ ] JSONB handling for `ai_insights`
- [ ] Array handling for `tags`
- [ ] Real-time unread count

#### Chat Messages Module
**Files Needed (6 files):**
- [ ] Domain, DTO, Repository, Service, Handler, Main module

**Endpoints (4 total):**
- [ ] `GET /crm/chats/:chat_id/messages` - Get conversation
- [ ] `POST /crm/chats/:chat_id/messages` - Send message
- [ ] `PUT /crm/messages/:id/mark-read` - Mark as read
- [ ] Support media files (images, videos, files)

**Special Features:**
- [ ] JSONB handling for `metadata`
- [ ] Media upload handling
- [ ] Pagination for messages

---

### 8. FASE 6: Activities Module

**Files Needed (6 files):**
- [ ] Domain, DTO, Repository, Service, Handler, Main module

**Endpoints (8 total):**
- [ ] Standard CRUD (6 endpoints)
- [ ] `GET /crm/activities/timeline` - Activity timeline
- [ ] `POST /crm/activities/:id/complete` - Mark as completed

**Special Features:**
- [ ] JSONB handling for `metadata`
- [ ] Support multiple entity references (lead_id, deal_id, chat_id)
- [ ] Reminder system

---

### 9. FASE 7: Sales Performance & Analytics

#### Sales Performance Module
**Files Needed (6 files):**
- [ ] Domain, DTO, Repository, Service, Handler, Main module

**Endpoints (5 total):**
- [ ] `GET /crm/performance` - List performance
- [ ] `GET /crm/performance/me` - Current user performance
- [ ] `GET /crm/performance/team` - Team comparison
- [ ] `GET /crm/performance/leaderboard` - Rankings
- [ ] Background job for auto-calculation

#### Dashboard & Reports
**Endpoints (3 total):**
- [ ] `GET /crm/dashboard/stats` - Use materialized view
- [ ] `GET /crm/reports/revenue-by-source` - Use materialized view
- [ ] `POST /crm/reports/refresh` - Refresh materialized views

---

### 10. FASE 8: Integration & Router Setup

#### Main CRM Module
**File:** `internal/modules/crm/main.crm.go`
- [ ] Initialize all submodules
- [ ] Setup routing for all endpoints
- [ ] Apply middleware (JWTAuth, RequirePermission, CompanyContext)
- [ ] Permission mapping

#### Router Integration
**File:** `internal/router/router.go`
- [ ] Uncomment CRM imports
- [ ] Add CRM module initialization
- [ ] Setup `/crm/v1` route group

---

### 11. FASE 9: Testing & Documentation

- [ ] Add swagger comments to all new handlers
- [ ] Regenerate swagger docs
- [ ] Test all endpoints via Swagger UI
- [ ] Test validation rules
- [ ] Test business rules (status transitions, permissions)
- [ ] Test soft delete & restore
- [ ] Test pagination & filtering

#### Seeder (Optional)
- [ ] Create sample data seeder
- [ ] `internal/database/seeders/crm/001_lead_sources.sql`
- [ ] `internal/database/seeders/crm/002_sample_leads.sql`

---

### 12. FASE 10: Advanced Features (Optional/Future)

#### Bulk Operations
- [ ] `POST /crm/leads/bulk-assign` - Bulk assign
- [ ] `POST /crm/leads/bulk-delete` - Bulk delete
- [ ] `POST /crm/leads/bulk-update-category` - Bulk update

#### Export Features
- [ ] `GET /crm/leads/export` - Export to CSV/Excel
- [ ] `GET /crm/deals/export` - Export deals
- [ ] `GET /crm/performance/export` - Export performance

#### Real-time Features
- [ ] WebSocket for chat messages
- [ ] Real-time notifications for new chats
- [ ] Live dashboard updates

---

## 📊 Progress Statistics

### Overall Progress: **~15%**

| Phase | Module | Progress | Files | Endpoints |
|-------|--------|----------|-------|-----------|
| ✅ Infrastructure | Multi-Tenancy | 100% | 5 modified | - |
| ✅ Migration | CRM Schema | 100% | 2 files | - |
| ✅ FASE 1 | Lead Sources | 100% | 6 files | 6 endpoints |
| ⏳ FASE 2 | Sales Targets | 5% | 1/6 files | 0/7 endpoints |
| ⏳ FASE 2 | Auto Reply Rules | 0% | 0/6 files | 0/9 endpoints |
| ⏳ FASE 3 | Leads | 0% | 0/6 files | 0/12 endpoints |
| ⏳ FASE 4 | Deals | 0% | 0/6 files | 0/10 endpoints |
| ⏳ FASE 5 | Chats | 0% | 0/6 files | 0/9 endpoints |
| ⏳ FASE 5 | Chat Messages | 0% | 0/6 files | 0/4 endpoints |
| ⏳ FASE 6 | Activities | 0% | 0/6 files | 0/8 endpoints |
| ⏳ FASE 7 | Sales Performance | 0% | 0/6 files | 0/5 endpoints |
| ⏳ FASE 7 | Dashboard | 0% | - | 0/3 endpoints |
| ⏳ FASE 8 | Integration | 0% | 0/2 files | - |
| ⏳ FASE 9 | Testing | 0% | - | - |

**Total Estimated:**
- **Files to create:** ~60-70 files
- **Endpoints to implement:** ~100+ endpoints
- **Time estimate:** 3-5 days full development

---

## 🔧 Technical Notes

### Multi-Tenancy Pattern
```go
// In every CRM handler
companyID := middleware.GetCompanyID(c)  // From JWT token
userID := middleware.MustGetUserID(c)    // From JWT token

// All queries must filter by company_id
WHERE company_id = $1 AND deleted_at IS NULL
```

### Middleware Stack
```go
// Required for all CRM routes
router.Use(middleware.JWTAuth())        // Authenticate
router.Use(middleware.CompanyContext()) // Validate company
router.Use(middleware.RequirePermission("resource:action"))
```

### Error Handling Pattern
```go
var (
    ErrResourceNotFound = errors.New("resource not found")
    ErrCodeAlreadyExists = errors.New("code already exists")
)

// In handler
if errors.Is(err, service.ErrResourceNotFound) {
    response.Error(c, http.StatusNotFound, "Resource not found", "")
    return
}
```

### Soft Delete Pattern
```sql
-- In all tables
deleted_at TIMESTAMP,
deleted_by UUID,

-- In queries
WHERE deleted_at IS NULL

-- Restore
UPDATE table SET deleted_at = NULL, deleted_by = NULL
WHERE id = $1 AND deleted_at IS NOT NULL
```

---

## 🚀 How to Continue

### Start from FASE 2:
1. Copy pattern from Lead Sources module
2. Adjust domain model for Sales Targets
3. Implement DTOs with proper validations
4. Create repository with business-specific queries
5. Implement service with business rules
6. Create handler with Swagger docs
7. Setup routes in main module
8. Test endpoints

### Commands:
```bash
# Run migration
make migrate-up

# Generate swagger
make swagger-gen

# Run dev server
make dev

# Build
make build

# Test endpoint
curl http://localhost:8080/crm/v1/lead-sources \
  -H "Authorization: Bearer <token>"
```

---

## 📝 Important Reminders

1. **Always filter by company_id** in all queries
2. **Use MustGetUserID()** in protected handlers
3. **Validate business rules** in service layer
4. **Add proper error handling** with custom errors
5. **Write Swagger comments** for all endpoints
6. **Test soft delete & restore** functionality
7. **Use JSONB** for flexible data (ai_insights, conditions, metadata)
8. **Use Arrays** for tags
9. **No Foreign Key constraints** - validate in application
10. **Materialized views** need manual refresh

---

## 🔗 References

- Migration: [internal/database/migrations/crm/000006_crm_tables.up.sql](internal/database/migrations/crm/000006_crm_tables.up.sql)
- Lead Sources Example: [internal/modules/crm/lead_sources/](internal/modules/crm/lead_sources/)
- Middleware: [internal/middleware/](internal/middleware/)
- JWT: [pkg/jwt/](pkg/jwt/)

---

**Next Session:** Start with FASE 2 - Sales Targets & Auto Reply Rules

**Priority:** Medium-High (Core CRM functionality)

**Blockers:** None - All infrastructure ready
