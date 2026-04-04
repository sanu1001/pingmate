# PingMate — Project Document

> **Type:** Backend REST API  
> **Language:** Go  
> **Status:** Active Development (V1)

---

## 1. The Problem

Scheduled reminders are a deceptively common requirement in software. Nearly every productivity app, notification system, or workflow tool needs some version of: *"at this time, do this thing."*

Most developers face the same frustrating choice:

**Option A — Build it from scratch**  
Every time. Reinvent the schema, the auth layer, the scheduler logic. No standards, no reusability.

**Option B — Reach for a SaaS platform**  
Lock in to a third-party service with opaque pricing, rate limits, and no control over your data.

**Option C — Overkill architecture**  
Stand up Kafka, RabbitMQ, or a full job queue system for what is fundamentally a simple polling problem.

None of these options are satisfying for a developer building something small-to-medium who just needs a **reliable, controllable reminder backend**.

---

## 2. What PingMate Solves

PingMate is a **self-contained, developer-first reminder API** that you run yourself.

It exposes a clean REST interface for:
- Registering users and authenticating them securely
- Creating reminders with optional recurrence rules
- Letting a background scheduler handle delivery timing automatically
- Logging every triggered reminder for auditability

You own the data. You control the deployment. You understand every line.

---

## 3. Target Users

PingMate is designed for:

- **Backend developers** who need a reference implementation of a scheduled job system in Go
- **Teams** who want to embed a reminder service into a larger product without third-party dependencies
- **Learners** studying Go backend patterns — auth, CRUD, background workers, Docker — in one coherent project

---

## 4. Core Design Philosophy

### Simple over clever
PingMate uses a polling goroutine — not Kafka, not Redis Streams, not pg_notify. A 30-second poll is accurate enough for human-scale reminders and requires zero infrastructure beyond what's already there.

### Stateless API, stateful where it counts
JWT tokens keep the API stateless on every request. Redis adds just enough statefulness to support logout (token blacklisting) — without making every request hit a session store.

### Ownership enforced at the data layer
Every reminder query includes `AND user_id = $1`. Regardless of what the application layer does, the database never returns another user's data. This is not just a best practice — it's a security guarantee.

### Fail fast, fail loud
If `JWT_SECRET` is missing at startup, the server does not start. If the DB or Redis can't be reached, the server does not start. Silent misconfiguration is more dangerous than an immediate crash.

### Explicit over magic
No ORM. No reflection-based query builders. SQL is written, read, and understood directly. What the query does is what the code says it does.

---

## 5. Architecture Summary

PingMate is a **monolith** — one binary, one process, one deployment unit.

Inside it, concerns are cleanly separated:

```
HTTP Layer (Gin)
    ↓
Handler     — request parsing, input validation, response formatting
    ↓
Service     — business logic, rules enforcement
    ↓
Repository  — database access, SQL queries only
    ↓
PostgreSQL  — persistent data storage
```

Redis sits adjacent — used only by the auth middleware for blacklist lookups.

The scheduler runs as a goroutine launched at startup, independent of the HTTP server.

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

---

## 7. Security Model

| Layer | Control |
|---|---|
| Password | bcrypt (cost 12) — irreversible hash, brute-force resistant |
| Authentication | JWT HS256, signed with a server-held secret |
| Logout | Token blacklisted in Redis for remaining TTL |
| Authorization | `user_id` extracted from verified JWT, never from request body |
| Data isolation | Every query scopes by `user_id` — no cross-user leakage possible |

---

## 8. What This Project Is Not

- **Not a production notification delivery system** — PingMate logs that a reminder triggered. Actual delivery (push, email, SMS) is intentionally out of scope for V1 and would be added as a webhook/integration layer in V2.
- **Not horizontally scalable in V1** — The goroutine scheduler assumes a single running instance. Multi-instance deployment would require a distributed lock or a proper job queue. That's a deliberate trade-off, not an oversight.
- **Not an ORM-based project** — If you're looking for GORM usage, PingMate won't show you that. It shows you what lies beneath.

---

## 9. Build Phases

### Phase 1 — Foundation ✅
Project scaffold, Go module init, environment config, PostgreSQL + Redis connections, SQL migrations, Gin server, Docker Compose.

### Phase 2 — Core API
Auth handlers (register, login, logout), JWT middleware with Redis blacklist check, full Reminder CRUD with input validation and user-scoped access control.

### Phase 3 — Scheduler
Background goroutine worker, due-reminder polling query, notification log writes, recurrence advancement logic.

### Phase 4 — Polish
Swagger/OpenAPI documentation via swaggo, production Dockerfile, environment variable documentation, README finalization.

---

## 10. Repository Quality Standards

This project is built for a professional portfolio. Every commit should be:

- **Atomic** — one logical change per commit
- **Conventionally named** — `feat:`, `fix:`, `chore:`, `docs:`
- **Buildable** — `go build ./...` passes at every commit

Branch strategy: `main` is always deployable. Feature work happens on `feature/` branches, merged via PR.