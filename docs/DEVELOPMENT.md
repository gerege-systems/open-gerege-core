# Development Guide

> 🌐 **English** · [Монгол](DEVELOPMENT_MN.md)

This guide helps developers set up and work with the **Gerege Backend Template
v27** codebase.

> **Origin.** Derived from the open-source
> [snykk/go-rest-boilerplate](https://github.com/snykk/go-rest-boilerplate)
> (MIT, by Najib Fikri), with the HTTP layer ported **Gin → Fiber v3** and the
> data layer **sqlx → GORM**. See [ARCHITECTURE.md](./ARCHITECTURE.md#credits--license)
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

Migrations are raw SQL files in `migrations/`, applied by the runner in
`internal/datasources/migration/`, followed by an idempotent GORM
`AutoMigrate` of the models in `internal/datasources/records/`.

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

3. **GORM Model + Repository Impl** — `internal/datasources/records/record.products.go`
   and `internal/datasources/repositories/postgres/products/`
   ```go
   // record.products.go
   type Products struct {
       Id        string         `gorm:"column:id;primaryKey"`
       Name      string         `gorm:"column:name"`
       Price     int64          `gorm:"column:price"`
       CreatedAt time.Time      `gorm:"column:created_at"`
       DeletedAt gorm.DeletedAt `gorm:"column:deleted_at;index"`
   }
   func (Products) TableName() string { return "products" }
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

6. **Handler** — `internal/http/handlers/v1/products/products.handler.go`
   ```go
   func (h Handler) Create(c fiber.Ctx) error {
       var req requests.CreateProductRequest
       if err := c.Bind().Body(&req); err != nil {
           return v1.NewErrorResponse(c, http.StatusBadRequest, "invalid request body")
       }
       if err := validators.ValidatePayloads(req); err != nil {
           return v1.RespondWithError(c, err)
       }
       // ... call h.usecase.Create(c.Context(), ...) ...
   }
   ```

7. **Route** — `internal/http/routes/route.products.go` (mirror `route.users.go`)
   ```go
   func (r *productsRoute) Routes() {
       v1 := r.router.Group("/v1")
       grp := v1.Group("/products")
       grp.Use(r.authMiddleware)
       grp.Post("/", r.handler.Create)
       grp.Get("/:id", r.handler.GetByID)
   }
   ```

8. **Wire Up** — in `cmd/api/server/server.go`, construct repo → usecase →
   route alongside the existing ones:
   ```go
   productRepo := productspostgres.NewProductRepository(conn)
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

#### Handler Tests (Fiber)

```go
func TestHandler_Create(t *testing.T) {
    app := fiber.New(fiber.Config{ErrorHandler: func(c fiber.Ctx, err error) error {
        return v1.RespondWithError(c, err)
    }})
    // ... register route with a mocked usecase ...
    resp, _ := app.Test(httptest.NewRequest(http.MethodPost, "/api/v1/products", body))
    assert.Equal(t, http.StatusCreated, resp.StatusCode)
}
```

#### Integration Tests (Repository Layer)

```go
//go:build integration

func TestProductRepository_Store(t *testing.T) {
    db := testenv.SetupPostgres(t)      // testcontainers — real Postgres
    repo := postgres.NewProductRepository(db)
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

Always pass `context.Context` first; in handlers read it via `c.Context()`:

```go
func (r *postgreUserRepository) GetByID(ctx context.Context, id string) (domain.User, error) {
    var rec records.Users
    err := r.conn.WithContext(ctx).Where(`"id" = ?`, id).First(&rec).Error
    // ...
}
```

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
func (h Handler) Login(c fiber.Ctx) error { /* ... */ }
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
