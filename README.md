<div align="center">

# 🏓 PingMate

**A developer-first scheduled reminder REST API.**
Stateless auth, recurring reminders, autonomous background scheduler — built in Go.

![Go](https://img.shields.io/badge/Go-1.24-00ADD8?style=flat-square&logo=go&logoColor=white)
![PostgreSQL](https://img.shields.io/badge/PostgreSQL-17-4169E1?style=flat-square&logo=postgresql&logoColor=white)
![Redis](https://img.shields.io/badge/Redis-7-DC382D?style=flat-square&logo=redis&logoColor=white)
![Docker](https://img.shields.io/badge/Docker-Compose-2496ED?style=flat-square&logo=docker&logoColor=white)
![JWT](https://img.shields.io/badge/Auth-JWT-000000?style=flat-square&logo=jsonwebtokens&logoColor=white)
![Swagger](https://img.shields.io/badge/Docs-Swagger-85EA2D?style=flat-square&logo=swagger&logoColor=black)
![License](https://img.shields.io/badge/License-MIT-green?style=flat-square)

</div>

---

## What it does

PingMate is a backend service that lets users create reminders with optional recurrence rules (daily, weekly, monthly). A background scheduler running inside the same process polls the database, fires due reminders, logs them for audit, and advances recurring ones automatically — all without any external job queue or cron service.

It's designed as the kind of API you'd embed inside a productivity app or workflow tool when you don't want to glue together a SaaS platform or stand up Kafka.

---

## Features

- **JWT authentication** with bcrypt password hashing
- **Redis-backed token blacklist** for true logout (stateless JWT + stateful invalidation)
- **Full reminder CRUD** with user-scoped ownership enforced at the database level
- **Recurrence support** — `none`, `daily`, `weekly`, `monthly`
- **Autonomous scheduler** — goroutine-based polling, no external dependencies
- **Rate limiting** — Redis-backed fixed window counter on write routes
- **Notification logs** — every triggered reminder is logged with status
- **Swagger UI** — auto-generated interactive API documentation
- **Dockerized** — single command spins up the full infrastructure
- **Layered architecture** with strict dependency inversion across handlers, services, and repositories

---

## Tech Stack

| Layer | Technology |
|---|---|
| Language | Go 1.24 |
| Web Framework | Gin |
| Database | PostgreSQL 17 |
| Cache & Blacklist | Redis 7 |
| Auth | JWT (golang-jwt/v5) + bcrypt |
| Background Worker | Native Go goroutines |
| API Docs | Swagger via swaggo |
| Containerization | Docker + Docker Compose |
| Config | godotenv |

---

## Architecture

PingMate is a monolithic Go binary with two concurrent loops — the HTTP server and the scheduler — sharing the same repository layer and database connection pool.

```
                ┌──────────────────────────────────────────┐
                │             PingMate (Go binary)         │
                │                                          │
   HTTP ───────►│  Gin Router                              │
                │     │                                    │
                │     ▼                                    │
                │  Middleware (auth + rate limit)          │
                │     │                                    │
                │     ▼                                    │
                │  Handlers ──► Services ──► Repository ──┐│
                │                                         ││
                │  ┌────────────────────────────────────┐ ││
                │  │ Scheduler (goroutine, 30s polling) │ ││
                │  │  └──► Repository ───────────────────┤│
                │  └────────────────────────────────────┘ ││
                └─────────────────────────────────────────┼┘
                                                          │
                                  ┌───────────────────────┘
                                  ▼
                       ┌──────────────────┐   ┌─────────────────┐
                       │  PostgreSQL 17   │   │     Redis 7     │
                       │  users           │   │  JWT blacklist  │
                       │  reminders       │   │  rate counters  │
                       │  notif. logs     │   │                 │
                       └──────────────────┘   └─────────────────┘
```

Every layer depends on the interface of the layer below — never the concrete type. All wiring happens in `cmd/server/main.go`. This makes every layer testable in isolation by swapping implementations with mocks.

For the full technical breakdown including sequence diagrams and design trade-offs, see **[`docs/ARCHITECTURE.md`](docs/ARCHITECTURE.md)**.

---

## Quick Start

### Prerequisites

- Go 1.22+
- Docker Desktop
- Git

### Run locally

```bash
# 1. Clone
git clone https://github.com/sanu1001/pingmate.git
cd pingmate

# 2. Set up environment
cp .env.example .env
# edit .env if needed — defaults work for local dev

# 3. Start PostgreSQL + Redis
docker compose up -d

# 4. Run the server
go run ./cmd/server/main.go
```

Server boots on `http://localhost:8080`. Confirm with:

```bash
curl http://localhost:8080/health
# → {"status":"ok","service":"PingMate"}
```

**Swagger UI** → http://localhost:8080/swagger/index.html

---

## API Overview

| Method | Endpoint | Auth | Description |
|---|---|---|---|
| POST | `/api/v1/auth/register` | ❌ | Register a new user |
| POST | `/api/v1/auth/login` | ❌ | Login, receive JWT |
| POST | `/api/v1/auth/logout` | ✅ | Invalidate token (blacklist in Redis) |
| POST | `/api/v1/reminders` | ✅ | Create a reminder |
| GET | `/api/v1/reminders` | ✅ | List all your reminders |
| GET | `/api/v1/reminders/:id` | ✅ | Get a single reminder |
| PUT | `/api/v1/reminders/:id` | ✅ | Update a reminder |
| DELETE | `/api/v1/reminders/:id` | ✅ | Delete a reminder |

Protected routes require `Authorization: Bearer <token>` header.

### Example — Create a reminder

```bash
curl -X POST http://localhost:8080/api/v1/reminders \
  -H "Authorization: Bearer <your-jwt-token>" \
  -H "Content-Type: application/json" \
  -d '{
    "title": "Submit report",
    "description": "Final draft",
    "scheduled_at": "2026-05-20T18:00:00Z",
    "recurrence": "daily"
  }'
```

---

## Project Structure

```
pingmate/
├── cmd/server/main.go              # Entry point + dependency wiring
├── internal/
│   ├── models/                     # Pure data structs
│   ├── repository/                 # SQL queries (interfaces + implementations)
│   ├── services/                   # Business logic
│   ├── handlers/                   # HTTP layer (Gin handlers)
│   ├── middleware/                 # JWT auth + rate limiting
│   └── scheduler/                  # Background reminder worker
├── config/                         # Env loader + DB + Redis connection
├── db/migrations/                  # SQL schema (auto-run on Docker init)
├── docs/                           # ARCHITECTURE.md, PROJECT.md, Swagger docs
├── docker-compose.yml              # Postgres + Redis services
└── README.md
```

---

## Design Highlights

A few decisions worth calling out:

- **No ORM.** Raw SQL via `database/sql` for full control and transparency. Every query is readable as-is.
- **Strict dependency inversion.** Repository, service, and handler layers communicate only through interfaces. Swapping PostgreSQL for an in-memory store would touch only `main.go`.
- **JWT + Redis blacklist.** Stateless authentication on the hot path, with stateful logout achieved by storing invalidated tokens in Redis with TTL equal to their remaining lifetime. Auto-expires, zero cleanup code.
- **Goroutine scheduler over external job queues.** For V1's scale, a polling goroutine is simpler, more debuggable, and removes a dependency. Documented trade-off: ~30s delivery variance and single-instance only.
- **Rate limit on writes only.** Reads are cheap, writes hit the DB harder — limiter is scoped to `POST/PUT/DELETE` routes.
- **Documentation-first.** Architecture and project docs were written before code, ensuring the implementation matched a deliberate design.

---

## Build Phases

- [x] **Phase 1** — Scaffold, config, DB + Redis connection, migrations
- [x] **Phase 2** — Auth (register/login/logout), JWT middleware, Reminder CRUD
- [x] **Phase 2.5** — Endpoint testing, repository UUID handling
- [x] **Phase 2.6** — Redis-backed rate limiting middleware
- [x] **Phase 3** — Background scheduler, notification logs, recurrence advancement
- [x] **Phase 4** — Swagger documentation, GIN release mode, README polish

---

## Author

**Sanu Mukherjee** — Final year CSE student aiming for software engineering roles.
Full-stack development with Flutter and Go, with strong interest in backend systems and DSA.

- 🌐 [sanu-portfolio.vercel.app](https://sanu-portfolio.vercel.app/)
- 💼 [LinkedIn](https://www.linkedin.com/in/sanu-mukherjee1001/)
- 📧 [sanumukhopadhyay123@gmail.com](mailto:sanumukhopadhyay123@gmail.com)
- 🐙 [GitHub](https://github.com/sanu1001)

---

## License

[MIT](LICENSE) © 2026 Sanu Mukherjee