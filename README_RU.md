# Gerege Template Platform V3.0 — Бэкенд (Go)

> _Одна основа — все государственные и частные услуги._

> 🌐 **Русский** · [English](README.md) · [Монгол](README_MN.md) · [中文](README_ZH.md)

[![Go](https://img.shields.io/badge/Go-1.26-blue.svg)](https://golang.org/)
[![chi](https://img.shields.io/badge/chi-v5-00ADD8.svg)](https://github.com/go-chi/chi)
[![pgx](https://img.shields.io/badge/pgx-v5-336791.svg)](https://github.com/jackc/pgx)
[![License](https://img.shields.io/badge/License-MIT-green.svg)](LICENSE)

Go-бэкенд **Gerege Template Platform V3.0** — готовой к продакшену основы, на
которой можно построить *любую* цифровую услугу государственного или частного
сектора. Он сочетает дисциплинированное ядро **Clean Architecture** с рукописным
**SQL на pgx** (без ORM) и сразу поставляется с полным набором возможностей
государственного уровня: аутентификация **eID Mongolia**, привязка аккаунта
**Google**, подписание документов **PAdES**, конвейер **Gemini AI** и
эшелонированное усиление безопасности — всё на четырёх языках (mn/en/zh/ru) и с
наблюдаемостью с первого дня. Построен на **chi (net/http)** для HTTP,
**pgx (pgxpool) + PostgreSQL** для данных и **Redis + Ristretto** для кэша.

> **Эталонное развёртывание:** **Gerege Template Platform**
> ([template.gerege.mn](https://template.gerege.mn)) — платформа государственных
> услуг и доверяющая сторона Gerege SSO, построенная на этой основе; она
> демонстрирует единый вход через eID и встроенный провайдер OIDC для других приложений.

## 📌 Происхождение и открытый код

> Этот шаблон **основан на открытом проекте
> [snykk/go-rest-boilerplate](https://github.com/snykk/go-rest-boilerplate)**
> (автор: Najib Fikri, **лицензия MIT**) и вдохновлён им. Оттуда унаследованы
> структура Clean Architecture, аутентификация JWT/OTP, аудит, кэш,
> наблюдаемость и стратегия тестирования.
>
> Мы **перенесли** две вещи:
> - Слой HTTP: **Gin → chi (net/http)**
> - Слой данных: **sqlx → pgx (pgxpool, рукописный SQL)**
>
> Исходный проект распространяется по MIT, и его авторские права и условия
> лицензии соблюдены и сохранены (см. раздел [Благодарности](#-благодарности-и-лицензия)
> ниже). Сам шаблон также распространяется по **лицензии MIT**.

## Возможности

- **Clean Architecture** — `handler → usecase → repository → domain`, зависимости
  направлены внутрь, без обратных импортов
- **chi (net/http)** — идиоматичный роутер на стандартной библиотеке
- **pgx (pgxpool)** — рукописный SQL без ORM; явное мягкое удаление через `deleted_at IS NULL`
- **Аутентификация eID** — единственный способ входа: доверяющая сторона eID
  Mongolia (QR / мобильный deep-link / push по регистрационному номеру) с
  long-poll-сессией; выдаёт JWT access + refresh (ротация, защита claim `kind`)
- **Привязка Google OAuth** — привязать аккаунт Google к пользователю eID (обмен
  кода только на сервере) и затем входить через него
- **Провайдер OIDC (SSO)** — необязательный фронтенд над Ory Hydra, чтобы DAN
  выступал провайдером идентификации; потоки вход/согласие/выход плюс поверхность
  `/admin` для регистрации клиентов RP (включается только при настроенной Hydra)
- **Профиль eID PKI** — связанные организации и подписанты вошедшего гражданина,
  сертификаты, устройства и активность
- **Организации и членство** — создание/поиск организаций (запрос в
  государственный реестр через Gerege Verify/XYP) и управление участниками/ролями,
  изолированные по пользователям через RLS
- **Портал государственных услуг** — каталог, заявки, справки, уведомления,
  платежи, запись на приём
- **API-шлюз** — сервисы / маршруты / consumers / API-ключи / политики + телеметрия
  запросов (управляется админом)
- **Подписание документов (PAdES)** — серверное подписание PDF через eID Mongolia
  `/v3` с постоянным сертификатом Document-Signer; опциональный ретранслятор
  подписи для сторонних RP
- **Интеграции и хранилище** — OAuth-интеграции по пользователям (Google
  Drive/Meet, Dropbox) с шифрованием токенов AES-256-GCM; собственное
  SFTP-хранилище приложения Gerege Space
- **AI-конвейер (Gemini)** — REST-клиент без SDK + function calling: текстовый и
  голосовой чат, STT, TTS, синхронный перевод; слоистые промпты (жёстко заданные
  ограничения + настраиваемая из базы область) и инструмент `search_knowledge` на базе БД
- **RBAC и суперадмин** — динамические роли и каталог разрешений; модель из
  4 ролей (суперадмин → админ → менеджер → пользователь)
- **Оформление сайта** — настраиваемый админом общесайтовый вид
  (акцент/шрифт/плотность/тема) и переопределения по пользователям
- **Журнал аудита** — с хеш-цепочкой, только на добавление (чтение и проверка
  целостности только для админа)
- **Наблюдаемость** — трассировка OpenTelemetry + метрики Prometheus;
  `/metrics` и `/swagger` в продакшене закрыты bearer-токеном
- **Кэш** — двухуровневый Redis + Ristretto
- **Интеграционное тестирование** — testcontainers-go (настоящие Postgres + Redis)
- **Swagger** — автоматическая документация API из аннотаций godoc
- **Структурированное логирование** — Zap с пробросом request ID
- **Безопасность** — заголовки безопасности, CORS, ограничение запросов и размера
  тела, полные таймауты сервера, RLS в Postgres + проверка применимости при
  старте, список запрещённых access-токенов при выходе
- **Корректное завершение** — по очереди закрывает HTTP, пул БД, Redis, tracer

## Структура проекта

```
.
├── cmd/
│   ├── api/main.go              # Точка входа приложения
│   ├── api/server/server.go     # Корень композиции (ручной DI)
│   ├── migration/               # CLI миграций
│   ├── seed/                    # CLI наполнения базы
│   └── healthcheck/             # Проба здоровья для distroless
├── internal/
│   ├── business/
│   │   ├── domain/              # Доменные сущности (внутренний слой)
│   │   └── usecases/           # Бизнес-логика (интерфейс + реализация), по пакету на модуль:
│   │       #  auth · users · rbac · superadmin · ai · audit · security · site
│   │       #  org · gov · gateway · core · sso · provider · sign · assets
│   │       #  integrations · gspace
│   ├── datasources/
│   │   ├── drivers/             # Подключение к Postgres через pgx (pgxpool) (driver_pgx.go)
│   │   ├── caches/              # Redis + Ristretto
│   │   ├── migration/           # Исполнитель миграций
│   │   ├── records/             # Структуры записей pgx + маппинг запись↔домен
│   │   └── repositories/        # интерфейсы + реализация на postgres
│   ├── http/
│   │   ├── handlers/v1/         # HTTP-обработчики
│   │   ├── middlewares/         # Стек middleware
│   │   ├── routes/              # Регистрация маршрутов
│   │   ├── datatransfers/       # DTO запросов/ответов
│   │   └── auth/                # CurrentUser из контекста
│   └── config/ apperror/ constants/
├── migrations/                  # SQL-миграции
├── docs/                        # Swagger + ARCHITECTURE.md + DEVELOPMENT.md
└── pkg/                         # jwt, logger, clock, helpers, validators,
                                 # audit, observability, gemini,
                                 # eid, google, oidc, hydra, xyp, gspace, verify
```

## Быстрый старт

### Требования

- Go 1.26+
- PostgreSQL 15+
- Redis 7+
- Docker (для интеграционных тестов / локального стека)
- Make

### Установка

```bash
# 1. Скопируйте файл окружения (он лежит в internal/config/)
cp internal/config/.env.example internal/config/.env
# Отредактируйте .env — JWT_SECRET должен быть не короче 32 символов

# 2. Поднимите стек (Postgres + Redis + API)

# 3. Или запустите локально: миграции → сервер
```

Сервер: `http://localhost:8080`, Swagger UI: `http://localhost:8080/swagger/`.

### Команды make

```bash
make build              # Собрать бинарник
make test               # Юнит-тесты (моки — быстро, без Docker)
make test-integration   # Интеграционные тесты (нужен Docker)
make swag               # Сгенерировать документацию Swagger
make lint               # golangci-lint
make pre-push           # Проверки CI локально (lint+тесты+swag+сборка)
```

## Конфигурация

Ключевые переменные из `internal/config/.env.example`:

```env
# Основное
PORT=8080
ENVIRONMENT=development          # development | production
JWT_SECRET=...                   # >= 32 символов (HS256)
JWT_EXPIRED=5                    # TTL access-токена (часы, 1..24)
JWT_REFRESH_EXPIRED=7            # TTL refresh-токена (дни)
DB_POSTGRE_DSN=...               # DSN в разработке
DB_POSTGRE_URL=...               # URL в продакшене (обязательно sslmode=verify-full/verify-ca)
REDIS_HOST=localhost:6379
BCRYPT_COST=12                   # 10..31
ALLOWED_ORIGINS=                 # обязателен в продакшене (через запятую)
TRUSTED_PROXIES=                 # IP/CIDR обратных прокси, которым доверяем X-Forwarded-For
OBSERVABILITY_TOKEN=             # bearer-токен, закрывающий /metrics + /swagger в продакшене

# eID Mongolia (доверяющая сторона) — основной вход; разумные значения по умолчанию, старт не ломается
EID_BASE_URL=https://eidmongolia.mn/v3
EID_RP_UUID=                     # UUID RP, зарегистрированный у IdP
EID_RP_NAME=                     # отображаемое имя RP
EID_RP_SECRET=                   # секрет API RP (используется и для ретранслятора /rp/sign)
EID_CERT_LEVEL=ADVANCED          # ADVANCED | QUALIFIED | QSCD
EID_CALLBACK_URL=                # должен быть в белом списке у IdP
EID_DISPLAY_TEXT=

# Google OAuth — привязка аккаунта Google к пользователю eID
GOOGLE_CLIENT_ID=
GOOGLE_CLIENT_SECRET=

# Сторона провайдера OIDC (платформа сама является issuer) — потоки неактивны, пока не задано
OAUTH_ISSUER=                    # issuer, например https://template.gerege.mn (пусто = провайдер выключен)
SSO_STATE_KEY=                   # >= 32 байт; HMAC для cookie состояния входа/согласия
SSO_FIRSTPARTY_CLIENTS=          # client_id через запятую, которые пропускают экран согласия
SSO_ADMIN_API_KEYS=              # стартовые ключи для поверхности /admin, через запятую

# Подписание документов (PAdES) — постоянный материал Document-Signer (обязателен в продакшене)
SIGN_SIGNER_CERT_FILE=
SIGN_SIGNER_KEY_FILE=
SIGN_RELAY_TOKEN=                # общий токен, чтобы сторонние RP подписывали через учётные данные eID DAN

# Государственные сервисы Gerege
XYP_API_BASE=https://xyp.dgov.mn # поиск организаций (государственный реестр); Basic auth
XYP_CLIENT_ID=
XYP_CLIENT_SECRET=
CORE_API_BASE=https://core.gerege.mn  # поиск пользователей/организаций в Gerege Core
CORE_API_TOKEN=

# Gerege Space — собственное SFTP-хранилище приложения (пусто = функция выключена)
GSPACE_HOST=
GSPACE_PORT=22
GSPACE_USER=
GSPACE_PASSWORD=
GSPACE_BASE_PATH=gerege-space
GSPACE_QUOTA_BYTES=2097152       # квота на пользователя (по умолчанию 2 МБ)

# Шифрование интеграционных токенов (AES-256-GCM) — обязателен в продакшене
INTEGRATION_ENC_KEY=

# GeregeCloud Verify (verify.gecloud.mn) — транспорт OTP; обязателен в продакшене
VERIFY_API_KEY=
VERIFY_API_BASE=https://verify.gecloud.mn/v1
VERIFY_CHANNEL=email

# AI-конвейер (/api/v1/ai/*)
GEMINI_API_KEY=                  # пусто = AI выключен (endpoints возвращают 500)
GEMINI_MODEL=gemini-2.5-flash    # необязательное переопределение (чат / STT / перевод)
GEMINI_TTS_MODEL=gemini-2.5-flash-preview-tts  # необязательное переопределение (TTS)
GEMINI_VOICE=Kore                # необязательный предустановленный голос TTS
GEMINI_EMBED_MODEL=text-embedding-004  # модель векторов базы знаний (768 измерений — как ai_knowledge.embedding)
GEMINI_API_BASE=                 # необязательное переопределение (по умолчанию Google generativelanguage v1beta)
AI_SCOPE_PROMPT=                 # запасное значение области AI, когда слой 'scope' в базе пуст

# Наблюдаемость и начальная настройка
OTEL_EXPORTER=                   # пусто=выкл | stdout | otlp
SUPERADMIN_EMAIL=                # необязательно: повысить этого (уже вошедшего) пользователя до суперадмина при старте
```

### Роли и суперадмин

Роли упорядочены по уровню прав (id 1 — высший): **суперадмин=1, админ=2,
менеджер=3, пользователь=4** (создаются/перенумеровываются миграцией
`23_superadmin_role`). **Суперадмин** стоит выше админа и является единственной
ролью, которая может управлять учётными записями администраторов (создание /
выдача / отзыв) через `/api/v1/superadmin/*` (`RequireSuperAdmin`); обычные админы
туда не попадают. API никогда не создаёт суперадмина — задайте
`SUPERADMIN_EMAIL` существующему пользователю, который уже входил через eID
(повышение произойдёт при следующем старте), либо обновите `role_id=1` в базе.

> **Ломающее изменение (для существующих развёртываний):** миграция `23`
> перенумеровывает роли, поэтому JWT, выданные до неё, интерпретируются иначе
> (старый `admin=1` → суперадмин, `user=2` → админ). При применении к
> существующей базе **смените `JWT_SECRET`** (или принудительно разлогиньте всех),
> чтобы устаревшие токены не получили лишних прав. Новые установки не затронуты.

### Слои промпта AI

AI-ассистент работает на слоистом системном промпте: **базовые ограничения**
(жёстко заданы — только монгольский, соблюдение области, устойчивость к prompt
injection) + **область применения** (в чём помогает ассистент) + **инструкции**
(необязательные тон/правила). Область и инструкции хранятся в таблице
`ai_prompts` и редактируются в рантайме через
`GET/PUT /api/v1/admin/ai/prompts` (требуется `settings.manage`; интерфейс в
Админ → Настройки). Ассистент отклоняет всё вне заданной области и отвечает на
вопросы о платформе, ища данные в таблице `ai_knowledge` через инструмент
`search_knowledge`.

## Endpoint API

Все находятся под `/api/v1` (операционные — в корне). **Нет ни одного endpoint
пароля / email-OTP / регистрации / восстановления** — аутентификация только
eID + Google.

### Публичные (аутентификация)

| Метод | Путь | Описание |
|--------|------|---------|
| POST | `/api/v1/auth/eid/start` | Начать вход через eID (QR / мобильный deep-link) |
| POST | `/api/v1/auth/eid/start-id` | Начать вход по регистрационному номеру (push на зарегистрированное устройство) |
| POST | `/api/v1/auth/eid/poll` | Длинный опрос сессии eID до завершения |
| POST | `/api/v1/auth/google` | Колбэк Google OAuth — обмен кода + привязка/вход eID |
| POST | `/api/v1/auth/refresh` | Ротация токенов |
| POST | `/api/v1/auth/logout` | Отзыв refresh + добавление access в список запрещённых |

### Защищённые (нужен JWT)

| Метод | Путь | Описание |
|--------|------|---------|
| GET | `/api/v1/users/me` | Профиль пользователя |
| GET | `/api/v1/rbac/me` | Действующие роли/разрешения текущего пользователя |
| DELETE | `/api/v1/auth/google/link` | Отвязать подключённый аккаунт Google |
| GET | `/api/v1/me/*`, `/api/v1/users/me/eid/*` | Профиль eID PKI — организации, подписанты, сертификаты, устройства, активность |
| CRUD | `/api/v1/org/*` | Организации и членство (поиск в госреестре, участники, роли) |
| GET/POST | `/api/v1/gov/*` | Портал госуслуг — услуги, заявки, справки, уведомления, платежи, запись |
| CRUD | `/api/v1/gateway/*` | API-шлюз — сервисы, маршруты, consumers, ключи, политики, логи |
| GET | `/api/v1/core/users` · `/organizations` | Поиск в Gerege Core (пользователи/организации) |
| CRUD | `/api/v1/integrations/*` | OAuth-интеграции по пользователям (зашифрованные токены) |
| GET | `/api/v1/assets/*` | Изображение подписи и печать организации |
| GET | `/api/v1/gspace/*` | Хранилище Gerege Space по SFTP (список + скачивание) |
| POST/GET | `/api/v1/sign/*` | Подписание документов (PAdES) — старт, статус, скачивание |
| POST | `/api/v1/ai/chat` | Чат с AI (конвейер Gemini, function calling, текст/голос) |
| POST | `/api/v1/ai/stt` | Речь в текст (аудио base64 → расшифровка) |
| POST | `/api/v1/ai/tts` | Текст в речь (текст → WAV в base64) |
| POST | `/api/v1/ai/translate` | Синхронный перевод (текст/аудио → целевой язык, опционально TTS) |
| GET | `/api/v1/site/appearance` | Общесайтовое оформление по умолчанию (публичное чтение) |
| GET/PUT | `/api/v1/admin/ai/prompts` | Слои промпта AI — scope/instructions (settings.manage) |
| GET | `/api/v1/audit` · `/audit/verify` | Чтение журнала аудита и проверка его хеш-цепочки (админ) |
| POST | `/api/v1/security/events` | Приём клиентского события безопасности |
| GET | `/api/v1/superadmin/admins` | Список учётных записей уровня админа (только суперадмин) |
| POST | `/api/v1/superadmin/admins` | Создать учётную запись админа (только суперадмин) |
| PUT | `/api/v1/superadmin/admins/{id}/grant` | Выдать права админа существующему пользователю (только суперадмин) |
| DELETE | `/api/v1/superadmin/admins/{id}` | Отозвать права админа (только суперадмин) |

### Провайдер OIDC (только при настроенной Hydra)

`GET /api/v1/provider/login` · `/consent`, а также accept/reject для
входа/согласия/выхода (экраны, управляемые Hydra). Регистрация OAuth2-клиентов RP
находится на смонтированной поверхности `/admin`.

### Эксплуатация

`GET /health` (liveness) · `GET /ready` (БД+Redis) · `GET /metrics` · `GET /swagger/doc.json`
— в продакшене `/metrics` и `/swagger` требуют bearer `OBSERVABILITY_TOKEN` (иначе 404).

### Формат ответа

```json
{ "status": true, "message": "login success", "data": { }, "request_id": "…" }
```

При ошибке `status:false`. Ошибка валидации → `422`, каждое поле — в `data.errors`.

## Разработка

Подробности:

- **[docs/ARCHITECTURE.md](docs/ARCHITECTURE.md)** — структура слоёв, поток зависимостей, безопасность
- **[docs/DEVELOPMENT.md](docs/DEVELOPMENT.md)** — 8 шагов добавления функциональности, тесты, стиль кода, диагностика
- **[docs/AI_PIPELINE.md](docs/AI_PIPELINE.md)** — внутренности AI-ассистента: потоки, слои промпта, инструменты, голос, расширение

```bash
make test               # Юнит-тесты
make test-integration   # Интеграционные тесты (Docker)
make test-cover         # Покрытие
```

## Docker

```bash
make build              # Бинарник
curl http://localhost:8080/health
```

## 🙏 Благодарности и лицензия

Этот шаблон стоит на плечах открытых проектов:

| Проект | Автор | Лицензия | Что использовано |
|-------|---------|--------|--------------|
| [snykk/go-rest-boilerplate](https://github.com/snykk/go-rest-boilerplate) | Najib Fikri | MIT | Базовая архитектура, auth/OTP/аудит, кэш, наблюдаемость, тесты |
| [chi](https://github.com/go-chi/chi) · [pgx](https://github.com/jackc/pgx) | — | MIT | Роутер · драйвер Postgres |

**Наши изменения:** слой HTTP перенесён с **Gin на chi (net/http)**, слой данных —
с **sqlx на pgx (pgxpool, рукописный SQL)**; всё остальное сохранено без искажений.
В духе традиции MIT уведомления об авторских правах исходных проектов сохранены,
а сам шаблон распространяется по **лицензии MIT** (см. файл `LICENSE`).

---

**Gerege Template Platform V3.0** — совместная разработка **команды Gerege
Systems** и **Claude AI**, 2026.
