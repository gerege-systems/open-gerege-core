# Gerege Backend Template v27

> 🌐 **English** · [Монгол](README_MN.md)

[![Go](https://img.shields.io/badge/Go-1.26-blue.svg)](https://golang.org/)
[![Fiber](https://img.shields.io/badge/Fiber-v3-00ACD7.svg)](https://gofiber.io/)
[![GORM](https://img.shields.io/badge/GORM-v2-CB3837.svg)](https://gorm.io/)
[![License](https://img.shields.io/badge/License-MIT-green.svg)](LICENSE)

A high-performance Go backend template built on Clean Architecture principles.
Based on **Fiber v3** (HTTP), **GORM + PostgreSQL** (data), **Redis + Ristretto**
(cache), and **JWT + OTP** (authentication).

## 📌 Origin & Open Source

> This template is **based on and inspired by the open-source project
> [snykk/go-rest-boilerplate](https://github.com/snykk/go-rest-boilerplate)**
> (author: Najib Fikri, **MIT License**). The Clean Architecture structure,
> JWT/OTP authentication, mailer, audit, cache, observability, and test strategy
> are inherited from there.
>
> We **ported** the following two things:
> - HTTP layer: **Gin → Fiber v3**
> - Data layer: **sqlx → GORM**
>
> Fiber v3 usage was cross-referenced against the open-source
> [rachmanzz/fiber-starter](https://github.com/rachmanzz/fiber-starter) (MIT).
> All upstream projects are MIT-licensed, and their copyright and license terms
> are honored and preserved (see the [Credits](#-credits--license) section
> below). This template itself is also **MIT-licensed**.

## Features

- **Clean Architecture** — `handler → usecase → repository → domain`, inward-facing dependencies, no back-imports
- **Fiber v3** — high-performance web framework
- **GORM** — PostgreSQL-backed ORM with soft-delete (`gorm.DeletedAt`)
- **JWT authentication** — access + refresh token (rotation, `kind` claim guard)
- **OTP registration** — email OTP verification, brute-force lockout
- **Async Mailer** — sends OTP email off the request path, with retry
- **Audit log** — logging of authentication events
- **Observability** — OpenTelemetry trace + Prometheus metrics
- **Cache** — two-tier Redis + Ristretto
- **Integration Testing** — testcontainers-go (real Postgres + Redis)
- **Swagger** — automatic API documentation from godoc annotations
- **Structured Logging** — Zap, with request ID propagation
- **Security** — security headers, CORS, rate limiting, body size limit
- **Graceful Shutdown** — shuts down HTTP, mailer queue, DB, Redis, tracer in order

## Project Structure

```
.
├── cmd/
│   ├── api/main.go              # Application entry point
│   ├── api/server/server.go     # Composition root (manual DI)
│   ├── migration/               # Migration CLI
│   └── seed/                    # Seed CLI
├── internal/
│   ├── business/
│   │   ├── domain/              # Domain entities (innermost layer)
│   │   └── usecases/{auth,users}/  # Business logic (interface + impl)
│   ├── datasources/
│   │   ├── drivers/             # GORM Postgres connection
│   │   ├── caches/              # Redis + Ristretto
│   │   ├── migration/           # Migration runner
│   │   ├── records/             # GORM model + mapper
│   │   └── repositories/        # interface + postgres impl
│   ├── http/
│   │   ├── handlers/v1/         # HTTP handlers
│   │   ├── middlewares/         # Middleware stack
│   │   ├── routes/              # Route registration
│   │   ├── datatransfers/       # Request/Response DTO
│   │   └── auth/                # CurrentUser from Locals
│   └── config/ apperror/ constants/
├── migrations/                  # SQL migrations
├── docs/                        # Swagger + ARCHITECTURE.md + DEVELOPMENT.md
└── pkg/                         # jwt, logger, clock, helpers, validators,
                                 # mailer, audit, observability
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
OTEL_EXPORTER=                   # empty=off | stdout | otlp
ALLOWED_ORIGINS=                 # required in production (comma-separated)
```

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
| [snykk/go-rest-boilerplate](https://github.com/snykk/go-rest-boilerplate) | Najib Fikri | MIT | Base architecture, auth/OTP/mailer/audit, cache, observability, tests |
| [rachmanzz/fiber-starter](https://github.com/rachmanzz/fiber-starter) | rachmanzz | MIT | Fiber v3 idioms reference |
| [GoFiber](https://github.com/gofiber/fiber) · [GORM](https://github.com/go-gorm/gorm) | — | MIT | Framework · ORM |

**Our changes:** ported the HTTP layer **Gin → Fiber v3** and the data layer
**sqlx → GORM**; everything else was preserved faithfully. In keeping with the
MIT tradition, the upstream projects' copyright notices are retained, and this
template is itself **MIT-licensed** (see the `LICENSE` file).

---

**Gerege Template Version 27.0** — Co-developed by the **Gerege Systems
Development Team** and **Claude AI**, 2026.
