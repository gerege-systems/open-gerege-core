# Government Template Platform V3.0 — Backend (Go)

> **Цахим засаглалыг бүтээх суурь** — _Нэг суурь — бүх төрийн үйлчилгээ._

> 🌐 [English](README.md) · **Монгол**

[![Go](https://img.shields.io/badge/Go-1.26-blue.svg)](https://golang.org/)
[![chi](https://img.shields.io/badge/chi-v5-00ADD8.svg)](https://github.com/go-chi/chi)
[![pgx](https://img.shields.io/badge/pgx-v5-336791.svg)](https://github.com/jackc/pgx)
[![License](https://img.shields.io/badge/License-MIT-green.svg)](LICENSE)

**Government Template Platform V3.0**-ийн Go backend — *аль ч* цахим төрийн
үйлчилгээг дээр нь босгож болох, үйлдвэрлэлд бэлэн суурь (foundation). Сахилга
баттай **Clean Architecture** цөмийг гар бичсэн **pgx SQL** (ORM-гүй)-тэй хослуулж,
төрийн түвшний бүрэн чадамжийг хайрцгаас нь гарган ирдэг: **eID Mongolia** танилт,
**Google** account холболт, **PAdES** баримтын гарын үсэг,
**Gemini AI** pipeline, олон давхаргат аюулгүй байдлын хатууруулалт — бүгд хос
хэлтэй (mn/en), эхнээсээ observable. **chi (net/http)** (HTTP), **pgx (pgxpool) +
PostgreSQL** (өгөгдөл), **Redis + Ristretto** (кэш) дээр суурилсан.

> **Жишиг deployment:** **Government Template Platform** ([template.dgov.mn](https://template.dgov.mn))
> — төрийн үйлчилгээний платформ бөгөөд Government SSO-ийн Relying Party — энэ суурин
> дээр бүтээгдсэн жишээ бөгөөд eID нэвтрэлт болон бусад аппад зориулсан өөрийн OIDC
> provider-ийг харуулдаг.

## 📌 Эх сурвалж ба нээлттэй эх (Open Source)

> Энэ template нь **нээлттэй эх кодын төсөл
> [snykk/go-rest-boilerplate](https://github.com/snykk/go-rest-boilerplate)**
> (зохиогч: Najib Fikri, **MIT лиценз**) дээр **суурилж, түүнээс санаа авч**
> бүтээгдсэн. Clean Architecture бүтэц, JWT/OTP танилт, audit, кэш,
> observability, тестийн стратеги зэрэг нь тэндээс уламжлагдсан.
>
> Бид дараах хоёр зүйлийг **хөрвүүлсэн**:
> - HTTP давхарга: **Gin → chi (net/http)**
> - Өгөгдлийн давхарга: **sqlx → pgx (pgxpool, гар бичсэн SQL)**
>
> Эх төсөл MIT лицензтэй бөгөөд түүний зохиогчийн эрх,
> лицензийн нөхцлийг хүндэтгэн хадгалсан (доорх [Зохиогчид](#-зохиогчид--лиценз)
> хэсгийг үз). Энэ template өөрөө мөн **MIT лицензтэй**.

## Онцлог

- **Clean Architecture** — `handler → usecase → repository → domain`, дотогшоо чиглэсэн хамаарал, back-import байхгүй
- **chi (net/http)** — стандарт сангийн идиоматик router
- **pgx (pgxpool)** — гар бичсэн SQL, ORM-гүй; `deleted_at IS NULL`-аар тодорхой soft-delete
- **eID танилт** — цорын ганц нэвтрэх арга: eID Mongolia Relying Party (QR / мобайл deep-link / РД push) + long-poll session; JWT access + refresh token гаргана (rotation, `kind` claim guard)
- **Google OAuth холболт** — Google account-ийг eID хэрэглэгчид холбоно (code exchange зөвхөн server талд), дараа нь түүгээр нэвтэрнэ
- **OIDC provider (SSO)** — DAN-ийг identity provider болгох сонголттой Ory Hydra урд тал; login/consent/logout урсгал + RP client бүртгэлийн `/admin` гадаргуу (зөвхөн Hydra тохируулагдсан үед)
- **eID PKI профайл** — нэвтэрсэн иргэний холбоотой байгууллага ба гарын үсэг зурагчид, гэрчилгээ, төхөөрөмж, идэвх
- **Байгууллага ба гишүүнчлэл** — байгууллага үүсгэх/хайх (Gerege Verify/XYP улсын бүртгэлийн лавлагаа) + гишүүн/эрх удирдах, хэрэглэгч тус бүрт RLS-ээр
- **Төрийн үйлчилгээний портал** — каталог, хүсэлт, лавлагаа, мэдэгдэл, төлбөр, цаг захиалга
- **API gateway** — services / routes / consumers / API key / policy + хүсэлтийн телеметр (админ удирддаг)
- **Баримт бичгийн гарын үсэг (PAdES)** — eID Mongolia `/v3`-ээр server талын PDF гарын үсэг, байнгын Document-Signer гэрчилгээтэй; 3 дагч RP-д зориулсан сонголттой sign-relay
- **Интеграци ба хадгалалт** — хэрэглэгч тус бүрийн OAuth интеграци (Google Drive/Meet, Dropbox) AES-256-GCM токен шифрлэлттэй; Gerege Space апп-ын өөрийн SFTP хадгалалт
- **AI pipeline (Gemini)** — SDK-гүй REST client + function calling: текст/дуут чат, STT, TTS, шууд орчуулга; давхаргат prompt (кодод хатуу суурь дүрэм + DB-ээс тохируулдаг хүрээ) ба DB-д суурилсан `search_knowledge` tool
- **RBAC ба super admin** — динамик role + permission каталог; 4-үүрэгт загвар (superadmin → admin → manager → user)
- **Сайтын харагдац** — админ тохируулдаг сайт-даяар харагдац (accent/font/density/theme) + хэрэглэгч тус бүрийн override
- **Audit log** — hash-chain холбоост, зөвхөн-нэмэх audit бүртгэл (админ-л унших + бүрэн бүтэн байдал шалгах)
- **Observability** — OpenTelemetry trace + Prometheus metrics; production-д `/metrics` + `/swagger` bearer token-оор хаагдана
- **Кэш** — Redis + Ristretto хоёр түвшний
- **Integration Testing** — testcontainers-go (жинхэнэ Postgres + Redis)
- **Swagger** — godoc annotation-аас автомат API баримтжуулалт
- **Structured Logging** — Zap, request ID дамжуулалттай
- **Security** — security headers, CORS, rate limiting, body size limit, серверийн бүрэн timeout-ууд, Postgres RLS + boot-үеийн мөрдөлтийн guard, logout-ийн access deny-list
- **Graceful Shutdown** — HTTP, DB pool, Redis, tracer-ийг дарааллаар drain хийх

## Төслийн бүтэц

```
.
├── cmd/
│   ├── api/main.go              # Аппликейшн эхлэх цэг
│   ├── api/server/server.go     # Composition root (гар DI)
│   ├── migration/               # Migration CLI
│   ├── seed/                    # Seed CLI
│   └── healthcheck/             # Distroless health probe
├── internal/
│   ├── business/
│   │   ├── domain/              # Domain entities (хамгийн дотоод давхарга)
│   │   └── usecases/           # Business logic (interface + impl), модуль тус бүрт нэг package:
│   │       #  auth · users · rbac · superadmin · ai · audit · security · site
│   │       #  org · gov · gateway · core · sso · provider · sign · assets
│   │       #  integrations · gspace
│   ├── datasources/
│   │   ├── drivers/             # pgx (pgxpool) Postgres холболт (driver_pgx.go)
│   │   ├── caches/              # Redis + Ristretto
│   │   ├── migration/           # Migration runner
│   │   ├── records/             # pgx record struct + record↔domain mapper
│   │   └── repositories/        # interface + postgres impl
│   ├── http/
│   │   ├── handlers/v1/         # HTTP handlers
│   │   ├── middlewares/         # Middleware stack
│   │   ├── routes/              # Route бүртгэл
│   │   ├── datatransfers/       # Request/Response DTO
│   │   └── auth/                # context-аас CurrentUser
│   └── config/ apperror/ constants/
├── migrations/                  # SQL migrations
├── docs/                        # Swagger + ARCHITECTURE.md + DEVELOPMENT.md
└── pkg/                         # jwt, logger, clock, helpers, validators,
                                 # audit, observability, gemini,
                                 # eid, google, oidc, hydra, xyp, gspace, verify
```

## Түргэн эхлүүлэх

### Шаардлага
- Go 1.26+
- PostgreSQL 15+
- Redis 7+
- Docker (integration тест / локал стек-д)
- Make

### Суулгалт

```bash
# 1. Environment файл хуулах (internal/config/ дотор байрладаг)
cp internal/config/.env.example internal/config/.env
# .env засах — JWT_SECRET доод тал нь 32 тэмдэгт байх ёстой

# 2. Стек өргөх (Postgres + Redis + API)

# 3. Эсвэл локалаар: migration → server
```

Сервер: `http://localhost:8080`, Swagger UI: `http://localhost:8080/swagger/`.

### Make командууд

```bash
make build              # Binary бүтээх
make test               # Unit тест (mock — хурдан, Docker-гүй)
make test-integration   # Integration тест (Docker шаардана)
make swag               # Swagger docs үүсгэх
make lint               # golangci-lint
make pre-push           # CI шалгалтыг локалаар (lint+test+swag+build)
```

## Тохиргоо

`internal/config/.env.example`-аас үндсэн хувьсагчид:

```env
# Үндсэн
PORT=8080
ENVIRONMENT=development          # development | production
JWT_SECRET=...                   # >= 32 тэмдэгт (HS256)
JWT_EXPIRED=5                    # access token TTL (цаг, 1..24)
JWT_REFRESH_EXPIRED=7            # refresh token TTL (хоног)
DB_POSTGRE_DSN=...               # dev үед DSN
DB_POSTGRE_URL=...               # production үед URL (sslmode=verify-full/verify-ca байх ёстой)
REDIS_HOST=localhost:6379
BCRYPT_COST=12                   # 10..31
ALLOWED_ORIGINS=                 # production-д заавал (таслалаар)
TRUSTED_PROXIES=                 # X-Forwarded-For-д итгэх урвуу proxy IP/CIDR
OBSERVABILITY_TOKEN=             # production-д /metrics + /swagger-ийг хаах bearer token

# eID Mongolia (Relying Party) — үндсэн нэвтрэлт; boot эвдэхгүй зохистой default-той
EID_BASE_URL=https://eidmongolia.mn/v3
EID_RP_UUID=                     # IdP-д бүртгэгдсэн RP UUID
EID_RP_NAME=                     # RP-ийн харагдах нэр
EID_RP_SECRET=                   # RP API secret (мөн /rp/sign relay-д ашиглана)
EID_CERT_LEVEL=ADVANCED          # ADVANCED | QUALIFIED | QSCD
EID_CALLBACK_URL=                # IdP-ийн allowlist-д бүртгэгдсэн байх ёстой
EID_DISPLAY_TEXT=

# Google OAuth — Google account-ийг eID хэрэглэгчид холбоно
GOOGLE_CLIENT_ID=
GOOGLE_CLIENT_SECRET=

# OIDC PROVIDER тал (платформ өөрөө issuer) — тохируулаагүй бол урсгал inert
OAUTH_ISSUER=                    # issuer, жишээ https://template.dgov.mn (хоосон = provider унтарна)
SSO_STATE_KEY=                   # >= 32 байт; login/consent state cookie HMAC
SSO_FIRSTPARTY_CLIENTS=          # consent дэлгэцийг алгасах client_id-уудын CSV
SSO_ADMIN_API_KEYS=              # /admin гадаргуугийн bootstrap key-үүдийн CSV

# Баримт бичгийн гарын үсэг (PAdES) — байнгын Document-Signer материал (production-д заавал)
SIGN_SIGNER_CERT_FILE=
SIGN_SIGNER_KEY_FILE=
SIGN_RELAY_TOKEN=                # 3 дагч RP-ууд DAN-ий eID креденшлээр гарын үсэг зурах shared token

# Gerege улсын үйлчилгээ
XYP_API_BASE=https://xyp.dgov.mn # байгууллагын лавлагаа (улсын бүртгэл); Basic auth
XYP_CLIENT_ID=
XYP_CLIENT_SECRET=
CORE_API_BASE=https://core.gerege.mn  # Gerege Core user/org find
CORE_API_TOKEN=

# Gerege Space — апп-ын өөрийн SFTP хадгалалт (хоосон = функц идэвхгүй)
GSPACE_HOST=
GSPACE_PORT=22
GSPACE_USER=
GSPACE_PASSWORD=
GSPACE_BASE_PATH=gerege-space
GSPACE_QUOTA_BYTES=2097152       # хэрэглэгч тус бүрийн квот (default 2 MB)

# Интеграцийн токен шифрлэлт (AES-256-GCM) — production-д заавал
INTEGRATION_ENC_KEY=

# GeregeCloud Verify (verify.gecloud.mn) — OTP transport; production-д заавал
VERIFY_API_KEY=
VERIFY_API_BASE=https://verify.gecloud.mn/v1
VERIFY_CHANNEL=email

# AI pipeline (/api/v1/ai/*)
GEMINI_API_KEY=                  # хоосон = AI идэвхгүй (endpoint 500 буцаана)
GEMINI_MODEL=gemini-2.5-flash    # сонголттой override (чат / STT / орчуулга)
GEMINI_TTS_MODEL=gemini-2.5-flash-preview-tts  # сонголттой override (TTS)
GEMINI_VOICE=Kore                # сонголттой prebuilt TTS дуу хоолой
GEMINI_API_BASE=                 # сонголттой override (өгөгдмөл: Google generativelanguage v1beta)
AI_SCOPE_PROMPT=                 # DB-ийн 'scope' давхарга хоосон үеийн хамрах хүрээний fallback

# Observability + bootstrap
OTEL_EXPORTER=                   # хоосон=унтраах | stdout | otlp
SUPERADMIN_EMAIL=                # сонголттой: boot үед энэ (аль хэдийн нэвтэрсэн) хэрэглэгчийг super admin болгоно
```

> **Google нэвтрэлт:** Google account-ийг eID хэрэглэгчид холбоно — эхний удаа
> ЗААВАЛ eID-ээр баталгаажуулж бодит хүнтэй холбоно, дараа нь Google-ээр шууд
> нэвтэрнэ. Google Cloud Console-д OAuth client үүсгэж, authorized redirect URI-д
> `https://<domain>/api/auth/google/callback` нэмнэ. Frontend-д `GOOGLE_CLIENT_ID`
> (BFF redirect-д) + `APP_ORIGIN` хэрэгтэй; backend-д `GOOGLE_CLIENT_ID` +
> `GOOGLE_CLIENT_SECRET` (code exchange). Хоосон бол Google товч ажиллахгүй.

> **eID Mongolia нэвтрэлт:** энэ template нь Smart-ID нийцтэй eID Mongolia
> (eidmongolia.mn) v3 RP API-аар нэвтэрдэг — QR (device-link/anonymous) болон
> РД push (notification/etsi) урсгал. `EID_RP_UUID`/`EID_RP_SECRET` хоосон бол
> нэвтрэлт ажиллахгүй; оператороос RP-гээ бүртгүүлж авна (support@eidmongolia.mn).

### Эрх (role) ба super admin

Role-ууд эрхийн зэрэглэлээр дугаарлагдсан (id 1 = хамгийн дээд):
**superadmin=1, admin=2, manager=3, user=4** (`23_superadmin_role` migration-оор
seed/remap хийгдэнэ). **Super admin** нь admin-аас дээгүүр бөгөөд админ
бүртгэлүүдийг удирдах (үүсгэх/эрх олгох/хасах) цорын ганц эрх —
`/api/v1/superadmin/*` (`RequireSuperAdmin`); энгийн admin энэ гадаргууд
хүрэхгүй. API нь super admin-г хэзээ ч үүсгэдэггүй — bootstrap хийхдээ
`SUPERADMIN_EMAIL`-д аль хэдийн eID-ээр нэвтэрсэн хэрэглэгчийн и-мэйлийг заана
(дараагийн boot-д ахиулна) эсвэл DB-д `role_id=1` болгоно.

> **Эвдрэлтэй өөрчлөлт (одоо ажиллаж буй deployment):** `23` migration нь role-
> уудыг дахин дугаарладаг тул түүнээс өмнө олгосон JWT-үүд өөр утгаар унших
> болно (хуучин `admin=1` → superadmin, `user=2` → admin). Одоо байгаа DB дээр
> хэрэглэхдээ **`JWT_SECRET`-ээ солино** (эсвэл бүх хэрэглэгчийг дахин нэвтрүүлнэ)
> — эс бөгөөс хуучин токен буруу эрх авна. Шинэ суулгацад нөлөөгүй.

### AI prompt давхаргууд

AI туслах давхаргат system prompt-оор ажиллана: **suurь дүрэм** (кодод
хатуу — зөвхөн Монголоор, хүрээний сахилт, prompt-injection эсэргүүцэл)
+ **хамрах хүрээ** (юугаар туслахыг заана) + **нэмэлт заавар** (сонголттой).
Хүрээ/зааврыг `ai_prompts` хүснэгтэд хадгалж, `GET/PUT /api/v1/admin/ai/prompts`
(`settings.manage` эрх; UI: Админ → Тохиргоо)-оор ажиллаж байх үед нь
өөрчилнө. Туслах хүрээнээс гадуурх асуултад татгалзаж, платформын асуултад
`search_knowledge` tool-оор `ai_knowledge` хүснэгтээс хайж тулгуурлан хариулна.

## API Endpoints

Бүгд `/api/v1` дор (ops endpoint-ууд root дээр). **Нууц үг / и-мэйл-OTP /
бүртгэл / нууц үг сэргээх endpoint байхгүй** — танилт зөвхөн eID + Google.

### Нийтийн (Authentication)
| Method | Path | Тайлбар |
|--------|------|---------|
| POST | `/api/v1/auth/eid/start` | eID нэвтрэлт эхлүүлэх (QR / мобайл deep-link) |
| POST | `/api/v1/auth/eid/start-id` | Иргэний РД-аар eID нэвтрэлт (бүртгэлтэй төхөөрөмж рүү push) |
| POST | `/api/v1/auth/eid/poll` | eID session-ийг дуустал long-poll хийх |
| POST | `/api/v1/auth/google` | Google OAuth callback — code exchange + eID холбох / нэвтрэх |
| POST | `/api/v1/auth/refresh` | Token rotation |
| POST | `/api/v1/auth/logout` | Refresh хүчингүй болгож + access deny-list |

### Хамгаалагдсан (JWT шаардана)
| Method | Path | Тайлбар |
|--------|------|---------|
| GET | `/api/v1/users/me` | Хэрэглэгчийн профайл |
| GET | `/api/v1/rbac/me` | Одоогийн хэрэглэгчийн үр дүнтэй role/permission |
| DELETE | `/api/v1/auth/google/link` | Холбосон Google account-ийг салгах |
| GET | `/api/v1/me/*`, `/api/v1/users/me/eid/*` | eID PKI профайл — байгууллага, гарын үсэг зурагчид, гэрчилгээ, төхөөрөмж, идэвх |
| CRUD | `/api/v1/org/*` | Байгууллага + гишүүнчлэл (улсын бүртгэл лавлагаа, гишүүд, эрх) |
| GET/POST | `/api/v1/gov/*` | Төрийн үйлчилгээний портал — үйлчилгээ, хүсэлт, лавлагаа, мэдэгдэл, төлбөр, цаг захиалга |
| CRUD | `/api/v1/gateway/*` | API gateway — services, routes, consumers, keys, policies, logs |
| GET | `/api/v1/core/users` · `/organizations` | Gerege Core find (user/org лавлагаа) |
| CRUD | `/api/v1/integrations/*` | Хэрэглэгчийн OAuth интеграци (токен шифрлэгдсэн) |
| GET | `/api/v1/assets/*` | Гарын үсгийн зураг + байгууллагын тамгын asset |
| GET | `/api/v1/gspace/*` | Gerege Space SFTP хадгалалт (жагсаах + татах) |
| POST/GET | `/api/v1/sign/*` | Баримт бичгийн гарын үсэг (PAdES) — init, төлөв, татах |
| POST | `/api/v1/ai/chat` | AI чат (Gemini pipeline, function calling, текст/дуут мессеж) |
| POST | `/api/v1/ai/stt` | Яриа→текст (audio base64 → transcript) |
| POST | `/api/v1/ai/tts` | Текст→яриа (текст → WAV base64) |
| POST | `/api/v1/ai/translate` | Шууд орчуулга (текст/audio → зорилтот хэл, сонголтоор TTS) |
| GET | `/api/v1/site/appearance` | Сайт-даяар харагдацын default (нийтийн унших) |
| GET/PUT | `/api/v1/admin/ai/prompts` | AI prompt давхарга — хүрээ/заавар (settings.manage) |
| GET | `/api/v1/audit` · `/audit/verify` | Audit log унших + hash chain шалгах (админ) |
| POST | `/api/v1/security/events` | Client security event бүртгэх |
| GET | `/api/v1/superadmin/admins` | Админ түвшний бүртгэлүүдийг жагсаах (зөвхөн super admin) |
| POST | `/api/v1/superadmin/admins` | Шинэ админ үүсгэх (зөвхөн super admin) |
| PUT | `/api/v1/superadmin/admins/{id}/grant` | Байгаа хэрэглэгчид админ эрх олгох (зөвхөн super admin) |
| DELETE | `/api/v1/superadmin/admins/{id}` | Админ эрх хасах (зөвхөн super admin) |

### OIDC provider (зөвхөн Hydra тохируулагдсан үед)
`GET /api/v1/provider/login` · `/consent`, мөн login/consent/logout-ийн
accept/reject (Hydra-аар жолоодогдох login/consent дэлгэц). RP OAuth2 client
бүртгэл нь mount хийсэн `/admin` гадаргуу дор.

### Ops
`GET /health` (liveness) · `GET /ready` (DB+Redis) · `GET /metrics` · `GET /swagger/doc.json`
— production-д `/metrics` ба `/swagger` нь `OBSERVABILITY_TOKEN` bearer шаардана (эс бөгөөс 404).

### Response формат
```json
{ "status": true, "message": "login success", "data": { }, "request_id": "…" }
```
Алдааны үед `status:false`. Validation алдаа → `422`, `data.errors` дотор талбар бүрээр.

## Хөгжүүлэлт

Дэлгэрэнгүйг үз:
- **[docs/ARCHITECTURE_MN.md](docs/ARCHITECTURE_MN.md)** — давхаргын бүтэц, dependency flow, security
- **[docs/DEVELOPMENT_MN.md](docs/DEVELOPMENT_MN.md)** — шинэ фичер нэмэх 8 алхам, тест, code style, troubleshooting
- **[docs/AI_PIPELINE_MN.md](docs/AI_PIPELINE_MN.md)** — AI туслахын дотоод бүтэц: урсгал, prompt давхарга, tools, voice, өргөтгөх заавар

```bash
make test               # Unit тест
make test-integration   # Integration тест (Docker)
make test-cover         # Coverage
```

## Docker

```bash
make build              # Binary
curl http://localhost:8080/health
```

## 🙏 Зохиогчид & Лиценз

Энэ template нь нээлттэй эх кодын ажил дээр тулгуурласан:

| Төсөл | Зохиогч | Лиценз | Юу ашигласан |
|-------|---------|--------|--------------|
| [snykk/go-rest-boilerplate](https://github.com/snykk/go-rest-boilerplate) | Najib Fikri | MIT | Үндсэн архитектур, auth/OTP/audit, кэш, observability, тест |
| [chi](https://github.com/go-chi/chi) · [pgx](https://github.com/jackc/pgx) | — | MIT | Router · Postgres драйвер |

**Бидний өөрчлөлт:** HTTP давхаргыг **Gin → chi (net/http)**, өгөгдлийн давхаргыг
**sqlx → pgx (pgxpool, гар бичсэн SQL)** болгосон; бусдыг үнэнчээр хадгалсан. MIT уламжлалын дагуу
эх төслүүдийн зохиогчийн эрхийн мэдэгдлийг хадгалсан бөгөөд энэ template нь
**MIT License**-тэй (`LICENSE` файлыг үз).

---

**Government Template Platform V3.0** — **Gerege Systems Development Team** болон
**Claude AI** хамтран бүтээв, 2026.
