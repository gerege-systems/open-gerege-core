# 开发指南

> 🌐 **中文** · [English](DEVELOPMENT.md) · [Монгол](DEVELOPMENT_MN.md) · [Русский](DEVELOPMENT_RU.md)

本指南帮助开发者搭建并使用 **Gerege Template Platform V3.0**
（Цахим үйлчилгээг бүтээх суурь，即「构建数字服务的基础」）代码库 —
一套可直接投入生产的基础平台，公共部门或私营部门的任何数字服务都可在其上构建。
其旗舰参考部署是 **Gerege Template Platform**（template.gerege.mn），
一个基于 eID、构建在本技术栈之上的公共与私营服务平台。

> **溯源。** 派生自开源项目
> [snykk/go-rest-boilerplate](https://github.com/snykk/go-rest-boilerplate)
> （MIT，作者 Najib Fikri），其中 HTTP 层由 **Gin → chi (net/http)** 移植，
> 数据层由 **sqlx → pgx (pgxpool)** 移植。完整致谢见
> [ARCHITECTURE.md](./ARCHITECTURE.md#致谢与许可)。

## 前置条件

- Go 1.26+
- Docker 与 Docker Compose（仅用于集成测试 / 本地技术栈）
- PostgreSQL 15+（或使用 Docker）
- Make

## 快速开始

```bash
# 1. 复制环境文件（注意：它位于 internal/config/ 下）
cp internal/config/.env.example internal/config/.env
# 编辑 .env — JWT_SECRET 至少 32 个字符

# 2. 启动技术栈（Postgres + Redis + API）

# 3. 或在本地运行：先应用迁移，再启动服务
```

服务地址为 `http://localhost:8080`；Swagger UI 在
`http://localhost:8080/swagger/`。

## 开发命令

```bash
make build              # 构建 API 二进制
make tidy               # go mod tidy
make lint               # golangci-lint
make fmt                # 对所有文件执行 gofmt
make swag               # 从 godoc 注解重新生成 OpenAPI 规范（docs/）
make pre-push           # 在本地复现 CI：lint + 测试 + swag 漂移检查 + 构建
```

## 测试

```bash
make test               # 单元测试（仅 mock — 快速，无需 Docker）
make test-integration   # 集成测试（需要 Docker：Postgres + Redis）
                        # CI 也会实际运行（独立 job），部署会等待它们
make test-cover         # 带覆盖率报告的测试
```

## 数据库

### 迁移

```bash
```

迁移是位于 `backend/migrations/` 的原始 SQL 文件（`N_name.up.sql` +
`N_name.down.sql` 成对出现）。Go 包 `internal/datasources/migration/`
只包含**执行器**（不含 SQL）；CLI 入口是 `cmd/migration/main.go`
（`migrationsDir = "migrations"`）。要修改数据库结构，请在 `backend/migrations/`
中新增一个前向 SQL 迁移文件；执行器会幂等地应用它 —
文件按开头的编号排序，每个文件连同其 `schema_migrations` 记录在一个事务中提交，
整轮执行还持有会话级 advisory lock，使并发执行器串行化。**没有 ORM AutoMigrate** —
`internal/datasources/records/` 中的记录结构体只是由 pgx 扫描的普通结构体，
并非结构定义；数据库结构只来自 `*.up.sql` 文件。

## 代码组织

### 添加新功能

按分层由内向外推进。可参照现有的 `users` / `auth` 模块 — 后端在
`internal/business/usecases/` 下已附带约 18 个 usecase 切片（`ai`、`assets`、
`audit`、`auth`、`core`、`gateway`、`gov`、`gspace`、`integrations`、`org`、
`provider`、`rbac`、`security`、`sign`、`site`、`sso`、`superadmin`、`users`），
每个都遵循同样的模式。示例：新增一个 `Product` 资源。

1. **领域实体** — `internal/business/domain/domain.products.go`

   ```go
   package domain

   type Product struct {
       ID        string
       Name      string
       Price     int64
       CreatedAt time.Time
   }
   ```

2. **仓储接口** — 添加到 `internal/datasources/repositories/interface/interface.go`

   ```go
   type ProductRepository interface {
       Store(ctx context.Context, in *domain.Product) (domain.Product, error)
       GetByID(ctx context.Context, id string) (domain.Product, error)
   }
   ```

3. **记录结构体 + 仓储实现** — `internal/datasources/records/record_products.go`
   和 `internal/datasources/repositories/postgres/products/`

   记录是带 `db:"..."` 标签的**普通 Go 结构体**。`pgx.RowToStructByName`
   按名称把结果列映射到字段，软删除就是一个普通的可空 `*time.Time DeletedAt`
   （NULL → nil）— **没有 gorm 标签，没有 AutoMigrate**。

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

   仓储接收一个 `*pgxpool.Pool` 并执行手写 SQL —
   `INSERT ... RETURNING`，用 `pgx.CollectExactlyOneRow` +
   `pgx.RowToStructByName` 收集结果。`23505` 唯一性冲突转换为 `apperror.Conflict`；
   读取时显式添加 `deleted_at IS NULL` 谓词。

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

4. **Usecase 接口 + 实现** — `internal/business/usecases/products/`

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

6. **Handler** — `internal/http/handlers/v1/products/products_handler.go`

   Handler 的签名为 `func(w http.ResponseWriter, r *http.Request) error`，
   在路由注册时由 `v1.Wrap` 包装（它把返回的错误转换为 JSON 信封）。
   用 `v1.DecodeBody` 解码请求体，用 `r.Context()` 读取 context，
   并通过 `v1.NewSuccessResponse` / `v1.RespondWithError` 返回。

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

7. **路由** — `internal/http/routes/route_products.go`（照搬 `route_users.go`）

   路由使用 chi router；每个 handler 都用 `v1.Wrap` 包装。
   路径参数用 `chi.URLParam(r, "id")` 读取。

   ```go
   func (rt *productsRoute) Routes() {
       rt.router.Route("/v1/products", func(r chi.Router) {
           r.Use(rt.authMiddleware)
           r.Post("/", v1.Wrap(rt.handler.Create))
           r.Get("/{id}", v1.Wrap(rt.handler.GetByID))
       })
   }
   ```

8. **接线** — 在 `cmd/api/server/server.go` 中，与现有代码并列构造
   repo → usecase → route：

   ```go
   productRepo := productspostgres.NewProductRepository(pool)
   productsUC := products.NewUsecase(productRepo)
   routes.NewProductsRoute(api, productsUC, authMiddleware).Routes()
   ```

9. **行级安全（按用户 / 按租户的表）** — 如果新表存放的是属于特定公民的数据
   （而非公共参考目录），它**必须**配备 RLS 策略。请遵循
   `migrations/14_organizations.up.sql`、`migrations/20_gov_services.up.sql`
   和 `migrations/21_user_integrations.up.sql` 中已确立的模式：
   `ALTER TABLE … ENABLE ROW LEVEL SECURITY` **且** `FORCE ROW LEVEL SECURITY`，
   然后是以 `app.user_id` / `app.user_role` 会话 GUC 为键的
   `service` / `admin` / `self` 三条策略。
   仓储必须**感知 RLS** — 开启一个 `withRLS` 事务，从请求身份发出
   `SET LOCAL app.user_id` / `SET LOCAL app.user_role`
   （`internal/datasources/rls` 通过 context 携带它；完整示例见
   `repositories/postgres/org` / `repositories/postgres/gov`）。
   无身份的请求会设置空 GUC，于是所有策略拒绝所有行（fail-closed）。
   只有当 api 以非超级用户数据库角色连接时 RLS 才会生效 —
   启动守卫会在生产中阻止超级用户 / `BYPASSRLS` 连接（参见 [SECURITY.md](SECURITY.md)）。
   公共参考表（例如 `gov_services` 目录）不启用 RLS，改由表级授权保护。

### 编写测试

#### 单元测试（Usecase 层）

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

#### Handler 测试（net/http）

用 `net/http/httptest` 驱动 chi router（或被 `v1.Wrap` 包装的 handler）—
`httptest.NewRequest` 构建请求，`httptest.NewRecorder` 捕获响应。不使用 Fiber 测试应用。

```go
func TestHandler_Create(t *testing.T) {
    // ... 用 mock 的 usecase 构建 router ...
    req := httptest.NewRequest(http.MethodPost, "/api/v1/products", strings.NewReader(body))
    rec := httptest.NewRecorder()
    router.ServeHTTP(rec, req)
    require.Equal(t, http.StatusCreated, rec.Code)
}
```

#### 集成测试（仓储层）

```go
//go:build integration

func TestProductRepository_Store(t *testing.T) {
    pool := testenv.SetupPostgres(t)    // testcontainers — 真实 Postgres（pgxpool）
    repo := postgres.NewProductRepository(pool)
    got, err := repo.Store(context.Background(), &domain.Product{Name: "X", Price: 100})
    assert.NoError(t, err)
    assert.NotEmpty(t, got.ID)
}
```

### 生成 Mock

```bash
# 为某个接口生成 mock
make mock interface=ProductRepository \
          dir=internal/datasources/repositories/interface \
          filename=mock.repository_products.go
```

## 代码风格

### 命名约定

| 类型 | 约定 | 示例 |
|-------------|--------------|--------------------|
| 包 | 全小写 | `repository` |
| 接口 | 大驼峰 | `UserRepository` |
| 结构体 | 大驼峰 | `Handler` |
| 函数 | 大驼峰 | `GetByID` |
| 变量 | 小驼峰 | `userCount` |
| 常量 | 大驼峰 / 哨兵值 | `RoleAdmin`、`ErrEmptyEmail` |
| JSON 字段 | 蛇形命名 | `request_id` |

### 错误处理

返回类型化的领域错误（`internal/apperror`）— 绝不 panic，
绝不把库层错误泄漏给客户端：

```go
user, err := s.repo.GetByID(ctx, id)
if err != nil {
    return domain.User{}, err   // apperror.NotFound 会呈现为 404
}
```

`RespondWithError`（位于 `handler_base_response.go`）把错误类型映射为状态码，
记录 5xx 的原因，并渲染干净的信封。信封相关的辅助函数都在该文件中：
`v1.DecodeBody`（限制大小、拒绝未知字段的 JSON 解码）、
`validators.ValidatePayloads`（struct 标签校验 → 422，带逐字段细节）、
`v1.NewSuccessResponse`、`v1.NewErrorResponse` 和 `v1.RespondWithError`。

### Context 使用

始终把 `context.Context` 作为第一个参数传递；在 handler 中通过 `r.Context()`
读取，并贯穿传递给每一次 pgx 调用：

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

## 扩展 AI 助手

> 深入阅读：[AI_PIPELINE.md](AI_PIPELINE.md) — 流程、提示词分层、语音、故障排查。

Gemini 流水线（`internal/business/usecases/ai`）设计为可按项目扩展：

- **添加工具** — 实现一个 `ai.ToolDef`（Gemini 函数声明 + 一个 Go `Execute` 函数），
  并把它追加到 `cmd/api/server/server.go` 的工具列表中。由模型决定何时调用它；
  后端以请求 context 执行它（因此任何数据库访问都受 RLS 约束）。
  `KnowledgeSearchTool`（检索 `ai_knowledge`）和 `get_server_time` 是随附示例。
- **改变助手协助的范围** — 在运行时编辑 `scope` 提示词层
  （管理 → 设置，或 `PUT /admin/ai/prompts/scope`）。基础防护层
  （语言、范围约束、抗提示词注入）硬编码在 `ai_prompts.go` 中，且应保持如此。
- **扩充知识库** — 向 `ai_knowledge` 插入数据行（title/content/tags）。
  `repositories/postgres/ai` 中的 ILIKE 检索只是一条查询 —
  当语料库增大时，可换成 tsvector 或 pgvector。
- **模型** — 聊天/STT/翻译使用 `GEMINI_MODEL`；TTS 使用 `GEMINI_TTS_MODEL`
  （单独的、具备音频能力的模型）。二者都只能通过环境变量配置。

## API 文档

### Swagger 注解

Handler 携带由 `swag` 消费的 godoc 注解：

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

### 重新生成文档

```bash
make swag
```

Swagger UI：`http://localhost:8080/swagger/`。如果 `docs/` 与注解发生漂移，
CI 会失败（`make ci-swag-check`）。

## 故障排查

**数据库连接失败**

```bash
docker-compose ps                 # Postgres 起来了吗？
# 检查 internal/config/.env 中的 DB_POSTGRE_DSN
```

**迁移失败** — 检查 `migrations/` 的顺序和 `schema_migrations` 表；
执行器使用 advisory lock + 每文件一个事务。

**测试失败**

```bash
go test -v ./...                  # 详细输出
go test -v -run TestUsecase_Create ./internal/business/usecases/products/...
```

**Lint 报错**

```bash
golangci-lint run --fix
```

## 安全检查清单

部署之前，请确认：

- [ ] 所有受保护端点都带有认证中间件
- [ ] 匿名端点（`/auth/*`）保留限流器 + 请求体上限
- [ ] `JWT_SECRET` 为 ≥ 32 个随机字符，且不是示例值
- [ ] 输入校验（`validate:` 标签）覆盖每个请求 DTO
- [ ] 密钥来自环境变量，绝不提交入库
- [ ] 生产环境已设置 `ALLOWED_ORIGINS`（无通配符）
- [ ] 在边缘 / 负载均衡器上强制 HTTPS

---

**Gerege Template Platform V3.0** — 由 **Gerege Systems 开发团队**与 **Claude AI** 共同打造，2026。
