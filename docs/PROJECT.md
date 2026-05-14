# PingMate — Project Document

> **Type:** Backend REST API
> **Language:** Go
> **Status:** V1 Complete

---

## 1. The Problem

Scheduled reminders are a deceptively common requirement in software. Nearly every productivity app, notification system, or workflow tool needs some version of: *"at this time, do this thing."*

Most developers face the same frustrating choice:

**Option A — Build it from scratch every time.** Reinvent the schema, the auth layer, the scheduler logic. No standards, no reusability.

**Option B — Reach for a SaaS platform.** Lock in to a third-party service with opaque pricing, rate limits, and no control over your data.

**Option C — Overkill architecture.** Stand up Kafka, RabbitMQ, or a full job queue system for what is fundamentally a simple polling problem.

None of these are satisfying for a developer building something small-to-medium who just needs a **reliable, controllable reminder backend**.

---

## 2. What PingMate Solves

PingMate is a **self-contained, developer-first reminder API** that you run yourself.

It exposes a clean REST interface for:

- Registering users and authenticating them securely
- Creating reminders with optional recurrence rules
- Letting a background scheduler handle delivery timing automatically
- Logging every triggered reminder for auditability
- Rate-limiting write operations to protect the database

You own the data. You control the deployment. You understand every line.

---

## 3. Target Users

PingMate is designed for:

- **Backend developers** who need a reference implementation of a scheduled job system in Go
- **Teams** who want to embed a reminder service into a larger product without third-party dependencies
- **Learners** studying Go backend patterns — auth, CRUD, background workers, rate limiting, Docker — in one coherent project

---

## 4. Core Design Philosophy

### Simple over clever
PingMate uses a polling goroutine — not Kafka, not Redis Streams, not pg_notify. A 30-second poll is accurate enough for human-scale reminders and requires zero infrastructure beyond what's already there.

### Stateless API, stateful where it counts
JWT tokens keep the API stateless on every request. Redis adds just enough statefulness to support logout (token blacklisting) and rate limiting — without making every request hit a session store or relational DB for auth state.

### Ownership enforced at the data layer
Every reminder query includes `AND user_id = $1`. Regardless of what the application layer does, the database never returns another user's data. This is not just a best practice — it's a security guarantee.

### Fail fast, fail loud
If `JWT_SECRET` or `DATABASE_URL` are missing at startup, the server does not start. If the DB or Redis can't be reached, the server does not start. Silent misconfiguration is more dangerous than an immediate crash.

### Explicit over magic
No ORM. No reflection-based query builders. SQL is written, read, and understood directly. What the query does is what the code says it does.

### Dependency inversion across all layers
Handlers depend on service interfaces. Services depend on repository interfaces. All wiring happens exclusively in `cmd/server/main.go`. This makes every layer independently testable and swap-friendly.

---

## 5. Architecture Summary

PingMate is a **monolith** — one binary, one process, one deployment unit.

Inside it, concerns are cleanly separated:

```
HTTP Layer (Gin)
    ↓
Middleware  — JWT validation, rate limiting
    ↓
Handler     — request parsing, input validation, response formatting
    ↓
Service     — business logic, rules enforcement
    ↓
Repository  — database access, SQL queries only
    ↓
PostgreSQL  — persistent data storage
```

Redis sits adjacent — used by the auth middleware for blacklist lookups, and by the rate limit middleware for per-user request counters.

The scheduler runs as a goroutine launched at startup, independent of the HTTP server, sharing the same repository layer.

For a full technical breakdown of each component, see [`ARCHITECTURE.md`](./ARCHITECTURE.md).

---

## 6. Data Model Rationale

**Why UUIDs as primary keys?**
Sequential integer IDs are guessable. If a user knows their reminder ID is `42`, they might try `41` and `43`. UUIDs make enumeration attacks meaningless, which matters for a multi-user API.

**Why a PostgreSQL ENUM for recurrence?**
Constraining to `none | daily | weekly | monthly` at the database level means invalid recurrence values are rejected before application code even sees them. The DB is the last line of defence.

**Why a separate `notification_logs` table?**
Keeping trigger history separate from the reminder itself means:
- The reminder record stays clean and current
- You can query full delivery history for any reminder
- Failed deliveries are visible without polluting the main data

**Why a partial index on `is_active = TRUE`?**
The scheduler queries reminders by `scheduled_at <= NOW() AND is_active = TRUE` constantly. A partial index only indexes active reminders — so the index shrinks automatically as one-shot reminders complete, keeping the query fast.

---

## 7. Security Model

| Layer | Control |
|---|---|
| Password storage | bcrypt cost 12 — irreversible, brute-force resistant |
| Authentication | JWT HS256, signed with server-held secret |
| Logout | Token blacklisted in Redis with TTL = remaining lifetime |
| Algorithm confusion defence | Middleware explicitly rejects non-HMAC signing methods |
| Authorization | `user_id` extracted from verified JWT, never from request body |
| Data isolation | Every query scopes by `user_id` — no cross-user leakage possible |
| Rate limiting | Write routes capped at 30 req/min per user |
| Secret management | All credentials via `.env`, never committed |

---

## 8. What This Project Is Not

- **Not a production notification delivery system.** PingMate logs that a reminder triggered. Actual delivery (push, email, SMS) is intentionally out of scope for V1 and would be added as a webhook/integration layer in V2.
- **Not horizontally scalable in V1.** The goroutine scheduler assumes a single running instance. Multi-instance deployment would require a distributed lock or a proper job queue. That's a deliberate trade-off, not an oversight.
- **Not ORM-based.** If you're looking for GORM patterns, PingMate won't show you that. It shows you what lies beneath.

---

## 9. Build Phases — All Complete ✅

### Phase 1 — Foundation
Project scaffold, Go module init, environment config, PostgreSQL + Redis connections, SQL migrations, Gin server, Docker Compose, `/health` endpoint.

### Phase 2 — Core API
Auth handlers (register, login, logout), JWT middleware with Redis blacklist check, full Reminder CRUD with input validation and user-scoped access control.

### Phase 2.5 — Validation & Testing
End-to-end testing via Bruno API client. Repository hardening for invalid UUID input (returns 404 instead of 500). Verification of Redis blacklist behaviour and database integrity.

### Phase 2.6 — Rate Limiting
Redis-backed fixed-window rate limiter on write routes (POST/PUT/DELETE). Includes standard `X-RateLimit-*` response headers.

### Phase 3 — Scheduler
Background goroutine worker polling every 30 seconds. Notification log writes per trigger. Recurrence advancement logic (daily/weekly/monthly) and one-shot deactivation.

### Phase 4 — Polish
Swagger/OpenAPI documentation via swaggo with bearer auth integration. `GIN_MODE=release` for production. README and architecture documentation finalized.

---

## 10. Repository Quality Standards

This project is built as a portfolio piece. Every commit is:

- **Atomic** — one logical change per commit
- **Conventionally named** — `feat:`, `fix:`, `chore:`, `docs:`, `refactor:`
- **Buildable** — `go build ./...` passes at every commit

The development followed a deliberate documentation-first approach — README, architecture, and project docs were written before implementation, ensuring the code matched a thought-through design rather than the reverse.

---

## 11. Future Work (Post V1)

Documented but intentionally out of scope:

- Webhook delivery — HTTP POST to user-configured URLs on trigger
- Push notifications — Firebase Cloud Messaging / APNs
- `pg_notify` — replace polling with Postgres LISTEN/NOTIFY for instant delivery
- Refresh tokens — short-lived access + long-lived refresh pattern
- Pagination — cursor-based pagination on reminder list
- Metrics — Prometheus endpoint for scheduler health and request latency
- Multi-instance scheduler — distributed lock or dedicated job runner