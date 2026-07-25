# Обзор архитектуры

> 🌐 **Русский** · [English](ARCHITECTURE.md) · [Монгол](ARCHITECTURE_MN.md) · [中文](ARCHITECTURE_ZH.md)

Этот документ описывает высокоуровневую архитектуру **Gerege Template Platform
V3.0** (Цахим үйлчилгээг бүтээх суурь) — готовой к продакшену основы, на которой
можно построить любую цифровую услугу государственного или частного сектора.
Её флагманское эталонное развёртывание — **Gerege Template Platform**
(на **template.gerege.mn**), **платформа государственных услуг на базе eID** и
доверяющая сторона Gerege SSO. Модуль бэкенда называется `template`; стек —
**chi (net/http) + pgx (pgxpool) + PostgreSQL + Redis + Gemini AI**, организованный
по принципам Clean Architecture, с Next.js BFF впереди.

В этом эталонном развёртывании платформа выступает одновременно **доверяющей
стороной eID** (пользователи входят через eID) и **провайдером OIDC** (другие
приложения входят *через неё* с помощью встроенного Go-провайдера). Row-Level
Security в PostgreSQL — несущая граница изоляции по пользователям, см.
[Row-Level Security](#row-level-security-rls).

> **Происхождение.** Слои Clean Architecture, слой данных на pgx, кэширование,
> наблюдаемость и стратегия тестирования происходят из открытого проекта
> [snykk/go-rest-boilerplate](https://github.com/snykk/go-rest-boilerplate)
> Najib Fikri (MIT). Стек аутентификации, модель безопасности RLS, интеграции
> eID/SSO/OIDC-провайдера и функциональные модули ниже созданы для этой платформы.
> Как MIT-производная работа она сохраняет исходное авторское право — см.
> [Благодарности](#благодарности-и-лицензия).

## Схема слоёв

```
┌─────────────────────────────────────────────────────────────────┐
│                        Слой HTTP                                  │
│  cmd/api/server → Middleware → internal/http/handlers/v1          │
│  internal/http/{routes, datatransfers, middlewares, auth}         │
│  + internal/provider/{adminapi, adminkeys, devapps, signrelay}    │
├─────────────────────────────────────────────────────────────────┤
│                       Слой Usecase                                │
│  internal/business/usecases/*  (19 ограниченных контекстов)       │
│  (бизнес-логика, валидация, оркестрация)                          │
├─────────────────────────────────────────────────────────────────┤
│                     Слой Repository                               │
│  internal/datasources/repositories/{interface, postgres}          │
│  (рукописный SQL на pgx, транзакции RLS, мягкое удаление, кэш)    │
├─────────────────────────────────────────────────────────────────┤
│                       Слой Domain                                 │
│  internal/business/domain                                         │
│  (сущности, объекты-значения, бизнес-правила)                     │
└─────────────────────────────────────────────────────────────────┘
```

## Функциональные модули (ограниченные контексты)

Платформа состоит из **19 usecase-модулей** в `internal/business/usecases/`,
каждый из которых — интерфейс и реализация, связанные вручную в корне композиции.
Помимо базового ядра (`auth`, `users`, `rbac`, `ai`) платформа добавляет
поверхность eID/SSO/оказания услуг:

| Модуль | Ответственность |
|----------------|----------------|
| `auth`         | **Вход через eID** (QR / мобильный deep-link / push по регистрационному номеру + long-poll), привязка аккаунта **Google OAuth**, обновление и завершение сессии. Паролей нет. |
| `users`        | Чтение/запись пользователей, переиспользуемые auth, admin, sign, superadmin; блокировка после неудачных входов; отсечка токенов при смене пароля. |
| `rbac`         | Динамический каталог ролей и разрешений и резолвер разрешений для RBAC-middleware. |
| `ai`           | Конвейер Gemini — чат с function calling, STT/TTS, синхронный перевод, слоистые промпты, серверные инструменты и база знаний. |
| `org`          | Организации и членство (связаны с eID; **RLS**). |
| `gov`          | Гражданский портал «Государственные услуги» — заявки, справки, уведомления, платежи, запись на приём (по пользователям, **RLS**) поверх публичного каталога услуг. |
| `gateway`      | API-шлюз — сервисы / маршруты / политики + телеметрия (каждый сервис несёт OAuth-`scope`). |
| `applications` | Единый **реестр OAuth2-клиентов** (RP + m2m) на базе **Ory Hydra** — объединяет прежних consumers/API-ключи шлюза и регистрацию RP в SSO; доступ к сервисам = OAuth-scope (`application_services` → `gateway_services.scope`). Управляется админом (`gateway.manage`), зависит от Hydra. |
| `core`         | Обёртка над Gerege Core (`core.gerege.mn`) для USER FIND / ORG FIND. |
| `provider`     | **Провайдер OIDC** — ядро вход/согласие/выход перед **Ory Hydra**; dan сам является SSO-провайдером. |
| `integrations` | Пользовательские OAuth-интеграции (Google Drive/Meet, Dropbox); токены хранятся **зашифрованными AES-256-GCM** (**RLS**). |
| `assets`       | Личное изображение подписи и печать организации (изображения в Google Drive, URL в базе). |
| `gspace`       | Gerege Space — собственное SFTP-хранилище приложения, квота на пользователя (по умолчанию 2 МБ). |
| `audit`        | Сохраняемый **журнал аудита с хеш-цепочкой, только на добавление** (API чтения для админа). |
| `superadmin`   | Управление администраторами (создание / выдача / отзыв); каждое изменение пишется в журнал аудита. |
| `security`     | Приём событий безопасности (пишут аутентифицированные пользователи, читает админ). |
| `site`         | Общесайтовые значения оформления (акцент / шрифт / плотность / тема). |
| `sign`         | Подписание PDF (**PAdES**) через eidmongolia `/v3` с серверным сертификатом Document-Signer. |

## Структура каталогов

```
.
├── cmd/
│   ├── api/
│   │   ├── main.go                 # Точка входа (инициализация конфига и логгера)
│   │   └── server/server.go        # Корень композиции (ручной DI) — здесь видны все монтирования
│   ├── migration/                  # CLI миграций (только SQL; НЕТ ORM/AutoMigrate)
│   └── seed/                       # CLI наполнения базы
├── docs/                           # Документация EN/MN + спецификация OpenAPI (swagger.json/yaml, docs.go)
├── internal/
│   ├── apperror/                   # Типизированные доменные ошибки (→ HTTP-статус)
│   ├── business/
│   │   ├── domain/                 # Сущности предприятия (внутренний круг)
│   │   └── usecases/               # 19 ограниченных контекстов (интерфейс + реализация)
│   ├── config/                     # Конфиг на Viper + .env.example
│   ├── constants/                  # Константы окружения, логгера, ошибок, endpoint
│   ├── datasources/
│   │   ├── caches/                 # Двухуровневый кэш Redis + Ristretto
│   │   ├── drivers/                # Подключение pgx (pgxpool) + проверка применимости RLS при старте
│   │   ├── migration/              # Исполнитель SQL-миграций
│   │   ├── records/                # Структуры записей pgx + маппинг запись↔домен
│   │   ├── rls/                    # Личность RLS на запрос, передаваемая через context.Context
│   │   └── repositories/
│   │       ├── interface/          # Абстракции-шлюзы (пакет _interface)
│   │       └── postgres/*          # Реализации на pgx (рукописный SQL, withRLS)
│   ├── http/
│   │   ├── auth/                   # CurrentUser из контекста запроса
│   │   ├── datatransfers/          # DTO запросов и ответов
│   │   ├── handlers/v1/            # HTTP-обработчики (по модулям)
│   │   ├── middlewares/            # Глобальные и групповые middleware
│   │   └── routes/                 # Регистрация маршрутов (по модулям)
│   └── provider/                   # Операторские поверхности OIDC-провайдера:
│       ├── adminapi/               #   управление OAuth2-клиентами RP в /admin
│       ├── adminkeys/ devapps/     #   админские API-ключи + хранилище приложений разработчиков
│       └── signrelay/              #   ретранслятор /rp/sign для нижестоящих RP
├── migrations/                     # Нумерованные SQL-миграции (N_name.up.sql + .down.sql)
├── pkg/                            # Клиенты и утилиты вне фреймворка (15 пакетов)
│   ├── eid/ google/ hydra/         # Идентификация: eID RP, Google OAuth, админ Hydra
│   ├── xyp/ gspace/ verify/        # Реестр организаций XYP, SFTP-хранилище, GeregeCloud Verify OTP
│   ├── gemini/                     # Gemini REST без SDK (function calling, аудио, PCM→WAV)
│   ├── jwt/ logger/ clock/         # JWT, логирование Zap, абстракция времени
│   ├── helpers/ validators/        # Утилиты + валидация payload по тегам структур
│   ├── audit/                      # Помощники аудита событий аутентификации
│   └── observability/              # Настройка трассировки OTel + метрик Prometheus
└── internal/test/                  # Моки, фикстуры, обвязка testcontainers
```

## Поток зависимостей

Зависимости направлены только внутрь (принцип Clean Architecture):

```
HTTP → Usecase → Repository → Domain
  │        │          │
  ▼        ▼          ▼
 DTO   Интерфейс   pgx/SQL
```

- **Слой HTTP** зависит от интерфейсов **Usecase** (`auth.Usecase`, `users.Usecase`, …).
- **Слой Usecase** зависит от интерфейсов **Repository**
  (`_interface.UserRepository`, …), но никогда от адаптеров postgres.
- **Слой Repository** зависит от сущностей **Domain**.
- **Слой Domain** импортирует только стандартную библиотеку и
  `golang.org/x/crypto/bcrypt` — никогда `internal/` или `pkg/`.

Это проверяется структурно: `internal/business/**` и
`internal/datasources/repositories/**` **не** импортируют ни один веб-пакет
chi/net-http, поэтому фреймворк доставки можно заменить, не трогая бизнес-код.
Одно исключение из правила «домен не импортирует ничего внутреннего» сделано
намеренно: листовой пакет `internal/datasources/rls` зависит только от `context`
из стандартной библиотеки и используется всеми тремя слоями, чтобы переносить
личность RLS запроса без цикла импортов.

## Ключевые компоненты

### 1. Слой HTTP

**Корень композиции:** `cmd/api/server/server.go` — единственная точка ручного DI.
Прочитайте его целиком, чтобы увидеть все монтирования. Он:

- Инициализирует трассировку, пул pgx (с проверкой RLS при старте),
  Redis/Ristretto, сервис JWT и все внешние клиенты (eID, Google, XYP, OIDC/Hydra,
  Gemini, GeregeCloud Verify, Gerege Space, Gerege Core).
- Вручную связывает репозитории → usecase → маршруты (без глобальных синглтонов и DI-контейнера).
- Собирает роутер chi, ставит глобальный стек middleware и монтирует каждый модуль маршрутов под `/api/v1`.
- Условно монтирует поверхности провайдера OIDC (`/admin`, `/rp/sign`) — только при наличии их конфигурации.
- Отвечает за корректное завершение (HTTP, лимитеры, пул pgx, Redis, tracer).

**Маршруты:** `internal/http/routes/` — по файлу на модуль (`route_auth.go`,
`route_gov.go`, `route_provider.go`, …). Каждый монтирует `/v1/<module>` под `/api`.

**Обработчики:** `internal/http/handlers/v1/` — по пакету на модуль. Сигнатура
обработчика — `func(w http.ResponseWriter, r *http.Request) error`, обёрнутая
`v1.Wrap`; тело декодируется `v1.DecodeBody`, DTO валидируются
`validators.ValidatePayloads`, ответы — через `v1.NewSuccessResponse` /
`v1.RespondWithError`. Обработчики несут swagger-аннотации.

### 2. Стек middleware

Глобальные middleware применяются в `server.go` в таком порядке (порядок важен:
трассировка первой, чтобы span/`trace_id` существовал до логирования request-ID;
Recoverer сразу после Request ID, чтобы паники ниже перехватывались и ответ
восстановления нёс `request_id`):

1. **Tracing** (`TracingMiddleware`) — OTel-span на запрос.
2. **Request ID** (`RequestIDMiddleware`) — генерирует/пробрасывает `X-Request-ID` в контекст и логгер.
3. **Recoverer** (`RecovererMiddleware`) — ловит паники ниже, отдаёт аккуратную 500.
4. **Metrics** (`MetricsMiddleware`) — счётчики запросов Prometheus и задержки.
5. **Заголовки безопасности** (`SecurityHeadersMiddleware`) — HSTS, CSP, nosniff, frame options, referrer policy.
6. **CORS** (`CORSMiddleware`) — origin из `ALLOWED_ORIGINS` (wildcard только в разработке).
7. **Ограничение размера тела** (`BodySizeLimitMiddleware`) — общий потолок (на маршрутах строже).
8. **Access Log** (`AccessLogMiddleware`) — структурированный однострочный лог доступа.
9. **Timeout** (`TimeoutMiddleware`) — дедлайн на запрос: 30 с по умолчанию и 50 с на `/api/v1/ai/*` (TTS/STT в Gemini занимают 10–20 с). Серверный `WriteTimeout` выводится из наибольшего из них, чтобы middleware сработал первым.

**Middleware групп и маршрутов:**

- **Auth** (`NewAuthMiddleware`) — проверяет bearer-токен JWT, кладёт `CurrentUser`
  в контекст и **задаёт личность RLS**: `rls.WithAdmin` для админов, иначе
  `rls.WithUser` (`middleware_auth.go`).
- **Сервисный контекст RLS** (`ServiceRLSContext`) — ставится на анонимную группу
  `/auth`, чтобы дозапросные потоки (upsert eID, поиск личности при refresh)
  выполнялись под доверенной ролью `service` (`middleware_rls.go`).
- **RBAC** (`RequirePermission`, `RequireAdmin`, `RequireSuperAdmin`) —
  декларативная авторизация после аутентификации; админы обходят проверки
  разрешений, `RequireSuperAdmin` закрывает поверхность `/superadmin`.
  Fail-closed при ошибке резолвера.
- **Гейт наблюдаемости** (`ObservabilityGate`) — защищает `/metrics` и
  `/swagger/doc.json` (см. [Операционные endpoint](#операционные-endpoint)).
- **Лимитеры** — четыре отдельных: `/auth` ~5/мин, `/ai` ~20/мин (burst 10, для
  потоков перевода), `/eid/poll` ~60/мин (burst 30, для long-poll) и **записи** в
  gov/assets/gspace/профиль eID ~30/мин (burst 15).

`clientIP()` (`middleware_clientip.go`) — вспомогательная функция, а не глобальный
middleware; она определяет IP клиента для лимитов и аудита, доверяя
`X-Forwarded-For` только от `TRUSTED_PROXIES` (безопасное поведение: по умолчанию доверия нет).

### 3. Слой Usecase

**Расположение:** `internal/business/usecases/` — каждый ограниченный контекст
даёт интерфейс и реализацию. Обязанности: проверка бизнес-правил, оркестрация
репозитория, кэша и внешних клиентов, возврат значений `apperror.*` (внутренние
причины оборачиваются `apperror.InternalCause`, чтобы ошибки библиотек не доходили
до клиента). Usecase зависят только от `repositories/interface`, никогда от
адаптеров postgres.

### 4. Слой Repository

**Расположение:** `internal/datasources/repositories/` — пакет `interface/`
(с именем `_interface`, поскольку `interface` — ключевое слово) содержит
абстракции-шлюзы; `postgres/*` реализует их на pgx и рукописном SQL.
Ключевые особенности:

- Запросы принимают `ctx` напрямую; строки сканируются `pgx.RowToStructByName`.
- Мягкое удаление через явные предикаты `deleted_at IS NULL`.
- `Store` использует `INSERT … RETURNING` за один round-trip.
- Дубликаты ключей определяются по коду pgconn `23505` → `apperror.Conflict`.
- Пользовательские репозитории выполняют каждый запрос внутри **транзакции
  `withRLS`**, которая публикует личность запроса в GUC со сроком `SET LOCAL`
  (см. [Row-Level Security](#row-level-security-rls)).

### 5. Слой Domain

**Расположение:** `internal/business/domain/` — сущности несут бизнес-правила и не
зависят ни от чего внутреннего. `domain_users.go` определяет модель ролей и
конструктор пользователя eID (`NewEIDUser` — без пароля, `Active=true`, ключ по
`civil_id`). Константы ролей см. в разделе [Авторизация](#авторизация).

## Аутентификация

Платформа выдаёт **JWT access + refresh** (`pkg/jwt`), но **не имеет входа по
паролю, регистрации по email/OTP и сброса пароля**. Личность приходит только от
внешних провайдеров. Формы endpoint описаны в
[API_CONTRACT.md](API_CONTRACT.md); маршруты регистрируются в
`internal/http/routes/route_auth.go` и `route_eidprofile.go`.

**1. Вход через eID (основной способ).** Приложение — доверяющая сторона eID
Mongolia (`pkg/eid`, конфигурация `EID_*`):

- `POST /api/v1/auth/eid/start` начинает сессию и возвращает QR-код / мобильный deep-link.
- `POST /api/v1/auth/eid/start-id` начинает по регистрационному номеру (реестр),
  отправляя push на зарегистрированное устройство гражданина.
- `POST /api/v1/auth/eid/poll` опрашивается фронтендом **длинным опросом**
  (примерно каждые 2,5 с; IdP удерживает соединение до 25 с), пока сессия eID не
  дойдёт до `COMPLETE`. По завершении выполняется upsert пользователя (ключ
  `civil_id`; публичные RP получают `civil_id`, а не `national_id`) и выдаётся пара токенов.

**2. Привязка аккаунта Google OAuth** (`pkg/google`, `GOOGLE_*`):
`POST /api/v1/auth/google` обменивает код и привязывает (или выполняет вход через)
аккаунт Google, привязанный к пользователю eID; `DELETE /api/v1/auth/google/link` отвязывает.

**Жизненный цикл сессии** (не зависит от способа входа):

- `POST /api/v1/auth/refresh` ротирует пару токенов; токены, выданные до отсечки
  по смене учётных данных, отклоняются (`User.TokensRevokedBefore`). Защита по
  claim `kind` не даёт использовать refresh-токен как access-токен.
- `POST /api/v1/auth/logout` отзывает refresh-токен.

> **Замечание.** Файлы обработчиков `auth_login.go`, `auth_register.go`,
> `auth_send_otp.go`, `auth_forgot_password.go`, `auth_reset_password.go`
> по-прежнему есть в дереве, но **не подключены ни к одному маршруту** —
> `route_auth.go` регистрирует только endpoint eID / Google / refresh / logout выше.

## Авторизация

Авторизация обеспечивается на двух уровнях: **роль/разрешение из JWT** на границе
HTTP и **RLS** в базе данных.

**Модель ролей** (`domain_users.go`; миграция `23_superadmin_role`) — четыре
ранжированные роли, `1` — высшая:

```go
RoleSuperAdmin = 1  // управляет администраторами; закрыт RequireSuperAdmin
RoleAdmin      = 2  // полный доступ; IsAdmin() истинно
RoleManager    = 3
RoleUser        = 4  // по умолчанию для новых пользователей eID
```

`IsAdmin()` истинно для `RoleAdmin` **и** `RoleSuperAdmin` (суперадмин наследует
пути JWT/RLS/разрешений админа); `IsSuperAdmin()` истинно только для
`RoleSuperAdmin`. Идентификатор роли `0` — маркер устаревших токенов без claim,
RBAC-middleware понижает его до `RoleUser`.

**Динамический RBAC** — помимо грубого ранга роли, `rbac.Usecase` вычисляет набор
разрешений роли из базы (миграция `8_rbac_roles_permissions`).
`RequirePermission(resolver, perm)` закрывает маршрут именованным разрешением;
админы проходят. Суперадмин создаётся через `SUPERADMIN_EMAIL` (или в базе),
никогда через API.

## Row-Level Security (RLS)

RLS — несущая граница изоляции по пользователям: эшелонированная защита под
условиями `WHERE user_id = …`, которые уже пишут репозитории. Она гарантирует,
что даже ошибка в запросе не вернёт чужие строки.

**Личность в контексте** (`internal/datasources/rls/rls.go`) — листовой пакет
(только `context` из стандартной библиотеки) переносит `Identity{ UserID, Role }`,
где `Role` — одна из трёх строковых констант, которые **обязаны** совпадать с
литералами в SQL-политиках:

- `service` — доверенные дозапросные/системные потоки (upsert eID, поиск личности
  при refresh, начальная настройка). Задаётся `ServiceRLSContext` на `/auth`; полный доступ.
- `admin` — полный доступ ко всем строкам. Задаётся middleware аутентификации через `rls.WithAdmin` для админских JWT.
- `user` — только собственные строки вызывающего. Задаётся через `rls.WithUser`.

**Публикация личности** (`…/postgres/users/users_postgres.go` и аналоги в `org`,
`gov`, `security`, `userintegrations`) — помощник `withRLS(ctx, fn)` оборачивает
каждый запрос в транзакцию и выполняет:

```go
SELECT set_config('app.user_id',   $1, true),   -- is_local = true ⇒ семантика SET LOCAL
       set_config('app.user_role',  $2, true)
```

`set_config(..., true)` ограничивает значения транзакцией, поэтому личность не
утекает между соединениями пула. Когда контекст **не несёт** личности, оба GUC
пусты — пустой `app.user_role` не совпадает ни с одной политикой, поэтому все
строки скрыты, а записи отклоняются (**fail-closed**). Репозиторий `audit`
использует вариант только с ролью.

**Политики по таблицам** — каждая таблица с RLS использует `ENABLE` **и**
`FORCE ROW LEVEL SECURITY` (FORCE применяет RLS даже к владельцу таблицы).
Политики разрешительные (объединяются по ИЛИ) и распознают те же три роли из GUC.
Политика `user` опирается на
`user_id = NULLIF(current_setting('app.user_id', true), '')::uuid` (`NULLIF`
превращает пустой GUC в `NULL`, поэтому приведение типа не падает, а строка просто исключается):

| Миграция | Таблица(ы) | RLS |
|-----------|----------|-----|
| `7_enable_rls_users`      | `users`                                                                     | ENABLE + FORCE; service / admin / собственные |
| `14_organizations`        | `organizations`, `organization_memberships`                                 | ENABLE + FORCE; видимость по **членству** |
| `17_org_rls_recursion_fix`| (пересоздаёт политики организаций)                                          | использует `SECURITY DEFINER` `app_is_org_member()`, чтобы разорвать рекурсию политик (SQLSTATE 42P17) |
| `20_gov_services`         | `gov_applications`, `gov_references`, `gov_notifications`, `gov_payments`, `gov_appointments` | ENABLE + FORCE; service / admin / собственные. (Каталог `gov_services` публичный, без RLS) |
| `21_user_integrations`    | `user_integrations`                                                         | ENABLE + FORCE; service / admin / собственные |

Глобальные конфигурационные таблицы намеренно **не** защищены RLS; их страховка на
уровне базы — `REVOKE` табличных привилегий у роли `app_user`
(`17_least_privilege_config_grants` для `permissions` / `role_permissions` /
`ai_prompts` / `ai_knowledge`; `27_site_appearance` для единственной строки
оформления). Таблицы провайдера (`26_sso_provider`: `developer_apps`,
`admin_api_keys`, `login_events`) и `org_stamps` (`25`) также без RLS и
защищаются на уровне usecase/обработчиков.

**Проверка применимости при старте** — RLS молча обходится суперпользователями
Postgres и ролями `BYPASSRLS`, поэтому `guardRLSEnforceable`
(`internal/datasources/drivers/driver_pgx.go`) проверяет `pg_roles` для роли
подключения при запуске:

- Если у роли есть `rolsuper` или `rolbypassrls`: **в продакшене fail closed**
  (старт прерывается, пул закрывается); **в разработке пишется предупреждение**, и
  работа продолжается (migrate/тесты могут выполняться суперпользователем).
- Поэтому в продакшене api должен подключаться ролью с минимальными правами
  (например, `app_user`). (Compose-стек намеренно работает с
  `ENVIRONMENT=development`, поэтому проверка жёстко падает только в продакшене.)

## Провайдер OIDC (Ory Hydra)

Платформа может сама выступать **провайдером идентификации**: другие приложения
делегируют вход dan через **Ory Hydra**. Эта поверхность активируется, только если
`ProviderConfigured()` истинно (`HYDRA_ADMIN_URL` + `HYDRA_PUBLIC_URL` +
`SSO_STATE_KEY ≥ 32 байт`); иначе она неактивна, и её маршруты не регистрируются.

- **Ядро вход / согласие / выход** — `usecases/provider` + `pkg/hydra` обрабатывают
  challenge Hydra; собственные клиенты (`SSO_FIRSTPARTY_CLIENTS`) пропускают экран
  согласия. Монтируется под `/api/v1/provider`.
- **Applications (единый реестр клиентов)** — `usecases/applications`
  (монтируется на `/api/v1/applications`, защищён `gateway.manage`) — актуальный
  способ регистрировать OAuth2-клиентов: приложения RP «Login with DAN»
  (`web`/`spa`/`native` → `authorization_code`; `spa`/`native` публичные, PKCE, без
  секрета) и m2m-клиенты (`client_credentials`). Каждый — OAuth2-клиент Hydra,
  чьи scope соответствуют разрешённым сервисам шлюза (`application_services` →
  `gateway_services.scope`); `client_secret` конфиденциального клиента
  показывается один раз при создании/ротации.
- **Операторская поверхность (устаревшая)** — `internal/provider/adminapi`
  монтируется на **`/admin`** (через `http.StripPrefix`) для регистрации и
  управления OAuth2-клиентами RP, опираясь на хранилище `devapps`
  (`developer_apps`) и `adminkeys` (стартовые ключи из `SSO_ADMIN_API_KEYS`,
  сверяются по SHA-256). Эта поверхность с админскими API-ключами и наложение
  `developer_apps` ещё существуют, но **для новой работы заменены единой моделью Applications**.
- **Ретранслятор подписи** — `internal/provider/signrelay` монтируется на
  **`/rp/sign/*`**; это обратный прокси, позволяющий нижестоящим RP подписывать
  PDF через eID *посредством* dan, используя его учётные данные RP eidmongolia
  (включается `SIGN_RELAY_TOKEN` + `EID_RP_SECRET`).

> **Оговорка про принуждение.** Назначение сервисов приложению задаёт **scope**
> его OAuth-клиента — это только регистрация/конфигурация. *Рантайм-принуждение*
> на каждый запрос потребовало бы прокси-шлюза, который выполняет интроспекцию
> предъявленного токена (`hydra.Admin.Introspect` существует) против scope сервиса
> для каждого маршрута, и такого прокси **пока нет**. Поэтому сегодня назначение
> сервисов — не действующая авторизация; не принимайте его за принудительный контроль доступа.

## База данных

- **Драйвер:** pgx v5 (`github.com/jackc/pgx/v5` + pgxpool), рукописный SQL — **без ORM**.
- **База:** PostgreSQL, где RLS — граница по пользователям.
- **Миграции:** нумерованные SQL-файлы в `migrations/` (`N_name.up.sql` +
  `.down.sql`), применяются сервисом `migrate` в compose / `cmd/migration`.
  **AutoMigrate нет** — схема берётся только из файлов `*.up.sql` (`cmd/migration/main.go`).
- **Трассировка:** OpenTelemetry через инструментирование пула pgx (`otelpgx`).

> **Коллизия нумерации миграций.** Две миграции имеют префикс `17_`:
> `17_least_privilege_config_grants` и `17_org_rls_recursion_fix`. Они независимы и
> обе применяются; исполнитель упорядочивает файлы по номеру, поэтому учитывайте
> это при добавлении миграций от `18_` и выше или при разборе порядка применения.

### Управление подключениями

Пул настраивается из окружения (`internal/datasources/drivers/driver_pgx.go`,
`SetupPgxPostgres`):

```go
poolCfg.MaxConns        = cfg.MaxConns    // DB_MAX_OPEN_CONNS   (по умолчанию 25)
poolCfg.MinConns        = cfg.MinConns    // DB_MAX_IDLE_CONNS   (по умолчанию 5)
poolCfg.MaxConnLifetime = cfg.MaxLifetime // DB_CONN_MAX_LIFE_MINS (по умолчанию 15)
```

В продакшене требуется DSN с проверкой TLS (`sslmode=verify-full` или
`verify-ca`) — это обеспечивает проверка конфигурации.

## Наблюдаемость

### Логирование

- **Библиотека:** Zap (структурированное), через `pkg/logger`. JSON в продакшене,
  консоль в разработке. Request ID и trace ID пробрасываются через помощники `*WithContext`.

### Метрики

- **Библиотека:** Prometheus, endpoint `GET /metrics` (закрыт — см.
  [Операционные endpoint](#операционные-endpoint)). Счётчики и задержки HTTP-запросов,
  попадания/промахи/ошибки кэша по слоям, результаты отправки OTP и живая статистика пула pgx.

### Трассировка

- **Библиотека:** OpenTelemetry; экспортер выбирается через `OTEL_EXPORTER`
  (пусто = noop, `stdout` или `otlp`), выборка — `OTEL_SAMPLE_RATIO`.

## Операционные endpoint

| Endpoint | Доступ |
|----------|--------|
| `GET /health` | Открыт — liveness (для балансировщиков / оркестраторов). |
| `GET /ready`  | Открыт — readiness: ping базы (пул pgx) + проверка Redis. |
| `GET /metrics` | **Закрыт** `ObservabilityGate`. |
| `GET /swagger/doc.json` | **Закрыт** `ObservabilityGate`. |

`ObservabilityGate` (`middleware_observability_gate.go`) защищает два
чувствительных для оператора endpoint: в **разработке** они всегда открыты;
в **продакшене** требуют `Authorization: Bearer <OBSERVABILITY_TOKEN>` (сравнение
за постоянное время) и возвращают **404** — а не 401 — при любом несовпадении или
если `OBSERVABILITY_TOKEN` не задан, так что само их существование остаётся
скрытым от разведки.

## Функции безопасности

| Функция | Реализация | Расположение |
|-------------------|-----------------------------------------|--------------------------------------------|
| Row-Level Security| изоляция в базе по пользователям + проверка при старте | `datasources/rls/`, `drivers/driver_pgx.go`, миграции `7/14/20/21` |
| Аутентификация    | eID RP + Google OAuth                   | `usecases/auth`, `pkg/{eid,google}`        |
| Авторизация       | модель из 4 ролей + динамический RBAC   | `domain_users.go`, `middlewares/middleware_rbac.go` |
| Заголовки безопасности | HSTS, CSP, nosniff, frame options  | `middlewares/middleware_security.go`       |
| CORS              | белый список из окружения, wildcard только в разработке | `middlewares/middleware_cors.go`           |
| Ограничение запросов | по IP (auth / ai / poll / запись gov) | `middlewares/middleware_ratelimit.go`      |
| Ограничение тела  | общее + строже на `/auth`               | `middlewares/middleware_bodysizelimit.go`  |
| Гейт операционных endpoint | bearer-токен, 404 в проде      | `middlewares/middleware_observability_gate.go` |
| Валидация ввода   | теги структур `validate:`               | `internal/http/datatransfers/requests/`    |
| Шифрование секретов | OAuth-токены AES-256-GCM              | `usecases/integrations` (`INTEGRATION_ENC_KEY`) |
| SQL-инъекции      | pgx (параметризованные запросы)         | `internal/datasources/repositories/`       |
| Подписание PDF    | PAdES через серверный сертификат Document-Signer | `usecases/sign` (`SIGN_SIGNER_*`)          |

## Дизайн API

Все маршруты API находятся под `/api/v1`; каждый модуль монтирует `/v1/<module>`:
`auth`, `users`, `users/me/eid`, `rbac`, `org`, `gov`, `integrations`, `assets`,
`gspace`, `gateway`, `core`, `sso`, `admin`, `superadmin`, `ai`, `audit`,
`security`, `site`, `sign`, а также (при настроенной Hydra) `provider` +
`applications`. Инфраструктурные endpoint (`/health`, `/ready`, `/metrics`,
`/swagger`) и поверхности провайдера (`/admin`, `/rp/sign`) находятся в корне.
**Полные таблицы endpoint — в [API_CONTRACT.md](API_CONTRACT.md)** и в
сгенерированной спецификации OpenAPI (`/swagger`).

### Формат ответа

Единый конверт (`internal/http/handlers/v1/handler_base_response.go`):

**Успех**

```json
{ "status": true, "message": "login success", "data": { }, "request_id": "…" }
```

**Ошибка**

```json
{ "status": false, "message": "user not found", "request_id": "…" }
```

**Ошибка валидации (422)**

```json
{ "status": false, "message": "validation failed",
  "data": { "errors": { "national_id": "national_id is required" } }, "request_id": "…" }
```

Доменные ошибки (`internal/apperror`) отображаются на коды: NotFound→404,
Unauthorized→401, Forbidden→403, Conflict→409, BadRequest→400, Internal→500.
Причины 5xx логируются и заменяются в теле общим сообщением.

## Стратегия тестирования

- **Юнит-тесты** — слои usecase и обработчиков с моками mockery
  (`internal/test/mocks/`). Быстро, без Docker. `go test ./...`.
- **Интеграционные тесты** — репозитории (включая политики RLS) против настоящих
  Postgres + Redis через testcontainers-go (`internal/test/testenv/`).
  `make test-integration`.
- **Моки** — генерируются mockery. `make mock interface=… dir=… filename=…`.
- **Матрица авторизации** — `routes/routes_authz_matrix_test.go` проверяет
  аутентификацию/разрешения на каждом маршруте.

## Конфигурация

Загружается из `.env` / окружения через Viper (`internal/config/config.go`; см.
`internal/config/.env.example`). Проверка конфигурации обеспечивает
продакшен-инварианты (TLS DSN, `ALLOWED_ORIGINS`, `VERIFY_API_KEY`, длина секрета
JWT). Избранные ключи:

| Группа | Переменные |
|-------|-----------|
| **Сервер** | `PORT`, `ENVIRONMENT` (`development`/`production`), `DEBUG` |
| **База данных** | `DB_POSTGRE_DRIVER`, `DB_POSTGRE_DSN` (dev), `DB_POSTGRE_URL` (prod; `sslmode=verify-full`/`verify-ca`), `DB_MAX_OPEN_CONNS` (25), `DB_MAX_IDLE_CONNS` (5), `DB_CONN_MAX_LIFE_MINS` (15) |
| **JWT** | `JWT_SECRET` (≥32), `JWT_EXPIRED` (ч, 1–24), `JWT_ISSUER`, `JWT_REFRESH_EXPIRED` (дн., 7) |
| **Redis** | `REDIS_HOST`, `REDIS_PASS`, `REDIS_EXPIRED` (мин) |
| **Криптография** | `BCRYPT_COST` (12) |
| **Verify (OTP)** | `OTP_MAX_ATTEMPTS` (5), `VERIFY_API_BASE`, `VERIFY_API_KEY` (обязателен в проде), `VERIFY_CHANNEL` |
| **eID** | `EID_BASE_URL` (`…/v3`), `EID_RP_UUID`, `EID_RP_NAME`, `EID_RP_SECRET`, `EID_CERT_LEVEL` (ADVANCED), `EID_CALLBACK_URL`, `EID_DISPLAY_TEXT`, `SIGN_RELAY_TOKEN` |
| **Подпись** | `SIGN_SIGNER_CERT_FILE`, `SIGN_SIGNER_KEY_FILE` (в проде fail-closed) |
| **Google OAuth** | `GOOGLE_CLIENT_ID`, `GOOGLE_CLIENT_SECRET` |
| **XYP** | `XYP_API_BASE` (`https://xyp.dgov.mn`), `XYP_CLIENT_ID`, `XYP_CLIENT_SECRET` |
| **Gerege Space** | `GSPACE_HOST`, `GSPACE_PORT` (22), `GSPACE_USER`, `GSPACE_PASSWORD`, `GSPACE_BASE_PATH` (gerege-space), `GSPACE_QUOTA_BYTES` (2 МБ) |
| **Gemini AI** | `GEMINI_API_KEY`, `GEMINI_MODEL`, `GEMINI_TTS_MODEL`, `GEMINI_VOICE`, `GEMINI_API_BASE`, `AI_SCOPE_PROMPT` |
| **Gerege Core** | `CORE_API_BASE` (`https://core.gerege.mn`), `CORE_API_TOKEN` |
| **Интеграции** | `INTEGRATION_ENC_KEY` (AES-256-GCM; обязателен в проде) |
| **Провайдер OIDC (Hydra)** | `HYDRA_ADMIN_URL` (`http://hydra:4445`), `HYDRA_PUBLIC_URL`, `SSO_STATE_KEY` (≥32), `SSO_FIRSTPARTY_CLIENTS`, `SSO_ADMIN_API_KEYS`, `SSO_ADMIN_SUBS` |
| **Наблюдаемость** | `OTEL_EXPORTER` (``/`stdout`/`otlp`), `OTEL_SAMPLE_RATIO`, `OBSERVABILITY_TOKEN` |
| **Сеть** | `ALLOWED_ORIGINS` (обязателен в проде), `TRUSTED_PROXIES` |
| **Начальная настройка** | `SUPERADMIN_EMAIL` |

## Развёртывание

```bash
go build ./...                 # сборка
docker compose up -d --build   # db + redis + migrate (разовый) + api + web
```

Проверка здоровья: `curl http://localhost:8080/health`. Топология развёртывания —
в `docs/DEPLOYMENT.md`.

## Благодарности и лицензия

Платформа опирается на открытые проекты:

| Проект | Автор | Лицензия | Что использовано |
|---------|--------|---------|--------------|
| [snykk/go-rest-boilerplate](https://github.com/snykk/go-rest-boilerplate) | Najib Fikri | MIT | Слои Clean Architecture, кэширование, наблюдаемость и стратегия тестирования |

Слой доставки переведён с **Gin на chi (net/http)**, слой данных — с
**sqlx на pgx (pgxpool)**; стек аутентификации, модель безопасности RLS,
интеграции eID/SSO/OIDC-провайдера и функциональные модули созданы для этой
платформы. Как MIT-производная работа она сохраняет исходное уведомление об
авторском праве, а код распространяется по лицензии MIT (см. `LICENSE`).

---

**Gerege Template Platform V3.0** — совместная разработка **команды Gerege Systems** и **Claude AI**, 2026.
