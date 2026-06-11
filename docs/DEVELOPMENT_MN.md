# Development Guide

> 🌐 [English](DEVELOPMENT.md) · **Монгол**

Энэ заавар нь хөгжүүлэгчдэд **Gerege Backend Template v27** кодын бааз дээр тохиргоо
хийж, ажиллахад туслана.

> **Эх сурвалж.** Najib Fikri-ийн нээлттэй эх
> [snykk/go-rest-boilerplate](https://github.com/snykk/go-rest-boilerplate)
> (MIT)-аас гаралтай бөгөөд HTTP давхаргыг **Gin → chi (net/http)**, өгөгдлийн
> давхаргыг **sqlx → pgx (pgxpool)** болгож хөрвүүлсэн. Бүрэн зохиогчдын мэдээллийг
> [ARCHITECTURE.md](./ARCHITECTURE.md#credits--license)-аас үз.

## Шаардлага (Prerequisites)

- Go 1.26+
- Docker & Docker Compose (зөвхөн integration тест / локал стек-д)
- PostgreSQL 15+ (эсвэл Docker ашиглах)
- Make

## Түргэн эхлүүлэх (Quick Start)

```bash
# 1. Copy environment file (note: it lives under internal/config/)
cp internal/config/.env.example internal/config/.env
# Edit .env — JWT_SECRET must be at least 32 characters

# 2. Start the stack (Postgres + Redis + API)

# 3. Or run locally: apply migrations, then serve
```

Сервер `http://localhost:8080` дээр ажиллана; Swagger UI нь
`http://localhost:8080/swagger/` дээр байна.

## Хөгжүүлэлтийн командууд (Development Commands)

```bash
make build              # Build the API binary
make tidy               # go mod tidy
make lint               # golangci-lint
make fmt                # gofmt all files
make swag               # Regenerate OpenAPI spec (docs/) from godoc annotations
make pre-push           # Mirror CI locally: lint + test + swag drift + build
```

## Тест (Testing)

```bash
make test               # Unit tests (mocks only — fast, no Docker)
make test-integration   # Integration tests (requires Docker: Postgres + Redis)
make test-cover         # Tests with coverage report
```

## Өгөгдлийн сан (Database)

### Migration-ууд

```bash
```

Migration-ууд нь `internal/datasources/migration/` (болон `migrations/`) доторх
түүхий SQL файлууд бөгөөд migration runner-аар хэрэгждэг. Schema-г өөрчлөхдөө
урагшлах (forward) SQL migration файл нэм; migration runner үүнийг idempotent
байдлаар хэрэгжүүлнэ (advisory lock + файл тус бүрийн transaction +
`schema_migrations` хяналт). ORM AutoMigrate байхгүй — `internal/datasources/records/`
доторх record struct-ууд нь schema тодорхойлолт биш, харин pgx-ээр уншигддаг
энгийн struct-ууд юм.

## Кодын зохион байгуулалт (Code Organization)

### Шинэ фичер нэмэх (Adding a New Feature)

Давхаргуудыг дотноос гадагшаа дагана. Лавлагаа болгож одоо байгаа `users` / `auth`
модулиудыг ашигла. Жишээ: `Product` нөөц нэмэх.

1. **Domain Entity** — `internal/business/domain/domain.products.go`
   ```go
   package domain

   type Product struct {
       ID        string
       Name      string
       Price     int64
       CreatedAt time.Time
   }
   ```

2. **Repository Interface** — `internal/datasources/repositories/interface/interface.go` руу нэм
   ```go
   type ProductRepository interface {
       Store(ctx context.Context, in *domain.Product) (domain.Product, error)
       GetByID(ctx context.Context, id string) (domain.Product, error)
   }
   ```

3. **Record struct + Repository Impl** — `internal/datasources/records/record_products.go`
   болон `internal/datasources/repositories/postgres/products/`

   Record нь `db:"..."` tag-тай **энгийн Go struct** юм. `pgx.RowToStructByName`
   нь үр дүнгийн баганануудыг нэрээр нь талбаруудтай тааруулдаг бөгөөд soft-delete
   нь энгийн nullable `*time.Time DeletedAt` (NULL → nil) — **gorm tag, AutoMigrate
   байхгүй**.
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
   Repository нь `*pgxpool.Pool` авч, гараар бичсэн SQL ажиллуулдаг —
   `INSERT ... RETURNING`-г `pgx.CollectExactlyOneRow` + `pgx.RowToStructByName`-ээр
   цуглуулна. `23505` unique violation нь `apperror.Conflict` болж буудаг; уншихдаа
   `deleted_at IS NULL` нөхцөлийг ИЛ-ээр нэмнэ.
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

4. **Usecase Interface + Impl** — `internal/business/usecases/products/`
   ```go
   // products.usecase.go
   type Usecase interface {
       Create(ctx context.Context, in CreateRequest) (domain.Product, error)
       GetByID(ctx context.Context, id string) (domain.Product, error)
   }
   ```

5. **DTOs** — `internal/http/datatransfers/{requests,responses}/`
   ```go
   type CreateProductRequest struct {
       Name  string `json:"name" validate:"required,min=1,max=255"`
       Price int64  `json:"price" validate:"required,gt=0"`
   }
   ```

6. **Handler** — `internal/http/handlers/v1/products/products_handler.go`

   Handler-ууд нь `func(w http.ResponseWriter, r *http.Request) error` гарын
   үсэгтэй бөгөөд route бүртгэх үед `v1.Wrap`-ээр ороогддог (буцаасан алдааг JSON
   envelope болгон хувиргадаг). Body-г `v1.DecodeBody`-ээр унш, контекстийг
   `r.Context()`-ээр унш, `v1.NewSuccessResponse` / `v1.RespondWithError`-ээр буцаа.
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

7. **Route** — `internal/http/routes/route_products.go` (`route_users.go`-г дуурайлга)

   Route-ууд нь chi router ашигладаг; handler бүрийг `v1.Wrap`-ээр оро. Path
   параметрийг `chi.URLParam(r, "id")`-ээр унш.
   ```go
   func (rt *productsRoute) Routes() {
       rt.router.Route("/v1/products", func(r chi.Router) {
           r.Use(rt.authMiddleware)
           r.Post("/", v1.Wrap(rt.handler.Create))
           r.Get("/{id}", v1.Wrap(rt.handler.GetByID))
       })
   }
   ```

8. **Wire Up** — `cmd/api/server/server.go` дотор одоо байгаагийнх нь хажууд
   repo → usecase → route-ийг бүтээ:
   ```go
   productRepo := productspostgres.NewProductRepository(pool)
   productsUC := products.NewUsecase(productRepo)
   routes.NewProductsRoute(api, productsUC, authMiddleware).Routes()
   ```

### Тест бичих (Writing Tests)

#### Unit тестүүд (Usecase давхарга)

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

#### Handler тестүүд (net/http)

chi router-г (эсвэл `v1.Wrap`-ээр ороосон handler-г) `net/http/httptest`-ээр
жолоодоно — `httptest.NewRequest` нь хүсэлтийг бүтээж, `httptest.NewRecorder`
нь хариуг барьж авна. Fiber тест app байхгүй.

```go
func TestHandler_Create(t *testing.T) {
    // ... mock usecase-тай router бүтээх ...
    req := httptest.NewRequest(http.MethodPost, "/api/v1/products", strings.NewReader(body))
    rec := httptest.NewRecorder()
    router.ServeHTTP(rec, req)
    require.Equal(t, http.StatusCreated, rec.Code)
}
```

#### Integration тестүүд (Repository давхарга)

```go
//go:build integration

func TestProductRepository_Store(t *testing.T) {
    pool := testenv.SetupPostgres(t)    // testcontainers — бодит Postgres (pgxpool)
    repo := postgres.NewProductRepository(pool)
    got, err := repo.Store(context.Background(), &domain.Product{Name: "X", Price: 100})
    assert.NoError(t, err)
    assert.NotEmpty(t, got.ID)
}
```

### Mock үүсгэх (Generating Mocks)

```bash
# Generate a mock for one interface
make mock interface=ProductRepository \
          dir=internal/datasources/repositories/interface \
          filename=mock.repository_products.go
```

## Кодын хэв маяг (Code Style)

### Нэрлэх дүрэм (Naming Conventions)

| Type        | Convention   | Example            |
|-------------|--------------|--------------------|
| Package     | lowercase    | `repository`       |
| Interface   | CamelCase    | `UserRepository`   |
| Struct      | CamelCase    | `Handler`          |
| Function    | CamelCase    | `GetByID`          |
| Variable    | camelCase    | `userCount`        |
| Constant    | CamelCase / sentinel | `RoleAdmin`, `ErrEmptyEmail` |
| JSON field  | snake_case   | `request_id`       |

### Алдаа боловсруулалт (Error Handling)

Typed domain алдаануудыг (`internal/apperror`) буцаа — хэзээ ч panic болгож,
санах сангийн алдааг client руу алдуулж болохгүй:

```go
user, err := s.repo.GetByID(ctx, id)
if err != nil {
    return domain.User{}, err   // apperror.NotFound surfaces as 404
}
```

`RespondWithError` (`handler.base_response.go` дотор) нь алдааны төрлийг статус
кодод буулгаж, 5xx-ийн шалтгаанг log-д бичиж, цэвэр envelope-ийг render хийнэ.

### Контекст ашиглах (Context Usage)

`context.Context`-ийг үргэлж эхэнд нь дамжуул; handler дотор үүнийг
`r.Context()`-ээр унш, pgx дуудлага бүрд дамжуул:

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

## AI туслахыг өргөтгөх

> Гүн тайлбар: [AI_PIPELINE_MN.md](AI_PIPELINE_MN.md) — урсгал, prompt давхарга, voice, troubleshooting.

Gemini pipeline (`internal/business/usecases/ai`) нь проект бүрд өргөтгөгдөхөөр
зохиогдсон:

- **Tool нэмэх** — `ai.ToolDef` (Gemini function declaration + Go `Execute`
  функц) бичээд `cmd/api/server/server.go`-ийн tool жагсаалтад нэмнэ. Model
  хэзээ дуудахаа өөрөө шийднэ; backend хүсэлтийн context-оор гүйцэтгэдэг тул
  DB хандалтад RLS үйлчилнэ. Жишээ: `KnowledgeSearchTool` (`ai_knowledge`-ээс
  хайдаг), `get_server_time`.
- **Туслахын чиглэлийг өөрчлөх** — `scope` давхаргыг ажиллаж байх үед нь
  засна (Админ → Тохиргоо, эсвэл `PUT /admin/ai/prompts/scope`). Suurь
  хамгаалалтын давхарга (хэл, хүрээний сахилт, prompt-injection эсэргүүцэл)
  `ai_prompts.go`-д хатуу бичигдсэн — тэр хэвээрээ байх ёстой.
- **Мэдлэгийн санг өргөтгөх** — `ai_knowledge`-д мөр нэмнэ
  (title/content/tags). `repositories/postgres/ai`-ийн ILIKE хайлт нэг query —
  сан томрох үед tsvector эсвэл pgvector-оор солино.
- **Model-ууд** — чат/STT/орчуулга `GEMINI_MODEL`, TTS `GEMINI_TTS_MODEL`
  (audio гаргадаг тусдаа model) хэрэглэнэ; хоёулаа зөвхөн env тохиргоо.

## API баримтжуулалт (API Documentation)

### Swagger annotation-ууд

Handler-ууд нь `swag`-ийн ашигладаг godoc annotation-уудыг агуулна:

```go
// @Summary      Login
// @Description  Authenticate and issue an access + refresh token pair
// @Tags         auth
// @Accept       json
// @Produce      json
// @Param        body body requests.LoginRequest true "Credentials"
// @Success      200 {object} v1.BaseResponse
// @Failure      401 {object} v1.BaseResponse
// @Router       /auth/login [post]
func (h Handler) Login(w http.ResponseWriter, r *http.Request) error { /* ... */ }
```

### Баримтжуулалтыг дахин үүсгэх (Regenerate Docs)

```bash
make swag
```

Swagger UI: `http://localhost:8080/swagger/`. Хэрэв `docs/` нь annotation-аас
зөрвөл CI алдаа гаргана (`make ci-swag-check`).

## Алдаа засах (Troubleshooting)

**Database connection failed**
```bash
docker-compose ps                 # is Postgres up?
# check DB_POSTGRE_DSN in internal/config/.env
```

**Migration failed** — `migrations/` дараалал болон `schema_migrations`
хүснэгтийг шалга; runner нь advisory lock + файл тус бүрийн transaction ашигладаг.

**Tests failing**
```bash
go test -v ./...                  # verbose
go test -v -run TestUsecase_Create ./internal/business/usecases/products/...
```

**Lint errors**
```bash
golangci-lint run --fix
```

## Аюулгүй байдлын шалгах жагсаалт (Security Checklist)

Deploy хийхээс өмнө дараахыг баталгаажуул:

- [ ] Бүх хамгаалагдсан endpoint auth middleware-тэй
- [ ] Нэргүй endpoint-ууд (`/auth/*`) rate limiter + body cap-аа хадгалсан
- [ ] `JWT_SECRET` нь ≥ 32 санамсаргүй тэмдэгт бөгөөд жишээ утга биш
- [ ] Input validation (`validate:` tag-ууд) нь хүсэлтийн DTO бүрийг хамарсан
- [ ] Нууц утгууд environment-ээс ирдэг, хэзээ ч commit хийгддэггүй
- [ ] Production-д `ALLOWED_ORIGINS` тохируулагдсан (wildcard байхгүй)
- [ ] Edge / load balancer дээр HTTPS албадан хэрэгжсэн

---

**Gerege Template Version 27.0** — **Gerege Systems Development Team** болон **Claude AI** хамтран бүтээв, 2026.
