# PingMate — Architecture Document

> **Version:** 1.1
> **Status:** Active
> **Scope:** V1 — Single-service, single-region, developer-local to production-ready

---

## 1. Overview

PingMate is a **monolithic REST API** written in Go. It handles user authentication, reminder management, and scheduled reminder delivery via a background goroutine scheduler — all within a single deployable binary.

The design intentionally avoids distributed complexity. There is no message broker, no microservice mesh, no external job runner. The goal is a system that is **fully understandable, debuggable, and deployable by a single developer** while still being production-honest in its patterns.

---

## 2. System Context

```
┌─────────────────────────────────────────────────────────┐
│                        CLIENT                           │
│         (Mobile App / Web App / curl / Postman)         │
└────────────────────────┬────────────────────────────────┘
                         │ HTTP/REST
                         ▼
┌─────────────────────────────────────────────────────────┐
│                    PINGMATE API                         │
│                  (Gin HTTP Server)                      │
│                                                         │
│  ┌─────────────┐   ┌──────────────┐   ┌─────────────┐  │
│  │  Auth Layer │   │ Reminder API │   │  Scheduler  │  │
│  │  (JWT/bcrypt│   │  (CRUD)      │   │  (goroutine)│  │
│  └──────┬──────┘   └──────┬───────┘   └──────┬──────┘  │
│         │                 │                   │         │
└─────────┼─────────────────┼───────────────────┼─────────┘
          │                 │                   │
    ┌─────▼─────┐     ┌─────▼──────┐     ┌─────▼──────┐
    │   Redis   │     │ PostgreSQL │     │ PostgreSQL │
    │ (JWT      │     │ (users,    │     │ (reminders,│
    │ blacklist)│     │  reminders)│     │  logs)     │
    └───────────┘     └────────────┘     └────────────┘
```

All three data interactions go to PostgreSQL. Redis is exclusively used for JWT blacklisting on logout.

---

## 3. Layer Architecture

PingMate uses **dependency inversion across all three layers**. Each layer depends on the interface of the layer below it — never the concrete type. Concrete implementations are wired together only in `cmd/server/main.go`.

```
HTTP Request
     │
     ▼
┌────────────┐
│ Middleware │  ← JWT validation. Attaches user_id to Gin context.
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
svc     := services.NewAuthService(repo)
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
| `db.go` | Opens and pings PostgreSQL. Sets connection pool limits. Exposes `config.DB`. |
| `redis.go` | Opens and pings Redis. Exposes `config.Redis`. |

**Design decision:** All config is loaded once at startup into a package-level struct. No `os.Getenv` scattered through the codebase.

---

### 4.2 Models (`internal/models/`)

Pure data structs. No methods, no logic, no imports from other internal packages. Every other layer imports from here — nothing imports from above.

| File | Contents |
|---|---|
| `models/user.go` | `User` struct matching the `users` table |
| `models/reminder.go` | `Reminder` struct, `NotificationLog` struct, `RecurrenceType` and `LogStatus` type aliases |

---

### 4.3 Repository (`internal/repository/`)

Database access only. Each file defines an interface and its concrete PostgreSQL implementation. No business logic lives here — only SQL.

| File | Interface | Responsibility |
|---|---|---|
| `repository/user_repository.go` | `UserRepository` | `CreateUser`, `FindByEmail`, `FindByID` |
| `repository/reminder_repository.go` | `ReminderRepository` | `Create`, `FindAll`, `FindByID`, `Update`, `Delete`, `FindDueReminders` |

The service layer only ever calls the `UserRepository` or `ReminderRepository` interface — never the concrete struct.

#### Repository query strategy

| Operation | Query |
|---|---|
| Create | `INSERT` with `RETURNING id` |
| FindAll | `SELECT WHERE user_id = $1 ORDER BY scheduled_at ASC` |
| FindByID | `SELECT WHERE id = $1 AND user_id = $2` — ownership enforced at DB level |
| Update | `UPDATE WHERE id = $1 AND user_id = $2` |
| Delete | `DELETE WHERE id = $1 AND user_id = $2` |
| FindDueReminders | `SELECT WHERE scheduled_at <= NOW() AND is_active = TRUE` |

The `AND user_id` clause on every mutating query means even if an ID is guessed, a different user's data is never touched.

---

### 4.4 Services (`internal/services/`)

Business logic layer. Calls repository interfaces, enforces rules, returns domain models or errors. Has no knowledge of Gin, HTTP status codes, or `sql.DB`.

| File | Interface | Responsibility |
|---|---|---|
| `services/auth_service.go` | `AuthService` | Register (hash + store), Login (verify + issue JWT), Logout (blacklist token in Redis) |
| `services/reminder_service.go` | `ReminderService` | Create, List, Get, Update, Delete — all scoped by `user_id` from JWT context |

#### Auth service flows

**Register:**
```
ValidateInput → FindByEmail (conflict check) → bcrypt hash → CreateUser → return user
```

**Login:**
```
FindByEmail → bcrypt.CompareHashAndPassword → GenerateJWT → return token
```

**Logout:**
```
ParseJWT claims → extract exp → Redis SET token with TTL = remaining lifetime
```

---

### 4.5 Handlers (`internal/handlers/`)

HTTP layer only. Parses and validates incoming requests, calls the service interface, and writes JSON responses. Has no knowledge of SQL, bcrypt, or Redis.

| File | Responsibility |
|---|---|
| `handlers/auth_handler.go` | `POST /auth/register`, `POST /auth/login`, `POST /auth/logout` |
| `handlers/reminder_handler.go` | `POST`, `GET`, `GET/:id`, `PUT/:id`, `DELETE/:id` on `/reminders` |

`user_id` is always read from the Gin context set by middleware — never from the request body.

---

### 4.6 Middleware (`internal/middleware/`)

Sits between the Gin router and all protected handlers.

| File | Responsibility |
|---|---|
| `middleware/auth_middleware.go` | Extract Bearer token → verify signature + expiry → check Redis blacklist → attach `user_id` to context → `c.Next()` |

**JWT Validation flow:**
```
Every protected route:
  │
  ├── Extract Bearer token from Authorization header
  ├── Verify signature + expiry (golang-jwt)
  ├── Check Redis blacklist → reject if found
  ├── Attach user_id to Gin context
  └── c.Next()
```

---

### 4.7 Scheduler (`internal/scheduler/`)

Runs as a long-running goroutine launched at server startup. Receives a `ReminderRepository` interface — no direct `sql.DB` access.

```
scheduler.Start(repo ReminderRepository)
  │
  └── goroutine:
        loop every 30 seconds:
          │
          ├── repo.FindDueReminders()
          │
          ├── for each reminder:
          │     ├── Log the trigger
          │     ├── repo.CreateNotificationLog(status: sent/failed)
          │     └── if recurrence != 'none':
          │           repo.Update(scheduled_at = next occurrence)
          │         else:
          │           repo.Update(is_active = false)
          │
          └── sleep(30s)
```

**Why polling and not a push model?**
For V1 scope, a polling loop is simpler, has zero external dependencies, and is accurate to within 30 seconds — sufficient for reminders. A push model (e.g. pg_notify or a job queue) would be the natural V2 upgrade.

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
| Token claims | `user_id`, `email`, `exp` |
| Token lifetime | Configurable via `JWT_EXPIRY_HOURS` (default 72h) |
| Logout / invalidation | Token stored in Redis with TTL = remaining lifetime |
| Route protection | Gin middleware — rejects missing, invalid, or blacklisted tokens |
| Ownership enforcement | All DB queries include `user_id` in WHERE clause |

---

## 7. Error Handling Strategy

All error responses follow a consistent envelope:

```json
{
  "error": "human-readable message"
}
```

HTTP status codes are used semantically:

| Status | Usage |
|---|---|
| `200` | Success |
| `201` | Resource created |
| `400` | Bad request / validation failure |
| `401` | Missing or invalid token |
| `403` | Valid token, wrong owner |
| `404` | Resource not found |
| `409` | Conflict (e.g. email already registered) |
| `500` | Internal server error |

Errors from the repository layer are never leaked raw to the client. The service and handler layers translate DB errors into appropriate HTTP responses.

---

## 8. Infrastructure

### Docker Compose (local dev)

```
┌─────────────────────────────┐
│     docker-compose.yml      │
│                             │
│  ┌──────────┐ ┌──────────┐  │
│  │ postgres │ │  redis   │  │
│  │  :5432   │ │  :6379   │  │
│  └──────────┘ └──────────┘  │
└─────────────────────────────┘
      ↑ Go server runs locally, connects to both
```

Migrations are mounted into `docker-entrypoint-initdb.d/` and run automatically on first container start, in filename order.

### Dockerfile (multi-stage)

```
Stage 1: golang:1.22-alpine  → compile binary
Stage 2: alpine:latest        → copy binary only
```

Final image contains only the compiled binary — no Go toolchain, no source code.

---

## 9. Sequence Diagrams

### Create Reminder (Happy Path)
```
Client       Router      Middleware      Handler        Service       Repository    PostgreSQL
  │             │              │              │              │               │              │
  │─POST /reminders──►         │              │              │               │              │
  │             │──validate JWT►              │              │               │              │
  │             │              │──attach uid─►│              │               │              │
  │             │              │              │─CreateReminder►              │              │
  │             │              │              │              │─Insert(reminder►             │
  │             │              │              │              │               │─INSERT SQL──►│
  │             │              │              │              │               │◄─id returned─│
  │             │              │              │              │◄─reminder obj─│              │
  │◄────────────────────────────────────201 + body──────────│               │              │
```

### Scheduler Tick
```
Scheduler Goroutine         Repository              PostgreSQL
        │                        │                       │
        │──FindDueReminders()───►│                       │
        │                        │──SELECT SQL──────────►│
        │                        │◄─[]Reminder───────────│
        │◄──[]Reminder───────────│                       │
        │                        │                       │
        │  for each reminder:    │                       │
        │──CreateLog()──────────►│──INSERT SQL──────────►│
        │──Update(next/inactive)─►──UPDATE SQL──────────►│
        │◄──ok───────────────────│◄─ok───────────────────│
        │                        │                       │
        │  sleep 30s → loop      │                       │
```

---

## 10. Design Decisions & Trade-offs

| Decision | Reasoning | Trade-off |
|---|---|---|
| Dependency inversion via interfaces | Handlers and services are fully testable via mocks, no layer is tightly coupled | Slightly more boilerplate than calling concrete types directly |
| `database/sql` over ORM | Full SQL control, no magic, easier to reason about queries | More boilerplate than GORM |
| Goroutine scheduler over cron/queue | Zero external dependencies, simple to understand | ~30s delivery variance, not horizontally scalable |
| Redis for JWT blacklist | Stateless JWT + stateful logout without DB writes on every request | Adds Redis as a dependency |
| PostgreSQL ENUMs | Type safety enforced at DB level | Requires migration to add new values |
| Monolith | Simpler deploy, single process, ideal for V1 scope | Would need extraction if scaled to multiple services |
| `uuid` as primary keys | No sequential ID guessing, safe for public APIs | Slightly larger index size vs int |

---

## 11. Future Improvements (Post V1)

- **Webhook delivery** — HTTP POST to a user-configured URL when a reminder fires
- **Push notifications** — Firebase/APNS integration
- **pg_notify** — Replace polling with Postgres LISTEN/NOTIFY for instant delivery
- **Rate limiting** — Gin middleware with Redis token bucket
- **Refresh tokens** — Short-lived access tokens + long-lived refresh tokens
- **Pagination** — Cursor-based pagination on reminder list
- **Metrics** — Prometheus endpoint for scheduler health