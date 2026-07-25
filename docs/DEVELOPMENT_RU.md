# Руководство разработчика

> 🌐 **Русский** · [English](DEVELOPMENT.md) · [Монгол](DEVELOPMENT_MN.md) · [中文](DEVELOPMENT_ZH.md)

Это руководство помогает развернуть и вести кодовую базу **Gerege Template
Platform V3.0** (Цахим үйлчилгээг бүтээх суурь) — готовой к продакшену основы, на
которой можно построить любую цифровую услугу государственного или частного
сектора. Её флагманское эталонное развёртывание — **Gerege Template Platform**
(template.gerege.mn), платформа государственных и частных услуг на базе eID,
построенная на этом стеке.

> **Происхождение.** Производная от открытого проекта
> [snykk/go-rest-boilerplate](https://github.com/snykk/go-rest-boilerplate)
> (MIT, автор Najib Fikri): слой HTTP переведён с **Gin на chi (net/http)**, слой
> данных — с **sqlx на pgx (pgxpool)**. Полные благодарности см. в
> [ARCHITECTURE.md](./ARCHITECTURE.md#credits--license).

## Требования

- Go 1.26+
- Docker и Docker Compose (только для интеграционных тестов / локального стека)
- PostgreSQL 15+ (или Docker)
- Make

## Быстрый старт

```bash
# 1. Скопируйте файл окружения (он лежит в internal/config/)
cp internal/config/.env.example internal/config/.env
# Отредактируйте .env — JWT_SECRET должен быть не короче 32 символов

# 2. Поднимите стек (Postgres + Redis + API)

# 3. Или запустите локально: примените миграции, затем сервер
```

Сервер доступен на `http://localhost:8080`; Swagger UI — на
`http://localhost:8080/swagger/`.

## Команды разработки

```bash
make build              # Собрать бинарник API
make tidy               # go mod tidy
make lint               # golangci-lint
make fmt                # gofmt по всем файлам
make swag               # Перегенерировать спецификацию OpenAPI (docs/) из аннотаций godoc
make pre-push           # Воспроизвести CI локально: lint + тесты + дрейф swag + сборка
```

## Тестирование

```bash
make test               # Юнит-тесты (только моки — быстро, без Docker)
make test-integration   # Интеграционные тесты (нужен Docker: Postgres + Redis)
                        # CI тоже их запускает (отдельный job), деплой их ждёт
make test-cover         # Тесты с отчётом о покрытии
```

## База данных

### Миграции

```bash
```

Миграции — это обычные SQL-файлы в `backend/migrations/` (пары `N_name.up.sql` +
`N_name.down.sql`). Go-пакет `internal/datasources/migration/` содержит только
**исполнитель** (без SQL); точка входа CLI — `cmd/migration/main.go`
(`migrationsDir = "migrations"`). Чтобы изменить схему, добавьте прямой SQL-файл
миграции в `backend/migrations/`; исполнитель применит его идемпотентно — файлы
упорядочиваются по ведущему номеру, каждый файл вместе со строкой в
`schema_migrations` фиксируется в одной транзакции, а весь прогон удерживает
сессионный advisory lock, поэтому параллельные исполнители сериализуются.
**AutoMigrate из ORM нет** — структуры записей в `internal/datasources/records/` —
это обычные структуры, которые сканирует pgx, а не описания схемы; схема берётся
только из файлов `*.up.sql`.

## Организация кода

### Добавление новой функциональности

Двигайтесь по слоям изнутри наружу. Ориентируйтесь на существующие модули
`users` / `auth` — в бэкенде уже около 18 usecase-срезов в
`internal/business/usecases/` (`ai`, `assets`, `audit`, `auth`, `core`,
`gateway`, `gov`, `gspace`, `integrations`, `org`, `provider`, `rbac`,
`security`, `sign`, `site`, `sso`, `superadmin`, `users`), и каждый следует этому
же шаблону. Пример: добавляем ресурс `Product`.

1. **Доменная сущность** — `internal/business/domain/domain.products.go`

   ```go
   package domain

   type Product struct {
       ID        string
       Name      string
       Price     int64
       CreatedAt time.Time
   }
   ```

2. **Интерфейс репозитория** — добавьте в `internal/datasources/repositories/interface/interface.go`

   ```go
   type ProductRepository interface {
       Store(ctx context.Context, in *domain.Product) (domain.Product, error)
       GetByID(ctx context.Context, id string) (domain.Product, error)
   }
   ```

3. **Структура записи и реализация репозитория** —
   `internal/datasources/records/record_products.go` и
   `internal/datasources/repositories/postgres/products/`

   Запись — это **обычная структура Go** с тегами `db:"..."`.
   `pgx.RowToStructByName` сопоставляет колонки результата с полями по имени, а
   мягкое удаление — это обычное nullable-поле `*time.Time DeletedAt`
   (NULL → nil): **никаких тегов gorm, никакого AutoMigrate**.

   ```go
   // internal/datasources/records/record_products.go
   type Product struct {
       ID        string     `db:"id"`
       Name      string     `db:"name"`
       Price     int64      `db:"price"`
       CreatedAt time.Time  `db:"created_at"`
       DeletedAt *time.Time `db:"deleted_at"`
   }
   ```

   Репозиторий принимает `*pgxpool.Pool` и выполняет рукописный SQL —
   `INSERT ... RETURNING`, результат собирается через `pgx.CollectExactlyOneRow` +
   `pgx.RowToStructByName`. Нарушение уникальности `23505` превращается в
   `apperror.Conflict`; при чтении добавляется явный предикат `deleted_at IS NULL`.

   ```go
   func (r *productRepository) Create(ctx context.Context, p *records.Product) (records.Product, error) {
       rows, _ := r.pool.Query(ctx, `INSERT INTO products (id, name, price) VALUES ($1,$2,$3)
           RETURNING id, name, price, created_at, deleted_at`, p.ID, p.Name, p.Price)
       out, err := pgx.CollectExactlyOneRow(rows, pgx.RowToStructByName[records.Product])
       if err != nil {
           var pgErr *pgconn.PgError
           if errors.As(err, &pgErr) && pgErr.Code == "23505" {
               return records.Product{}, apperror.Conflict("product exists")
           }
           return records.Product{}, err
       }
       return out, nil
   }
   ```

4. **Интерфейс и реализация usecase** — `internal/business/usecases/products/`

   ```go
   // products.usecase.go
   type Usecase interface {
       Create(ctx context.Context, in CreateRequest) (domain.Product, error)
       GetByID(ctx context.Context, id string) (domain.Product, error)
   }
   ```

5. **DTO** — `internal/http/datatransfers/{requests,responses}/`

   ```go
   type CreateProductRequest struct {
       Name  string `json:"name" validate:"required,min=1,max=255"`
       Price int64  `json:"price" validate:"required,gt=0"`
   }
   ```

6. **Обработчик** — `internal/http/handlers/v1/products/products_handler.go`

   Обработчики имеют сигнатуру
   `func(w http.ResponseWriter, r *http.Request) error` и оборачиваются `v1.Wrap`
   при регистрации маршрута (обёртка превращает возвращённую ошибку в JSON-конверт).
   Тело декодируйте через `v1.DecodeBody`, контекст читайте из `r.Context()`,
   отвечайте через `v1.NewSuccessResponse` / `v1.RespondWithError`.

   ```go
   func (h Handler) Create(w http.ResponseWriter, r *http.Request) error {
       var req requests.CreateProductRequest
       if err := v1.DecodeBody(r, &req); err != nil {
           return v1.NewErrorResponse(w, r, http.StatusBadRequest, "invalid request body")
       }
       if err := validators.ValidatePayloads(req); err != nil {
           return v1.RespondWithError(w, r, err)
       }
       data, err := h.usecase.Create(r.Context(), products.CreateRequest{Name: req.Name, Price: req.Price})
       if err != nil {
           return v1.RespondWithError(w, r, err)
       }
       return v1.NewSuccessResponse(w, r, http.StatusCreated, "created", data)
   }
   ```

7. **Маршрут** — `internal/http/routes/route_products.go` (по образцу `route_users.go`)

   Маршруты используют роутер chi; каждый обработчик оборачивайте `v1.Wrap`.
   Параметры пути читаются через `chi.URLParam(r, "id")`.

   ```go
   func (rt *productsRoute) Routes() {
       rt.router.Route("/v1/products", func(r chi.Router) {
           r.Use(rt.authMiddleware)
           r.Post("/", v1.Wrap(rt.handler.Create))
           r.Get("/{id}", v1.Wrap(rt.handler.GetByID))
       })
   }
   ```

8. **Связывание** — в `cmd/api/server/server.go` создайте репозиторий → usecase →
   маршрут рядом с существующими:

   ```go
   productRepo := productspostgres.NewProductRepository(pool)
   productsUC := products.NewUsecase(productRepo)
   routes.NewProductsRoute(api, productsUC, authMiddleware).Routes()
   ```

9. **Row-Level Security (таблицы по пользователям / арендаторам)** — если новая
   таблица хранит данные конкретного гражданина (а не публичный справочник), она
   **обязана** иметь политики RLS. Следуйте устоявшемуся шаблону из
   `migrations/14_organizations.up.sql`, `migrations/20_gov_services.up.sql` и
   `migrations/21_user_integrations.up.sql`: `ALTER TABLE … ENABLE ROW LEVEL
   SECURITY` **и** `FORCE ROW LEVEL SECURITY`, затем тройка политик
   `service` / `admin` / `self` на сессионных GUC `app.user_id` / `app.user_role`.
   Репозиторий должен **учитывать RLS** — открывать транзакцию `withRLS`, которая
   выдаёт `SET LOCAL app.user_id` / `SET LOCAL app.user_role` из личности запроса
   (`internal/datasources/rls` переносит её в контексте; готовый пример —
   `repositories/postgres/org` / `repositories/postgres/gov`). Запрос без личности
   ставит пустые GUC, поэтому каждая политика запрещает каждую строку (fail-closed).
   RLS работает только тогда, когда api подключается ролью без прав
   суперпользователя — проверка при старте блокирует подключение суперпользователя
   / `BYPASSRLS` в продакшене (см. [SECURITY.md](SECURITY.md)). Публичные
   справочные таблицы (например, каталог `gov_services`) остаются без RLS и
   защищаются табличными привилегиями.

### Написание тестов

#### Юнит-тесты (слой usecase)

```go
// internal/business/usecases/products/products.create_test.go
func TestUsecase_Create(t *testing.T) {
    repo := mocks.NewProductRepository(t)
    repo.On("Store", mock.Anything, mock.AnythingOfType("*domain.Product")).
        Return(domain.Product{ID: "p1", Name: "X"}, nil)

    uc := products.NewUsecase(repo)
    got, err := uc.Create(context.Background(), products.CreateRequest{Name: "X", Price: 100})

    assert.NoError(t, err)
    assert.Equal(t, "p1", got.ID)
    repo.AssertExpectations(t)
}
```

#### Тесты обработчиков (net/http)

Прогоняйте роутер chi (или обёрнутый `v1.Wrap` обработчик) через
`net/http/httptest` — `httptest.NewRequest` строит запрос,
`httptest.NewRecorder` перехватывает ответ. Никакого тестового приложения Fiber.

```go
func TestHandler_Create(t *testing.T) {
    // ... собрать роутер с замоканным usecase ...
    req := httptest.NewRequest(http.MethodPost, "/api/v1/products", strings.NewReader(body))
    rec := httptest.NewRecorder()
    router.ServeHTTP(rec, req)
    require.Equal(t, http.StatusCreated, rec.Code)
}
```

#### Интеграционные тесты (слой репозитория)

```go
//go:build integration

func TestProductRepository_Store(t *testing.T) {
    pool := testenv.SetupPostgres(t)    // testcontainers — настоящий Postgres (pgxpool)
    repo := postgres.NewProductRepository(pool)
    got, err := repo.Store(context.Background(), &domain.Product{Name: "X", Price: 100})
    assert.NoError(t, err)
    assert.NotEmpty(t, got.ID)
}
```

### Генерация моков

```bash
# Сгенерировать мок для одного интерфейса
make mock interface=ProductRepository \
          dir=internal/datasources/repositories/interface \
          filename=mock.repository_products.go
```

## Стиль кода

### Соглашения об именовании

| Тип | Соглашение | Пример |
|-------------|--------------|--------------------|
| Пакет | нижний регистр | `repository` |
| Интерфейс | CamelCase | `UserRepository` |
| Структура | CamelCase | `Handler` |
| Функция | CamelCase | `GetByID` |
| Переменная | camelCase | `userCount` |
| Константа | CamelCase / маркер | `RoleAdmin`, `ErrEmptyEmail` |
| Поле JSON | snake_case | `request_id` |

### Обработка ошибок

Возвращайте типизированные доменные ошибки (`internal/apperror`) — никогда не
паникуйте и не выпускайте библиотечные ошибки к клиенту:

```go
user, err := s.repo.GetByID(ctx, id)
if err != nil {
    return domain.User{}, err   // apperror.NotFound превращается в 404
}
```

`RespondWithError` (в `handler_base_response.go`) сопоставляет тип ошибки со
статусом, логирует причины 5xx и отдаёт аккуратный конверт. Все помощники
конверта живут в том же файле: `v1.DecodeBody` (декодирование JSON с
ограничением размера и запретом неизвестных полей),
`validators.ValidatePayloads` (валидация по тегам структур → 422 с деталями по
полям), `v1.NewSuccessResponse`, `v1.NewErrorResponse` и `v1.RespondWithError`.

### Использование контекста

Всегда передавайте `context.Context` первым аргументом; в обработчиках читайте
его через `r.Context()` и пробрасывайте в каждый вызов pgx:

```go
func (r *postgreUserRepository) GetByID(ctx context.Context, id string) (domain.User, error) {
    rows, err := r.pool.Query(ctx,
        `SELECT `+records.UserColumns+` FROM users WHERE id = $1 AND deleted_at IS NULL`, id)
    if err != nil {
        return domain.User{}, err
    }
    rec, err := pgx.CollectExactlyOneRow(rows, pgx.RowToStructByName[records.Users])
    // ...
}
```

## Расширение AI-ассистента

> Подробно: [AI_PIPELINE.md](AI_PIPELINE.md) — потоки, слои промпта, голос, диагностика.

Конвейер Gemini (`internal/business/usecases/ai`) рассчитан на расширение под проект:

- **Добавить инструмент** — реализуйте `ai.ToolDef` (объявление функции Gemini +
  Go-функция `Execute`) и добавьте его в список инструментов в
  `cmd/api/server/server.go`. Модель решает, когда его вызвать; бэкенд выполняет
  его с контекстом запроса (поэтому к любому обращению к базе применяется RLS).
  Готовые примеры — `KnowledgeSearchTool` (ищет в `ai_knowledge`) и `get_server_time`.
- **Изменить, в чём помогает ассистент** — отредактируйте слой промпта `scope`
  в рантайме (Админ → Настройки или `PUT /admin/ai/prompts/scope`). Базовый слой
  ограничений (язык, соблюдение области, устойчивость к prompt injection) жёстко
  задан в `ai_prompts.go` и должен таким и остаться.
- **Пополнить базу знаний** — вставляйте строки в `ai_knowledge`
  (title/content/tags). Поиск через ILIKE в `repositories/postgres/ai` — один
  запрос; при росте корпуса замените его на tsvector или pgvector.
- **Модели** — чат/STT/перевод используют `GEMINI_MODEL`; TTS — `GEMINI_TTS_MODEL`
  (отдельная модель с поддержкой аудио). Обе настраиваются только через окружение.

## Документация API

### Аннотации Swagger

Обработчики несут аннотации godoc, которые читает `swag`:

```go
// @Summary      Start eID login
// @Description  Begin an eID login session (returns a QR / deep-link challenge to poll)
// @Tags         auth
// @Accept       json
// @Produce      json
// @Success      200 {object} v1.BaseResponse{data=responses.EIDStartResponse}
// @Failure      500 {object} v1.BaseResponse
// @Router       /auth/eid/start [post]
func (h Handler) EIDStart(w http.ResponseWriter, r *http.Request) error { /* ... */ }
```

### Перегенерация документации

```bash
make swag
```

Swagger UI: `http://localhost:8080/swagger/`. CI падает, если `docs/` расходится с
аннотациями (`make ci-swag-check`).

## Диагностика

**Не удаётся подключиться к базе**

```bash
docker-compose ps                 # Postgres поднят?
# проверьте DB_POSTGRE_DSN в internal/config/.env
```

**Миграция упала** — проверьте порядок в `migrations/` и таблицу
`schema_migrations`; исполнитель использует advisory lock и транзакцию на файл.

**Падают тесты**

```bash
go test -v ./...                  # подробный вывод
go test -v -run TestUsecase_Create ./internal/business/usecases/products/...
```

**Ошибки линтера**

```bash
golangci-lint run --fix
```

## Чек-лист безопасности

Перед развёртыванием убедитесь, что:

- [ ] На всех защищённых endpoint стоит middleware аутентификации
- [ ] У анонимных endpoint (`/auth/*`) сохранены лимитер и ограничение тела
- [ ] `JWT_SECRET` — не менее 32 случайных символов и не значение из примера
- [ ] Валидация ввода (теги `validate:`) покрывает каждый request-DTO
- [ ] Секреты берутся из окружения и никогда не коммитятся
- [ ] В продакшене задан `ALLOWED_ORIGINS` (без wildcard)
- [ ] HTTPS обеспечивается на границе / балансировщике

---

**Gerege Template Platform V3.0** — совместная разработка **команды Gerege Systems** и **Claude AI**, 2026.
