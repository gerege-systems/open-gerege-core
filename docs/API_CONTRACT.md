# API Contract

> 🌐 **English** · [Монгол](API_CONTRACT_MN.md)

REST API reference for the **Government Template Platform V3.0** (Цахим засаглалыг
бүтээх суурь) — a production-ready foundation for building digital-government
services (Clean-Architecture Go backend + Next.js BFF + Gemini AI). This contract
tracks its reference deployment, **Government Template Platform** (template.dgov.mn), an
eID-based government service platform. The live, auto-generated spec is served at `GET
/swagger/` (source: `docs/swagger.json`).

> **Note on paths.** Every module below mounts under the `/api` group, and each
> route group adds a `/v1` prefix, so the real request path is
> `/api/v1/<group>/…` even though the swagger `@Router` annotations are written
> relative (e.g. annotation `/auth/eid/start` → real path
> `/api/v1/auth/eid/start`). The tables in this document use the **full** path.

## Conventions

- **Base URL:** `http://localhost:8080/api/v1`
- **Content-Type:** `application/json`
- **Auth:** protected endpoints require `Authorization: Bearer <access_token>`
  (tokens are minted by the eID / Google login flows below)
- **Rate limits (per IP):** `/auth/*` ~5 req/min, `/auth/eid/poll` a separate
  looser ~60 req/min (long-poll must not 429 itself), `/ai/*` ~20 req/min, and
  the `/gov`, `/gspace`, `/me`, `/users/me/eid` **mutating** endpoints ~30
  req/min (429 on excess)
- **Body cap:** `/auth/*` and `/provider/*` bodies are limited to 4
  KiB; everything else to 1 MiB

### Response envelope

Every response uses one envelope:

```json
{
  "status": true,
  "message": "human-readable summary",
  "data": { },
  "request_id": "b1d2…"
}
```

- `status` — `true` on success, `false` on error
- `data` — present on success (omitted/null on error)
- `request_id` — correlation id (also echoed in the `X-Request-ID` header)

### Status codes

| Code | Meaning | When |
|------|---------|------|
| 200 | OK | Successful read / action |
| 201 | Created | Resource created |
| 400 | Bad Request | Malformed body |
| 401 | Unauthorized | Missing/invalid/expired token |
| 403 | Forbidden | Authenticated but lacks the required role/permission |
| 404 | Not Found | Resource does not exist |
| 409 | Conflict | Duplicate / state conflict |
| 422 | Unprocessable Entity | Validation failed (see below) |
| 429 | Too Many Requests | Rate limit exceeded |
| 500 | Internal Server Error | Unexpected failure (cause logged, generic message returned) |

### Validation error (422)

Field-level detail is returned under `data.errors`, which is an **array** of
`{ field, tag, message }` objects. `field` is the JSON tag name:

```json
{
  "status": false,
  "message": "validation failed",
  "data": { "errors": [ { "field": "target_lang", "tag": "required", "message": "target_lang is required" } ] },
  "request_id": "b1d2…"
}
```

### Legend

- 🔒 — requires `Authorization: Bearer <access_token>`
- 🛡️ `perm` — additionally requires the named RBAC permission (an **admin** role
  auto-resolves the full permission catalogue; a **super admin** is required
  where noted). Path parameters are shown in `{braces}`.

---

## Authentication (`/api/v1/auth`)

The **only** login method is **Login with eID** (eID Mongolia Relying Party),
plus **Google OAuth** account-linking.
There is no password, email/OTP, or registration surface. This group is
rate-limited and body-capped (4 KiB); the pre-login flows run under a service
RLS identity.

| Method | Path | Auth | Description |
|--------|------|------|-------------|
| POST | `/auth/eid/start` | — | Start an eID login; returns a QR code / mobile deep-link and a session token to poll. |
| POST | `/auth/eid/start-id` | — | Start eID login by national registration number; pushes an approval to the citizen's registered device. |
| POST | `/auth/eid/poll` | — | Long-poll the pending eID session (~25 s hold); returns `PENDING` or, on approval, the access + refresh token pair. Separate looser rate limiter. |
| POST | `/auth/google` | — | Google OAuth callback — exchanges the auth `code`, then links the Google account to (or logs in as) the eID user. |
| DELETE | `/auth/google/link` | 🔒 | Unlink the authenticated user's Google account (linking happens only via the login flow). |
| POST | `/auth/refresh` | — | Rotate the token pair using a valid refresh token. Refresh **rotates** the token, so the old refresh token is invalidated. |
| POST | `/auth/logout` | — | Revoke the supplied refresh token; if `access_token` is also supplied, its jti is added to a Redis deny-list so it stops working immediately. |

On success the login/refresh flows return the token pair in `data`
(`token` = access JWT, `refresh_token` = refresh JWT) alongside the user's
identity (`id`, `role_id`, name fields).

---

## Users (`/api/v1/users`)

| Method | Path | Auth | Description |
|--------|------|------|-------------|
| GET | `/users/me` | 🔒 | Return the authenticated user's profile (`id`, `username`, `email`, `role_id`, timestamps). |

## eID profile (`/api/v1/users/me/eid`) 🔒

Extended eID data for the logged-in citizen. Mutating (`POST`/`DELETE`)
endpoints take the ~30 req/min write limiter; reads are unlimited.

| Method | Path | Description |
|--------|------|-------------|
| GET | `/users/me/eid/organizations` | Organizations the citizen represents. |
| POST | `/users/me/eid/organizations` | Link an organization (by reg. no., verified via XYP). |
| DELETE | `/users/me/eid/organizations/{regNo}` | Unlink an organization. |
| GET | `/users/me/eid/organizations/{regNo}/signers` | List the organization's authorized signers. |
| POST | `/users/me/eid/organizations/{regNo}/signers` | Add an org signer. |
| POST | `/users/me/eid/organizations/{regNo}/signers/resend` | Re-send a signer invitation. |
| DELETE | `/users/me/eid/organizations/{regNo}/signers` | Remove an org signer. |
| GET | `/users/me/eid/summary` | eID profile summary. |
| GET | `/users/me/eid/certificates` | Citizen's eID certificates. |
| GET | `/users/me/eid/devices` | Registered eID devices. |
| GET | `/users/me/eid/activity` | Recent eID activity. |

---

## RBAC (`/api/v1/rbac`) 🔒

Dynamic roles + permissions. `/rbac/me` is open to any authenticated user; the
rest require 🛡️ `roles.manage`.

| Method | Path | Guard | Description |
|--------|------|-------|-------------|
| GET | `/rbac/me` | 🔒 | The caller's effective permissions (used to filter the UI menu). |
| GET | `/rbac/roles` | 🛡️ `roles.manage` | List roles. |
| GET | `/rbac/permissions` | 🛡️ `roles.manage` | List the permission catalogue. |
| POST | `/rbac/roles` | 🛡️ `roles.manage` | Create a role. |
| PUT | `/rbac/roles/{id}` | 🛡️ `roles.manage` | Rename/update a role. |
| PUT | `/rbac/roles/{id}/permissions` | 🛡️ `roles.manage` | Replace a role's permission set. |
| DELETE | `/rbac/roles/{id}` | 🛡️ `roles.manage` | Delete a role. |

## Organizations (`/api/v1/org`) 🔒

Organization + membership management. Ownership/admin checks are enforced in the
usecase layer.

| Method | Path | Description |
|--------|------|-------------|
| POST | `/org/` | Create an organization. |
| GET | `/org/` | List the caller's organizations. |
| GET | `/org/lookup/{regNo}` | Look up an organization by registration number. |
| GET | `/org/{id}` | Get one organization. |
| GET | `/org/{id}/members` | List members. |
| POST | `/org/{id}/members` | Add a member. |
| PUT | `/org/{id}/members/{userID}` | Change a member's role. |
| DELETE | `/org/{id}/members/{userID}` | Remove a member. |

---

## Government services portal (`/api/v1/gov`) 🔒

The citizen "Төрийн үйлчилгээ" portal. All data is per-user (userID from the
token). Mutating endpoints take the ~30 req/min write limiter.

| Method | Path | Description |
|--------|------|-------------|
| GET | `/gov/services` | Service catalogue. |
| GET | `/gov/overview` | Dashboard overview. |
| GET | `/gov/applications` | List the citizen's applications. |
| POST | `/gov/applications` | Submit a new application. |
| POST | `/gov/applications/{id}/cancel` | Cancel an application. |
| GET | `/gov/references` | List reference (лавлагаа) requests. |
| POST | `/gov/references` | Request a reference. |
| GET | `/gov/notifications` | List notifications. |
| POST | `/gov/notifications/read-all` | Mark all notifications read. |
| POST | `/gov/notifications/{id}/read` | Mark one notification read. |
| GET | `/gov/payments` | List payments. |
| POST | `/gov/payments/{id}/pay` | Pay a pending payment. |
| GET | `/gov/appointments` | List appointments. |
| POST | `/gov/appointments` | Book an appointment. |
| POST | `/gov/appointments/{id}/cancel` | Cancel an appointment. |

---

## Integrations (`/api/v1/integrations`) 🔒

Manage the user's third-party OAuth connections (Google Drive/Meet, Dropbox).
Tokens are stored encrypted per user (RLS).

| Method | Path | Description |
|--------|------|-------------|
| GET | `/integrations/` | List connected providers. |
| POST | `/integrations/` | Connect a provider (OAuth). |
| GET | `/integrations/{provider}/token` | Get a usable access token for a connected provider. |
| DELETE | `/integrations/{provider}` | Disconnect a provider. |

## Assets — signature / latin name / org stamp (`/api/v1/me`) 🔒

Mounted under `/api/v1/me` (not `/users/me`) to avoid shadowing the who-am-I
route. Mutations take the ~30 req/min write limiter. The org stamp is writable
only by an org **admin**.

| Method | Path | Description |
|--------|------|-------------|
| GET | `/me/signature` | Get the personal signature image URL. |
| PUT | `/me/signature` | Set the personal signature image. |
| DELETE | `/me/signature` | Delete the personal signature. |
| PUT | `/me/latin-name` | Correct the citizen's Latin (transliterated) name. |
| PUT | `/me/org-name-latin/{regNo}` | Correct an organization's Latin name. |
| GET | `/me/orgstamp/{regNo}` | Get an organization's stamp image. |
| PUT | `/me/orgstamp/{regNo}` | Set an organization's stamp image (org admin only). |
| DELETE | `/me/orgstamp/{regNo}` | Delete an organization's stamp image (org admin only). |

## Gerege Space (`/api/v1/gspace`) 🔒

The app's own per-user SFTP storage. Returns 500 until `GSPACE_*` is configured.
Mutations take the write limiter.

| Method | Path | Description |
|--------|------|-------------|
| GET | `/gspace/` | Storage overview (usage/quota, file list). |
| GET | `/gspace/download` | Download a file. |
| POST | `/gspace/upload` | Upload a file. |
| DELETE | `/gspace/` | Delete a file. |

---

## API Gateway (`/api/v1/gateway`) 🛡️ `gateway.manage`

Upstream **service** registry plus telemetry. Every endpoint requires 🔒 + 🛡️
`gateway.manage`. Gateway **clients** (the former "consumers + API keys") now
live in the **Applications** group below; each service carries a `scope` (the
OAuth scope apps request to reach it). The old Kong-style **routes** and
**policies** were removed (no runtime proxy consumed them). The **request log**
is now real: a middleware records every actual `/api` request
(method/path/status/latency/client_ip) — the overview aggregates from it.

| Method | Path | Description |
|--------|------|-------------|
| GET | `/gateway/overview` | Telemetry overview (services/apps counts + 24h request stats from real traffic). |
| GET | `/gateway/logs` | Real request log (method/path/status/latency/client_ip). |
| GET | `/gateway/services` | List services. |
| POST | `/gateway/services` | Create a service (its OAuth `scope` = `svc:`+name). |
| PUT | `/gateway/services/{id}` | Update a service. |
| DELETE | `/gateway/services/{id}` | Delete a service. |

## Applications (`/api/v1/applications`) 🛡️ `gateway.manage`

Unified OAuth2 **client registry** — the merged replacement for the old gateway
"consumers + API keys" and the separate SSO RP registration. Each application is
an **Ory Hydra OAuth2 client**; its per-service access is expressed as OAuth
**scopes** (`application_services` → `gateway_services.scope`). Every endpoint
requires 🔒 + 🛡️ `gateway.manage`, and the group is **registered only when Hydra
is configured** (`ProviderConfigured()`).

`app_type` selects the grant + auth method:

| `app_type` | Grant | Client | Use |
|------------|-------|--------|-----|
| `web` | `authorization_code` (+ `refresh_token`) | confidential (secret) | RP "Login with DAN" — server-side web app |
| `spa` | `authorization_code` (+ `refresh_token`) | **public** (PKCE, no secret) | Browser SPA |
| `native` | `authorization_code` (+ `refresh_token`) | **public** (PKCE, no secret) | Mobile / native app |
| `m2m` | `client_credentials` | confidential (secret) | Server-to-server |

The OAuth2 **`client_secret`** (confidential types only) is revealed **once** — in
the create / rotate response — and never again.

| Method | Path | Description |
|--------|------|-------------|
| GET | `/applications` | List applications. |
| POST | `/applications` | Create; provisions a Hydra OAuth2 client and returns the app incl. the one-time `secret` (confidential types). |
| GET | `/applications/{id}` | Get one application. |
| PUT | `/applications/{id}` | Update the overlay + the Hydra client's desired state. |
| DELETE | `/applications/{id}` | Delete the Hydra client + overlay. |
| POST | `/applications/{id}/rotate-secret` | Issue a new client secret, returned once (confidential only). |
| PUT | `/applications/{id}/services` | Replace the allowed gateway services — they become the client's OAuth scopes. |

**Create/update body** — `{ name, app_type (web\|spa\|native\|m2m), redirect_uris[], tags[], service_ids[], enabled }`; **set-services body** — `{ service_ids[] }`.

## Gerege Core (`/api/v1/core`) 🔒

Search wrapper over Gerege Core (core.gerege.mn); the service token stays on the
backend.

| Method | Path | Description |
|--------|------|-------------|
| GET | `/core/users` | Find users. |
| GET | `/core/organizations` | Find organizations. |

---

## OIDC provider — login/consent/logout (`/api/v1/provider`)

Active **only when the provider is configured** (`ProviderConfigured()`). This is
the platform acting as an OIDC **provider** (its own built-in Go provider); the Next.js
BFF `/login`, `/consent`, `/logout` pages call these. Body-capped (4 KiB). The
`get`/`reject`/`logout-accept` endpoints are challenge-authenticated (no
bearer); the `accept` endpoints require a logged-in citizen (subject = dan user
ID).

| Method | Path | Auth | Description |
|--------|------|------|-------------|
| GET | `/provider/login` | challenge | Fetch login-challenge details. |
| GET | `/provider/consent` | challenge | Fetch consent-challenge details. |
| POST | `/provider/login/reject` | challenge | Reject the login challenge. |
| POST | `/provider/consent/reject` | challenge | Reject the consent challenge. |
| POST | `/provider/logout/accept` | challenge | Accept the logout challenge. |
| POST | `/provider/login/accept` | 🔒 | Accept the login challenge for the logged-in citizen. |
| POST | `/provider/consent/accept` | 🔒 | Accept the consent challenge. |

---

## Admin — users & AI prompts (`/api/v1/admin`) 🔒

| Method | Path | Guard | Description |
|--------|------|-------|-------------|
| GET | `/admin/users` | 🛡️ `users.manage` | List users. |
| PUT | `/admin/users/{id}/role` | 🛡️ `users.manage` | Change a user's role. |
| PUT | `/admin/users/{id}/active` | 🛡️ `users.manage` | Enable/disable a user. |
| DELETE | `/admin/users/{id}` | 🛡️ `users.manage` | Delete a user. |
| GET | `/admin/ai/prompts` | 🛡️ `settings.manage` | List the configurable AI prompt layers. |
| PUT | `/admin/ai/prompts/{key}` | 🛡️ `settings.manage` | Update a prompt layer (`key` ∈ `scope` \| `instructions`). |

> **Naming note.** This in-app `/api/v1/admin` group is unrelated to the
> top-level `/admin` Hydra operator surface documented under *Non-`/api`
> mounts* below — same word, different mount.

## Super admin (`/api/v1/superadmin`) 🔒

Guarded by `RequireSuperAdmin` — only `RoleSuperAdmin` may enter; a regular
admin cannot. Every mutation is written to the audit log.

| Method | Path | Description |
|--------|------|-------------|
| GET | `/superadmin/admins` | List admins. |
| POST | `/superadmin/admins` | Create an admin. |
| PUT | `/superadmin/admins/{id}/grant` | Grant admin to an existing user. |
| DELETE | `/superadmin/admins/{id}` | Revoke admin. |

---

## Audit log (`/api/v1/audit`) 🔒 admin

Hash-chained, append-only audit log; admin-only (`RequireAdmin`).

| Method | Path | Description |
|--------|------|-------------|
| GET | `/audit/` | List audit entries. |
| GET | `/audit/verify` | Verify the hash chain integrity. |

## Security events (`/api/v1/security`) 🔒

RASP-style client telemetry. Ingest is open to any authenticated user (RLS
stamps the `user_id`); listing is admin-only.

| Method | Path | Guard | Description |
|--------|------|-------|-------------|
| POST | `/security/events` | 🔒 | Ingest a security event. |
| GET | `/security/events` | 🔒 admin | List security events (`RequireAdmin`). |

## Site appearance (`/api/v1/site`)

Site-wide default appearance (accent/font/density/theme).

| Method | Path | Guard | Description |
|--------|------|-------|-------------|
| GET | `/site/appearance` | — (public) | Read the public appearance defaults (landing/anonymous). |
| PUT | `/site/appearance` | 🛡️ `settings.manage` | Update the appearance defaults. |

## PDF signing — PAdES (`/api/v1/sign`) 🔒

Server-assisted PAdES signing via eID Mongolia `/v3`.

| Method | Path | Description |
|--------|------|-------------|
| POST | `/sign/init` | Start a signing session (returns an `id` + approval prompt). |
| GET | `/sign/{id}` | Poll the signing session status. |
| GET | `/sign/{id}/download` | Download the signed PDF. |

---

## AI (Gemini pipeline) (`/api/v1/ai`) 🔒

All `/ai/*` endpoints require a bearer token and share a dedicated rate limit
(~20 req/min per IP). They return 500 until `GEMINI_API_KEY` is configured. The
assistant runs on a layered system prompt — hardcoded guardrails + an
admin-configurable **scope** (it refuses anything outside it) + optional
**instructions** — and grounds platform answers in the `ai_knowledge` table via
its `search_knowledge` tool.

### POST `/ai/chat` 🔒
Chat with the assistant. Send text, voice (base64 audio the model understands
directly), or both. Stateless — pass prior turns in `history`.

**Request**
```json
{ "message": "what time is it?",
  "audio": { "mime": "audio/webm", "data": "<base64>" },
  "history": [ { "role": "user", "text": "…" }, { "role": "model", "text": "…" } ] }
```
| Field | Rules |
|-------|-------|
| `message` | optional (required if no `audio`), ≤ 4000 chars |
| `audio` | optional; `mime` ∈ webm/ogg/wav/mpeg/mp3/mp4/m4a/aac/flac, `data` base64 ≤ ~700 KB |
| `history` | optional, ≤ 20 turns |

**Response `200`**
```json
{ "status": true, "message": "ai reply generated", "data": {
  "reply": "Одоо 12:30 цаг болж байна.",
  "steps": [ { "tool": "get_server_time", "args": {}, "result": { } } ],
  "degraded": false }, "request_id": "…" }
```
`steps` lists the function calls the model executed (pipeline trace). When
Gemini is temporarily unavailable the endpoint still returns `200` with a
Mongolian fallback `reply` and `degraded: true`.

### POST `/ai/stt` 🔒
Speech-to-text. **Request** `{ "audio": { "mime": "audio/webm", "data": "<base64>" } }`
**Response `200`** — `data: { "text": "…" }` (empty when no speech detected).

### POST `/ai/tts` 🔒
Text-to-speech. **Request** `{ "text": "Сайн байна уу", "voice": "Kore" }` (`voice` optional)
**Response `200`** — `data: { "mime": "audio/wav", "data": "<base64 WAV>" }` — playable directly in a browser.

### POST `/ai/translate` 🔒
Live translation. Provide `text` **or** `audio` (audio goes through an internal
STT step first); `speak: true` additionally returns a spoken (TTS) version.
Silent audio chunks return empty fields — the live-translation UI streams short
recorded segments here.

**Request** `{ "audio": { … }, "target_lang": "en", "speak": false }`
(`target_lang`: required, e.g. `mn|en|ru|zh|ja|ko|de`)
**Response `200`** — `data: { "source_text": "Сайн уу", "translated": "Hello", "audio": { … } }`.

> Prompt-layer configuration lives under **Admin — users & AI prompts** above
> (`GET`/`PUT /api/v1/admin/ai/prompts`). The base guardrail layer is hardcoded
> and never exposed.

---

## Non-`/api` mounts

### OIDC provider admin surface — `/admin` (operator)

Active **only when Hydra is configured** (`ProviderConfigured()`). A plain
`http.ServeMux` mounted at `/admin` (via `StripPrefix`, so its internal patterns
read `/api/v1/…`). It manages **RP OAuth2 client registration** and **admin API
keys**, and is authenticated by an **admin API key** —
`Authorization: Bearer <key>` or `X-API-Key: <key>` — **not** by a user JWT.

> ⚠️ **Naming collision.** This top-level `/admin` operator surface is a
> different thing from the in-app `/api/v1/admin` group above. Its own routes
> also happen to read `/api/v1/…` after strip, but they are reached at
> `/admin/api/v1/…`.

| Method | Path (under `/admin`) | Description |
|--------|-----------------------|-------------|
| GET | `/api/v1/me` | Identify the calling admin key. |
| GET | `/api/v1/clients` | List registered RP OAuth2 clients. |
| POST | `/api/v1/clients` | Register a new RP client. |
| GET | `/api/v1/clients/{client_id}` | Get one RP client. |
| PATCH | `/api/v1/clients/{client_id}` | Update an RP client. |
| DELETE | `/api/v1/clients/{client_id}` | Delete an RP client. |
| POST | `/api/v1/clients/{client_id}/rotate-secret` | Rotate an RP client secret. |
| GET | `/api/v1/keys` | List admin API keys. |
| POST | `/api/v1/keys` | Create an admin API key. |
| DELETE | `/api/v1/keys/{id}` | Revoke an admin API key. |

### Sign relay — `/rp/sign/*` (RP proxy)

Active **only when `SIGN_RELAY_TOKEN` and `EID_RP_SECRET` are both set**. A
reverse proxy that lets third-party RPs sign via dan's eID Mongolia credentials:
the caller presents the shared relay token as `Authorization: Bearer <token>`;
the relay swaps it for dan's real eID RP secret and forwards to eID Mongolia.
Both `/rp/sign` and `/rp/sign/*` are handled.

---

## Operations (no `/api/v1` prefix)

| Method | Path | Gate | Description |
|--------|------|------|-------------|
| GET | `/health` | open | Liveness — always 200 if the process is up. |
| GET | `/ready` | open | Readiness — pings Postgres (pgx pool) + Redis. |
| GET | `/metrics` | ObservabilityGate | Prometheus exposition (bearer-gated + 404-hidden in production). |
| GET | `/swagger/*` · `/swagger/doc.json` | ObservabilityGate | Swagger UI + spec (gated in production). |
| GET | `/api/` | open | Root "alive" JSON. |

`ObservabilityGate` requires the observability bearer token and returns 404
(not 401) in production when unauthenticated.

---

🔒 = requires `Authorization: Bearer <access_token>`; 🛡️ = additionally requires
the named RBAC permission. Regenerate the swagger spec from handler annotations
with `make swag`. (Seven legacy `auth_*` handlers still carry `@Router`
annotations for password/OTP endpoints that are **not** registered — the auth
surface above reflects `route_auth.go`, which is authoritative.)

---

**Government Template Platform V3.0** — Co-developed by the **Gerege Systems Development Team** and **Claude AI**, 2026.
</content>
</invoke>
