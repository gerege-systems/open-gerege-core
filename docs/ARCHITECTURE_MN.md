# Architecture Overview

> 🌐 [English](ARCHITECTURE.md) · **Монгол**

Энэ баримт нь **Government Template Platform V3.0** (Цахим засаглалыг бүтээх суурь)
— аливаа цахим засаглалын үйлчилгээг дээр нь босгох боломжтой production-д бэлэн
суурийн ерөнхий архитектурыг тайлбарлана. Түүний тэргүүлэх жишиг deployment нь
**Government Template Platform** (**template.dgov.mn** дээр байрласан) — **eID-д суурилсан
төрийн үйлчилгээний платформ** (Government SSO-ийн Relying Party) юм. Backend модуль нь `template`; стек нь **chi (net/http) + pgx
(pgxpool) + PostgreSQL + Redis + Gemini AI**, Clean Architecture зарчмаар зохион
байгуулагдсан бөгөөд Next.js BFF-ээр хучигдсан.

Уг жишиг deployment-д платформ нь нэгэн зэрэг **eID Relying Party** (хэрэглэгч
eID-ээр нэвтэрнэ) бөгөөд **OIDC Identity Provider** (бусад төрийн апп-ууд Ory
Hydra-аар дамжуулан dan-*аар* нэвтэрнэ) болж ажилладаг. PostgreSQL дахь Row-Level
Security нь хэрэглэгч тус бүрийн тусгаарлалтыг үүрдэг гол хамгаалалтын хил юм —
[Row-Level Security](#row-level-security-rls) хэсгийг үз.

> **Эх сурвалж.** Clean Architecture давхаргалал, pgx өгөгдлийн давхарга, кэш,
> observability, тестийн стратеги нь нээлттэй эх төсөл
> [snykk/go-rest-boilerplate](https://github.com/snykk/go-rest-boilerplate)
> (Najib Fikri, MIT)-оос гаралтай. Auth стек, RLS аюулгүй байдлын загвар,
> eID/SSO/OIDC-provider интеграцууд, доорх feature модулиуд нь энэ платформд
> зориулж бүтээгдсэн. MIT уламжлалт бүтээл болохын хувьд эх зохиогчийн эрхийг
> хадгалсан — [Зохиогчид](#credits--license) хэсгийг үз.

## Давхаргын диаграм (Layer Diagram)

```
┌─────────────────────────────────────────────────────────────────┐
│                        HTTP Layer                                 │
│  cmd/api/server → Middleware → internal/http/handlers/v1          │
│  internal/http/{routes, datatransfers, middlewares, auth}         │
│  + internal/provider/{adminapi, adminkeys, devapps, signrelay}    │
├─────────────────────────────────────────────────────────────────┤
│                       Usecase Layer                               │
│  internal/business/usecases/*  (19 bounded contexts)              │
│  (Business logic, validation, orchestration)                      │
├─────────────────────────────────────────────────────────────────┤
│                     Repository Layer                              │
│  internal/datasources/repositories/{interface, postgres}          │
│  (pgx hand-written SQL, RLS transactions, soft-delete, caching)   │
├─────────────────────────────────────────────────────────────────┤
│                       Domain Layer                                │
│  internal/business/domain                                         │
│  (Entities, value objects, business rules)                        │
└─────────────────────────────────────────────────────────────────┘
```

## Feature модулиуд (bounded contexts)

Платформ нь `internal/business/usecases/` дор **19 usecase модулиас** бүрддэг —
тус бүр нь interface + implementation бөгөөд composition root дотор гараар
холбогддог. Boilerplate цөмөөс (`auth`, `users`, `rbac`, `ai`) гадна платформ нь
eID/SSO/төрийн үйлчилгээний гадаргууг нэмдэг:

| Модуль         | Үүрэг |
|----------------|-------|
| `auth`         | **eID нэвтрэлт** (QR / mobile deep-link / РД-аар push + long-poll), **Google OAuth** account холболт, session refresh/logout. Нууц үггүй. |
| `users`        | auth, admin, sign, superadmin-д дахин ашиглагдах хэрэглэгчийн уншилт/бичилт; нэвтрэлтийн lockout; нууц үг солих токены cutoff. |
| `rbac`         | Динамик role + permission каталог, RBAC middleware-ийн permission resolver. |
| `ai`           | Gemini pipeline — function-calling чат, STT/TTS, шууд орчуулга, давхаргат prompt, server-тал tools + мэдлэгийн сан. |
| `org`          | Байгууллага + гишүүнчлэл (eID-тэй холбогдсон; **RLS**). |
| `gov`          | Иргэний "Төрийн үйлчилгээ" портал — хүсэлт, лавлагаа, мэдэгдэл, төлбөр, цаг захиалга (per-user, **RLS**); каталог нийтийн. |
| `gateway`      | API gateway — services / routes / policies + телеметр (service бүр OAuth `scope`-той). |
| `applications` | Нэгдсэн OAuth2 **client бүртгэл** (RP + m2m), **Ory Hydra**-аар дэмжигдсэн — хуучин gateway consumers/API-key болон SSO RP бүртгэлийг нэгтгэнэ; service тус бүрийн хандалт = OAuth scope (`application_services` → `gateway_services.scope`). Админ удирдана (`gateway.manage`), Hydra дээр gated. |
| `core`         | Gerege Core (`core.gerege.mn`) USER FIND / ORG FIND лавлагааны wrap. |
| `provider`     | **OIDC Provider** — **Ory Hydra**-гийн урд талын login/consent/logout цөм; dan өөрөө SSO IdP. |
| `integrations` | Хэрэглэгчийн гуравдагч этгээдийн OAuth (Google Drive/Meet, Dropbox); токеныг **AES-256-GCM шифрлэн** хадгална (**RLS**). |
| `assets`       | Хувь хүний гарын үсгийн зураг + байгууллагын тамга (зураг Google Drive-д, URL DB-д). |
| `gspace`       | Gerege Space — апп-ын өөрийн SFTP хадгалалт, per-user квот (default 2 MB). |
| `audit`        | Persisted **hash-chained, append-only** audit log (admin унших API). |
| `superadmin`   | Админ хэрэглэгчдийг удирдах (үүсгэх / эрх олгох / хасах); мутаци бүр audit log-д бичигдэнэ. |
| `security`     | Security-event ingest (нэвтэрсэн хэрэглэгч бичнэ, admin уншина). |
| `site`         | Сайтын нийтийн харагдацын default (accent / font / density / theme). |
| `sign`         | PDF гарын үсэг (**PAdES**) eidmongolia `/v3`-ээр, серверийн Document-Signer гэрчилгээтэйгээр. |

## Лавлахын бүтэц (Directory Structure)

```
.
├── cmd/
│   ├── api/
│   │   ├── main.go                 # Entry point (config + logger init)
│   │   └── server/server.go        # Composition root (manual DI) — бүх mount энд
│   ├── migration/                  # Migration CLI (зөвхөн SQL; ORM/AutoMigrate БАЙХГҮЙ)
│   └── seed/                       # DB seeder CLI
├── docs/                           # EN/MN docs + OpenAPI spec (swagger.json/yaml, docs.go)
├── internal/
│   ├── apperror/                   # Typed domain errors (→ HTTP status)
│   ├── business/
│   │   ├── domain/                 # Enterprise entities (хамгийн дотоод хүрээ)
│   │   └── usecases/               # 19 bounded contexts (interface + impl)
│   ├── config/                     # Viper-backed config + .env.example
│   ├── constants/                  # Env, logger, error, endpoint constants
│   ├── datasources/
│   │   ├── caches/                 # Redis + Ristretto two-tier cache
│   │   ├── drivers/                # pgx (pgxpool) conn + RLS-enforceability boot guard
│   │   ├── migration/              # SQL migration runner
│   │   ├── records/                # pgx record structs + record↔domain mappers
│   │   ├── rls/                    # Per-request RLS identity-г context.Context-оор зөөнө
│   │   └── repositories/
│   │       ├── interface/          # Gateway abstractions (package _interface)
│   │       └── postgres/*          # pgx implementations (hand-written SQL, withRLS)
│   ├── http/
│   │   ├── auth/                   # CurrentUser from request context
│   │   ├── datatransfers/          # Request / Response DTOs
│   │   ├── handlers/v1/            # HTTP handlers (модуль тус бүр)
│   │   ├── middlewares/            # Global + per-group middleware
│   │   └── routes/                 # Route registration (модуль тус бүр)
│   └── provider/                   # OIDC-provider операторын гадаргуу:
│       ├── adminapi/               #   /admin RP OAuth2-client удирдлага
│       ├── adminkeys/ devapps/     #   admin API key + developer-app store
│       └── signrelay/              #   доод RP-д зориулсан /rp/sign relay
├── migrations/                     # Дугаарласан SQL migrations (N_name.up.sql + .down.sql)
├── pkg/                            # Framework-agnostic client & utility (15 багц)
│   ├── eid/ google/ hydra/         # Identity: eID RP, Google OAuth, Hydra admin
│   ├── xyp/ gspace/ verify/        # XYP байгууллагын лавлагаа, SFTP хадгалалт, GeregeCloud Verify OTP
│   ├── gemini/                     # SDK-гүй Gemini REST (function calling, audio, PCM→WAV)
│   ├── jwt/ logger/ clock/         # JWT, Zap logging, цагийн абстракц
│   ├── helpers/ validators/        # Утилит + struct-tag payload validation
│   ├── audit/                      # Auth-event audit туслах
│   └── observability/              # OTel tracing + Prometheus metrics тохиргоо
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

- **HTTP давхарга** нь **Usecase** интерфейсүүдээс (`auth.Usecase`, `users.Usecase`, …) хамаарна.
- **Usecase давхарга** нь **Repository** интерфейсүүдээс (`_interface.UserRepository`, …) хамаарна — postgres adapter-ээс хэзээ ч биш.
- **Repository давхарга** нь **Domain** entity-үүдээс хамаарна.
- **Domain давхарга** нь зөвхөн стандарт сан + `golang.org/x/crypto/bcrypt`-ийг л import хийдэг — `internal/` эсвэл `pkg/`-ийг хэзээ ч биш.

Үүнийг бүтцийн хувьд баталгаажуулсан: `internal/business/**` болон
`internal/datasources/repositories/**` нь chi/net-http web package-ийг import
хийдэггүй тул business код руу гар хүрэлгүйгээр delivery framework-ийг солих
боломжтой. "Domain нь дотоод юмыг import хийхгүй" дүрмийн нэг санаатай онцгой
тохиолдол: `internal/datasources/rls` leaf багц нь зөвхөн стандарт `context`-оос
хамаардаг бөгөөд import cycle үүсгэлгүйгээр per-request RLS identity-г зөөхийн тулд
гурван давхаргад хуваалцагдана.

## Гол бүрэлдэхүүн хэсгүүд (Key Components)

### 1. HTTP давхарга

**Composition root:** `cmd/api/server/server.go` — цорын ганц гар DI холболтын цэг.
Бүх mount-ыг харахын тулд эхнээс нь дуустал уншина. Энэ нь:
- Tracing, pgx pool (RLS boot guard-тай), Redis/Ristretto, JWT service, бүх гадаад client-ийг (eID, Google, XYP, OIDC/Hydra, Gemini, GeregeCloud Verify, Gerege Space, Gerege Core) эхлүүлнэ.
- repository → usecase → route-ийг гараар холбоно (global singleton, DI container байхгүй).
- chi router-ийг бүтээж, global middleware stack суулгаж, route модуль бүрийг `/api/v1` дор mount хийнэ.
- OIDC-provider гадаргууг (`/admin`, `/rp/sign`) зөвхөн тохиргоо байгаа үед нөхцөлтэйгээр mount хийнэ.
- Graceful shutdown-ийг хариуцна (HTTP, rate limiter, pgx pool, Redis, tracer).

**Routes:** `internal/http/routes/` — модуль тус бүрт нэг файл (`route_auth.go`,
`route_gov.go`, `route_provider.go`, …). Тус бүр `/api` дор `/v1/<module>`-ийг mount хийнэ.

**Handlers:** `internal/http/handlers/v1/` — модуль тус бүрт нэг package. Handler-ийн
гарын үсэг нь `func(w http.ResponseWriter, r *http.Request) error`, `v1.Wrap`-ээр
боогдоно; body-г `v1.DecodeBody`-оор задалж, DTO-г `validators.ValidatePayloads`-аар
шалгаж, хариуг `v1.NewSuccessResponse` / `v1.RespondWithError`-оор буцаана. Handler-ууд
swagger annotation тээнэ.

### 2. Middleware stack

`server.go` дотор дараах дарааллаар суулгагдах global middleware (дараалал чухал —
эхэлд tracing тул request-ID лог-оос өмнө span/`trace_id` тогтоно; Request ID-ийн
дараа шууд Recoverer тул доош урсгалын panic баригдаж, recovery хариунд `request_id`
орно):

1. **Tracing** (`TracingMiddleware`) — хүсэлт тус бүрийн OTel span.
2. **Request ID** (`RequestIDMiddleware`) — `X-Request-ID`-г үүсгэж / context + logger руу дамжуулна.
3. **Recoverer** (`RecovererMiddleware`) — доош урсгалын panic-ийг барьж цэвэр 500 буцаана.
4. **Metrics** (`MetricsMiddleware`) — Prometheus хүсэлтийн тоолуур + latency.
5. **Security Headers** (`SecurityHeadersMiddleware`) — HSTS, CSP, nosniff, frame options, referrer policy.
6. **CORS** (`CORSMiddleware`) — `ALLOWED_ORIGINS`-аас origin (wildcard зөвхөн dev-д).
7. **Body Size Limit** (`BodySizeLimitMiddleware`) — global дээд хязгаар (route тус бүр чанга хязгаартай).
8. **Access Log** (`AccessLogMiddleware`) — бүтэцлэгдсэн нэг мөрийн access log.
9. **Timeout** (`TimeoutMiddleware`) — хүсэлт тус бүрийн deadline (сервер `WriteTimeout` үүнээс урт тул энэ эхэлж ажиллана).

**Бүлэг / route тус бүрийн middleware:**
- **Auth** (`NewAuthMiddleware`) — JWT bearer токеныг баталгаажуулж, `CurrentUser`-ийг context-д хийж, context дээр **RLS identity тогтооно**: admin бол `rls.WithAdmin`, эс бөгөөс `rls.WithUser` (`middleware_auth.go`).
- **Service RLS context** (`ServiceRLSContext`) — нэргүй `/auth` бүлэгт суулгагдаж нэвтрэхээс өмнөх урсгалуудыг (eID upsert, refresh identity хайлт) итгэмжит `service` RLS role дор ажиллуулна (`middleware_rls.go`).
- **RBAC** (`RequirePermission`, `RequireAdmin`, `RequireSuperAdmin`) — auth-ийн дараах declarative эрх олголт; admin permission шалгалтыг давна, `RequireSuperAdmin` нь `/superadmin` гадаргууг хаана. resolver алдаа гарвал fail-closed.
- **Observability gate** (`ObservabilityGate`) — `/metrics` ба `/swagger/doc.json`-г хамгаална ([Ops endpoint-ууд](#ops-endpoint-үүд)-ийг үз).
- **Rate limiter-ууд** — 4 тусдаа limiter: `/auth` ~5/мин, `/ai` ~20/мин (burst 10, орчуулгын stream-д), `/eid/poll` ~60/мин (burst 30, long-poll-д), болон gov/assets/gspace/eID-profile **бичих** ~30/мин (burst 15).

`clientIP()` (`middleware_clientip.go`) нь global middleware БИШ — rate-limit ба
audit-д клиентийн IP-г шийддэг туслах бөгөөд `X-Forwarded-For`-д зөвхөн
`TRUSTED_PROXIES`-аас итгэдэг (fail-safe: өгөгдмөлөөр итгэхгүй).

### 3. Usecase давхарга

**Байршил:** `internal/business/usecases/` — bounded context бүр interface +
implementation-ийг дэлгэнэ. Үүрэг: бизнес дүрмийн validation, repository + кэш +
гадаад client-ийн зохицуулалт (orchestration), `apperror.*` утга буцаах (дотоод
шалтгааныг `apperror.InternalCause`-оор боож library алдаа хэзээ ч client руу
хүрэхгүй). Usecase нь зөвхөн `repositories/interface`-ээс хамаарна, postgres
adapter-ээс хэзээ ч биш.

### 4. Repository давхарга

**Байршил:** `internal/datasources/repositories/` — `interface/` package (`interface`
нь түлхүүр үг тул нэр нь `_interface`) gateway абстракцуудыг агуулна; `postgres/*`
нь тэдгээрийг pgx болон гараар бичсэн SQL-ээр хэрэгжүүлнэ. Гол онцлогууд:

- Query-ууд `ctx`-г шууд авна; мөрүүдийг `pgx.RowToStructByName`-ээр scan хийнэ.
- `deleted_at IS NULL` ил predicate-ээр soft delete.
- `Store` нь нэг round-trip-д `INSERT … RETURNING` ашиглана.
- Давхардсан key-үүдийг pgconn код `23505`-аар илрүүлж `apperror.Conflict` болгоно.
- Per-user repository-ууд query бүрийг **`withRLS` транзакц** дотор ажиллуулж, хүсэлтийн identity-г `SET LOCAL`-scope-той GUC болгон нийтэлнэ ([Row-Level Security](#row-level-security-rls)-ийг үз).

### 5. Domain давхарга

**Байршил:** `internal/business/domain/` — entity-үүд бизнесийн дүрмийг агуулж,
дотоод ямар ч зүйлээс хамаарахгүй. `domain_users.go` нь role загвар болон eID
хэрэглэгчийн constructor (`NewEIDUser` — нууц үггүй, `Active=true`, `civil_id`-ээр
түлхүүрлэсэн)-ыг тодорхойлно. Role тогтмолуудыг [Эрх олголт](#эрх-олголт)-оос үз.

## Танилт (Authentication)

Платформ нь **JWT access + refresh токен** (`pkg/jwt`) олгодог ч **нууц
үгээр нэвтрэх, email/OTP бүртгэл, нууц үг сэргээх зэрэг байхгүй**. Identity нь
зөвхөн гадаад provider-оос ирнэ. Endpoint-ийн хэлбэрийг
[API_CONTRACT.md](API_CONTRACT_MN.md)-ээс үз; route-ууд нь
`internal/http/routes/route_auth.go`, `route_eidprofile.go`-д
бүртгэгддэг.

**1. eID-ээр нэвтрэх (үндсэн арга).** Апп нь eID Mongolia-ийн Relying Party
(`pkg/eid`, `EID_*` тохиргоо):
- `POST /api/v1/auth/eid/start` — session эхлүүлж QR код / mobile deep-link буцаана.
- `POST /api/v1/auth/eid/start-id` — иргэний РД-аар эхлүүлж, бүртгэлтэй төхөөрөмж рүү push хийнэ.
- `POST /api/v1/auth/eid/poll` — frontend **long-poll** хийнэ (~2.5с тутам; IdP-г poll бүрд 25с хүртэл барина) eID session `COMPLETE` болтол. Дуусахад хэрэглэгчийг upsert хийж (`civil_id`-ээр түлхүүрлэнэ; нийтийн RP нь `national_id` биш `civil_id` авдаг) токен хос олгоно.

**2. Google OAuth account холболт** (`pkg/google`, `GOOGLE_*`): `POST
/api/v1/auth/google` нь code-ийг exchange хийж, eID хэрэглэгчид холбогдсон Google
account-аар нэвтрүүлнэ (эсвэл холбоно); `DELETE /api/v1/auth/google/link` салгана.

**Session-ийн амьдралын мөчлөг** (нэвтрэх аргаас үл хамаарна):
- `POST /api/v1/auth/refresh` — токен хосыг сэлгэнэ; credential-солих cutoff-оос өмнө олгогдсон токенуудыг татгалзана (`User.TokensRevokedBefore`). `kind` claim guard нь refresh токеныг access болгон ашиглахаас сэргийлнэ.
- `POST /api/v1/auth/logout` — refresh токеныг хүчингүй болгоно.

> **Тэмдэглэл.** `auth_login.go`, `auth_register.go`, `auth_send_otp.go`,
> `auth_forgot_password.go`, `auth_reset_password.go` зэрэг handler файлууд мод дотор
> үлдсэн ч **ямар ч route-д холбогдоогүй** — `route_auth.go` нь зөвхөн дээрх eID /
> Google / refresh / logout endpoint-уудыг бүртгэдэг.

## Эрх олголт (Authorization)

Эрх олголт хоёр давхаргад хэрэгждэг: HTTP ирмэг дээр **JWT role/permission**, DB
дээр **RLS**.

**Role загвар** (`domain_users.go`; `23_superadmin_role` migration) — зэрэглэлтэй 4
role, `1` = хамгийн дээд:

```go
RoleSuperAdmin = 1  // админ хэрэглэгчдийг удирдана; RequireSuperAdmin-аар хаагдана
RoleAdmin      = 2  // бүх эрх; IsAdmin() true
RoleManager    = 3
RoleUser        = 4  // шинэ eID хэрэглэгчийн default
```

`IsAdmin()` нь `RoleAdmin` **болон** `RoleSuperAdmin` хоёуланд true (super admin нь
admin-ийн JWT/RLS/permission замыг өвлөнө); `IsSuperAdmin()` зөвхөн `RoleSuperAdmin`-д
true. Role ID `0` нь claim-гүй хуучин токенуудын sentinel бөгөөд RBAC middleware
үүнийг `RoleUser` рүү буулгана.

**Динамик RBAC** — role-ийн бүдүүн зэрэглэлээс гадна `rbac.Usecase` нь role-ийн
permission багцыг DB-ээс шийддэг (`8_rbac_roles_permissions` migration).
`RequirePermission(resolver, perm)` нь route-ийг нэрлэсэн permission-оор хаана;
admin давна. Super admin-ыг `SUPERADMIN_EMAIL` (эсвэл DB)-ээс bootstrap хийнэ,
хэзээ ч API-аар биш.

## Row-Level Security (RLS)

RLS нь платформын хэрэглэгч тус бүрийн тусгаарлалтыг үүрдэг гол хамгаалалтын хил —
repository-ийн аль хэдийн бичдэг `WHERE user_id = …` нөхцлийн доор defense-in-depth.
Query-ийн алдаа хүртэл өөр хэрэглэгчийн мөрийг буцааж чадахгүйг баталгаажуулна.

**Context дээрх identity** (`internal/datasources/rls/rls.go`) — leaf багц (зөвхөн
стандарт `context`) нь `Identity{ UserID, Role }`-г зөөнө; `Role` нь SQL policy-ийн
литералтай ЯГ таарах ёстой 3 string тогтмолын нэг:

- `service` — итгэмжит нэвтрэхээс өмнөх / системийн урсгал (eID upsert, refresh identity хайлт, bootstrap). `/auth` дээр `ServiceRLSContext`-оор тавигдана; бүрэн эрх.
- `admin` — бүх мөрд бүрэн хандана. admin JWT-д auth middleware `rls.WithAdmin`-аар тавина.
- `user` — зөвхөн дуудагчийн өөрийн мөр. auth middleware `rls.WithUser`-аар тавина.

**Identity-г нийтлэх** (`…/postgres/users/users_postgres.go`, мөн `org`, `gov`,
`security`, `userintegrations`-т хуулбар) — `withRLS(ctx, fn)` туслах нь
query бүрийг транзакцид боож дараахыг ажиллуулна:

```go
SELECT set_config('app.user_id',   $1, true),   -- is_local = true ⇒ SET LOCAL семантик
       set_config('app.user_role',  $2, true)
```

`set_config(..., true)` нь утгыг транзакцид scope хийдэг тул identity нь pool дахь
холболтуудаар алдагдахгүй. Context-д identity **байхгүй** үед хоёр GUC хоосон болно —
хоосон `app.user_role` нь ямар ч policy-д таарахгүй тул бүх мөр нуугдаж, бүх бичилт
татгалзагдана (**fail-closed**). `audit` repository role-only хувилбар ашигладаг.

**Хүснэгт тус бүрийн policy** — RLS-тэй хүснэгт бүр `ENABLE` **болон** `FORCE ROW
LEVEL SECURITY` ашиглана (FORCE нь хүснэгтийн эзэнд ч RLS-ийг хэрэгжүүлнэ). Policy-ууд
permissive (OR) бөгөөд ижил 3 GUC role-ийг таньдаг. `user` policy нь `user_id =
NULLIF(current_setting('app.user_id', true), '')::uuid`-ээр хаадаг (`NULLIF` нь хоосон
GUC-ийг `NULL` болгож cast алдаанаас сэргийлж мөрийг зүгээр л хасна):

| Migration | Хүснэгт(үүд) | RLS |
|-----------|--------------|-----|
| `7_enable_rls_users`      | `users`                                                                     | ENABLE + FORCE; service / admin / self |
| `14_organizations`        | `organizations`, `organization_memberships`                                 | ENABLE + FORCE; **гишүүнчлэлээр** харагдац |
| `17_org_rls_recursion_fix`| (org policy-уудыг дахин үүсгэнэ)                                             | policy рекурс (SQLSTATE 42P17)-ыг таслах `SECURITY DEFINER` `app_is_org_member()` ашиглана |
| `20_gov_services`         | `gov_applications`, `gov_references`, `gov_notifications`, `gov_payments`, `gov_appointments` | ENABLE + FORCE; service / admin / self. (`gov_services` каталог нийтийн, RLS-гүй) |
| `21_user_integrations`    | `user_integrations`                                                         | ENABLE + FORCE; service / admin / self |

Нийтийн config хүснэгтүүд санаатайгаар **RLS-гүй**; тэдгээрийн DB backstop нь
`app_user` role-ийн эсрэг хүснэгтийн эрхийн `REVOKE` юм
(`17_least_privilege_config_grants` — `permissions` / `role_permissions` /
`ai_prompts` / `ai_knowledge`; `27_site_appearance` — singleton appearance мөр).
Provider хүснэгтүүд (`26_sso_provider`: `developer_apps`, `admin_api_keys`,
`login_events`) болон `org_stamps` (`25`) мөн RLS-гүй, usecase/handler давхаргад
хамгаалагдана.

**Boot үеийн enforceability guard** — RLS-ийг Postgres superuser болон `BYPASSRLS`
role чимээгүй алгасдаг тул `guardRLSEnforceable`
(`internal/datasources/drivers/driver_pgx.go`) нь эхлэлд холбогдож буй role-ийн
`pg_roles`-ийг шалгана:

- Role-д `rolsuper` эсвэл `rolbypassrls` байвал: **production fail-closed** (boot зогсоно, pool хаагдана); **development анхааруулга** логоод үргэлжилнэ (migrate/тест superuser хэрэглэж болно).
- Иймд api нь production-д least-privilege non-superuser role-оор (жишээ `app_user`) холбогдох ёстой. (Compose стек санаатайгаар `ENVIRONMENT=development` ажилладаг тул guard зөвхөн production-д хатуу унагана.)

## OIDC Provider (Ory Hydra)

Платформ өөрөө **Identity Provider** болж чадна: бусад төрийн апп-ууд **Ory Hydra**-аар
дамжуулан нэвтрэлтээ dan-д даалгадаг. Энэ гадаргуу нь зөвхөн `ProviderConfigured()`
true үед идэвхжинэ (`HYDRA_ADMIN_URL` + `HYDRA_PUBLIC_URL` + `SSO_STATE_KEY ≥ 32
байт`); эс бөгөөс inert бөгөөс route нь огт бүртгэгдэхгүй.

- **Login / consent / logout цөм** — `usecases/provider` + `pkg/hydra` нь Hydra-гийн challenge-ийг зохицуулна; first-party client-ууд (`SSO_FIRSTPARTY_CLIENTS`) consent UI-г алгасна. `/api/v1/provider` дор mount.
- **Applications (нэгдсэн client бүртгэл)** — `usecases/applications` (`/api/v1/applications` дор mount, `gateway.manage`-ээр хамгаалагдсан) нь OAuth2 client бүртгэх одоогийн арга: RP "Login with DAN" апп-ууд (`web`/`spa`/`native` → `authorization_code`; `spa`/`native` нь public, PKCE, secret-гүй) болон m2m client-ууд (`client_credentials`). Тус бүр нь Hydra OAuth2 client бөгөөс scope нь зөвшөөрсөн gateway service-үүд (`application_services` → `gateway_services.scope`); confidential `client_secret` нь create/rotate үед нэг удаа харагдана.
- **Операторын гадаргуу (legacy)** — `internal/provider/adminapi` нь **`/admin`** дор (`http.StripPrefix`-ээр) RP OAuth2-client бүртгэл/удирдлагад mount; `devapps` (`developer_apps`) store болон `adminkeys` (bootstrap key нь `SSO_ADMIN_API_KEYS`-аас, SHA-256-аар тааруулна)-аар дэмжигдэнэ. Энэ admin-API-key операторын гадаргуу болон `developer_apps` overlay нь хэвээр байгаа ч шинэ ажилд **нэгдсэн Applications загвараар орлогдсон**.
- **Sign relay** — `internal/provider/signrelay` нь **`/rp/sign/*`** дор mount; доод RP-үүд dan-ий eidmongolia RP credential-ээр *дамжин* eID PDF гарын үсэг зурах reverse proxy (`SIGN_RELAY_TOKEN` + `EID_RP_SECRET`-ээр идэвхжинэ).

> **Enforcement caveat (хэрэгжүүлэлтийн анхааруулга).** Апп-д service оноох нь тухайн
> client-ийн OAuth **scope**-ыг тохируулна — энэ нь зөвхөн бүртгэл/тохиргоо.
> *Runtime* дахь хүсэлт тус бүрийн шалгалтад танилцуулсан токеныг
> (`hydra.Admin.Introspect` байдаг) route бүрийн service scope-той тулгаж
> introspect хийдэг gateway proxy хэрэгтэй бөгөөд тэр proxy **одоогоор
> байхгүй**. Тиймээс өнөөдөр service оноолт нь амьд authorization биш — үүнийг
> хэрэгжсэн authz гэж бүү андуур.

## Өгөгдлийн сан (Database)

- **Driver:** pgx v5 (`github.com/jackc/pgx/v5` + pgxpool), гараар бичсэн SQL — **ORM-гүй**.
- **Database:** PostgreSQL, **Row-Level Security**-г хэрэглэгч тус бүрийн хил болгосон.
- **Migrations:** `migrations/` доторх дугаарласан SQL файлууд (`N_name.up.sql` + `.down.sql`), `migrate` compose service / `cmd/migration`-оор хэрэгжинэ. **AutoMigrate байхгүй** — schema нь зөвхөн `*.up.sql` файлуудаас гарна (`cmd/migration/main.go`).
- **Tracing:** pgx pool instrumentation (`otelpgx`)-аар OpenTelemetry.

> **Migration дугаарлалтын мөргөлдөөн.** Хоёр migration `17_` prefix-ийг хуваалцана:
> `17_least_privilege_config_grants` болон `17_org_rls_recursion_fix`. Тэдгээр нь
> хамааралгүй бөгөөд хоёул хэрэгжинэ; runner нь дугаарласан файлуудыг эрэмбэлдэг тул
> `18_`-аас дээш migration нэмэх эсвэл хэрэгжих дарааллыг тооцоолохдоо үүнийг санана уу.

### Холболтын удирдлага (Connection Management)

Pool нь env-ээс тохируулагдана (`internal/datasources/drivers/driver_pgx.go`,
`SetupPgxPostgres`):

```go
poolCfg.MaxConns        = cfg.MaxConns    // DB_MAX_OPEN_CONNS   (default 25)
poolCfg.MinConns        = cfg.MinConns    // DB_MAX_IDLE_CONNS   (default 5)
poolCfg.MaxConnLifetime = cfg.MaxLifetime // DB_CONN_MAX_LIFE_MINS (default 15)
```

Production нь TLS-баталгаажсан DSN шаардана (`sslmode=verify-full` эсвэл `verify-ca`)
— config guard-аар хэрэгжинэ.

## Observability

### Logging
- **Сан:** Zap (бүтэцлэгдсэн), `pkg/logger`-ээр. production-д JSON, development-д console. Request ID + trace ID нь `*WithContext` туслахуудаар дамжина.

### Metrics
- **Сан:** Prometheus, endpoint `GET /metrics` (хаалттай — [Ops endpoint-ууд](#ops-endpoint-үүд)-ийг үз). HTTP хүсэлтийн тоолуур/latency, давхарга бүрийн кэш hit/miss/error, OTP илгээлтийн үр дүн, pgx pool-ийн бодит статистик.

### Tracing
- **Сан:** OpenTelemetry; exporter-ийг `OTEL_EXPORTER`-оор сонгоно (хоосон = noop, `stdout`, эсвэл `otlp`), sampling-ийг `OTEL_SAMPLE_RATIO`-оор.

## Ops endpoint-үүд

| Endpoint | Хандалт |
|----------|---------|
| `GET /health` | Нээлттэй — liveness (load balancer / orchestrator-т). |
| `GET /ready`  | Нээлттэй — readiness: DB ping (pgx pool) + Redis probe. |
| `GET /metrics` | **Хаалттай** `ObservabilityGate`-аар. |
| `GET /swagger/doc.json` | **Хаалттай** `ObservabilityGate`-аар. |

`ObservabilityGate` (`middleware_observability_gate.go`) нь операторын мэдрэмжтэй 2
endpoint-ийг хамгаална: **development**-д үргэлж нээлттэй; **production**-д
`Authorization: Bearer <OBSERVABILITY_TOKEN>` (constant-time харьцуулна) шаардаж,
аливаа таарахгүй эсвэл `OBSERVABILITY_TOKEN` хоосон үед **404** (401 биш) буцаана —
ингэснээр endpoint-ийн оршин байгаа нь reconnaissance-аас нуугдана.

## Аюулгүй байдлын онцлогууд (Security Features)

| Онцлог            | Хэрэгжүүлэлт                             | Байршил                                    |
|-------------------|-----------------------------------------|--------------------------------------------|
| Row-Level Security| per-user DB тусгаарлалт + boot guard     | `datasources/rls/`, `drivers/driver_pgx.go`, migration `7/14/20/21` |
| Танилт (identity) | eID RP + Google OAuth                    | `usecases/auth`, `pkg/{eid,google}`        |
| Эрх олголт        | 4-role загвар + динамик RBAC             | `domain_users.go`, `middlewares/middleware_rbac.go` |
| Security headers  | HSTS, CSP, nosniff, frame options        | `middlewares/middleware_security.go`       |
| CORS              | env whitelist, wildcard зөвхөн dev       | `middlewares/middleware_cors.go`           |
| Rate limiting     | per-IP (auth / ai / poll / gov-write)    | `middlewares/middleware_ratelimit.go`      |
| Body size limit   | global + `/auth` дээр чанга хязгаар       | `middlewares/middleware_bodysizelimit.go`  |
| Ops-endpoint gate | bearer token, prod-д 404                 | `middlewares/middleware_observability_gate.go` |
| Input validation  | `validate:` struct tag                   | `internal/http/datatransfers/requests/`    |
| Шифрлэсэн нууц     | AES-256-GCM OAuth токен                   | `usecases/integrations` (`INTEGRATION_ENC_KEY`) |
| SQL injection     | pgx (parameterized query)                | `internal/datasources/repositories/`       |
| PDF гарын үсэг     | PAdES, серверийн Document-Signer гэрчилгээ| `usecases/sign` (`SIGN_SIGNER_*`)          |

## API дизайн (API Design)

Бүх API route нь `/api/v1` дор; модуль тус бүр `/v1/<module>`-ийг mount хийнэ:
`auth`, `users`, `users/me/eid`, `rbac`, `org`, `gov`, `integrations`, `assets`,
`gspace`, `gateway`, `core`, `sso`, `admin`, `superadmin`, `ai`, `audit`,
`security`, `site`, `sign`, болон (Hydra тохируулагдсан үед) `provider` +
`applications`. Infra endpoint
(`/health`, `/ready`, `/metrics`, `/swagger`) болон provider гадаргуу (`/admin`,
`/rp/sign`) нь root дээр байрлана. **Endpoint-ийн бүрэн хүснэгтийг
[API_CONTRACT.md](API_CONTRACT_MN.md)** болон үүсгэсэн OpenAPI spec (`/swagger`)-ээс үз.

### Хариуны формат (Response Format)

Нэг envelope (`internal/http/handlers/v1/handler_base_response.go`):

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
  "data": { "errors": { "national_id": "national_id is required" } }, "request_id": "…" }
```

Domain алдаанууд (`internal/apperror`) нь статус кодуудад буудаг: NotFound→404,
Unauthorized→401, Forbidden→403, Conflict→409, BadRequest→400, Internal→500.
5xx-ийн шалтгаануудыг log-д бичиж, body дотор ерөнхий мессежээр сольдог.

## Тестийн стратеги (Testing Strategy)

- **Unit тестүүд** — usecase + handler давхаргуудыг mockery mock-уудаар (`internal/test/mocks/`). Хурдан, Docker-гүй. `go test ./...`.
- **Integration тестүүд** — repository-уудыг (RLS policy-уудыг оруулаад) testcontainers-go-оор жинхэнэ Postgres + Redis-ийн эсрэг (`internal/test/testenv/`). `make test-integration`.
- **Mock-ууд** — mockery-ээр үүсгэгдсэн. `make mock interface=… dir=… filename=…`.
- **Authz matrix** — `routes/routes_authz_matrix_test.go` нь route бүр дээрх auth/permission gate-ийг батална.

## Тохиргоо (Configuration)

Viper нь `.env` / environment-аас ачаална (`internal/config/config.go`;
`internal/config/.env.example`-ийг үз). Config guard нь production-ийн шаардлагуудыг
(TLS DSN, `ALLOWED_ORIGINS`, `VERIFY_API_KEY`, JWT secret урт) хэрэгжүүлнэ. Сонгосон
key-үүд:

| Бүлэг | Variable-ууд |
|-------|--------------|
| **Server** | `PORT`, `ENVIRONMENT` (`development`/`production`), `DEBUG` |
| **Database** | `DB_POSTGRE_DRIVER`, `DB_POSTGRE_DSN` (dev), `DB_POSTGRE_URL` (prod; `sslmode=verify-full`/`verify-ca`), `DB_MAX_OPEN_CONNS` (25), `DB_MAX_IDLE_CONNS` (5), `DB_CONN_MAX_LIFE_MINS` (15) |
| **JWT** | `JWT_SECRET` (≥32), `JWT_EXPIRED` (ц, 1–24), `JWT_ISSUER`, `JWT_REFRESH_EXPIRED` (өдөр, 7) |
| **Redis** | `REDIS_HOST`, `REDIS_PASS`, `REDIS_EXPIRED` (мин) |
| **Crypto** | `BCRYPT_COST` (12) |
| **Verify (OTP)** | `OTP_MAX_ATTEMPTS` (5), `VERIFY_API_BASE`, `VERIFY_API_KEY` (prod заавал), `VERIFY_CHANNEL` |
| **eID** | `EID_BASE_URL` (`…/v3`), `EID_RP_UUID`, `EID_RP_NAME`, `EID_RP_SECRET`, `EID_CERT_LEVEL` (ADVANCED), `EID_CALLBACK_URL`, `EID_DISPLAY_TEXT`, `SIGN_RELAY_TOKEN` |
| **Sign** | `SIGN_SIGNER_CERT_FILE`, `SIGN_SIGNER_KEY_FILE` (prod fail-closed) |
| **Google OAuth** | `GOOGLE_CLIENT_ID`, `GOOGLE_CLIENT_SECRET` |
| **XYP** | `XYP_API_BASE` (`https://xyp.dgov.mn`), `XYP_CLIENT_ID`, `XYP_CLIENT_SECRET` |
| **Gerege Space** | `GSPACE_HOST`, `GSPACE_PORT` (22), `GSPACE_USER`, `GSPACE_PASSWORD`, `GSPACE_BASE_PATH` (gerege-space), `GSPACE_QUOTA_BYTES` (2 MB) |
| **Gemini AI** | `GEMINI_API_KEY`, `GEMINI_MODEL`, `GEMINI_TTS_MODEL`, `GEMINI_VOICE`, `GEMINI_API_BASE`, `AI_SCOPE_PROMPT` |
| **Gerege Core** | `CORE_API_BASE` (`https://core.gerege.mn`), `CORE_API_TOKEN` |
| **Integrations** | `INTEGRATION_ENC_KEY` (AES-256-GCM; prod заавал) |
| **OIDC Provider (Hydra)** | `HYDRA_ADMIN_URL` (`http://hydra:4445`), `HYDRA_PUBLIC_URL`, `SSO_STATE_KEY` (≥32), `SSO_FIRSTPARTY_CLIENTS`, `SSO_ADMIN_API_KEYS`, `SSO_ADMIN_SUBS` |
| **Observability** | `OTEL_EXPORTER` (``/`stdout`/`otlp`), `OTEL_SAMPLE_RATIO`, `OBSERVABILITY_TOKEN` |
| **Networking** | `ALLOWED_ORIGINS` (prod заавал), `TRUSTED_PROXIES` |
| **Bootstrap** | `SUPERADMIN_EMAIL` |

## Deployment

```bash
go build ./...                 # build
docker compose up -d --build   # db + redis + migrate (one-off) + api + web
```

Health check: `curl http://localhost:8080/health`. deployment топологийг
`docs/DEPLOYMENT.md`-ээс үз.

## Credits & License

Энэ платформ нь нээлттэй эх кодын ажил дээр тулгуурласан:

| Project | Author | License | What we used |
|---------|--------|---------|--------------|
| [snykk/go-rest-boilerplate](https://github.com/snykk/go-rest-boilerplate) | Najib Fikri | MIT | Clean Architecture давхаргалал, кэш, observability, тестийн стратеги |

Delivery давхаргыг **Gin → chi (net/http)**, өгөгдлийн давхаргыг **sqlx → pgx
(pgxpool)** болгож хөрвүүлсэн; auth стек, RLS аюулгүй байдлын загвар,
eID/SSO/OIDC-provider интеграцууд, feature модулиудыг энэ платформд зориулж
бүтээсэн. MIT уламжлалт бүтээл болохын хувьд эх зохиогчийн эрхийн мэдэгдлийг хадгалж,
энэ код MIT License-ийн дор тараагдана (`LICENSE`-ийг үз).

---

**Government Template Platform V3.0** — **Gerege Systems Development Team** болон **Claude AI** хамтран бүтээв, 2026.
