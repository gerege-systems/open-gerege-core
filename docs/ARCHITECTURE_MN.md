# Architecture Overview

> 🌐 [English](ARCHITECTURE.md) · **Монгол**

Энэ баримт нь **Gerege Backend Template v27** (модуль `template`)-ийн ерөнхий
архитектурыг тайлбарлана. Технологийн стек нь **chi (net/http) + pgx (pgxpool) +
PostgreSQL + Redis** бөгөөд Clean Architecture зарчмаар зохион байгуулагдсан.

> **Эх сурвалж & зохиогчид.** Энэ template нь Najib Fikri-ийн нээлттэй эх төсөл
> **[snykk/go-rest-boilerplate](https://github.com/snykk/go-rest-boilerplate)**
> (MIT лиценз) **дээр суурилсан** — Clean Architecture давхаргалал, JWT/OTP
> танилтын урсгал, кэш, observability, тестийн стратеги зэрэг нь тэндээс ирсэн.
> Бид HTTP давхаргыг **Gin → chi (net/http)**, өгөгдлийн давхаргыг
> **sqlx → pgx (pgxpool)** болгож хөрвүүлэн тохируулсан. Эх төсөл нь MIT
> лицензтэй бөгөөд түүний лицензийн нөхцлийг хүндэтгэсэн —
> [Зохиогчид](#credits--license) хэсгийг үз.

## Давхаргын диаграм (Layer Diagram)

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

## Лавлахын бүтэц (Directory Structure)

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
│   │       └── ai/                 # Gemini pipeline: function-calling чат,
│   │                               #   STT/TTS/орчуулга, давхаргат prompt, tools
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
│   ├── gemini/                     # SDK-гүй Gemini REST client (function calling,
│   │                               #   audio оролт/гаралт, retry+backoff, PCM→WAV)
│   ├── audit/                      # Auth-event audit log
│   └── observability/              # Tracing + metrics
└── internal/test/                  # Mocks, fixtures, testcontainers harness
```

## Хамаарлын урсгал (Dependency Flow)

Хамаарлууд зөвхөн дотогшоо чиглэнэ (Clean Architecture зарчим):

```
HTTP → Usecase → Repository → Domain
  │        │          │
  ▼        ▼          ▼
 DTO   Interface   pgx/SQL
```

- **HTTP давхарга** нь **Usecase** интерфейсүүдээс (`auth.Usecase`, `users.Usecase`) хамаарна
- **Usecase давхарга** нь **Repository** интерфейсүүдээс (`_interface.UserRepository`) хамаарна
- **Repository давхарга** нь **Domain** entity-үүдээс хамаарна
- **Domain давхарга** нь зөвхөн стандарт сан + `golang.org/x/crypto/bcrypt`-ийг л import хийдэг — `internal/` эсвэл `pkg/`-ийг хэзээ ч биш

Үүнийг бүтцийн хувьд баталгаажуулсан: `internal/business/**` болон
`internal/datasources/repositories/**` нь chi/net-http web package-ийг import
хийдэггүй тул business код руу гар хүрэлгүйгээр delivery framework-ийг солих
боломжтой.

## Гол бүрэлдэхүүн хэсгүүд (Key Components)

### 1. HTTP давхарга

**Composition root:** `cmd/api/server/server.go`
- Tracing, DB (pgx pool), Redis/Ristretto, JWT service, GeregeCloud Verify клиентийг эхлүүлнэ
- repository → usecase → route-ийг гараар холбоно (global singleton, DI container байхгүй)
- chi router-ийг бүтээж, middleware stack-ийг суулгана
- Graceful shutdown-ийг хариуцна (HTTP, rate-limiter, pgx pool, Redis, tracer-ийг хоослоно)

**Routes:** `internal/http/routes/`
- Бүх API route нь `/api/v1` дор байрладаг
- Нэргүй (anonymous) `/auth` гадаргуу дээр rate limiting + body cap-ийг хэрэглэнэ
- Хамгаалагдсан бүлгүүд дээр нэг JWT баталгаажуулагч middleware-ийг хуваалцана

**Handlers:** `internal/http/handlers/v1/`
- Хүсэлтийн DTO-г parse + validate хийж, usecase-ийг дуудаж, хариуг формат хийнэ
- Handler-ийн гарын үсэг: `func(w http.ResponseWriter, r *http.Request) error` (`v1.Wrap`-ээр боож)

**DTOs:** `internal/http/datatransfers/{requests,responses}/`
- Request struct-ууд `validate:` tag-уудтай; response-ууд нийтийн payload-ыг бүрдүүлнэ

### 2. Middleware stack

`server.go::setupRouter` дотор дарааллаар хэрэглэгддэг global middleware:

1. **Tracing** — хүсэлт тус бүрийн OTel span-ийг chi middleware-ээр эхлүүлнэ
2. **Request ID** — `X-Request-ID`-г үүсгэж / request context + logger руу дамжуулна
3. **Metrics** — Prometheus HTTP хүсэлтийн тоолуур + latency
4. **Security Headers** — HSTS, CSP, nosniff, frame options, referrer policy
5. **CORS** — `ALLOWED_ORIGINS`-аас гарал үүсэл (wildcard зөвхөн dev-д)
6. **Body Size Limit** — global дээд хязгаар (`/auth` дээр route тус бүр чанга хязгаартай)
7. **Access Log** — бүтэцлэгдсэн нэг мөрийн access log

Бүлэг тус бүрийн middleware: `/auth` дээр rate limiter + 4 KiB body cap; `/users`
болон `/auth/password/change` дээр JWT auth middleware.

### 3. Usecase давхарга

**Байршил:** `internal/business/usecases/`

Контекст бүр интерфейс + хэрэгжүүлэлтийг (implementation) дэлгэнэ:

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

Үүрэг: бизнес дүрмийн validation, repository + кэш + JWT + GeregeCloud Verify-ийн
зохицуулалт (orchestration), нэвтрэлтийн lockout, нууц үг солих токены cutoff.
`auth.Usecase` нь `users.Usecase`-ээс хамаарна (auth нь хэрэглэгчийн уншилт/бичилтийг дахин ашиглана).

### 4. Repository давхарга

**Байршил:** `internal/datasources/repositories/`

`interface/` package (package нэр `_interface` — `interface` нь Go-ийн түлхүүр
үг) нь gateway абстракцуудыг агуулдаг; `postgres/users/` нь тэдгээрийг pgx-ээр
хэрэгжүүлдэг:

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

Гол онцлогууд: query-ууд `ctx`-г шууд авна, `deleted_at IS NULL` гэсэн ил
predicate-ээр soft delete, `Store` нь нэг round-trip-д `INSERT … RETURNING`
ашиглана, давхардсан key-үүдийг pgconn-ийн алдааны код `23505`-аар илрүүлж
`apperror.Conflict` болгоно. Мөрүүдийг `pgx.RowToStructByName`-ээр scan хийнэ.

### 5. Domain давхарга

**Байршил:** `internal/business/domain/`

Domain entity-үүд нь бизнесийн дүрмийг агуулж, дотоод ямар ч зүйлээс хамаарахгүй:

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

## Танилт & Эрх олголт (Authentication & Authorization)

### Танилт (Authentication)

JWT access + refresh token (`pkg/jwt`):

- `POST /api/v1/auth/login` нь access + refresh хослолыг олгоно
- `POST /api/v1/auth/refresh` нь хослолыг сэлгэнэ; нууц үг солихоос өмнө олгогдсон
  токенуудыг татгалзана (`User.TokensRevokedBefore`)
- `kind` claim guard нь refresh token-ийг access token болгон ашиглахаас сэргийлнэ
- Auth middleware (`internal/http/middlewares/middleware.auth.go`) нь bearer
  token-ийг баталгаажуулж, claim-уудыг request context дотор хадгална

### Эрх олголт (Authorization)

Domain дотор кодлогдсон, дүрд суурилсан: `User.IsAdmin()` (`RoleAdmin = 1`).
HTTP давхаргын `CurrentUser` дүрслэлийг handler дотор
`auth.CurrentUserFromContext(c)`-ээр уншина.

## Өгөгдлийн сан (Database)

- **Driver:** pgx v5 (`github.com/jackc/pgx/v5` + pgxpool), гараар бичсэн SQL (ORM-гүй)
- **Database:** PostgreSQL
- **Migrations:** `migrations/` доторх SQL файлууд + idempotent `AutoMigrate`
- **Tracing:** chi middleware + pgx pool instrumentation-аар (`otelpgx`) дамжуулсан OpenTelemetry

### Холболтын удирдлага (Connection Management)

Pool нь env-ээс тохируулагдана (`internal/datasources/drivers/driver_pgx.go`,
`SetupPgxPostgres`):

```go
poolCfg.MaxConns        = cfg.MaxConns    // DB_MAX_OPEN_CONNS (default 25)
poolCfg.MinConns        = cfg.MinConns    // DB_MAX_IDLE_CONNS (default 5)
poolCfg.MaxConnLifetime = cfg.MaxLifetime // DB_CONN_MAX_LIFE_MINS (default 15)
```

## Observability

### Logging
- **Сан:** Zap (бүтэцлэгдсэн), `pkg/logger`-ээр дамжуулан
- **Формат:** production-д JSON, development-д console
- **Контекст:** request ID + trace ID нь `*WithContext` туслахуудаар дамжина

### Metrics
- **Сан:** Prometheus, endpoint `GET /metrics`
- HTTP хүсэлтийн тоолуур/latency, давхарга бүрийн кэш hit/miss/error, OTP илгээлтийн
  үр дүн (`otp_send_total`), DB pool статистик

### Tracing
- **Сан:** OpenTelemetry; exporter-ийг `OTEL_EXPORTER`-оор сонгоно
  (хоосон = noop, `stdout`, эсвэл `otlp`)

### Health Checks
- `GET /health` — liveness
- `GET /ready` — DB ping (pgx pool-оор) + Redis probe

## Аюулгүй байдлын онцлогууд (Security Features)

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

## API дизайн (API Design)

### Routes

Бүгд `/api/v1` үндсэн зам дор (мөн root дээр infra route-ууд):

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
| POST   | `/api/v1/ai/chat`             | JWT  | AI чат (текст/дуут, function calling) |
| POST   | `/api/v1/ai/stt`              | JWT  | Яриа→текст               |
| POST   | `/api/v1/ai/tts`              | JWT  | Текст→яриа (WAV)         |
| POST   | `/api/v1/ai/translate`        | JWT  | Шууд орчуулга (текст/audio) |
| GET/PUT| `/api/v1/admin/ai/prompts`    | JWT+эрх | AI prompt давхарга (settings.manage) |
| GET    | `/api/v1/rbac/*` `/api/v1/admin/users*` | JWT+эрх | RBAC + хэрэглэгчийн удирдлага |
| GET    | `/health` `/ready` `/metrics` | —    | Ops endpoints            |
| GET    | `/swagger/*`                  | —    | Swagger UI               |

### Хариуны формат (Response Format)

Нэг envelope (`internal/http/handlers/v1/handler.base_response.go`):

**Амжилт**
```json
{ "status": true, "message": "login success", "data": { }, "request_id": "…" }
```

**Алдаа**
```json
{ "status": false, "message": "user not found", "request_id": "…" }
```

**Validation алдаа (422)**
```json
{ "status": false, "message": "validation failed",
  "data": { "errors": { "email": "email is required" } }, "request_id": "…" }
```

Domain алдаанууд (`internal/apperror`) нь статус кодуудад буудаг: NotFound→404,
Unauthorized→401, Forbidden→403, Conflict→409, BadRequest→400, Internal→500.
5xx-ийн шалтгаануудыг log-д бичиж, body дотор ерөнхий мессежээр сольдог.

## Тестийн стратеги (Testing Strategy)

- **Unit тестүүд** — usecase + handler давхаргуудыг mockery mock-уудаар
  (`internal/test/mocks/`). Хурдан, Docker-гүй. `make test`.
- **Integration тестүүд** — repository-уудыг testcontainers-go-оор жинхэнэ
  Postgres + Redis-ийн эсрэг (`internal/test/testenv/`). `make test-integration`.
- **Mock-ууд** — mockery-ээр үүсгэгдсэн. `make mock interface=… dir=… filename=…`.

## Тохиргоо (Configuration)

Viper нь `.env` / environment-аас ачаална (`internal/config/config.go`).
`internal/config/.env.example`-ийг үз. Сонгосон key-үүд:

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

Энэ template нь нээлттэй эх кодын ажил дээр тулгуурласан:

| Project | Author | License | What we used |
|---------|--------|---------|--------------|
| [snykk/go-rest-boilerplate](https://github.com/snykk/go-rest-boilerplate) | Najib Fikri | MIT | Base architecture, auth/OTP/audit flows, caching, observability, tests |

Эх boilerplate-тай харьцуулсан бидний өөрчлөлт: **Gin → chi (net/http)** (HTTP
давхарга) болон **sqlx → pgx (pgxpool)** (өгөгдлийн давхарга); бусад бүхнийг үнэнчээр
дахин бүтээсэн. MIT-ийн уламжлалт бүтээл болохын хувьд энэ template нь эх
төслүүдийн зохиогчийн эрхийн мэдэгдлийг хадгалж, өөрөө MIT License-ийн дор
тараагддаг (`LICENSE`-ийг үз).

---

**Gerege Template Version 27.0** — **Gerege Systems Development Team** болон **Claude AI** хамтран бүтээв, 2026.
