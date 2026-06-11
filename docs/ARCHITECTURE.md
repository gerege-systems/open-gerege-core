# Architecture Overview

> 🌐 **English** · [Монгол](ARCHITECTURE_MN.md)

This document describes the high-level architecture of the **Gerege Backend
Template v27** (module `template`). The stack is **chi (net/http) + pgx
(pgxpool) + PostgreSQL + Redis**, organized along Clean Architecture lines.

> **Origin & credits.** This template is **derived from the open-source project
> [snykk/go-rest-boilerplate](https://github.com/snykk/go-rest-boilerplate)**
> by Najib Fikri (MIT License) — the Clean Architecture layering, JWT/OTP auth
> flows, caching, observability, and test strategy come from there. We adapted
> it by converting the HTTP layer **Gin → chi (net/http)** and the data layer
> **sqlx → pgx (pgxpool)**. The upstream is MIT-licensed; its license terms are
> honored — see [Credits](#credits--license).

## Layer Diagram

```
┌─────────────────────────────────────────────────────────────────┐
│                        HTTP Layer                                 │
│  cmd/api/server → Middleware → internal/http/handlers/v1          │
│  internal/http/{routes, datatransfers, middlewares, auth}         │
├─────────────────────────────────────────────────────────────────┤
│                       Usecase Layer                               │
│  internal/business/usecases/{auth,users,rbac,ai}                  │
│  (Business logic, validation, orchestration)                      │
├─────────────────────────────────────────────────────────────────┤
│                     Repository Layer                              │
│  internal/datasources/repositories/{interface, postgres}          │
│  (Data access via pgx hand-written SQL, explicit soft-delete,     │
│   caching)                                                        │
├─────────────────────────────────────────────────────────────────┤
│                       Domain Layer                                │
│  internal/business/domain                                         │
│  (Entities, value objects, business rules)                        │
└─────────────────────────────────────────────────────────────────┘
```

## Directory Structure

```
.
├── cmd/
│   ├── api/
│   │   ├── main.go                 # Entry point (config + logger init)
│   │   └── server/server.go        # Composition root (manual DI)
│   ├── migration/                  # Migration CLI
│   └── seed/                       # DB seeder CLI
├── docs/                           # OpenAPI spec (swagger.json/yaml, docs.go)
├── internal/
│   ├── apperror/                   # Typed domain errors (→ HTTP status)
│   ├── business/
│   │   ├── domain/                 # Enterprise entities (innermost circle)
│   │   └── usecases/{auth,users,rbac,ai}/  # Business logic — interface + impl
│   │       └── ai/                 # Gemini pipeline: function-calling chat,
│   │                               #   STT/TTS/translate, layered prompts, tools
│   ├── config/                     # Viper-backed config + .env.example
│   ├── constants/                  # Env, logger, error, endpoint constants
│   ├── datasources/
│   │   ├── caches/                 # Redis + Ristretto two-tier cache
│   │   ├── drivers/                # pgx (pgxpool) Postgres connection (driver_pgx.go)
│   │   ├── migration/              # Migration runner (SQL + AutoMigrate)
│   │   ├── records/                # pgx record structs + record↔domain mappers
│   │   └── repositories/
│   │       ├── interface/          # Gateway abstractions (package _interface)
│   │       └── postgres/{users,rbac,ai}/ # pgx implementations (hand-written SQL)
│   └── http/
│       ├── auth/                   # CurrentUser from request context
│       ├── datatransfers/          # Request / Response DTOs
│       ├── handlers/v1/            # HTTP handlers
│       ├── middlewares/            # Middleware stack
│       └── routes/                 # Route registration
├── migrations/                     # SQL migration files
├── pkg/                            # Framework-agnostic utilities
│   ├── jwt/ logger/ clock/ helpers/ validators/
│   ├── verify/                     # GeregeCloud Verify (OTP) client
│   ├── gemini/                     # SDK-free Gemini REST client (function calling,
│   │                               #   audio in/out, retry+backoff, PCM→WAV)
│   ├── audit/                      # Auth-event audit log
│   └── observability/              # Tracing + metrics
└── internal/test/                  # Mocks, fixtures, testcontainers harness
```

## Dependency Flow

Dependencies flow inward only (Clean Architecture principle):

```
HTTP → Usecase → Repository → Domain
  │        │          │
  ▼        ▼          ▼
 DTO   Interface   pgx/SQL
```

- **HTTP Layer** depends on **Usecase** interfaces (`auth.Usecase`, `users.Usecase`)
- **Usecase Layer** depends on **Repository** interfaces (`_interface.UserRepository`)
- **Repository Layer** depends on **Domain** entities
- **Domain Layer** imports only the standard library + `golang.org/x/crypto/bcrypt` — never `internal/` or `pkg/`

This is verified structurally: `internal/business/**` and
`internal/datasources/repositories/**` import **no** chi/net-http web package,
so the delivery framework can be swapped without touching business code.

## Key Components

### 1. HTTP Layer

**Composition root:** `cmd/api/server/server.go`
- Initializes tracing, DB (pgx pool), Redis/Ristretto, JWT service, GeregeCloud Verify client
- Wires repositories → usecases → routes by hand (no global singletons, no DI container)
- Builds the chi router and installs the middleware stack
- Owns graceful shutdown (drains HTTP, rate-limiter, pgx pool, Redis, tracer)

**Routes:** `internal/http/routes/`
- All API routes live under `/api/v1`
- Applies rate limiting + body cap to the anonymous `/auth` surface
- Shares one JWT-validating middleware across protected groups

**Handlers:** `internal/http/handlers/v1/`
- Parse + validate the request DTO, call the usecase, format the response
- Handler signature: `func(w http.ResponseWriter, r *http.Request) error` (wrapped by `v1.Wrap`)

**DTOs:** `internal/http/datatransfers/{requests,responses}/`
- Request structs carry `validate:` tags; responses shape the public payload

### 2. Middleware Stack

Global middleware, applied in order in `server.go::setupRouter`:

1. **Tracing** — starts the per-request OTel span via chi middleware
2. **Request ID** — generates / propagates `X-Request-ID` into the request context + logger
3. **Metrics** — Prometheus HTTP request counters + latency
4. **Security Headers** — HSTS, CSP, nosniff, frame options, referrer policy
5. **CORS** — origins from `ALLOWED_ORIGINS` (wildcard only in dev)
6. **Body Size Limit** — global ceiling (per-route tighter caps on `/auth`)
7. **Access Log** — structured one-line access log

Per-group middleware: rate limiter + 4 KiB body cap on `/auth`; JWT auth
middleware on `/users` and `/auth/password/change`.

### 3. Usecase Layer

**Location:** `internal/business/usecases/`

Each context exposes an interface + an implementation:

```go
// internal/business/usecases/users/users.usecase.go
type Usecase interface {
    Store(ctx context.Context, in StoreRequest) (domain.User, error)
    GetByEmail(ctx context.Context, email string) (domain.User, error)
    GetByID(ctx context.Context, id string) (domain.User, error)
    Activate(ctx context.Context, id string) error
    UpdatePassword(ctx context.Context, id, newPassword string) error
}
```

Responsibilities: business-rule validation, orchestration of repository +
cache + JWT + GeregeCloud Verify, login lockout, the password-rotation token cutoff.
`auth.Usecase` depends on `users.Usecase` (auth reuses user reads/writes).

### 4. Repository Layer

**Location:** `internal/datasources/repositories/`

The `interface/` package (package name `_interface` — `interface` is a Go
keyword) holds gateway abstractions; `postgres/users/` implements them with pgx:

```go
// internal/datasources/repositories/interface/interface.go
type UserRepository interface {
    Store(ctx context.Context, in *domain.User) (domain.User, error)
    GetByEmail(ctx context.Context, in *domain.User) (domain.User, error)
    GetByID(ctx context.Context, id string) (domain.User, error)
    List(ctx context.Context, filter UserListFilter, offset, limit int) ([]domain.User, error)
    ChangeActiveUser(ctx context.Context, in *domain.User) error
    UpdatePassword(ctx context.Context, in *domain.User) error
    SoftDelete(ctx context.Context, id string) error
}
```

Key features: queries take `ctx` directly, soft delete via explicit
`deleted_at IS NULL` predicates, `Store` uses single round-trip
`INSERT … RETURNING`, duplicate keys detected via pgconn error code `23505` →
`apperror.Conflict`. Rows are scanned with `pgx.RowToStructByName`.

### 5. Domain Layer

**Location:** `internal/business/domain/`

Domain entities carry the business rules and depend on nothing internal:

```go
type User struct {
    ID                string
    Username          string
    Email             string
    Password          string // bcrypt hash post-construction
    Active            bool
    RoleID            int
    CreatedAt         time.Time
    UpdatedAt         *time.Time
    DeletedAt         *time.Time
    PasswordChangedAt *time.Time
}

func (u User) VerifyPassword(plain string) bool { /* bcrypt compare */ }
func (u User) IsAdmin() bool                     { return u.RoleID == RoleAdmin }
func (u *User) ChangePassword(plain string, bcryptCost int) error
```

## Authentication & Authorization

### Authentication

JWT access + refresh tokens (`pkg/jwt`):

- `POST /api/v1/auth/login` issues an access + refresh pair
- `POST /api/v1/auth/refresh` rotates the pair; tokens issued before a
  password change are rejected (`User.TokensRevokedBefore`)
- `kind` claim guard prevents using a refresh token as an access token
- The auth middleware (`internal/http/middlewares/middleware.auth.go`) validates
  the bearer token and stashes the claims in the request context

### Authorization

Role-based, encoded in the domain: `User.IsAdmin()` (`RoleAdmin = 1`). The
HTTP-layer `CurrentUser` view is read with
`auth.CurrentUserFromContext(c)` inside handlers.

## Database

- **Driver:** pgx v5 (`github.com/jackc/pgx/v5` + pgxpool), hand-written SQL (no ORM)
- **Database:** PostgreSQL
- **Migrations:** SQL files in `migrations/` + idempotent `AutoMigrate`
- **Tracing:** OpenTelemetry via chi middleware + pgx pool instrumentation (`otelpgx`)

### Connection Management

Pool configured from env (`internal/datasources/drivers/driver_pgx.go`,
`SetupPgxPostgres`):

```go
poolCfg.MaxConns        = cfg.MaxConns    // DB_MAX_OPEN_CONNS (default 25)
poolCfg.MinConns        = cfg.MinConns    // DB_MAX_IDLE_CONNS (default 5)
poolCfg.MaxConnLifetime = cfg.MaxLifetime // DB_CONN_MAX_LIFE_MINS (default 15)
```

## Observability

### Logging
- **Library:** Zap (structured), via `pkg/logger`
- **Format:** JSON in production, console in development
- **Context:** request ID + trace ID propagated through `*WithContext` helpers

### Metrics
- **Library:** Prometheus, endpoint `GET /metrics`
- HTTP request counters/latency, cache hit/miss/error per layer, OTP send
  outcomes (`otp_send_total`), DB pool stats

### Tracing
- **Library:** OpenTelemetry; exporter selected by `OTEL_EXPORTER`
  (empty = noop, `stdout`, or `otlp`)

### Health Checks
- `GET /health` — liveness
- `GET /ready` — DB ping (via pgx pool) + Redis probe

## Security Features

| Feature           | Implementation                       | Location                                  |
|-------------------|--------------------------------------|-------------------------------------------|
| Security headers  | HSTS, CSP, nosniff, frame options    | `middlewares/middleware.security.go`      |
| CORS              | env whitelist, wildcard dev-only     | `middlewares/middleware.cors.go`          |
| Rate limiting     | per-IP on `/auth`                    | `middlewares/middleware.ratelimit.go`     |
| Body size limit   | global + tighter 4 KiB on `/auth`    | `middlewares/middleware.bodysizelimit.go` |
| Input validation  | `validate:` struct tags              | `internal/http/datatransfers/requests/`   |
| Password hashing  | bcrypt (cost 10–31, default 12)      | `internal/business/domain/domain.users.go`|
| SQL injection     | pgx (parameterized queries)          | `internal/datasources/repositories/`      |
| Login lockout     | brute-force attempt cap in Redis     | `internal/business/usecases/auth/`        |

## API Design

### Routes

All under base path `/api/v1` (plus infra routes at root):

| Method | Path                          | Auth | Description              |
|--------|-------------------------------|------|--------------------------|
| POST   | `/api/v1/auth/register`       | —    | Register (email+password)|
| POST   | `/api/v1/auth/login`          | —    | Issue token pair         |
| POST   | `/api/v1/auth/send-otp`       | —    | Send OTP email           |
| POST   | `/api/v1/auth/verify-otp`     | —    | Verify OTP, activate user|
| POST   | `/api/v1/auth/refresh`        | —    | Rotate token pair        |
| POST   | `/api/v1/auth/logout`         | —    | Revoke refresh token     |
| POST   | `/api/v1/auth/password/forgot`| —    | Start password reset     |
| POST   | `/api/v1/auth/password/reset` | —    | Complete password reset  |
| PUT    | `/api/v1/auth/password/change`| JWT  | Change password          |
| GET    | `/api/v1/users/me`            | JWT  | Current user profile     |
| POST   | `/api/v1/ai/chat`             | JWT  | AI chat (text/voice, function calling) |
| POST   | `/api/v1/ai/stt`              | JWT  | Speech-to-text           |
| POST   | `/api/v1/ai/tts`              | JWT  | Text-to-speech (WAV)     |
| POST   | `/api/v1/ai/translate`        | JWT  | Live translation (text/audio) |
| GET/PUT| `/api/v1/admin/ai/prompts`    | JWT+perm | AI prompt layers (settings.manage) |
| GET    | `/api/v1/rbac/*` `/api/v1/admin/users*` | JWT+perm | RBAC + user administration |
| GET    | `/health` `/ready` `/metrics` | —    | Ops endpoints            |
| GET    | `/swagger/*`                  | —    | Swagger UI               |

### Response Format

A single envelope (`internal/http/handlers/v1/handler.base_response.go`):

**Success**
```json
{ "status": true, "message": "login success", "data": { }, "request_id": "…" }
```

**Error**
```json
{ "status": false, "message": "user not found", "request_id": "…" }
```

**Validation error (422)**
```json
{ "status": false, "message": "validation failed",
  "data": { "errors": { "email": "email is required" } }, "request_id": "…" }
```

Domain errors (`internal/apperror`) map to status codes: NotFound→404,
Unauthorized→401, Forbidden→403, Conflict→409, BadRequest→400, Internal→500.
5xx causes are logged and replaced with a generic message in the body.

## Testing Strategy

- **Unit tests** — usecase + handler layers with mockery mocks
  (`internal/test/mocks/`). Fast, no Docker. `make test`.
- **Integration tests** — repositories against a real Postgres + Redis via
  testcontainers-go (`internal/test/testenv/`). `make test-integration`.
- **Mocks** — generated by mockery. `make mock interface=… dir=… filename=…`.

## Configuration

Loaded from `.env` / environment by Viper (`internal/config/config.go`). See
`internal/config/.env.example`. Selected keys:

| Variable              | Description                       | Default       |
|-----------------------|-----------------------------------|---------------|
| `PORT`                | HTTP port                         | —             |
| `ENVIRONMENT`         | `development` / `production`      | —             |
| `DB_POSTGRE_DSN`/`_URL`| Postgres DSN (dev) / URL (prod)  | —             |
| `JWT_SECRET`          | HS256 secret (≥ 32 chars)         | —             |
| `JWT_EXPIRED`         | Access token TTL (hours)          | —             |
| `JWT_REFRESH_EXPIRED` | Refresh token TTL (days)          | 7             |
| `REDIS_HOST`          | `host:port`                       | —             |
| `BCRYPT_COST`         | bcrypt cost (10–31)               | 12            |
| `OTEL_EXPORTER`       | ``/`stdout`/`otlp`                | `` (disabled) |
| `ALLOWED_ORIGINS`     | comma-separated CORS origins      | dev: `*`      |

## Deployment

```bash
make build            # build the API binary
```

Health check: `curl http://localhost:8080/health`

## Credits & License

This template stands on open-source work:

| Project | Author | License | What we used |
|---------|--------|---------|--------------|
| [snykk/go-rest-boilerplate](https://github.com/snykk/go-rest-boilerplate) | Najib Fikri | MIT | Base architecture, auth/OTP/audit flows, caching, observability, tests |

Our changes vs. the upstream boilerplate: **Gin → chi (net/http)** (HTTP layer) and
**sqlx → pgx (pgxpool)** (data layer); everything else was reproduced faithfully. As an
MIT derivative, this template retains the upstream copyright notices and is
itself distributed under the MIT License (see `LICENSE`).


---

**Gerege Template Version 27.0** — Co-developed by the **Gerege Systems Development Team** and **Claude AI**, 2026.
