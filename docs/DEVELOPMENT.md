# Development Guide

> 🌐 **English** · [Монгол](DEVELOPMENT_MN.md)

This guide helps developers set up and work with the **Gerege Backend Template
v27** codebase.

> **Origin.** Derived from the open-source
> [snykk/go-rest-boilerplate](https://github.com/snykk/go-rest-boilerplate)
> (MIT, by Najib Fikri), with the HTTP layer ported **Gin → chi (net/http)** and
> the data layer **sqlx → pgx (pgxpool)**. See [ARCHITECTURE.md](./ARCHITECTURE.md#credits--license)
> for full credits.

## Prerequisites

- Go 1.26+
- Docker & Docker Compose (only for integration tests / local stack)
- PostgreSQL 15+ (or use Docker)
- Make

## Quick Start

```bash
# 1. Copy environment file (note: it lives under internal/config/)
cp internal/config/.env.example internal/config/.env
# Edit .env — JWT_SECRET must be at least 32 characters

# 2. Start the stack (Postgres + Redis + API)

# 3. Or run locally: apply migrations, then serve
```

The server is available at `http://localhost:8080`; Swagger UI at
`http://localhost:8080/swagger/`.

## Development Commands

```bash
make build              # Build the API binary
make tidy               # go mod tidy
make lint               # golangci-lint
make fmt                # gofmt all files
make swag               # Regenerate OpenAPI spec (docs/) from godoc annotations
make pre-push           # Mirror CI locally: lint + test + swag drift + build
```

## Testing

```bash
make test               # Unit tests (mocks only — fast, no Docker)
make test-integration   # Integration tests (requires Docker: Postgres + Redis)
make test-cover         # Tests with coverage report
```

## Database

### Migrations

```bash
```

Migrations are raw SQL files in `internal/datasources/migration/` (and
`migrations/`), applied by the migration runner. To change the schema, add a
forward SQL migration file; the migration runner applies it idempotently
(advisory lock + per-file transaction + `schema_migrations` tracking). There
is no ORM AutoMigrate — the record structs in `internal/datasources/records/`
are plain structs scanned by pgx, not schema definitions.

## Code Organization

### Adding a New Feature

Follow the layers inward-out. Use the existing `users` / `auth` modules as the
reference. Example: adding a `Product` resource.

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

2. **Repository Interface** — add to `internal/datasources/repositories/interface/interface.go`
   ```go
   type ProductRepository interface {
       Store(ctx context.Context, in *domain.Product) (domain.Product, error)
       GetByID(ctx context.Context, id string) (domain.Product, error)
   }
   ```

3. **Record struct + Repository Impl** — `internal/datasources/records/record_products.go`
   and `internal/datasources/repositories/postgres/products/`

   The record is a **plain Go struct** with `db:"..."` tags. `pgx.RowToStructByName`
   maps result columns to fields by name, and soft-delete is a normal nullable
   `*time.Time DeletedAt` (NULL → nil) — **no gorm tags, no AutoMigrate**.
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
   The repository takes a `*pgxpool.Pool` and runs hand-written SQL —
   `INSERT ... RETURNING`, collected with `pgx.CollectExactlyOneRow` +
   `pgx.RowToStructByName`. A `23505` unique violation becomes `apperror.Conflict`;
   reads add an explicit `deleted_at IS NULL` predicate.
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

   Handlers have the signature `func(w http.ResponseWriter, r *http.Request) error`
   and are wrapped by `v1.Wrap` at route registration (it turns the returned
   error into the JSON envelope). Decode the body with `v1.DecodeBody`, read the
   context with `r.Context()`, and return via `v1.NewSuccessResponse` /
   `v1.RespondWithError`.
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

7. **Route** — `internal/http/routes/route_products.go` (mirror `route_users.go`)

   Routes use a chi router; wrap each handler with `v1.Wrap`. Path params are
   read with `chi.URLParam(r, "id")`.
   ```go
   func (rt *productsRoute) Routes() {
       rt.router.Route("/v1/products", func(r chi.Router) {
           r.Use(rt.authMiddleware)
           r.Post("/", v1.Wrap(rt.handler.Create))
           r.Get("/{id}", v1.Wrap(rt.handler.GetByID))
       })
   }
   ```

8. **Wire Up** — in `cmd/api/server/server.go`, construct repo → usecase →
   route alongside the existing ones:
   ```go
   productRepo := productspostgres.NewProductRepository(pool)
   productsUC := products.NewUsecase(productRepo)
   routes.NewProductsRoute(api, productsUC, authMiddleware).Routes()
   ```

### Writing Tests

#### Unit Tests (Usecase Layer)

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

#### Handler Tests (net/http)

Drive the chi router (or the `v1.Wrap`-ed handler) with `net/http/httptest` —
`httptest.NewRequest` builds the request, `httptest.NewRecorder` captures the
response. No Fiber test app.

```go
func TestHandler_Create(t *testing.T) {
    // ... build router with a mocked usecase ...
    req := httptest.NewRequest(http.MethodPost, "/api/v1/products", strings.NewReader(body))
    rec := httptest.NewRecorder()
    router.ServeHTTP(rec, req)
    require.Equal(t, http.StatusCreated, rec.Code)
}
```

#### Integration Tests (Repository Layer)

```go
//go:build integration

func TestProductRepository_Store(t *testing.T) {
    pool := testenv.SetupPostgres(t)    // testcontainers — real Postgres (pgxpool)
    repo := postgres.NewProductRepository(pool)
    got, err := repo.Store(context.Background(), &domain.Product{Name: "X", Price: 100})
    assert.NoError(t, err)
    assert.NotEmpty(t, got.ID)
}
```

### Generating Mocks

```bash
# Generate a mock for one interface
make mock interface=ProductRepository \
          dir=internal/datasources/repositories/interface \
          filename=mock.repository_products.go
```

## Code Style

### Naming Conventions

| Type        | Convention   | Example            |
|-------------|--------------|--------------------|
| Package     | lowercase    | `repository`       |
| Interface   | CamelCase    | `UserRepository`   |
| Struct      | CamelCase    | `Handler`          |
| Function    | CamelCase    | `GetByID`          |
| Variable    | camelCase    | `userCount`        |
| Constant    | CamelCase / sentinel | `RoleAdmin`, `ErrEmptyEmail` |
| JSON field  | snake_case   | `request_id`       |

### Error Handling

Return typed domain errors (`internal/apperror`) — never panic, never leak
library errors to the client:

```go
user, err := s.repo.GetByID(ctx, id)
if err != nil {
    return domain.User{}, err   // apperror.NotFound surfaces as 404
}
```

`RespondWithError` (in `handler.base_response.go`) maps the error type to a
status code, logs 5xx causes, and renders a clean envelope.

### Context Usage

Always pass `context.Context` first; in handlers read it via `r.Context()` and
thread it through every pgx call:

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

## Extending the AI assistant

> Deep dive: [AI_PIPELINE.md](AI_PIPELINE.md) — flows, prompt layers, voice, troubleshooting.

The Gemini pipeline (`internal/business/usecases/ai`) is built to be extended
per project:

- **Add a tool** — implement an `ai.ToolDef` (a Gemini function declaration +
  a Go `Execute` func) and append it to the tool list in
  `cmd/api/server/server.go`. The model decides when to call it; the backend
  executes it with the request context (so RLS applies to any DB access).
  `KnowledgeSearchTool` (searches `ai_knowledge`) and `get_server_time` are
  the shipped examples.
- **Change what the assistant helps with** — edit the `scope` prompt layer at
  runtime (Admin → Settings, or `PUT /admin/ai/prompts/scope`). The base
  guardrail layer (language, scope enforcement, prompt-injection resistance)
  is hardcoded in `ai_prompts.go` and should stay that way.
- **Grow the knowledge base** — insert rows into `ai_knowledge`
  (title/content/tags). The ILIKE search in
  `repositories/postgres/ai` is a single query — swap it for tsvector or
  pgvector when the corpus grows.
- **Models** — chat/STT/translate use `GEMINI_MODEL`; TTS uses
  `GEMINI_TTS_MODEL` (a separate, audio-capable model). Both are env-only
  config.

## API Documentation

### Swagger Annotations

Handlers carry godoc annotations consumed by `swag`:

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

### Regenerate Docs

```bash
make swag
```

Swagger UI: `http://localhost:8080/swagger/`. CI fails if `docs/` drifts from
the annotations (`make ci-swag-check`).

## Troubleshooting

**Database connection failed**
```bash
docker-compose ps                 # is Postgres up?
# check DB_POSTGRE_DSN in internal/config/.env
```

**Migration failed** — inspect `migrations/` ordering and the `schema_migrations`
table; the runner uses an advisory lock + per-file transaction.

**Tests failing**
```bash
go test -v ./...                  # verbose
go test -v -run TestUsecase_Create ./internal/business/usecases/products/...
```

**Lint errors**
```bash
golangci-lint run --fix
```

## Security Checklist

Before deploying, ensure:

- [ ] All protected endpoints carry the auth middleware
- [ ] Anonymous endpoints (`/auth/*`) keep the rate limiter + body cap
- [ ] `JWT_SECRET` is ≥ 32 random chars and not the example value
- [ ] Input validation (`validate:` tags) covers every request DTO
- [ ] Secrets come from environment, never committed
- [ ] `ALLOWED_ORIGINS` is set (no wildcard) in production
- [ ] HTTPS is enforced at the edge / load balancer

---

**Gerege Template Version 27.0** — Co-developed by the **Gerege Systems Development Team** and **Claude AI**, 2026.
