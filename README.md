<p align="center">
  <h1 align="center">🏓 PingMate</h1>
  <p align="center">
    <b>Developer-first scheduled reminder API — clean, stateless, and production-ready.</b><br/>
    Built with Go · PostgreSQL · Redis · JWT · Docker
  </p>
  <p align="center">
    <img src="https://img.shields.io/badge/Go-1.22-00ADD8?style=flat-square&logo=go&logoColor=white"/>
    <img src="https://img.shields.io/badge/PostgreSQL-16-4169E1?style=flat-square&logo=postgresql&logoColor=white"/>
    <img src="https://img.shields.io/badge/Redis-7-DC382D?style=flat-square&logo=redis&logoColor=white"/>
    <img src="https://img.shields.io/badge/Docker-Compose-2496ED?style=flat-square&logo=docker&logoColor=white"/>
    <img src="https://img.shields.io/badge/JWT-Auth-000000?style=flat-square&logo=jsonwebtokens&logoColor=white"/>
    <img src="https://img.shields.io/badge/Swagger-Docs-85EA2D?style=flat-square&logo=swagger&logoColor=black"/>
    <img src="https://img.shields.io/badge/Status-In_Progress-orange?style=flat-square"/>
  </p>
</p>

---

## What is PingMate?

PingMate is a **REST API for scheduling and managing reminders**, built for developers who need a reliable, embeddable reminder backend — not a bloated SaaS platform.

You register, authenticate, create reminders with optional recurrence, and PingMate's background scheduler takes care of the rest — polling the database, triggering due reminders, and logging every event.

No third-party queue. No external cron service. Just Go, Postgres, and a clean HTTP interface.

---

## Why PingMate?

Most reminder systems are either too simple (no recurrence, no logging) or overkill (RabbitMQ, Kafka, microservice sprawl). PingMate sits in the sweet spot:

| Problem | PingMate's answer |
|---|---|
| Need reminders without a full SaaS | Lightweight REST API you control |
| Token invalidation on logout | Redis-backed JWT blacklist |
| Recurring reminders | `none / daily / weekly / monthly` support |
| Audit trail | `notification_logs` table per trigger |
| Deployment complexity | Single `docker compose up` |
| No docs | Swagger UI included |

---

## Features

- **JWT Auth** — Register, login, logout (with Redis token blacklisting)
- **Full Reminder CRUD** — Create, list, get, update, delete — scoped per user
- **Recurrence Support** — `none`, `daily`, `weekly`, `monthly`
- **Background Scheduler** — Goroutine-based polling, no external dependencies
- **Notification Logs** — Every triggered reminder is logged with status (`sent` / `failed`)
- **Swagger Docs** — Auto-generated API docs via swaggo
- **Docker Ready** — Full stack runs with one command

---

## Tech Stack

| Layer | Technology |
|---|---|
| Language | Go 1.22 |
| Framework | Gin |
| Database | PostgreSQL 16 |
| Cache / Blacklist | Redis 7 |
| Auth | golang-jwt/jwt v5 |
| Background Jobs | Native Go goroutine scheduler |
| Docs | swaggo/swag |
| Config | godotenv |
| Containerization | Docker + Docker Compose |

---

## Getting Started

### Prerequisites

- [Go 1.22+](https://go.dev/dl/)
- [Docker Desktop](https://www.docker.com/products/docker-desktop/)
- [Git](https://git-scm.com/)

### 1. Clone the repo

```bash
git clone https://github.com/<your-username>/pingmate.git
cd pingmate
```

### 2. Configure environment

```bash
cp .env.example .env
```

Open `.env` and set your `JWT_SECRET` to any long random string. Everything else works with defaults for local dev.

### 3. Start infrastructure

```bash
docker compose up -d
```

This spins up PostgreSQL and Redis, and auto-runs all SQL migrations.

### 4. Run the server

```bash
go run ./cmd/server/main.go
```

### 5. Verify

```
GET http://localhost:8080/health
→ { "status": "ok", "service": "PingMate" }
```

Swagger UI → `http://localhost:8080/swagger/index.html`

---

## API Reference

### Auth

| Method | Endpoint | Description | Auth |
|---|---|---|---|
| POST | `/api/v1/auth/register` | Register a new user | ❌ |
| POST | `/api/v1/auth/login` | Login, receive JWT | ❌ |
| POST | `/api/v1/auth/logout` | Invalidate token | ✅ |

### Reminders

| Method | Endpoint | Description | Auth |
|---|---|---|---|
| POST | `/api/v1/reminders` | Create a reminder | ✅ |
| GET | `/api/v1/reminders` | List all your reminders | ✅ |
| GET | `/api/v1/reminders/:id` | Get a reminder by ID | ✅ |
| PUT | `/api/v1/reminders/:id` | Update a reminder | ✅ |
| DELETE | `/api/v1/reminders/:id` | Delete a reminder | ✅ |

All protected routes require:
```
Authorization: Bearer <token>
```

---

## Project Structure

```
pingmate/
├── cmd/server/main.go          # Entry point
├── internal/
│   ├── auth/
│   │   ├── handler.go          # HTTP handlers for auth
│   │   ├── service.go          # Business logic
│   │   └── middleware.go       # JWT validation middleware
│   ├── reminder/
│   │   ├── handler.go          # HTTP handlers for reminders
│   │   ├── service.go          # Business logic
│   │   └── repository.go       # DB queries
│   └── scheduler/
│       └── worker.go           # Background polling goroutine
├── db/migrations/              # Ordered SQL migration files
├── config/
│   ├── config.go               # Env loader
│   ├── db.go                   # PostgreSQL connection
│   └── redis.go                # Redis connection
├── docs/                       # Swagger + Architecture docs
├── .env.example
├── docker-compose.yml
├── Dockerfile
└── README.md
```

---

## Environment Variables

| Variable | Default | Description |
|---|---|---|
| `PORT` | `8080` | Server port |
| `DB_HOST` | `localhost` | PostgreSQL host |
| `DB_PORT` | `5432` | PostgreSQL port |
| `DB_USER` | `pingmate` | DB user |
| `DB_PASSWORD` | — | DB password |
| `DB_NAME` | `pingmate_db` | DB name |
| `REDIS_ADDR` | `localhost:6379` | Redis address |
| `JWT_SECRET` | **required** | JWT signing secret |
| `JWT_EXPIRY_HOURS` | `72` | Token lifetime in hours |

---

## Build Phases

- [x] **Phase 1** — Scaffold, config, DB + Redis connection, migrations
- [ ] **Phase 2** — Auth (register/login/logout), JWT middleware, Reminder CRUD
- [ ] **Phase 3** — Background scheduler, notification log writes
- [ ] **Phase 4** — Swagger docs, Dockerfile finalized, README polish

---

## License

MIT — see [LICENSE](LICENSE)

---

<p align="center">Built by <a href="https://github.com/<your-username>">@your-username</a> · Part of a backend portfolio series</p>