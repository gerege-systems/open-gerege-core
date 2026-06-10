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
| AuthZ | Role check in domain (`IsAdmin`), per-request `CurrentUser` | `domain.users.go`, `http/auth` | §2 |
| DB | Parameterized queries only (GORM) | `datasources/repositories/postgres` | §3.1 |
| DB | `INSERT … RETURNING` single round-trip; `TranslateError` | `users.store.go`, `driver.gorm.go` | §3 |
| API | Mass-assignment safe (explicit request DTOs) | `http/datatransfers/requests` | API3 §5.1 |
| API | Body size limit (global + 4 KiB on `/auth`) | `middleware.bodysizelimit`, `routes` | §5.3 |
| Web | Security headers: CSP `default-src 'none'`, HSTS (prod), nosniff, X-Frame DENY, Referrer-Policy, Permissions-Policy | `middleware.security.go` | §4.7 |
| Web | CORS strict origin list, never `*`+credentials | `middleware.cors.go` | §4.8 |
| Obs | Structured Zap logs w/ request-id; no secrets logged | `pkg/logger`, `handler.base_response.go` | §9.1–9.2 |
| Obs | OpenTelemetry tracing + Prometheus metrics | `pkg/observability`, `driver.gorm.go` | §9.4 |
| Ops | Graceful shutdown (drain HTTP, mailer, DB, Redis, tracer) | `cmd/api/server` | §7 |

## Hardening applied (this pass — against the guide)

1. **Cross-origin isolation headers** — added `Cross-Origin-Opener-Policy: same-origin`,
   `Cross-Origin-Resource-Policy: same-site`, `Cross-Origin-Embedder-Policy: require-corp`
   to `middleware.security.go` (guide §4.6/4.7). *Verified live in the running server.*
2. **Production DB TLS guard** — config validation now rejects a production
   `DB_POSTGRE_URL` unless `sslmode=verify-full` (or `verify-ca`); `.env.example`
   documents it (`internal/config/config.go`, guide §3.5).
3. **Per-request timeout** — `middleware.TimeoutMiddleware` sets a 30s context
   deadline that propagates to GORM queries, bounding stuck handlers
   (`middleware.timeout.go`, guide §5.3 / API4).
4. **Swagger served Fiber-v3-natively** — replaced the Fiber-v2-only
   `gofiber/swagger` handler (which panicked at runtime under Fiber v3) with a
   native `/swagger/doc.json` OpenAPI endpoint.

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
  (load it in Swagger Editor / Postman). A Fiber-v3-compatible UI handler can be
  added later.
- **Leaked-password check (HIBP)** — guide §1.1; not yet wired (needs outbound
  call, config-gated, fail-open). Password story already meets the OWASP baseline
  (bcrypt cost 12 + ≥12 chars + complexity).
- **Multi-tenancy / Postgres RLS** (guide §2.4/§3.3) — N/A: this template is
  single-tenant. Add `tenant_id` + RLS if it becomes multi-tenant.
- **Secrets manager / KMS** (guide §7.3) — use a real secret store in production;
  `.env` is local-dev only and gitignored.
- **DB role separation** (guide §3.4) — run the app as a least-privilege
  `app_user` (DML only), not a superuser, in production.

---

**Gerege Template Version 27.0** — Co-developed by the **Gerege Systems
Development Team** and **Claude AI**, 2026.
