# Gerege Backend Template v27

> 🌐 [English](README.md) · **Монгол**

[![Go](https://img.shields.io/badge/Go-1.26-blue.svg)](https://golang.org/)
[![chi](https://img.shields.io/badge/chi-v5-00ADD8.svg)](https://github.com/go-chi/chi)
[![pgx](https://img.shields.io/badge/pgx-v5-336791.svg)](https://github.com/jackc/pgx)
[![License](https://img.shields.io/badge/License-MIT-green.svg)](LICENSE)

Clean Architecture зарчмаар бүтээгдсэн, өндөр гүйцэтгэлтэй Go backend template.
**chi (net/http)** (HTTP), **pgx (pgxpool) + PostgreSQL** (өгөгдөл),
**Redis + Ristretto** (кэш), **JWT + OTP (GeregeCloud Verify)** (танилт) дээр
суурилсан.

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
- **JWT танилт** — access + refresh token (rotation, `kind` claim guard)
- **OTP бүртгэл** — имэйл OTP-оор баталгаажуулах, brute-force lockout
- **GeregeCloud Verify** — бүх имэйл/SMS OTP (бүртгэл + нууц үг сэргээх) verify.gecloud.mn-ээр; SMTP байхгүй
- **AI pipeline (Gemini)** — SDK-гүй REST client + function calling: текст/дуут чат, STT, TTS, шууд орчуулга; давхаргат prompt (кодод хатуу suurь дүрэм + DB-ээс тохируулдаг хүрээ) ба DB-д суурилсан `search_knowledge` tool
- **Audit log** — танилтын үйл явдлын бүртгэл
- **Observability** — OpenTelemetry trace + Prometheus metrics
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
│   │   └── usecases/{auth,users,rbac,ai}/  # Business logic (interface + impl)
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
                                 # audit, observability, verify, gemini
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
PORT=8080
ENVIRONMENT=development          # development | production
JWT_SECRET=...                   # >= 32 тэмдэгт (HS256)
JWT_EXPIRED=5                    # access token TTL (цаг)
JWT_REFRESH_EXPIRED=7            # refresh token TTL (хоног)
DB_POSTGRE_DSN=...               # dev үед DSN
DB_POSTGRE_URL=...               # production үед URL
REDIS_HOST=localhost:6379
BCRYPT_COST=12                   # 10..31
VERIFY_API_KEY=...               # GeregeCloud Verify OTP (production-д заавал)
VERIFY_API_BASE=https://verify.gecloud.mn/v1
VERIFY_CHANNEL=email
OTEL_EXPORTER=                   # хоосон=унтраах | stdout | otlp
ALLOWED_ORIGINS=                 # production-д заавал (таслалаар)
GEMINI_API_KEY=                  # AI pipeline (/api/v1/ai/*); хоосон = AI идэвхгүй
GEMINI_MODEL=gemini-2.5-flash    # сонголттой override (чат / STT / орчуулга)
GEMINI_TTS_MODEL=gemini-2.5-flash-preview-tts  # сонголттой override (TTS)
GEMINI_VOICE=Kore                # сонголттой prebuilt TTS дуу хоолой
GEMINI_API_BASE=                 # сонголттой override (өгөгдмөл: Google generativelanguage v1beta)
AI_SCOPE_PROMPT=                 # DB-ийн 'scope' давхарга хоосон үеийн хамрах хүрээний fallback
SUPERADMIN_EMAIL=                # сонголттой: boot үед энэ (бүртгэлтэй) хэрэглэгчийг super admin болгоно
EID_BASE_URL=https://eidmongolia.mn/v3  # eID Mongolia RP API (өгөгдмөл)
EID_RP_UUID=                     # оператороос олгосон RP UUID (нэвтрэлтэд заавал)
EID_RP_NAME=template-web         # бүртгэлтэй relyingPartyName
EID_RP_SECRET=rp_sk_...          # RP API secret (Authorization: Bearer); gitignored нууц
EID_CERT_LEVEL=ADVANCED          # нэвтрэлтэд шаардах гэрчилгээний ДООД түвшин (ADVANCED нь бүгдийг хүлээн авна)
EID_DISPLAY_TEXT=template.gerege.mn  # eID апп-ийн баталгаажуулах дэлгэцэнд харагдах текст
GOOGLE_CLIENT_ID=...apps.googleusercontent.com  # Google OAuth client_id (нууц биш)
GOOGLE_CLIENT_SECRET=...          # Google OAuth client secret (server-only, gitignored)
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
`SUPERADMIN_EMAIL`-д бүртгэлтэй хэрэглэгчийн и-мэйлийг заана (дараагийн boot-д
ахиулна) эсвэл DB-д `role_id=1` болгоно.

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

Бүгд `/api/v1` дор (ops endpoint-ууд root дээр):

### Нийтийн (Authentication)
| Method | Path | Тайлбар |
|--------|------|---------|
| POST | `/api/v1/auth/register` | Бүртгэл (email+password) |
| POST | `/api/v1/auth/login` | Token pair авах |
| POST | `/api/v1/auth/send-otp` | OTP илгээх |
| POST | `/api/v1/auth/verify-otp` | OTP баталгаажуулж идэвхжүүлэх |
| POST | `/api/v1/auth/refresh` | Token rotation |
| POST | `/api/v1/auth/logout` | Refresh token хүчингүй болгох |
| POST | `/api/v1/auth/password/forgot` | Нууц үг сэргээх эхлэл |
| POST | `/api/v1/auth/password/reset` | Нууц үг сэргээх төгсгөл |

### Хамгаалагдсан (JWT шаардана)
| Method | Path | Тайлбар |
|--------|------|---------|
| PUT | `/api/v1/auth/password/change` | Нууц үг солих |
| GET | `/api/v1/users/me` | Хэрэглэгчийн профайл |
| POST | `/api/v1/ai/chat` | AI чат (Gemini pipeline, function calling, текст/дуут мессеж) |
| POST | `/api/v1/ai/stt` | Яриа→текст (audio base64 → transcript) |
| POST | `/api/v1/ai/tts` | Текст→яриа (текст → WAV base64) |
| POST | `/api/v1/ai/translate` | Шууд орчуулга (текст/audio → зорилтот хэл, сонголтоор TTS) |
| GET/PUT | `/api/v1/admin/ai/prompts` | AI prompt давхарга — хүрээ/заавар (settings.manage) |
| GET | `/api/v1/superadmin/admins` | Админ түвшний бүртгэлүүдийг жагсаах (зөвхөн super admin) |
| POST | `/api/v1/superadmin/admins` | Шинэ админ үүсгэх (зөвхөн super admin) |
| PUT | `/api/v1/superadmin/admins/{id}/grant` | Байгаа хэрэглэгчид админ эрх олгох (зөвхөн super admin) |
| DELETE | `/api/v1/superadmin/admins/{id}` | Админ эрх хасах (зөвхөн super admin) |

### Ops
`GET /health` (liveness) · `GET /ready` (DB+Redis) · `GET /metrics` · `GET /swagger/*`

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

**Gerege Template Version 27.0** — **Gerege Systems Development Team** болон
**Claude AI** хамтран бүтээв, 2026.
