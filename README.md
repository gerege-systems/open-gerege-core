# Government Template Platform V3.0 — Backend (Go)

> _One foundation — every government service._

> 🌐 **English** · [Монгол](README_MN.md)

[![Go](https://img.shields.io/badge/Go-1.26-blue.svg)](https://golang.org/)
[![chi](https://img.shields.io/badge/chi-v5-00ADD8.svg)](https://github.com/go-chi/chi)
[![pgx](https://img.shields.io/badge/pgx-v5-336791.svg)](https://github.com/jackc/pgx)
[![License](https://img.shields.io/badge/License-MIT-green.svg)](LICENSE)

The Go backend of the **Government Template Platform V3.0** — a production-ready
foundation on which *any* digital-government service can be built. It pairs a
disciplined **Clean Architecture** core with hand-written **pgx SQL** (no ORM),
and ships with a full suite of government-grade capabilities out of the box:
**eID Mongolia** authentication, **Google** account-linking,
**PAdES** document signing, a **Gemini AI** pipeline, and defense-in-depth
security hardening — all bilingual (mn/en) and observable from day one. Built on
**chi (net/http)** for HTTP, **pgx (pgxpool) + PostgreSQL** for data, and
**Redis + Ristretto** for cache.

> **Reference deployment:** **Government Template Platform** ([template.dgov.mn](https://template.dgov.mn))
> — a government service platform and Relying Party of Government SSO built on this
> foundation, showcasing eID single sign-on and a built-in OIDC provider for other apps.

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
- **eID authentication** — the only login method: eID Mongolia Relying Party (QR / mobile deep-link / national-ID push) with a long-poll session; issues JWT access + refresh tokens (rotation, `kind` claim guard)
- **Google OAuth linking** — link a Google account to an eID user (code exchange server-side only) and log in with it thereafter
- **OIDC provider (SSO)** — an optional Ory Hydra front-end so DAN acts as an identity provider; login/consent/logout flows plus an `/admin` surface for RP client registration (enabled only when Hydra is configured)
- **eID PKI profile** — the signed-in citizen's linked organizations & signers, certificates, devices, and activity
- **Organizations & membership** — org create/lookup (Gerege Verify/XYP state-registry lookup) + member/role management, RLS-scoped per user
- **Government services portal** — catalogue, applications, references, notifications, payments, appointments
- **API gateway** — services / routes / consumers / API keys / policies + request telemetry (admin-managed)
- **Document signing (PAdES)** — server-side PDF signing via eID Mongolia `/v3` with a persistent Document-Signer certificate; optional sign-relay for third-party RPs
- **Integrations & storage** — per-user OAuth integrations (Google Drive/Meet, Dropbox) with AES-256-GCM token encryption; Gerege Space app-native SFTP storage
- **AI pipeline (Gemini)** — SDK-free REST client + function calling: text/voice chat, STT, TTS, live translation; layered prompts (hardcoded guardrails + DB-configurable scope) and a DB-backed `search_knowledge` tool
- **RBAC & super admin** — dynamic roles + permission catalogue; 4-role model (superadmin → admin → manager → user)
- **Site appearance** — admin-configurable site-wide look (accent/font/density/theme) + per-user overrides
- **Audit log** — hash-chained, append-only audit trail (admin-only read + integrity verify)
- **Observability** — OpenTelemetry trace + Prometheus metrics; `/metrics` + `/swagger` gated by a bearer token in production
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
│   │   └── usecases/           # Business logic (interface + impl), one package per module:
│   │       #  auth · users · rbac · superadmin · ai · audit · security · site
│   │       #  org · gov · gateway · core · sso · provider · sign · assets
│   │       #  integrations · gspace
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
                                 # audit, observability, gemini,
                                 # eid, google, oidc, hydra, xyp, gspace, verify
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
# Core
PORT=8080
ENVIRONMENT=development          # development | production
JWT_SECRET=...                   # >= 32 characters (HS256)
JWT_EXPIRED=5                    # access token TTL (hours, 1..24)
JWT_REFRESH_EXPIRED=7            # refresh token TTL (days)
DB_POSTGRE_DSN=...               # DSN in dev
DB_POSTGRE_URL=...               # URL in production (must use sslmode=verify-full/verify-ca)
REDIS_HOST=localhost:6379
BCRYPT_COST=12                   # 10..31
ALLOWED_ORIGINS=                 # required in production (comma-separated)
TRUSTED_PROXIES=                 # reverse-proxy IPs/CIDRs to trust X-Forwarded-For from
OBSERVABILITY_TOKEN=             # bearer token gating /metrics + /swagger in production

# eID Mongolia (Relying Party) — the primary login; sane defaults so boot never breaks
EID_BASE_URL=https://eidmongolia.mn/v3
EID_RP_UUID=                     # RP UUID registered with the IdP
EID_RP_NAME=                     # RP display name
EID_RP_SECRET=                   # RP API secret (also used for /rp/sign relay)
EID_CERT_LEVEL=ADVANCED          # ADVANCED | QUALIFIED | QSCD
EID_CALLBACK_URL=                # must be allowlisted at the IdP
EID_DISPLAY_TEXT=

# Google OAuth — link a Google account to an eID user
GOOGLE_CLIENT_ID=
GOOGLE_CLIENT_SECRET=

# OIDC PROVIDER side (the platform is its own issuer) — provider flows are inert unless set
OAUTH_ISSUER=                    # issuer, e.g. https://template.dgov.mn (empty = provider off)
SSO_STATE_KEY=                   # >= 32 bytes; login/consent state cookie HMAC
SSO_FIRSTPARTY_CLIENTS=          # CSV client_ids that skip the consent screen
SSO_ADMIN_API_KEYS=              # CSV bootstrap keys for the /admin surface

# Document signing (PAdES) — persistent Document-Signer material (required in production)
SIGN_SIGNER_CERT_FILE=
SIGN_SIGNER_KEY_FILE=
SIGN_RELAY_TOKEN=                # shared token so third-party RPs sign via DAN's eID creds

# Gerege state services
XYP_API_BASE=https://xyp.dgov.mn # org lookup (state registry); Basic auth
XYP_CLIENT_ID=
XYP_CLIENT_SECRET=
CORE_API_BASE=https://core.gerege.mn  # Gerege Core user/org find
CORE_API_TOKEN=

# Gerege Space — app-native SFTP storage (empty = feature disabled)
GSPACE_HOST=
GSPACE_PORT=22
GSPACE_USER=
GSPACE_PASSWORD=
GSPACE_BASE_PATH=gerege-space
GSPACE_QUOTA_BYTES=2097152       # per-user quota (default 2 MB)

# Integrations token encryption (AES-256-GCM) — required in production
INTEGRATION_ENC_KEY=

# GeregeCloud Verify (verify.gecloud.mn) — OTP transport; required in production
VERIFY_API_KEY=
VERIFY_API_BASE=https://verify.gecloud.mn/v1
VERIFY_CHANNEL=email

# AI pipeline (/api/v1/ai/*)
GEMINI_API_KEY=                  # empty = AI disabled (endpoints return 500)
GEMINI_MODEL=gemini-2.5-flash    # optional override (chat / STT / translate)
GEMINI_TTS_MODEL=gemini-2.5-flash-preview-tts  # optional override (TTS)
GEMINI_VOICE=Kore                # optional prebuilt TTS voice
GEMINI_API_BASE=                 # optional override (default: Google generativelanguage v1beta)
AI_SCOPE_PROMPT=                 # AI scope fallback when the DB 'scope' prompt layer is empty

# Observability + bootstrap
OTEL_EXPORTER=                   # empty=off | stdout | otlp
SUPERADMIN_EMAIL=                # optional: promote this (already-signed-in) user to super admin on boot
```

### Roles & super admin

Roles are ordered by privilege (id 1 = highest): **superadmin=1, admin=2,
manager=3, user=4** (seeded/remapped by migration `23_superadmin_role`). A
**super admin** sits above admin and is the only role that can manage admin
accounts (create / grant / revoke) via `/api/v1/superadmin/*`
(`RequireSuperAdmin`); regular admins cannot reach that surface. The API never
mints a super admin — bootstrap one by setting `SUPERADMIN_EMAIL` to an existing
user who has already signed in via eID (promoted on the next boot) or by updating
`role_id=1` in the DB.

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

All under `/api/v1` (ops endpoints at root). There is **no password / email-OTP /
register / forgot-reset endpoint** — authentication is eID + Google only.

### Public (Authentication)
| Method | Path | Description |
|--------|------|---------|
| POST | `/api/v1/auth/eid/start` | Start eID login (QR / mobile deep-link) |
| POST | `/api/v1/auth/eid/start-id` | Start eID login by national ID (push to a registered device) |
| POST | `/api/v1/auth/eid/poll` | Long-poll the eID session until it completes |
| POST | `/api/v1/auth/google` | Google OAuth callback — code exchange + eID link / login |
| POST | `/api/v1/auth/refresh` | Token rotation |
| POST | `/api/v1/auth/logout` | Revoke refresh + deny-list access token |

### Protected (requires JWT)
| Method | Path | Description |
|--------|------|---------|
| GET | `/api/v1/users/me` | User profile |
| GET | `/api/v1/rbac/me` | Current user's effective roles/permissions |
| DELETE | `/api/v1/auth/google/link` | Unlink the connected Google account |
| GET | `/api/v1/me/*`, `/api/v1/users/me/eid/*` | eID PKI profile — organizations, signers, certificates, devices, activity |
| CRUD | `/api/v1/org/*` | Organizations + membership (state-registry lookup, members, roles) |
| GET/POST | `/api/v1/gov/*` | Gov services portal — services, applications, references, notifications, payments, appointments |
| CRUD | `/api/v1/gateway/*` | API gateway — services, routes, consumers, keys, policies, logs |
| GET | `/api/v1/core/users` · `/organizations` | Gerege Core find (user/org lookup) |
| CRUD | `/api/v1/integrations/*` | Per-user OAuth integrations (encrypted tokens) |
| GET | `/api/v1/assets/*` | Signature image + org stamp assets |
| GET | `/api/v1/gspace/*` | Gerege Space SFTP storage (list + download) |
| POST/GET | `/api/v1/sign/*` | Document signing (PAdES) — init, status, download |
| POST | `/api/v1/ai/chat` | AI chat (Gemini pipeline, function calling, text/voice messages) |
| POST | `/api/v1/ai/stt` | Speech-to-text (audio base64 → transcript) |
| POST | `/api/v1/ai/tts` | Text-to-speech (text → WAV base64) |
| POST | `/api/v1/ai/translate` | Live translation (text/audio → target language, optional TTS) |
| GET | `/api/v1/site/appearance` | Site-wide appearance defaults (public read) |
| GET/PUT | `/api/v1/admin/ai/prompts` | AI prompt layers — scope/instructions (settings.manage) |
| GET | `/api/v1/audit` · `/audit/verify` | Read the audit log + verify its hash chain (admin) |
| POST | `/api/v1/security/events` | Ingest a client security event |
| GET | `/api/v1/superadmin/admins` | List admin-level accounts (super admin only) |
| POST | `/api/v1/superadmin/admins` | Create a new admin account (super admin only) |
| PUT | `/api/v1/superadmin/admins/{id}/grant` | Grant admin to an existing user (super admin only) |
| DELETE | `/api/v1/superadmin/admins/{id}` | Revoke admin (super admin only) |

### OIDC provider (only when Hydra is configured)
`GET /api/v1/provider/login` · `/consent`, plus accept/reject for login/consent/logout
(the Hydra-driven login/consent screens). RP OAuth2 client registration lives under
the mounted `/admin` surface.

### Ops
`GET /health` (liveness) · `GET /ready` (DB+Redis) · `GET /metrics` · `GET /swagger/doc.json`
— in production `/metrics` and `/swagger` require the `OBSERVABILITY_TOKEN` bearer (else 404).

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

**Government Template Platform V3.0** — Co-developed by the **Gerege Systems
Development Team** and **Claude AI**, 2026.
