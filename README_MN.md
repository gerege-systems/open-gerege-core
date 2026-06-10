# Gerege Backend Template v27

> 🌐 [English](README.md) · **Монгол**

[![Go](https://img.shields.io/badge/Go-1.26-blue.svg)](https://golang.org/)
[![Fiber](https://img.shields.io/badge/Fiber-v3-00ACD7.svg)](https://gofiber.io/)
[![GORM](https://img.shields.io/badge/GORM-v2-CB3837.svg)](https://gorm.io/)
[![License](https://img.shields.io/badge/License-MIT-green.svg)](LICENSE)

Clean Architecture зарчмаар бүтээгдсэн, өндөр гүйцэтгэлтэй Go backend template.
**Fiber v3** (HTTP), **GORM + PostgreSQL** (өгөгдөл), **Redis + Ristretto**
(кэш), **JWT + OTP** (танилт) дээр суурилсан.

## 📌 Эх сурвалж ба нээлттэй эх (Open Source)

> Энэ template нь **нээлттэй эх кодын төсөл
> [snykk/go-rest-boilerplate](https://github.com/snykk/go-rest-boilerplate)**
> (зохиогч: Najib Fikri, **MIT лиценз**) дээр **суурилж, түүнээс санаа авч**
> бүтээгдсэн. Clean Architecture бүтэц, JWT/OTP танилт, mailer, audit, кэш,
> observability, тестийн стратеги зэрэг нь тэндээс уламжлагдсан.
>
> Бид дараах хоёр зүйлийг **хөрвүүлсэн**:
> - HTTP давхарга: **Gin → Fiber v3**
> - Өгөгдлийн давхарга: **sqlx → GORM**
>
> Fiber v3-ийн хэрэглээг нээлттэй эх
> [rachmanzz/fiber-starter](https://github.com/rachmanzz/fiber-starter) (MIT)
> -ээс лавласан. Бүх эх төслүүд MIT лицензтэй бөгөөд тэдгээрийн зохиогчийн эрх,
> лицензийн нөхцлийг хүндэтгэн хадгалсан (доорх [Зохиогчид](#-зохиогчид--лиценз)
> хэсгийг үз). Энэ template өөрөө мөн **MIT лицензтэй**.

## Онцлог

- **Clean Architecture** — `handler → usecase → repository → domain`, дотогшоо чиглэсэн хамаарал, back-import байхгүй
- **Fiber v3** — өндөр гүйцэтгэлтэй вэб framework
- **GORM** — PostgreSQL дэмжлэгтэй ORM, soft-delete-тэй (`gorm.DeletedAt`)
- **JWT танилт** — access + refresh token (rotation, `kind` claim guard)
- **OTP бүртгэл** — имэйл OTP-оор баталгаажуулах, brute-force lockout
- **Async Mailer** — OTP имэйлийг хүсэлтийн замаас гадуур, retry-тэй илгээх
- **Audit log** — танилтын үйл явдлын бүртгэл
- **Observability** — OpenTelemetry trace + Prometheus metrics
- **Кэш** — Redis + Ristretto хоёр түвшний
- **Integration Testing** — testcontainers-go (жинхэнэ Postgres + Redis)
- **Swagger** — godoc annotation-аас автомат API баримтжуулалт
- **Structured Logging** — Zap, request ID дамжуулалттай
- **Security** — security headers, CORS, rate limiting, body size limit
- **Graceful Shutdown** — HTTP, mailer queue, DB, Redis, tracer-ийг дарааллаар хаах

## Төслийн бүтэц

```
.
├── cmd/
│   ├── api/main.go              # Аппликейшн эхлэх цэг
│   ├── api/server/server.go     # Composition root (гар DI)
│   ├── migration/               # Migration CLI
│   └── seed/                    # Seed CLI
├── internal/
│   ├── business/
│   │   ├── domain/              # Domain entities (хамгийн дотоод давхарга)
│   │   └── usecases/{auth,users}/  # Business logic (interface + impl)
│   ├── datasources/
│   │   ├── drivers/             # GORM Postgres холболт
│   │   ├── caches/              # Redis + Ristretto
│   │   ├── migration/           # Migration runner
│   │   ├── records/             # GORM model + mapper
│   │   └── repositories/        # interface + postgres impl
│   ├── http/
│   │   ├── handlers/v1/         # HTTP handlers
│   │   ├── middlewares/         # Middleware stack
│   │   ├── routes/              # Route бүртгэл
│   │   ├── datatransfers/       # Request/Response DTO
│   │   └── auth/                # Locals-аас CurrentUser
│   └── config/ apperror/ constants/
├── migrations/                  # SQL migrations
├── docs/                        # Swagger + ARCHITECTURE.md + DEVELOPMENT.md
└── pkg/                         # jwt, logger, clock, helpers, validators,
                                 # mailer, audit, observability
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
OTEL_EXPORTER=                   # хоосон=унтраах | stdout | otlp
ALLOWED_ORIGINS=                 # production-д заавал (таслалаар)
```

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
| [snykk/go-rest-boilerplate](https://github.com/snykk/go-rest-boilerplate) | Najib Fikri | MIT | Үндсэн архитектур, auth/OTP/mailer/audit, кэш, observability, тест |
| [rachmanzz/fiber-starter](https://github.com/rachmanzz/fiber-starter) | rachmanzz | MIT | Fiber v3 идиомын лавлагаа |
| [GoFiber](https://github.com/gofiber/fiber) · [GORM](https://github.com/go-gorm/gorm) | — | MIT | Framework · ORM |

**Бидний өөрчлөлт:** HTTP давхаргыг **Gin → Fiber v3**, өгөгдлийн давхаргыг
**sqlx → GORM** болгосон; бусдыг үнэнчээр хадгалсан. MIT уламжлалын дагуу
эх төслүүдийн зохиогчийн эрхийн мэдэгдлийг хадгалсан бөгөөд энэ template нь
**MIT License**-тэй (`LICENSE` файлыг үз).

---

**Gerege Template Version 27.0** — **Gerege Systems Development Team** болон
**Claude AI** хамтран бүтээв, 2026.
