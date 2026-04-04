# PingMate — Architecture Document

> **Version:** 1.0  
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

PingMate uses a clean **3-layer architecture** inside each domain package:

```
HTTP Request
     │
     ▼
┌──────────┐
│ Handler  │  ← Gin handler. Parses request, validates input, calls service, writes response.
└────┬─────┘
     │
     ▼
┌──────────┐
│ Service  │  ← Business logic. Enforces rules, orchestrates repository calls.
└────┬─────┘
     │
     ▼
┌────────────┐
│ Repository │  ← DB access only. Raw SQL via database/sql. No business logic here.
└────────────┘
```

This separation means:
- Handlers never touch `sql.DB` directly
- Repositories never make auth decisions
- Services are testable in isolation

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

### 4.2 Auth (`internal/auth/`)

#### Flow — Register
```
POST /auth/register
  │
  ├── Validate input (email format, password length)
  ├── Check email uniqueness
  ├── Hash password with bcrypt (cost 12)
  ├── Insert user into DB
  └── Return 201 Created
```

#### Flow — Login
```
POST /auth/login
  │
  ├── Find user by email
  ├── Compare bcrypt hash
  ├── Generate JWT (claims: user_id, email, exp)
  └── Return token
```

#### Flow — Logout
```
POST /auth/logout   [JWT required]
  │
  ├── Extract token from Authorization header
  ├── Parse expiry from JWT claims
  ├── SET token in Redis with TTL = remaining JWT lifetime
  └── Return 200 OK
  
  (All subsequent requests with this token fail middleware check)
```

#### Middleware — JWT Validation
```
Every protected route:
  │
  ├── Extract Bearer token from header
  ├── Verify signature + expiry (golang-jwt)
  ├── Check Redis blacklist → reject if present
  ├── Attach user_id to Gin context
  └── call c.Next()
```

---

### 4.3 Reminder (`internal/reminder/`)

All reminder routes are **user-scoped**. The `user_id` is extracted from the JWT context, not from the request body — preventing any user from accessing another user's reminders.

#### Repository queries

| Operation | Query strategy |
|---|---|
| Create | `INSERT` with `RETURNING id` |
| List | `SELECT WHERE user_id = $1 ORDER BY scheduled_at ASC` |
| Get | `SELECT WHERE id = $1 AND user_id = $2` (ownership enforced at DB level) |
| Update | `UPDATE WHERE id = $1 AND user_id = $2` |
| Delete | `DELETE WHERE id = $1 AND user_id = $2` |

The `AND user_id` clause on every mutating query means even if an ID is guessed, a different user's data is never touched.

---

### 4.4 Scheduler (`internal/scheduler/`)

The scheduler runs as a **long-running goroutine** launched at server startup.

```
scheduler.Start()
  │
  └── goroutine:
        loop every 30 seconds:
          │
          ├── SELECT reminders WHERE scheduled_at <= NOW()
          │     AND is_active = TRUE
          │
          ├── For each due reminder:
          │     ├── "Trigger" it (log, future: call webhook/push)
          │     ├── INSERT into notification_logs (status: sent/failed)
          │     └── If recurrence != 'none':
          │           UPDATE scheduled_at = next occurrence
          │         Else:
          │           UPDATE is_active = FALSE
          │
          └── Sleep(30s)
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

Errors from the repository layer are never leaked raw to the client. Service and handler layers translate DB errors into appropriate HTTP responses.

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
Client          Gin Router       Middleware        Handler          Service          Repository       PostgreSQL
  │                │                 │                │                │                 │                │
  │──POST /reminders─►               │                │                │                 │                │
  │                │──validate JWT──►│                │                │                 │                │
  │                │                 │──attach user──►│                │                 │                │
  │                │                 │                │──CreateReminder►│                 │                │
  │                │                 │                │                │──Insert(reminder)►               │
  │                │                 │                │                │                 │──INSERT SQL────►│
  │                │                 │                │                │                 │◄──id returned───│
  │                │                 │                │                │◄──reminder obj───│                │
  │                │◄────────────────────────────────────201 + body────│                 │                │
  │◄──201 Created───│                 │                │                │                 │                │
```

### Scheduler Tick
```
Scheduler Goroutine              PostgreSQL
        │                             │
        │──SELECT due reminders──────►│
        │◄──[]reminder────────────────│
        │                             │
        │  for each reminder:         │
        │──INSERT notification_log───►│
        │──UPDATE scheduled_at / is_active►│
        │◄──ok────────────────────────│
        │                             │
        │  sleep 30s                  │
        │  (loop)                     │
```

---

## 10. Design Decisions & Trade-offs

| Decision | Reasoning | Trade-off |
|---|---|---|
| `database/sql` over ORM | Full SQL control, no magic, easier to reason about queries | More boilerplate |
| Goroutine scheduler over cron/queue | Zero external dependencies, simple to understand | ~30s delivery variance, not horizontally scalable |
| Redis for JWT blacklist | Stateless JWT + stateful logout without DB writes on every request | Adds Redis as a dependency |
| PostgreSQL ENUMs | Type safety enforced at DB level | Requires migration to add new values |
| Monolith | Simpler deploy, single process, ideal for V1 scope | Would need extraction if scaled to multiple services |
| `uuid` as primary keys | No sequential ID guessing, safe for public APIs | Slightly larger index size vs. int |

---

## 11. Future Improvements (Post V1)

- **Webhook delivery** — HTTP POST to a user-configured URL when a reminder fires
- **Push notifications** — Firebase/APNS integration
- **pg_notify** — Replace polling with Postgres LISTEN/NOTIFY for instant delivery
- **Rate limiting** — Gin middleware with Redis token bucket
- **Refresh tokens** — Short-lived access tokens + long-lived refresh tokens
- **Pagination** — Cursor-based pagination on reminder list
- **Metrics** — Prometheus endpoint for scheduler health