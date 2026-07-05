# Gerege Backend Template v27

> 🌐 **English** · [Монгол](README_MN.md)

[![Go](https://img.shields.io/badge/Go-1.26-blue.svg)](https://golang.org/)
[![chi](https://img.shields.io/badge/chi-v5-00ADD8.svg)](https://github.com/go-chi/chi)
[![pgx](https://img.shields.io/badge/pgx-v5-336791.svg)](https://github.com/jackc/pgx)
[![License](https://img.shields.io/badge/License-MIT-green.svg)](LICENSE)

A high-performance Go backend template built on Clean Architecture principles.
Based on **chi (net/http)** for HTTP, **pgx (pgxpool) + PostgreSQL** for data,
**Redis + Ristretto** for cache, and **JWT + OTP (GeregeCloud Verify)** for
authentication.

## 📌 Origin & Open Source

> This template is **based on and inspired by the open-source project
> [snykk/go-rest-boilerplate](https://github.com/snykk/go-rest-boilerplate)**
> (author: Najib Fikri, **MIT License**). The Clean Architecture structure,
> JWT/OTP authentication, audit, cache, observability, and test strategy
> are inherited from there.
>
> We **ported** the following two things:
> - HTTP layer: **Gin → chi (net/http)**
> - Data layer: **sqlx → pgx (pgxpool, hand-written SQL)**
>
> The upstream project is MIT-licensed, and its copyright and license terms
> are honored and preserved (see the [Credits](#-credits--license) section
> below). This template itself is also **MIT-licensed**.

## Features

- **Clean Architecture** — `handler → usecase → repository → domain`, inward-facing dependencies, no back-imports
- **chi (net/http)** — idiomatic standard-library router
- **pgx (pgxpool)** — hand-written SQL, no ORM; explicit soft-delete via `deleted_at IS NULL`
- **JWT authentication** — access + refresh token (rotation, `kind` claim guard)
- **OTP registration** — email OTP verification, brute-force lockout
- **GeregeCloud Verify** — all email/SMS OTP (registration + password reset) via verify.gecloud.mn; no SMTP
- **AI pipeline (Gemini)** — SDK-free REST client + function calling: text/voice chat, STT, TTS, live translation; layered prompts (hardcoded guardrails + DB-configurable scope) and a DB-backed `search_knowledge` tool
- **Audit log** — logging of authentication events
- **Observability** — OpenTelemetry trace + Prometheus metrics
- **Cache** — two-tier Redis + Ristretto
- **Integration Testing** — testcontainers-go (real Postgres + Redis)
- **Swagger** — automatic API documentation from godoc annotations
- **Structured Logging** — Zap, with request ID propagation
- **Security** — security headers, CORS, rate limiting, body size limit, full server timeouts, Postgres RLS + boot-time enforceability guard, logout access-token deny-list
- **Graceful Shutdown** — drains HTTP, DB pool, Redis, tracer in order

## Project Structure

```
.
├── cmd/
│   ├── api/main.go              # Application entry point
│   ├── api/server/server.go     # Composition root (manual DI)
│   ├── migration/               # Migration CLI
│   ├── seed/                    # Seed CLI
│   └── healthcheck/             # Distroless health probe
├── internal/
│   ├── business/
│   │   ├── domain/              # Domain entities (innermost layer)
│   │   └── usecases/{auth,users,rbac,ai}/  # Business logic (interface + impl)
│   ├── datasources/
│   │   ├── drivers/             # pgx (pgxpool) Postgres connection (driver_pgx.go)
│   │   ├── caches/              # Redis + Ristretto
│   │   ├── migration/           # Migration runner
│   │   ├── records/             # pgx record structs + record↔domain mappers
│   │   └── repositories/        # interface + postgres impl
│   ├── http/
│   │   ├── handlers/v1/         # HTTP handlers
│   │   ├── middlewares/         # Middleware stack
│   │   ├── routes/              # Route registration
│   │   ├── datatransfers/       # Request/Response DTO
│   │   └── auth/                # CurrentUser from context
│   └── config/ apperror/ constants/
├── migrations/                  # SQL migrations
├── docs/                        # Swagger + ARCHITECTURE.md + DEVELOPMENT.md
└── pkg/                         # jwt, logger, clock, helpers, validators,
                                 # audit, observability, verify, gemini
```

## Quick Start

### Requirements
- Go 1.26+
- PostgreSQL 15+
- Redis 7+
- Docker (for integration tests / local stack)
- Make

### Installation

```bash
# 1. Copy environment file (it lives under internal/config/)
cp internal/config/.env.example internal/config/.env
# Edit .env — JWT_SECRET must be at least 32 characters

# 2. Bring up the stack (Postgres + Redis + API)

# 3. Or run locally: migration → server
```

Server: `http://localhost:8080`, Swagger UI: `http://localhost:8080/swagger/`.

### Make commands

```bash
make build              # Build the binary
make test               # Unit tests (mocks — fast, no Docker)
make test-integration   # Integration tests (requires Docker)
make swag               # Generate Swagger docs
make lint               # golangci-lint
make pre-push           # CI checks locally (lint+test+swag+build)
```

## Configuration

Key variables from `internal/config/.env.example`:

```env
PORT=8080
ENVIRONMENT=development          # development | production
JWT_SECRET=...                   # >= 32 characters (HS256)
JWT_EXPIRED=5                    # access token TTL (hours)
JWT_REFRESH_EXPIRED=7            # refresh token TTL (days)
DB_POSTGRE_DSN=...               # DSN in dev
DB_POSTGRE_URL=...               # URL in production
REDIS_HOST=localhost:6379
BCRYPT_COST=12                   # 10..31
VERIFY_API_KEY=...               # GeregeCloud Verify OTP (required in production)
VERIFY_API_BASE=https://verify.gecloud.mn/v1
VERIFY_CHANNEL=email
OTEL_EXPORTER=                   # empty=off | stdout | otlp
ALLOWED_ORIGINS=                 # required in production (comma-separated)
GEMINI_API_KEY=                  # AI pipeline (/api/v1/ai/*); empty = AI disabled
GEMINI_MODEL=gemini-2.5-flash    # optional override (chat / STT / translate)
GEMINI_TTS_MODEL=gemini-2.5-flash-preview-tts  # optional override (TTS)
GEMINI_VOICE=Kore                # optional prebuilt TTS voice
GEMINI_API_BASE=                 # optional override (default: Google generativelanguage v1beta)
AI_SCOPE_PROMPT=                 # AI scope fallback when the DB 'scope' prompt layer is empty
SUPERADMIN_EMAIL=                # optional: promote this (already-registered) user to super admin on boot
```

### Roles & super admin

Roles are ordered by privilege (id 1 = highest): **superadmin=1, admin=2,
manager=3, user=4** (seeded/remapped by migration `23_superadmin_role`). A
**super admin** sits above admin and is the only role that can manage admin
accounts (create / grant / revoke) via `/api/v1/superadmin/*`
(`RequireSuperAdmin`); regular admins cannot reach that surface. The API never
mints a super admin — bootstrap one by setting `SUPERADMIN_EMAIL` to an
already-registered user (promoted on the next boot) or by updating `role_id=1`
in the DB.

> **Breaking change (existing deployments):** migration `23` renumbers roles, so
> JWTs issued before it are reinterpreted (old `admin=1` → superadmin,
> `user=2` → admin). When applying to an existing DB, **rotate `JWT_SECRET`** (or
> force all users to re-login) so stale tokens don't gain the wrong privilege.
> Fresh installs are unaffected.

### AI prompt layers

The AI assistant runs on a layered system prompt: **base guardrails**
(hardcoded — Mongolian-only, scope enforcement, prompt-injection resistance)
+ **scope** (what the assistant helps with) + **instructions** (optional
tone/rules). Scope and instructions live in the `ai_prompts` table and are
editable at runtime via `GET/PUT /api/v1/admin/ai/prompts` (requires
`settings.manage`; UI under Admin → Settings). The assistant refuses
anything outside the configured scope, and answers platform questions by
searching the `ai_knowledge` table through its `search_knowledge` tool.

## API Endpoints

All under `/api/v1` (ops endpoints at root):

### Public (Authentication)
| Method | Path | Description |
|--------|------|---------|
| POST | `/api/v1/auth/register` | Register (email+password) |
| POST | `/api/v1/auth/login` | Get token pair |
| POST | `/api/v1/auth/send-otp` | Send OTP |
| POST | `/api/v1/auth/verify-otp` | Verify OTP and activate |
| POST | `/api/v1/auth/refresh` | Token rotation |
| POST | `/api/v1/auth/logout` | Revoke refresh token |
| POST | `/api/v1/auth/password/forgot` | Start password reset |
| POST | `/api/v1/auth/password/reset` | Complete password reset |

### Protected (requires JWT)
| Method | Path | Description |
|--------|------|---------|
| PUT | `/api/v1/auth/password/change` | Change password |
| GET | `/api/v1/users/me` | User profile |
| POST | `/api/v1/ai/chat` | AI chat (Gemini pipeline, function calling, text/voice messages) |
| POST | `/api/v1/ai/stt` | Speech-to-text (audio base64 → transcript) |
| POST | `/api/v1/ai/tts` | Text-to-speech (text → WAV base64) |
| POST | `/api/v1/ai/translate` | Live translation (text/audio → target language, optional TTS) |
| GET/PUT | `/api/v1/admin/ai/prompts` | AI prompt layers — scope/instructions (settings.manage) |
| GET | `/api/v1/superadmin/admins` | List admin-level accounts (super admin only) |
| POST | `/api/v1/superadmin/admins` | Create a new admin account (super admin only) |
| PUT | `/api/v1/superadmin/admins/{id}/grant` | Grant admin to an existing user (super admin only) |
| DELETE | `/api/v1/superadmin/admins/{id}` | Revoke admin (super admin only) |

### Ops
`GET /health` (liveness) · `GET /ready` (DB+Redis) · `GET /metrics` · `GET /swagger/*`

### Response format
```json
{ "status": true, "message": "login success", "data": { }, "request_id": "…" }
```
On error, `status:false`. Validation error → `422`, with each field under `data.errors`.

## Development

See for details:
- **[docs/ARCHITECTURE.md](docs/ARCHITECTURE.md)** — layer structure, dependency flow, security
- **[docs/DEVELOPMENT.md](docs/DEVELOPMENT.md)** — 8 steps to add a new feature, testing, code style, troubleshooting
- **[docs/AI_PIPELINE.md](docs/AI_PIPELINE.md)** — AI assistant internals: flows, prompt layers, tools, voice, how to extend

```bash
make test               # Unit tests
make test-integration   # Integration tests (Docker)
make test-cover         # Coverage
```

## Docker

```bash
make build              # Binary
curl http://localhost:8080/health
```

## 🙏 Credits & License

This template stands on open-source work:

| Project | Author | License | What we used |
|-------|---------|--------|--------------|
| [snykk/go-rest-boilerplate](https://github.com/snykk/go-rest-boilerplate) | Najib Fikri | MIT | Base architecture, auth/OTP/audit, cache, observability, tests |
| [chi](https://github.com/go-chi/chi) · [pgx](https://github.com/jackc/pgx) | — | MIT | Router · Postgres driver |

**Our changes:** ported the HTTP layer **Gin → chi (net/http)** and the data layer
**sqlx → pgx (pgxpool, hand-written SQL)**; everything else was preserved faithfully. In keeping with the
MIT tradition, the upstream projects' copyright notices are retained, and this
template is itself **MIT-licensed** (see the `LICENSE` file).

---

**Gerege Template Version 27.0** — Co-developed by the **Gerege Systems
Development Team** and **Claude AI**, 2026.
