# PingMate — Architecture Document

> **Version:** 2.0
> **Status:** V1 Complete
> **Scope:** Single-service, single-region, developer-local to production-ready

---

## 1. Overview

PingMate is a **monolithic REST API** written in Go. It handles user authentication, reminder management, and scheduled reminder delivery via a background goroutine scheduler — all within a single deployable binary.

The design intentionally avoids distributed complexity. There is no message broker, no microservice mesh, no external job runner. The goal is a system that is **fully understandable, debuggable, and deployable by a single developer** while still being production-honest in its patterns.

---

## 2. System Context

```
┌─────────────────────────────────────────────────────────┐
│                        CLIENT                           │
│         (Mobile App / Web App / curl / Bruno)           │
└────────────────────────┬────────────────────────────────┘
                         │ HTTP/REST
                         ▼
┌─────────────────────────────────────────────────────────┐
│                    PINGMATE API                         │
│                  (Gin HTTP Server)                      │
│                                                         │
│  ┌─────────────┐   ┌──────────────┐   ┌─────────────┐   │
│  │ Auth Layer  │   │ Reminder API │   │  Scheduler  │   │
│  │ JWT/bcrypt  │   │ CRUD + Rate  │   │ (goroutine) │   │
│  └──────┬──────┘   └──────┬───────┘   └──────┬──────┘   │
│         │                 │                  │          │
└─────────┼─────────────────┼──────────────────┼──────────┘
          │                 │                  │
    ┌─────▼──────┐    ┌─────▼──────┐    ┌─────▼──────┐
    │   Redis    │    │ PostgreSQL │    │ PostgreSQL │
    │ blacklist  │    │ users,     │    │ reminders, │
    │ rate ctr.  │    │ reminders  │    │ logs       │
    └────────────┘    └────────────┘    └────────────┘
```

PostgreSQL stores all persistent data. Redis is used exclusively for JWT blacklisting and rate limiting counters.

---

## 3. Layer Architecture

PingMate uses **dependency inversion across all layers**. Each layer depends on the interface of the layer below it — never the concrete type. Concrete implementations are wired together only in `cmd/server/main.go`.

```
HTTP Request
     │
     ▼
┌────────────┐
│ Middleware │  ← JWT validation + rate limiting. Attaches user_id to context.
└─────┬──────┘
      │
      ▼
┌──────────┐
│ Handler  │  ← Parses request, calls ServiceInterface, writes response.
└────┬─────┘    Knows nothing about repositories or SQL.
     │  (via interface)
     ▼
┌──────────┐
│ Service  │  ← Business logic. Calls RepositoryInterface.
└────┬─────┘    Knows nothing about Gin, HTTP, or sql.DB.
     │  (via interface)
     ▼
┌────────────┐
│ Repository │  ← SQL queries only. Returns domain models.
└────────────┘    Knows nothing about business rules or HTTP.
```

**Wiring happens only in `main.go`:**
```go
repo    := repository.NewUserRepo(config.DB)
svc     := services.NewAuthService(repo, config.Redis)
handler := handlers.NewAuthHandler(svc)
```

This means:
- Handlers are testable by mocking the service interface
- Services are testable by mocking the repository interface
- No circular imports — dependency flows strictly downward
- Swapping a Postgres repository for an in-memory one requires zero changes outside `main.go`

---

## 4. Component Breakdown

### 4.1 Config (`config/`)

| File | Responsibility |
|---|---|
| `config.go` | Loads `.env` via godotenv. Exposes a single `App` struct. Fails fast if required vars are missing. |
| `db.go` | Opens and pings PostgreSQL. Sets connection pool limits (25 max open, 10 idle). Exposes `config.DB`. |
| `redis.go` | Opens and pings Redis. Exposes `config.Redis`. |

All config is loaded once at startup into a package-level struct. No `os.Getenv` scattered through the codebase.

---

### 4.2 Models (`internal/models/`)

Pure data structs. No methods, no logic, no imports from other internal packages. Every other layer imports from here — nothing imports from above.

| File | Contents |
|---|---|
| `models/user.go` | `User`, `RegisterRequest`, `LoginRequest`, `AuthResponse` |
| `models/reminder.go` | `Reminder`, `NotificationLog`, `RecurrenceType`, `LogStatus`, request DTOs |

`User.Password` is tagged with `json:"-"` to guarantee the password hash never leaks in any JSON response, even accidentally.

---

### 4.3 Repository (`internal/repository/`)

Database access only. Each file defines an interface and its concrete PostgreSQL implementation.

| File | Interface | Methods |
|---|---|---|
| `user_repository.go` | `UserRepository` | `CreateUser`, `FindByEmail`, `FindByID` |
| `reminder_repository.go` | `ReminderRepository` | `Create`, `FindAll`, `FindByID`, `Update`, `Delete`, `FindDueReminders`, `CreateLog` |

The service layer only ever calls these interfaces — never the concrete struct.

#### Repository query strategy

| Operation | Query |
|---|---|
| Create | `INSERT` with `RETURNING id, created_at` |
| FindAll | `SELECT WHERE user_id = $1 ORDER BY scheduled_at ASC` |
| FindByID | `SELECT WHERE id = $1 AND user_id = $2` — ownership enforced at DB level |
| Update | `UPDATE WHERE id = $1 AND user_id = $2` |
| Delete | `DELETE WHERE id = $1 AND user_id = $2` |
| FindDueReminders | `SELECT WHERE scheduled_at <= NOW() AND is_active = TRUE` |

**Defensive UUID handling:** `FindByID` and `Delete` catch PostgreSQL's `invalid input syntax for type uuid` error and treat it as "not found" rather than a server error. This means invalid IDs return 404 to the client instead of 500.

---

### 4.4 Services (`internal/services/`)

Business logic layer. Calls repository interfaces, enforces rules, returns domain models or typed errors.

| File | Interface | Responsibility |
|---|---|---|
| `auth_service.go` | `AuthService` | Register, Login, Logout, IsBlacklisted |
| `reminder_service.go` | `ReminderService` | Create, GetAll, GetByID, Update, Delete |

#### Auth service flows

**Register:**
```
ValidateInput → FindByEmail (uniqueness check) → bcrypt hash → CreateUser → generateToken → return
```

**Login:**
```
FindByEmail → bcrypt.CompareHashAndPassword → generateToken → return
```

**Logout:**
```
ParseJWT claims → extract expiry → Redis SET blacklist:{token} with TTL = remaining lifetime
```

**Token generation:** HS256-signed JWT with claims `user_id`, `email`, `exp`, `iat`. Expiry configurable via `JWT_EXPIRY_HOURS`.

#### Reminder service patterns

- **Ownership-first:** every method takes `userID` as the first parameter. Never trusts request bodies for ownership.
- **Merge updates:** `Update` fetches current state first, then only overwrites fields the client sent. Empty/zero values are treated as "unchanged."
- **Empty slice vs nil:** `GetAll` returns `[]Reminder{}` instead of `nil` so JSON responses are always `[]`, never `null`.

---

### 4.5 Handlers (`internal/handlers/`)

HTTP layer only. Parses and validates incoming requests, calls the service interface, and writes JSON responses.

| File | Responsibility |
|---|---|
| `auth_handler.go` | `POST /auth/register`, `POST /auth/login`, `POST /auth/logout` |
| `reminder_handler.go` | `POST`, `GET`, `GET/:id`, `PUT/:id`, `DELETE/:id` on `/reminders` |

`user_id` is always read from the Gin context (set by middleware) — never from the request body.

Each handler maps typed service errors to appropriate HTTP status codes:

```go
errors.Is(err, services.ErrEmailExists)        → 409 Conflict
errors.Is(err, services.ErrInvalidCredentials) → 401 Unauthorized
errors.Is(err, services.ErrReminderNotFound)   → 404 Not Found
default                                        → 500 Internal Server Error
```

Handlers also include swaggo annotations (`@Summary`, `@Tags`, `@Param`, etc.) that auto-generate the Swagger UI documentation.

---

### 4.6 Middleware (`internal/middleware/`)

#### Auth middleware — `auth_middleware.go`

Sits between the router and all protected handlers.

```
Every protected route:
  │
  ├── Extract Bearer token from Authorization header
  ├── Verify signature + expiry (golang-jwt)
  ├── Reject any non-HMAC signing method (algorithm confusion defence)
  ├── Check Redis blacklist → reject if found
  ├── Attach user_id and user_email to Gin context
  └── c.Next()
```

#### Rate limit middleware — `rate_limit_middleware.go`

Redis-backed **fixed window counter** scoped per user.

```
Every write request:
  │
  ├── INCR rate:{user_id}
  ├── If count == 1: EXPIRE rate:{user_id} 60s (start window)
  ├── Read TTL and set X-RateLimit-* response headers
  ├── If count > limit (30): return 429 Too Many Requests
  └── Else: c.Next()
```

Headers returned:
- `X-RateLimit-Limit` — the configured limit (30)
- `X-RateLimit-Remaining` — requests left in the current window
- `X-RateLimit-Reset` — seconds until the counter resets

Applied only to `POST`, `PUT`, `DELETE` on `/reminders`. Read routes (`GET`) are not rate limited.

**Fail-open policy:** if Redis is unreachable, the middleware allows the request through rather than blocking all users due to an infrastructure hiccup.

---

### 4.7 Scheduler (`internal/scheduler/`)

Runs as a long-running goroutine launched at server startup. Receives a `ReminderRepository` interface — no direct `sql.DB` access.

```
scheduler.Start()
  │
  ├── Run tick() immediately on boot (don't wait 30s)
  │
  └── time.NewTicker(30s) → for range ticker.C:
        │
        └── tick():
              │
              ├── repo.FindDueReminders()  (WHERE scheduled_at <= NOW() AND is_active = TRUE)
              │
              └── for each reminder:
                    │
                    ├── Log to console: "🔔 TRIGGERED: <title> for <user_id>"
                    │
                    ├── repo.CreateLog(NotificationLog{status: sent})
                    │
                    └── Advance state:
                          ├── recurrence = 'none'    → IsActive = false
                          ├── recurrence = 'daily'   → ScheduledAt.AddDate(0, 0, 1)
                          ├── recurrence = 'weekly'  → ScheduledAt.AddDate(0, 0, 7)
                          └── recurrence = 'monthly' → ScheduledAt.AddDate(0, 1, 0)
                    
                    repo.Update(reminder)
```

**Why polling and not a push model?**
For V1 scope, a polling loop is simpler, has zero external dependencies, and is accurate to within 30 seconds — sufficient for human-scale reminders. A push model (e.g. pg_notify or a job queue) would be the natural V2 upgrade.

**Calendar-aware date math:** Uses `time.AddDate(years, months, days)` for recurrence. This correctly handles month boundaries (Feb 28 → Mar 28, not "Feb 31"). Raw `time.Hour * 24 * 30` would drift.

**Failure isolation:** if logging or updating one reminder errors out, the scheduler logs a warning and continues with the rest. One bad row never blocks the batch.

---

## 5. Database Schema

### `users`
```sql
id          UUID PRIMARY KEY DEFAULT gen_random_uuid()
email       TEXT NOT NULL UNIQUE
password    TEXT NOT NULL           -- bcrypt hash
created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
```

### `reminders`
```sql
id           UUID PRIMARY KEY DEFAULT gen_random_uuid()
user_id      UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE
title        TEXT NOT NULL
description  TEXT
scheduled_at TIMESTAMPTZ NOT NULL
recurrence   ENUM('none','daily','weekly','monthly') DEFAULT 'none'
is_active    BOOLEAN NOT NULL DEFAULT TRUE
created_at   TIMESTAMPTZ NOT NULL DEFAULT NOW()
```

**Indexes:**
```sql
idx_reminders_user_id       ON reminders(user_id)
idx_reminders_scheduled_at  ON reminders(scheduled_at) WHERE is_active = TRUE
```

The partial index on `is_active = TRUE` means the scheduler query only scans active reminders — the index shrinks automatically as reminders complete.

### `notification_logs`
```sql
id           UUID PRIMARY KEY DEFAULT gen_random_uuid()
reminder_id  UUID NOT NULL REFERENCES reminders(id) ON DELETE CASCADE
triggered_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
status       ENUM('sent','failed')
```

---

## 6. Authentication & Security

| Concern | Implementation |
|---|---|
| Password storage | `bcrypt` cost factor 12 — resistant to brute force |
| Token format | JWT (HS256), signed with `JWT_SECRET` |
| Token claims | `user_id`, `email`, `exp`, `iat` |
| Token lifetime | Configurable via `JWT_EXPIRY_HOURS` (default 72h) |
| Logout / invalidation | Token stored in Redis with TTL = remaining lifetime |
| Algorithm verification | Middleware explicitly rejects non-HMAC signing methods |
| Route protection | Gin middleware — rejects missing, invalid, or blacklisted tokens |
| Ownership enforcement | All DB queries include `user_id` in WHERE clause |
| Rate limiting | 30 req/min per user on write endpoints |
| Secret leakage | All credentials in `.env`, gitignored |

---

## 7. Error Handling Strategy

All error responses follow a consistent envelope:

```json
{
  "error": "human-readable message"
}
```

HTTP status codes used semantically:

| Status | Usage |
|---|---|
| `200` | Success |
| `201` | Resource created |
| `400` | Bad request / validation failure |
| `401` | Missing, invalid, or blacklisted token |
| `404` | Resource not found (also catches invalid UUID input) |
| `409` | Conflict (e.g. email already registered) |
| `429` | Rate limit exceeded |
| `500` | Unexpected internal server error |

Errors from the repository layer are never leaked raw to the client. Service and handler layers translate DB errors into appropriate HTTP responses.

---

## 8. Infrastructure

### Docker Compose (local dev)

```
┌─────────────────────────────────┐
│       docker-compose.yml        │
│                                 │
│  ┌──────────┐  ┌──────────┐     │
│  │ postgres │  │  redis   │     │
│  │  :5433   │  │  :6379   │     │
│  └──────────┘  └──────────┘     │
└─────────────────────────────────┘
      ↑ Go server runs locally, connects to both
```

PostgreSQL is exposed on `5433` (not the default `5432`) to avoid conflicts with local PostgreSQL installations.

Migrations are mounted into `/docker-entrypoint-initdb.d` and run automatically on first container creation, in filename order.

---

## 9. Sequence Diagrams

### Create Reminder (Happy Path)
```
Client     Router    AuthMW    RateLimitMW    Handler    Service    Repo    PostgreSQL
  │          │          │           │            │          │         │          │
  │-POST--►  │          │           │            │          │         │          │
  │          │-verify JWT►          │            │          │         │          │
  │          │          │-INCR Redis►            │          │         │          │
  │          │          │           │-Create───► │          │         │          │
  │          │          │           │            │-Create──►│         │          │
  │          │          │           │            │          │-Insert─►│          │
  │          │          │           │            │          │         │-INSERT──►│
  │          │          │           │            │          │         │◄─id──────│
  │          │          │           │            │          │◄────────│          │
  │◄─────────────────────────201 + body────────────────────│          │          │
```

### Scheduler Tick
```
Scheduler          Repository         PostgreSQL
    │                  │                   │
    │-FindDueReminders►│                   │
    │                  │-SELECT───────────►│
    │                  │◄─[]Reminder───────│
    │◄─────[]Reminder──│                   │
    │                                      │
    │  for each:                           │
    │-CreateLog(sent)─►│-INSERT───────────►│
    │-Update(advance)─►│-UPDATE───────────►│
    │◄─────────ok──────│◄──ok──────────────│
    │                                      │
    │  sleep 30s → loop                    │
```

### JWT Blacklist Check
```
Request with token  →  AuthMW  →  Redis  GET blacklist:{token}
                                    │
                          ┌─────────┴─────────┐
                          │                   │
                       returns nil       returns "1"
                          │                   │
                       allow → Next()    abort 401
```

---

## 10. Design Decisions & Trade-offs

| Decision | Reasoning | Trade-off |
|---|---|---|
| Dependency inversion via interfaces | Handlers and services are fully testable via mocks, no layer is tightly coupled | Slightly more boilerplate than calling concrete types directly |
| `database/sql` over ORM | Full SQL control, no magic, easier to reason about queries | More boilerplate than GORM |
| Goroutine scheduler over cron/queue | Zero external dependencies, simple to understand | ~30s delivery variance, not horizontally scalable |
| Redis for JWT blacklist | Stateless JWT + stateful logout without DB writes on every request | Adds Redis as a dependency |
| Redis fixed-window rate limit | Simple to implement, near-zero overhead | Burst at window edges (can do 2x limit at minute boundary) |
| PostgreSQL ENUMs | Type safety enforced at DB level | Requires migration to add new values |
| Monolith | Simpler deploy, single process, ideal for V1 scope | Would need extraction if scaled to multiple services |
| UUID primary keys | No sequential ID guessing, safe for public APIs | Slightly larger index size vs int |
| Docker initdb migrations | Zero extra tooling, runs automatically on first start | New migrations require volume reset; would migrate to golang-migrate for V2 |

---

## 11. Future Improvements (Post V1)

- **Webhook delivery** — HTTP POST to a user-configured URL when a reminder fires
- **Push notifications** — Firebase Cloud Messaging / APNs integration
- **pg_notify** — Replace polling with Postgres LISTEN/NOTIFY for instant delivery
- **Refresh tokens** — Short-lived access tokens + long-lived refresh tokens
- **Pagination** — Cursor-based pagination on reminder list
- **Metrics** — Prometheus endpoint for scheduler health and request latency
- **Distributed scheduler** — Postgres advisory lock or dedicated job queue for multi-instance deployment
- **Sliding-window rate limiter** — replace fixed window with token bucket for smoother bursts
- **golang-migrate** — proper migration tool with up/down support for V2 schema changes