# Architecture Overview

> 🌐 **English** · [Монгол](ARCHITECTURE_MN.md)

This document describes the high-level architecture of the **Government Template
Platform V3.0** (Цахим засаглалыг бүтээх суурь) — a production-ready foundation on
which any digital-government service can be built. Its flagship reference
deployment is **Government Template Platform** (at **template.dgov.mn**), an **eID-based
government service platform** — a Relying Party of Government SSO. The backend module is `template`; the stack is **chi (net/http)
+ pgx (pgxpool) + PostgreSQL + Redis + Gemini AI**, organized along Clean
Architecture lines and fronted by a Next.js BFF.

In that reference deployment the platform serves as both an **eID Relying Party**
(users log in with eID) and an **OIDC Identity Provider** (other government apps
log in *through* it via the built-in Go provider). Row-Level Security in PostgreSQL is the
load-bearing per-user isolation boundary — see
[Row-Level Security](#row-level-security-rls).

> **Origin.** The Clean-Architecture layering, pgx data layer, caching,
> observability, and test strategy descend from the open-source
> [snykk/go-rest-boilerplate](https://github.com/snykk/go-rest-boilerplate) by
> Najib Fikri (MIT). The auth stack, RLS security model, eID/SSO/OIDC-provider
> integrations, and the feature modules below were built for this platform. As an
> MIT derivative the upstream copyright is retained — see
> [Credits](#credits--license).

## Layer Diagram

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

## Feature Modules (bounded contexts)

The platform is composed of **19 usecase modules** under
`internal/business/usecases/`, each an interface + implementation wired by hand in
the composition root. Beyond the boilerplate core (`auth`, `users`, `rbac`, `ai`)
the platform adds the eID/SSO/government-service surface:

| Module         | Responsibility |
|----------------|----------------|
| `auth`         | **eID login** (QR / mobile deep-link / national-ID push + long-poll), **Google OAuth** account-linking, session refresh/logout. No passwords. |
| `users`        | User reads/writes reused by auth, admin, sign, superadmin; login lockout; password-change token cutoff. |
| `rbac`         | Dynamic roles + permissions catalogue and the permission resolver used by RBAC middleware. |
| `ai`           | Gemini pipeline — function-calling chat, STT/TTS, live translation, layered prompts, server-side tools + knowledge base. |
| `org`          | Organizations + memberships (eID-linked; **RLS**). |
| `gov`          | Citizen "Government services" portal — applications, references, notifications, payments, appointments (per-user, **RLS**) over a public service catalogue. |
| `gateway`      | API gateway — services / routes / policies + telemetry (each service carries an OAuth `scope`). |
| `applications` | Unified OAuth2 **client registry** (RP + m2m) backed by **Ory Hydra** — merges the old gateway consumers/API-keys and the SSO RP registration; per-service access = OAuth scopes (`application_services` → `gateway_services.scope`). Admin-managed (`gateway.manage`), gated on Hydra. |
| `core`         | Gerege Core (`core.gerege.mn`) USER FIND / ORG FIND lookup wrapper. |
| `provider`     | **OIDC Provider** — login/consent/logout core in front of **Ory Hydra**; dan is itself an SSO IdP. |
| `integrations` | User third-party OAuth (Google Drive/Meet, Dropbox); tokens stored **AES-256-GCM encrypted** (**RLS**). |
| `assets`       | Personal signature image + organization stamp (images to Google Drive, URL in DB). |
| `gspace`       | Gerege Space — the app's own SFTP storage, per-user quota (default 2 MB). |
| `audit`        | Persisted **hash-chained, append-only** audit log (admin read API). |
| `superadmin`   | Manage admin users (create / grant / revoke); every mutation written to the audit log. |
| `security`     | Security-event ingest (authenticated users write, admin reads). |
| `site`         | Site-wide appearance defaults (accent / font / density / theme). |
| `sign`         | PDF signing (**PAdES**) via eidmongolia `/v3` with a server-held Document-Signer certificate. |

## Directory Structure

```
.
├── cmd/
│   ├── api/
│   │   ├── main.go                 # Entry point (config + logger init)
│   │   └── server/server.go        # Composition root (manual DI) — reads all mounts here
│   ├── migration/                  # Migration CLI (SQL only; NO ORM/AutoMigrate)
│   └── seed/                       # DB seeder CLI
├── docs/                           # EN/MN docs + OpenAPI spec (swagger.json/yaml, docs.go)
├── internal/
│   ├── apperror/                   # Typed domain errors (→ HTTP status)
│   ├── business/
│   │   ├── domain/                 # Enterprise entities (innermost circle)
│   │   └── usecases/               # 19 bounded contexts (interface + impl)
│   ├── config/                     # Viper-backed config + .env.example
│   ├── constants/                  # Env, logger, error, endpoint constants
│   ├── datasources/
│   │   ├── caches/                 # Redis + Ristretto two-tier cache
│   │   ├── drivers/                # pgx (pgxpool) conn + RLS-enforceability boot guard
│   │   ├── migration/              # SQL migration runner
│   │   ├── records/                # pgx record structs + record↔domain mappers
│   │   ├── rls/                    # Per-request RLS identity carried on context.Context
│   │   └── repositories/
│   │       ├── interface/          # Gateway abstractions (package _interface)
│   │       └── postgres/*          # pgx implementations (hand-written SQL, withRLS)
│   ├── http/
│   │   ├── auth/                   # CurrentUser from request context
│   │   ├── datatransfers/          # Request / Response DTOs
│   │   ├── handlers/v1/            # HTTP handlers (per module)
│   │   ├── middlewares/            # Global + per-group middleware
│   │   └── routes/                 # Route registration (per module)
│   └── provider/                   # OIDC-provider operator surfaces:
│       ├── adminapi/               #   /admin RP OAuth2-client management
│       ├── adminkeys/ devapps/     #   admin API keys + developer-app store
│       └── signrelay/              #   /rp/sign relay for downstream RPs
├── migrations/                     # Numbered SQL migrations (N_name.up.sql + .down.sql)
├── pkg/                            # Framework-agnostic clients & utilities (15 packages)
│   ├── eid/ google/ hydra/         # Identity: eID RP, Google OAuth, Hydra admin
│   ├── xyp/ gspace/ verify/        # XYP org registry, SFTP storage, GeregeCloud Verify OTP
│   ├── gemini/                     # SDK-free Gemini REST (function calling, audio, PCM→WAV)
│   ├── jwt/ logger/ clock/         # JWT, Zap logging, time abstraction
│   ├── helpers/ validators/        # Utilities + struct-tag payload validation
│   ├── audit/                      # Auth-event audit helpers
│   └── observability/              # OTel tracing + Prometheus metrics setup
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

- **HTTP Layer** depends on **Usecase** interfaces (`auth.Usecase`, `users.Usecase`, …).
- **Usecase Layer** depends on **Repository** interfaces (`_interface.UserRepository`, …), never on postgres adapters.
- **Repository Layer** depends on **Domain** entities.
- **Domain Layer** imports only the standard library + `golang.org/x/crypto/bcrypt` — never `internal/` or `pkg/`.

This is verified structurally: `internal/business/**` and
`internal/datasources/repositories/**` import **no** chi/net-http web package, so
the delivery framework can be swapped without touching business code. One
exception to "domain imports nothing internal" is deliberate: the leaf package
`internal/datasources/rls` depends only on stdlib `context` and is shared across
all three layers to carry per-request RLS identity without an import cycle.

## Key Components

### 1. HTTP Layer

**Composition root:** `cmd/api/server/server.go` — the single manual-DI wiring
point. Read it end-to-end to see every mount. It:
- Initializes tracing, the pgx pool (with the RLS boot guard), Redis/Ristretto, the JWT service, and every external client (eID, Google, XYP, OIDC/Hydra, Gemini, GeregeCloud Verify, Gerege Space, Gerege Core).
- Wires repositories → usecases → routes by hand (no global singletons, no DI container).
- Builds the chi router, installs the global middleware stack, and mounts each route module under `/api/v1`.
- Conditionally mounts the OIDC-provider surfaces (`/admin`, `/rp/sign`) only when their config is present.
- Owns graceful shutdown (drains HTTP, rate limiters, pgx pool, Redis, tracer).

**Routes:** `internal/http/routes/` — one file per module (`route_auth.go`,
`route_gov.go`, `route_provider.go`, …). Each mounts `/v1/<module>` under `/api`.

**Handlers:** `internal/http/handlers/v1/` — one package per module. Handler
signature is `func(w http.ResponseWriter, r *http.Request) error`, wrapped by
`v1.Wrap`; bodies decoded with `v1.DecodeBody`, DTOs validated by
`validators.ValidatePayloads`, responses via `v1.NewSuccessResponse` /
`v1.RespondWithError`. Handlers carry swagger annotations.

### 2. Middleware Stack

Global middleware, applied in this order in `server.go` (order matters — tracing
first so a span/`trace_id` exists before request-ID logging; Recoverer right after
Request ID so panics downstream are caught and the recovery response carries a
`request_id`):

1. **Tracing** (`TracingMiddleware`) — per-request OTel span.
2. **Request ID** (`RequestIDMiddleware`) — generates / propagates `X-Request-ID` into context + logger.
3. **Recoverer** (`RecovererMiddleware`) — catches downstream panics, returns a clean 500.
4. **Metrics** (`MetricsMiddleware`) — Prometheus request counters + latency.
5. **Security Headers** (`SecurityHeadersMiddleware`) — HSTS, CSP, nosniff, frame options, referrer policy.
6. **CORS** (`CORSMiddleware`) — origins from `ALLOWED_ORIGINS` (wildcard only in dev).
7. **Body Size Limit** (`BodySizeLimitMiddleware`) — global ceiling (per-route tighter caps).
8. **Access Log** (`AccessLogMiddleware`) — structured one-line access log.
9. **Timeout** (`TimeoutMiddleware`) — per-request deadline (server `WriteTimeout` is set longer so it can fire first).

**Per-group / per-route middleware:**
- **Auth** (`NewAuthMiddleware`) — validates the JWT bearer token, stashes `CurrentUser` in context, and **sets the RLS identity** on the context: `rls.WithAdmin` for admins, `rls.WithUser` otherwise (`middleware_auth.go`).
- **Service RLS context** (`ServiceRLSContext`) — installed on the anonymous `/auth` group so pre-auth flows (eID upsert, refresh identity lookup) run under the trusted `service` RLS role (`middleware_rls.go`).
- **RBAC** (`RequirePermission`, `RequireAdmin`, `RequireSuperAdmin`) — declarative authorization after auth; admins bypass permission checks, `RequireSuperAdmin` gates the `/superadmin` surface. Fail-closed on resolver error.
- **Observability gate** (`ObservabilityGate`) — guards `/metrics` and `/swagger/doc.json` (see [Ops Endpoints](#ops-endpoints)).
- **Rate limiters** — four separate limiters: `/auth` ~5/min, `/ai` ~20/min (burst 10, for translation streams), `/eid/poll` ~60/min (burst 30, for long-poll), and gov/assets/gspace/eID-profile **writes** ~30/min (burst 15).

`clientIP()` (`middleware_clientip.go`) is a helper — not a global middleware —
that resolves the client IP for rate-limiting and audit, trusting
`X-Forwarded-For` only from `TRUSTED_PROXIES` (fail-safe: no trust by default).

### 3. Usecase Layer

**Location:** `internal/business/usecases/` — each bounded context exposes an
interface + an implementation. Responsibilities: business-rule validation,
orchestration of repository + cache + external clients, and returning `apperror.*`
values (wrapping internal causes with `apperror.InternalCause` so library errors
never reach clients). Usecases depend only on `repositories/interface`, never on
postgres adapters.

### 4. Repository Layer

**Location:** `internal/datasources/repositories/` — the `interface/` package
(named `_interface`, since `interface` is a keyword) holds gateway abstractions;
`postgres/*` implements them with pgx and hand-written SQL. Key features:

- Queries take `ctx` directly; rows scanned with `pgx.RowToStructByName`.
- Soft delete via explicit `deleted_at IS NULL` predicates.
- `Store` uses single round-trip `INSERT … RETURNING`.
- Duplicate keys detected via pgconn error code `23505` → `apperror.Conflict`.
- Per-user repositories run each query inside a **`withRLS` transaction** that
  publishes the request identity as `SET LOCAL`-scoped GUCs (see
  [Row-Level Security](#row-level-security-rls)).

### 5. Domain Layer

**Location:** `internal/business/domain/` — entities carry business rules and
depend on nothing internal. `domain_users.go` defines the role model and the eID
user constructor (`NewEIDUser` — passwordless, `Active=true`, keyed on `civil_id`).
See [Authorization](#authorization) for the role constants.

## Authentication

The platform issues **JWT access + refresh tokens** (`pkg/jwt`) but has **no
password login, no email/OTP registration, and no password reset**. Identity comes
only from external providers. Endpoint shapes are documented in
[API_CONTRACT.md](API_CONTRACT.md); routes are registered in
`internal/http/routes/route_auth.go` and `route_eidprofile.go`.

**1. Login with eID (the primary method).** The app is a Relying Party of eID
Mongolia (`pkg/eid`, `EID_*` config):
- `POST /api/v1/auth/eid/start` begins a session and returns a QR code / mobile deep-link.
- `POST /api/v1/auth/eid/start-id` starts by national-ID (реестр), pushing to the citizen's registered device.
- `POST /api/v1/auth/eid/poll` is **long-polled** by the frontend (~every 2.5 s; the IdP is held up to 25 s per poll) until the eID session reaches `COMPLETE`. On completion the user is upserted (keyed on `civil_id`; public RPs receive `civil_id`, not `national_id`) and a token pair is issued.

**2. Google OAuth account-linking** (`pkg/google`, `GOOGLE_*`): `POST
/api/v1/auth/google` exchanges the code and links (or logs in via) a Google account
attached to the eID user; `DELETE /api/v1/auth/google/link` unlinks.

**Session lifecycle** (independent of the login method):
- `POST /api/v1/auth/refresh` rotates the token pair; tokens issued before a credential-change cutoff are rejected (`User.TokensRevokedBefore`). A `kind` claim guard prevents using a refresh token as an access token.
- `POST /api/v1/auth/logout` revokes the refresh token.

> **Note.** Handler files such as `auth_login.go`, `auth_register.go`,
> `auth_send_otp.go`, `auth_forgot_password.go`, and `auth_reset_password.go`
> still exist in the tree but are **not wired to any route** — `route_auth.go`
> registers only the eID / Google / refresh / logout endpoints above.

## Authorization

Authorization is enforced at two layers: **JWT role/permission** at the HTTP edge
and **RLS** at the database.

**Role model** (`domain_users.go`; migration `23_superadmin_role`) — four ranked
roles, `1` = highest:

```go
RoleSuperAdmin = 1  // manages admin users; gated by RequireSuperAdmin
RoleAdmin      = 2  // full access; IsAdmin() true
RoleManager    = 3
RoleUser        = 4  // default for new eID users
```

`IsAdmin()` returns true for `RoleAdmin` **and** `RoleSuperAdmin` (super admin
inherits the admin JWT/RLS/permission paths); `IsSuperAdmin()` is true only for
`RoleSuperAdmin`. Role ID `0` is a sentinel for legacy claim-less tokens and is
downgraded to `RoleUser` by the RBAC middleware.

**Dynamic RBAC** — beyond the coarse role rank, `rbac.Usecase` resolves a role's
permission set from the database (migration `8_rbac_roles_permissions`).
`RequirePermission(resolver, perm)` gates a route on a named permission; admins
bypass. Super admin is bootstrapped from `SUPERADMIN_EMAIL` (or by DB), never via
API.

## Row-Level Security (RLS)

RLS is the platform's load-bearing per-user isolation boundary — defense-in-depth
beneath the `WHERE user_id = …` clauses the repositories already write. It ensures
that even a query bug cannot return another user's rows.

**Identity on the context** (`internal/datasources/rls/rls.go`) — a leaf package
(stdlib `context` only) carries an `Identity{ UserID, Role }` where `Role` is one
of three string constants that **must** match the SQL policy literals:

- `service` — trusted pre-auth / system flows (eID upsert, refresh identity lookup, bootstrap). Set by `ServiceRLSContext` on `/auth`; full access.
- `admin` — full access to every row. Set by the auth middleware via `rls.WithAdmin` for admin JWTs.
- `user` — only the caller's own rows. Set by the auth middleware via `rls.WithUser`.

**Publishing the identity** (`…/postgres/users/users_postgres.go`, and copies in
`org`, `gov`, `security`, `userintegrations`) — the `withRLS(ctx, fn)`
helper wraps each query in a transaction and runs:

```go
SELECT set_config('app.user_id',   $1, true),   -- is_local = true ⇒ SET LOCAL semantics
       set_config('app.user_role',  $2, true)
```

`set_config(..., true)` scopes the values to the transaction so identity cannot
leak across pooled connections. When the context carries **no** identity, both
GUCs are empty — the empty `app.user_role` matches no policy, so every row is
hidden and every write rejected (**fail-closed**). The `audit` repository uses a
role-only variant.

**Per-table policies** — every RLS-enabled table uses `ENABLE` **and** `FORCE ROW
LEVEL SECURITY` (FORCE applies RLS even to the table owner). Policies are
permissive (OR'd) and recognize the same three GUC roles. The `user` policy gates
on `user_id = NULLIF(current_setting('app.user_id', true), '')::uuid` (the `NULLIF`
turns an empty GUC into `NULL` so the cast never errors and the row is simply
excluded):

| Migration | Table(s) | RLS |
|-----------|----------|-----|
| `7_enable_rls_users`      | `users`                                                                     | ENABLE + FORCE; service / admin / self |
| `14_organizations`        | `organizations`, `organization_memberships`                                 | ENABLE + FORCE; visibility by **membership** |
| `17_org_rls_recursion_fix`| (recreates the org policies)                                                | uses `SECURITY DEFINER` `app_is_org_member()` to break policy recursion (SQLSTATE 42P17) |
| `20_gov_services`         | `gov_applications`, `gov_references`, `gov_notifications`, `gov_payments`, `gov_appointments` | ENABLE + FORCE; service / admin / self. (`gov_services` catalogue is public, no RLS) |
| `21_user_integrations`    | `user_integrations`                                                         | ENABLE + FORCE; service / admin / self |

Global config tables are deliberately **not** RLS-protected; their DB backstop is
table-privilege `REVOKE`s against the `app_user` role
(`17_least_privilege_config_grants` for `permissions` / `role_permissions` /
`ai_prompts` / `ai_knowledge`; `27_site_appearance` for the singleton appearance
row). The provider tables (`26_sso_provider`: `developer_apps`, `admin_api_keys`,
`login_events`) and `org_stamps` (`25`) are also non-RLS, guarded in the
usecase/handler layer.

**Boot-time enforceability guard** — RLS is silently bypassed by Postgres
superusers and `BYPASSRLS` roles, so `guardRLSEnforceable`
(`internal/datasources/drivers/driver_pgx.go`) inspects `pg_roles` for the
connecting role at startup:

- If the role has `rolsuper` or `rolbypassrls`: **production fails closed** (boot aborts, pool closed); **development logs a warning** and continues (migrate/tests may run as superuser).
- The api must therefore connect as a least-privilege non-superuser role (e.g. `app_user`) in production. (The compose stack runs `ENVIRONMENT=development` on purpose, so the guard only hard-fails in production.)

## OIDC Provider (Ory Hydra)

The platform can itself act as an **Identity Provider**: other government apps
delegate login to dan via **Ory Hydra**. This surface activates only when
`ProviderConfigured()` is true (`HYDRA_ADMIN_URL` + `HYDRA_PUBLIC_URL` +
`SSO_STATE_KEY ≥ 32 bytes`); otherwise it is inert and its routes are never
registered.

- **Login / consent / logout core** — `usecases/provider` + `pkg/hydra` handle Hydra's challenges; first-party clients (`SSO_FIRSTPARTY_CLIENTS`) skip the consent UI. Mounted under `/api/v1/provider`.
- **Applications (unified client registry)** — `usecases/applications` (mounted at `/api/v1/applications`, guarded by `gateway.manage`) is the current way to register OAuth2 clients: RP "Login with DAN" apps (`web`/`spa`/`native` → `authorization_code`; `spa`/`native` are public, PKCE, no secret) and m2m clients (`client_credentials`). Each is a Hydra OAuth2 client whose scopes are the allowed gateway services (`application_services` → `gateway_services.scope`); the confidential `client_secret` is revealed once on create/rotate.
- **Operator surface (legacy)** — `internal/provider/adminapi` is mounted at **`/admin`** (via `http.StripPrefix`) for RP OAuth2-client registration/management, backed by the `devapps` (`developer_apps`) store and `adminkeys` (bootstrap keys from `SSO_ADMIN_API_KEYS`, SHA-256 matched). This admin-API-key operator surface and the `developer_apps` overlay still exist but are **superseded by the unified Applications model for new work**.
- **Sign relay** — `internal/provider/signrelay` is mounted at **`/rp/sign/*`**, a reverse proxy that lets downstream RPs perform eID PDF signing *through* dan using dan's eidmongolia RP credentials (enabled by `SIGN_RELAY_TOKEN` + `EID_RP_SECRET`).

> **Enforcement caveat.** Assigning services to an application sets that client's
> OAuth **scopes** — this is registration/config only. *Runtime* per-request
> enforcement would require a gateway proxy that introspects the presented token
> (`hydra.Admin.Introspect` exists) against each route's service scope, and that
> proxy **does not exist yet**. So today the service assignment is not live
> authorization — don't mistake it for enforced authz.

## Database

- **Driver:** pgx v5 (`github.com/jackc/pgx/v5` + pgxpool), hand-written SQL — **no ORM**.
- **Database:** PostgreSQL, with **Row-Level Security** as the per-user boundary.
- **Migrations:** numbered SQL files in `migrations/` (`N_name.up.sql` + `.down.sql`), applied by the `migrate` compose service / `cmd/migration`. There is **no AutoMigrate** — the schema comes only from the `*.up.sql` files (`cmd/migration/main.go`).
- **Tracing:** OpenTelemetry via pgx pool instrumentation (`otelpgx`).

> **Migration-numbering collision.** Two migrations share the prefix `17_`:
> `17_least_privilege_config_grants` and `17_org_rls_recursion_fix`. They are
> independent and both applied; the runner orders numbered files, so keep this in
> mind when adding an `18_`-and-up migration or reasoning about apply order.

### Connection Management

Pool configured from env (`internal/datasources/drivers/driver_pgx.go`,
`SetupPgxPostgres`):

```go
poolCfg.MaxConns        = cfg.MaxConns    // DB_MAX_OPEN_CONNS   (default 25)
poolCfg.MinConns        = cfg.MinConns    // DB_MAX_IDLE_CONNS   (default 5)
poolCfg.MaxConnLifetime = cfg.MaxLifetime // DB_CONN_MAX_LIFE_MINS (default 15)
```

Production requires a TLS-verified DSN (`sslmode=verify-full` or `verify-ca`) —
enforced by the config guard.

## Observability

### Logging
- **Library:** Zap (structured), via `pkg/logger`. JSON in production, console in development. Request ID + trace ID propagated through `*WithContext` helpers.

### Metrics
- **Library:** Prometheus, endpoint `GET /metrics` (gated — see [Ops Endpoints](#ops-endpoints)). HTTP request counters/latency, cache hit/miss/error per layer, OTP send outcomes, and live pgx pool stats.

### Tracing
- **Library:** OpenTelemetry; exporter selected by `OTEL_EXPORTER` (empty = noop, `stdout`, or `otlp`), sampling by `OTEL_SAMPLE_RATIO`.

## Ops Endpoints

| Endpoint | Access |
|----------|--------|
| `GET /health` | Open — liveness (for load balancers / orchestrators). |
| `GET /ready`  | Open — readiness: DB ping (pgx pool) + Redis probe. |
| `GET /metrics` | **Gated** by `ObservabilityGate`. |
| `GET /swagger/doc.json` | **Gated** by `ObservabilityGate`. |

`ObservabilityGate` (`middleware_observability_gate.go`) protects the two
operator-sensitive endpoints: in **development** they are always open; in
**production** they require `Authorization: Bearer <OBSERVABILITY_TOKEN>` (constant-time
compared) and return **404** — not 401 — on any mismatch or when
`OBSERVABILITY_TOKEN` is unset, so their very existence stays hidden from
reconnaissance.

## Security Features

| Feature           | Implementation                          | Location                                   |
|-------------------|-----------------------------------------|--------------------------------------------|
| Row-Level Security| per-user DB isolation + boot guard      | `datasources/rls/`, `drivers/driver_pgx.go`, migrations `7/14/20/21` |
| Auth (identity)   | eID RP + Google OAuth                   | `usecases/auth`, `pkg/{eid,google}`        |
| Authorization     | 4-role model + dynamic RBAC             | `domain_users.go`, `middlewares/middleware_rbac.go` |
| Security headers  | HSTS, CSP, nosniff, frame options       | `middlewares/middleware_security.go`       |
| CORS              | env whitelist, wildcard dev-only        | `middlewares/middleware_cors.go`           |
| Rate limiting     | per-IP (auth / ai / poll / gov-write)   | `middlewares/middleware_ratelimit.go`      |
| Body size limit   | global + tighter caps on `/auth`        | `middlewares/middleware_bodysizelimit.go`  |
| Ops-endpoint gate | bearer token, 404 in prod               | `middlewares/middleware_observability_gate.go` |
| Input validation  | `validate:` struct tags                 | `internal/http/datatransfers/requests/`    |
| Encrypted secrets | AES-256-GCM OAuth tokens                 | `usecases/integrations` (`INTEGRATION_ENC_KEY`) |
| SQL injection     | pgx (parameterized queries)             | `internal/datasources/repositories/`       |
| PDF signing       | PAdES via server Document-Signer cert   | `usecases/sign` (`SIGN_SIGNER_*`)          |

## API Design

All API routes live under `/api/v1`; each module mounts `/v1/<module>`:
`auth`, `users`, `users/me/eid`, `rbac`, `org`, `gov`, `integrations`, `assets`,
`gspace`, `gateway`, `core`, `sso`, `admin`, `superadmin`, `ai`, `audit`,
`security`, `site`, `sign`, and (when Hydra is configured) `provider` +
`applications`. Infra endpoints
(`/health`, `/ready`, `/metrics`, `/swagger`) and the provider surfaces (`/admin`,
`/rp/sign`) sit at the root. **Full endpoint tables live in
[API_CONTRACT.md](API_CONTRACT.md)** and the generated OpenAPI spec (`/swagger`).

### Response Format

A single envelope (`internal/http/handlers/v1/handler_base_response.go`):

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
  "data": { "errors": { "national_id": "national_id is required" } }, "request_id": "…" }
```

Domain errors (`internal/apperror`) map to status codes: NotFound→404,
Unauthorized→401, Forbidden→403, Conflict→409, BadRequest→400, Internal→500.
5xx causes are logged and replaced with a generic message in the body.

## Testing Strategy

- **Unit tests** — usecase + handler layers with mockery mocks (`internal/test/mocks/`). Fast, no Docker. `go test ./...`.
- **Integration tests** — repositories (including RLS policies) against a real Postgres + Redis via testcontainers-go (`internal/test/testenv/`). `make test-integration`.
- **Mocks** — generated by mockery. `make mock interface=… dir=… filename=…`.
- **Authz matrix** — `routes/routes_authz_matrix_test.go` asserts the auth/permission gate on every route.

## Configuration

Loaded from `.env` / environment by Viper (`internal/config/config.go`; see
`internal/config/.env.example`). The config guard enforces production invariants
(TLS DSN, `ALLOWED_ORIGINS`, `VERIFY_API_KEY`, JWT secret length). Selected keys:

| Group | Variables |
|-------|-----------|
| **Server** | `PORT`, `ENVIRONMENT` (`development`/`production`), `DEBUG` |
| **Database** | `DB_POSTGRE_DRIVER`, `DB_POSTGRE_DSN` (dev), `DB_POSTGRE_URL` (prod; `sslmode=verify-full`/`verify-ca`), `DB_MAX_OPEN_CONNS` (25), `DB_MAX_IDLE_CONNS` (5), `DB_CONN_MAX_LIFE_MINS` (15) |
| **JWT** | `JWT_SECRET` (≥32), `JWT_EXPIRED` (h, 1–24), `JWT_ISSUER`, `JWT_REFRESH_EXPIRED` (d, 7) |
| **Redis** | `REDIS_HOST`, `REDIS_PASS`, `REDIS_EXPIRED` (min) |
| **Crypto** | `BCRYPT_COST` (12) |
| **Verify (OTP)** | `OTP_MAX_ATTEMPTS` (5), `VERIFY_API_BASE`, `VERIFY_API_KEY` (prod required), `VERIFY_CHANNEL` |
| **eID** | `EID_BASE_URL` (`…/v3`), `EID_RP_UUID`, `EID_RP_NAME`, `EID_RP_SECRET`, `EID_CERT_LEVEL` (ADVANCED), `EID_CALLBACK_URL`, `EID_DISPLAY_TEXT`, `SIGN_RELAY_TOKEN` |
| **Sign** | `SIGN_SIGNER_CERT_FILE`, `SIGN_SIGNER_KEY_FILE` (prod fail-closed) |
| **Google OAuth** | `GOOGLE_CLIENT_ID`, `GOOGLE_CLIENT_SECRET` |
| **XYP** | `XYP_API_BASE` (`https://xyp.dgov.mn`), `XYP_CLIENT_ID`, `XYP_CLIENT_SECRET` |
| **Gerege Space** | `GSPACE_HOST`, `GSPACE_PORT` (22), `GSPACE_USER`, `GSPACE_PASSWORD`, `GSPACE_BASE_PATH` (gerege-space), `GSPACE_QUOTA_BYTES` (2 MB) |
| **Gemini AI** | `GEMINI_API_KEY`, `GEMINI_MODEL`, `GEMINI_TTS_MODEL`, `GEMINI_VOICE`, `GEMINI_API_BASE`, `AI_SCOPE_PROMPT` |
| **Gerege Core** | `CORE_API_BASE` (`https://core.gerege.mn`), `CORE_API_TOKEN` |
| **Integrations** | `INTEGRATION_ENC_KEY` (AES-256-GCM; prod required) |
| **OIDC Provider (Hydra)** | `HYDRA_ADMIN_URL` (`http://hydra:4445`), `HYDRA_PUBLIC_URL`, `SSO_STATE_KEY` (≥32), `SSO_FIRSTPARTY_CLIENTS`, `SSO_ADMIN_API_KEYS`, `SSO_ADMIN_SUBS` |
| **Observability** | `OTEL_EXPORTER` (``/`stdout`/`otlp`), `OTEL_SAMPLE_RATIO`, `OBSERVABILITY_TOKEN` |
| **Networking** | `ALLOWED_ORIGINS` (prod required), `TRUSTED_PROXIES` |
| **Bootstrap** | `SUPERADMIN_EMAIL` |

## Deployment

```bash
go build ./...                 # build
docker compose up -d --build   # db + redis + migrate (one-off) + api + web
```

Health check: `curl http://localhost:8080/health`. See `docs/DEPLOYMENT.md` for
the deployment topology.

## Credits & License

This platform builds on open-source work:

| Project | Author | License | What we used |
|---------|--------|---------|--------------|
| [snykk/go-rest-boilerplate](https://github.com/snykk/go-rest-boilerplate) | Najib Fikri | MIT | Clean Architecture layering, caching, observability, and test strategy |

The delivery layer was adapted **Gin → chi (net/http)** and the data layer
**sqlx → pgx (pgxpool)**; the auth stack, RLS security model, eID/SSO/OIDC-provider
integrations, and feature modules were built for this platform. As an MIT
derivative the upstream copyright notice is retained and this code is distributed
under the MIT License (see `LICENSE`).

---

**Government Template Platform V3.0** — Co-developed by the **Gerege Systems Development Team** and **Claude AI**, 2026.
