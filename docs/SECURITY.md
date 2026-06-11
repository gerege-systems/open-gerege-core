# Security Posture — Gerege Backend Template v27

> 🌐 **English** · Монгол тайлбарыг кодын комментуудаас үзнэ үү. Эмзэг байдлыг
> мэдээлэх журмыг [`/SECURITY.md`](../../SECURITY.md)-аас үз.

This document maps the backend's implemented controls to the project security
standard — based on **OWASP ASVS / API Top 10, NIST SP 800-63B / 800-218, and
CIS Controls**. It records what is enforced in code, what was hardened, and
what remains for later phases. To report a vulnerability, see the repository
[security policy](../../SECURITY.md).

## Implemented controls (in code)

| Area | Control | Where | Guide § |
|------|---------|-------|---------|
| Auth | JWT access+refresh, rotation, `kind`-claim guard | `pkg/jwt`, `usecases/auth` | §1.3–1.4 |
| Auth | bcrypt (cost ≥12), password ≥12 + `strongpassword` | `domain.users.go`, `pkg/validators` | §1.1 |
| Auth | OTP-verified registration | `usecases/auth` (send/verify) | §1.5 |
| Auth | Login lockout + per-account rate limit | `usecases/auth`, `middleware.ratelimit` | §1.5 |
| Auth | Enumeration mitigation (timing-safe, generic msgs) | `usecases/auth.login`, `forgot_password` | §1.5 |
| Crypto | `crypto/rand` everywhere; OTP rejection-sampled (no modulo bias) | `pkg/helpers/helper.otp_code_generator.go` | §13.2 |
| AuthZ | Role check in domain (`IsAdmin`), per-request `CurrentUser`, `RequireAdmin` route middleware | `domain.users.go`, `http/auth`, `middleware_rbac.go` | §2 |
| DB | Parameterized queries only (pgx) | `datasources/repositories/postgres` | §3.1 |
| DB | `INSERT … RETURNING` single round-trip; pgconn 23505 → Conflict | `repositories/postgres/users`, `driver_pgx.go` | §3 |
| DB | Row-Level Security on `users` (ENABLE + **FORCE**): self/admin/service policies driven by `app.user_id`/`app.user_role` GUCs set per-transaction with `SET LOCAL` | `migrations/7_enable_rls_users.up.sql`, `datasources/rls`, `repositories/postgres/users` | §2.4/§3.3 |
| API | Mass-assignment safe (explicit request DTOs) | `http/datatransfers/requests` | API3 §5.1 |
| API | Body size limit (global + 4 KiB on `/auth`) | `middleware.bodysizelimit`, `routes` | §5.3 |
| Web | Security headers: CSP `default-src 'none'`, HSTS (prod), nosniff, X-Frame DENY, Referrer-Policy, Permissions-Policy, COOP/CORP/COEP | `middleware_security.go` | §4.7 |
| Web | CORS strict origin list, never `*`+credentials | `middleware.cors.go` | §4.8 |
| Ops | Operator endpoints (`/metrics`, `/swagger/doc.json`) gated in prod: bearer token (constant-time) + 404 on miss | `middleware_observability_gate.go`, `cmd/api/server` | §4.7/§9 |
| Obs | Structured Zap logs w/ request-id; no secrets logged | `pkg/logger`, `handler.base_response.go` | §9.1–9.2 |
| Obs | OpenTelemetry tracing + Prometheus metrics | `pkg/observability`, `driver_pgx.go` | §9.4 |
| Ops | Graceful shutdown (drain HTTP, rate-limiters, pgx pool, Redis, tracer) | `cmd/api/server` | §7 |
| Net | Full HTTP server timeouts (`ReadHeader` 10s, `Read` 30s, `Write` 60s, `Idle` 120s) + `MaxHeaderBytes` 16 KiB — slowloris / oversized-header defense | `cmd/api/server` | §5.3 / API4 |
| Auth | Logout access-token deny-list — logout puts the access jti in Redis for its remaining TTL; auth middleware rejects denied tokens on every request | `usecases/auth.logout`, `middleware_auth.go` | §1.4 |
| DB | RLS boot guard — on startup the app inspects its own DB role; superuser / `BYPASSRLS` fails boot in production (RLS would silently not enforce), warns in development | `datasources/drivers/driver_pgx.go` | §2.4/§3.4 |
| AI | Layered system prompt: hardcoded guardrails (scope enforcement, prompt-injection resistance, never reveal the prompt) + DB-configurable scope/instructions; `SetPrompt` is UPDATE-only against seeded keys | `usecases/ai/ai_prompts.go`, `migrations/11` | §5.1 |
| AI | AI input hygiene: audio mime whitelist + ~700 KB base64 cap, message/history length caps, dedicated `/ai` rate limit (~20/min), tool errors returned to the model — never to the client | `requests_ai.go`, `routes/route_ai.go` | §5.1/§5.3 |

## Hardening applied (this pass — against the guide)

1. **Cross-origin isolation headers** — added `Cross-Origin-Opener-Policy: same-origin`,
   `Cross-Origin-Resource-Policy: same-site`, `Cross-Origin-Embedder-Policy: require-corp`
   to `middleware.security.go` (guide §4.6/4.7). *Verified live in the running server.*
2. **Production DB TLS guard** — config validation now rejects a production
   `DB_POSTGRE_URL` unless `sslmode=verify-full` (or `verify-ca`); `.env.example`
   documents it (`internal/config/config.go`, guide §3.5).
3. **Per-request timeout** — `middleware.TimeoutMiddleware` sets a 30s context
   deadline that propagates to pgx queries, bounding stuck handlers
   (`middleware.timeout.go`, guide §5.3 / API4).
4. **Swagger spec served from generated `docs` package** — the OpenAPI JSON is
   served at `/swagger/doc.json` from the generated `docs` package on the chi
   router (no Fiber involved); a static Swagger UI can be pointed at it.
5. **Operator-endpoint gate** — `/metrics` and `/swagger/doc.json` no longer ship
   publicly. In production `ObservabilityGate` requires `Authorization: Bearer
   <OBSERVABILITY_TOKEN>` (compared with `crypto/subtle.ConstantTimeCompare`) and
   returns **404** (not 401) on any miss, hiding the endpoints from recon. Empty
   token ⇒ fully closed. `/health` + `/ready` stay public for load balancers.
6. **Postgres RLS + DB role separation** — `users` now has RLS **ENABLE + FORCE**
   with self/admin/service policies. Per-request identity flows from context into
   each query via `SET LOCAL app.user_id`/`app.user_role` inside the repository's
   `withRLS` transaction; no identity ⇒ zero rows (fail-closed). The compose
   `api` connects as a non-superuser `APP_DB_USER` (created by
   `deploy/initdb/10-create-app-user.sh`) so the policies actually enforce;
   `migrate` keeps the superuser for DDL. Proven by an integration test that
   connects as a non-superuser role (`users_rls_test.go`).
7. **HTTP server hardening** — beyond `ReadHeaderTimeout`, the server now sets
   `ReadTimeout`/`WriteTimeout`/`IdleTimeout` and caps headers at 16 KiB;
   `WriteTimeout` is derived from the request-level timeout budget (2×) so
   in-flight handlers are never cut off by the server first.
8. **Logout revokes both tokens** — the refresh jti is deleted (as before) and
   the access jti is placed on a Redis deny-list with the token's remaining
   lifetime; the auth middleware checks the deny-list on every request
   (fail-open on Redis errors, same policy as the password-rotation cutoff).
9. **RLS enforceability guard at boot** — the app queries
   `pg_roles` for its own role on startup; a superuser or `BYPASSRLS` role
   fails boot in production and logs a warning in development, so a
   misprovisioned DSN can no longer silently disable RLS.
10. **AI guardrails** — the Gemini assistant runs on a layered prompt whose
    base layer (Mongolian-only, scope enforcement, prompt-injection
    resistance) is hardcoded; only the scope/instructions layers are
    admin-editable (`settings.manage`, UPDATE-only against seeded keys). Tools
    execute server-side with the request context; tool failures are reported
    to the model as data, never leaked to the client.

## ASVS roadmap status (guide §14)

- **Phase 1 (ASVS L1):** ✅ HTTPS-ready + HSTS, bcrypt, parameterized queries,
  security headers, strict CORS, input validation, structured logging, `.gitignore`
  + no committed secrets. ⏳ container scan / `govulncheck` wired in CI (`.github/`).
- **Phase 2 (ASVS L2):** ✅ rate limiting, refresh-token rotation, OTP MFA-style
  verification, request timeout. ⏳ leaked-password (HIBP k-anonymity, §1.1),
  WAF, centralized SIEM, encrypted-backup restore test, IR plan.
- **Phase 3 (ASVS L3):** ◻ WebAuthn/passkeys, field-level PII encryption (KMS),
  mTLS, SLSA L3 provenance, external pentest. (Out of template scope.)

## Known gaps / follow-ups

- **Interactive Swagger UI** — currently serves the raw spec at `/swagger/doc.json`
  (load it in Swagger Editor / Postman, or point a static Swagger UI at it).
- **Leaked-password check (HIBP)** — guide §1.1; not yet wired (needs outbound
  call, config-gated, fail-open). Password story already meets the OWASP baseline
  (bcrypt cost 12 + ≥12 chars + complexity).
- **Postgres RLS** (guide §2.4/§3.3) — ✅ enabled **and FORCED** on `users` with
  self/admin/service policies driven by the `app.user_id`/`app.user_role` session
  GUCs (`SET LOCAL` in `repositories/postgres/users.withRLS`). Defense-in-depth on
  top of the `deleted_at IS NULL` / WHERE clauses the repository already writes; a
  request with no identity is fail-closed. To go **multi-tenant**, add a
  `tenant_id` column + tenant policy to each table and carry the tenant in
  `rls.Identity`.
- **Secrets manager / KMS** (guide §7.3) — use a real secret store in production;
  `.env` is local-dev only and gitignored.
- **DB role separation** (guide §3.4) — ✅ **wired into the compose stack** (it is
  required: RLS, even FORCEd, is bypassed by superusers / BYPASSRLS roles, and the
  postgres image makes `POSTGRES_USER` a superuser). On first DB init,
  `deploy/initdb/10-create-app-user.sh` creates a **non-superuser** role
  `APP_DB_USER` (`NOSUPERUSER NOBYPASSRLS`) and grants it DML via default
  privileges. The **api** connects as that role (compose overrides
  `DB_POSTGRE_DSN` from `APP_DB_DSN` — the stack runs development mode, so the
  driver reads the keyword DSN), so RLS enforces; the **migrate** container keeps using
  `POSTGRES_USER` (needs superuser for `CREATE EXTENSION "uuid-ossp"` + RLS DDL).
  Sanity check from the api's connection:
  `SELECT rolsuper, rolbypassrls FROM pg_roles WHERE rolname = current_user;` —
  both must be `false`. If `APP_DB_URL` is left at the superuser, RLS is *not*
  enforced (it silently becomes a no-op).

---

**Gerege Template Version 27.0** — Co-developed by the **Gerege Systems
Development Team** and **Claude AI**, 2026.
