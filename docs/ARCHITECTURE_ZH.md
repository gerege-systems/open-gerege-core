# 架构总览

> 🌐 **中文** · [English](ARCHITECTURE.md) · [Монгол](ARCHITECTURE_MN.md) · [Русский](ARCHITECTURE_RU.md)

本文描述 **Gerege Template Platform V3.0**（Цахим үйлчилгээг бүтээх суурь，
即「构建数字服务的基础」）的整体架构 — 一套可直接投入生产的基础平台，
公共部门或私营部门的任何数字服务都可在其上构建。其旗舰参考部署是
**Gerege Template Platform**（位于 **template.gerege.mn**），一个**基于 eID 的
政务服务平台** — 同时也是 Gerege SSO 的依赖方。后端模块名为 `template`；
技术栈为 **chi (net/http) + pgx (pgxpool) + PostgreSQL + Redis + Gemini AI**，
按整洁架构分层组织，前面由 Next.js BFF 承接。

在该参考部署中，平台既是 **eID 依赖方**（用户用 eID 登录），
又是 **OIDC 身份提供方**（其他依赖方应用*通过*它、经由内置 Go provider 登录）。
PostgreSQL 中的行级安全是承重的按用户隔离边界 — 参见
[行级安全（RLS）](#行级安全rls)。

> **溯源。** 整洁架构分层、pgx 数据层、缓存、可观测性与测试策略源自 Najib Fikri
> 的开源项目 [snykk/go-rest-boilerplate](https://github.com/snykk/go-rest-boilerplate)（MIT）。
> 认证体系、RLS 安全模型、eID/SSO/OIDC 提供方集成以及下文的各功能模块
> 则是为本平台专门构建的。作为 MIT 衍生作品，上游版权声明被保留 — 参见
> [致谢与许可](#致谢与许可)。

## 分层示意

```
┌─────────────────────────────────────────────────────────────────┐
│                        HTTP 层                                    │
│  cmd/api/server → 中间件 → internal/http/handlers/v1              │
│  internal/http/{routes, datatransfers, middlewares, auth}         │
│  + internal/provider/{adminapi, adminkeys, devapps, signrelay}    │
├─────────────────────────────────────────────────────────────────┤
│                       Usecase 层                                  │
│  internal/business/usecases/*  （19 个限界上下文）                 │
│  （业务逻辑、校验、编排）                                          │
├─────────────────────────────────────────────────────────────────┤
│                     Repository 层                                 │
│  internal/datasources/repositories/{interface, postgres}          │
│  （pgx 手写 SQL、RLS 事务、软删除、缓存）                          │
├─────────────────────────────────────────────────────────────────┤
│                       Domain 层                                   │
│  internal/business/domain                                         │
│  （实体、值对象、业务规则）                                        │
└─────────────────────────────────────────────────────────────────┘
```

## 功能模块（限界上下文）

平台由 `internal/business/usecases/` 下的 **19 个 usecase 模块**组成，
每个模块都是接口 + 实现，并在组合根中手工接线。除样板核心
（`auth`、`users`、`rbac`、`ai`）之外，平台还增加了 eID/SSO/服务交付相关的能力面：

| 模块 | 职责 |
|----------------|----------------|
| `auth`         | **eID 登录**（二维码 / 移动端 deep-link / 按登记号推送 + long-poll）、**Google OAuth** 账户绑定、会话刷新/登出。无密码。 |
| `users`        | 被 auth、admin、sign、superadmin 复用的用户读写；登录锁定；改密令牌截止。 |
| `rbac`         | 动态角色 + 权限目录，以及 RBAC 中间件使用的权限解析器。 |
| `ai`           | Gemini 流水线 — function-calling 聊天、STT/TTS、实时翻译、分层提示词、服务端工具 + 知识库。 |
| `org`          | 组织 + 成员关系（与 eID 关联；**RLS**）。 |
| `gov`          | 面向公民的「政务服务」门户 — 申请、证明、通知、缴费、预约（按用户，**RLS**），构建在公共服务目录之上。 |
| `gateway`      | API 网关 — 服务 / 路由 / 策略 + 遥测（每个服务携带一个 OAuth `scope`）。 |
| `applications` | 统一的 OAuth2 **客户端注册表**（RP + m2m），由 **Ory Hydra** 支撑 — 合并了旧的网关 consumer/API key 与 SSO RP 注册；按服务的访问权限 = OAuth scope（`application_services` → `gateway_services.scope`）。由管理员管理（`gateway.manage`），依赖 Hydra。 |
| `core`         | Gerege Core（`core.gerege.mn`）USER FIND / ORG FIND 查询封装。 |
| `provider`     | **OIDC 提供方** — 位于 **Ory Hydra** 之前的登录/授权/登出核心；dan 自身即为 SSO IdP。 |
| `integrations` | 用户的第三方 OAuth（Google Drive/Meet、Dropbox）；令牌以 **AES-256-GCM 加密**存储（**RLS**）。 |
| `assets`       | 个人签名图片 + 组织印章（图片存 Google Drive，URL 存数据库）。 |
| `gspace`       | Gerege Space — 应用自有的 SFTP 存储，按用户配额（默认 2 MB）。 |
| `audit`        | 持久化的**哈希链式、仅追加**审计日志（管理员读取 API）。 |
| `superadmin`   | 管理管理员用户（创建 / 授予 / 撤销）；每次变更都写入审计日志。 |
| `security`     | 安全事件接收（已认证用户写入，管理员读取）。 |
| `site`         | 站点级外观默认值（强调色 / 字体 / 密度 / 主题）。 |
| `sign`         | 通过 eidmongolia `/v3` 进行 PDF 签署（**PAdES**），使用服务端持有的 Document-Signer 证书。 |

## 目录结构

```
.
├── cmd/
│   ├── api/
│   │   ├── main.go                 # 入口（配置 + 日志初始化）
│   │   └── server/server.go        # 组合根（手动 DI）— 所有挂载点都在这里阅读
│   ├── migration/                  # 迁移 CLI（仅 SQL；无 ORM/AutoMigrate）
│   └── seed/                       # 数据库播种 CLI
├── docs/                           # EN/MN 文档 + OpenAPI 规范（swagger.json/yaml、docs.go）
├── internal/
│   ├── apperror/                   # 类型化领域错误（→ HTTP 状态码）
│   ├── business/
│   │   ├── domain/                 # 企业实体（最内圈）
│   │   └── usecases/               # 19 个限界上下文（接口 + 实现）
│   ├── config/                     # 基于 Viper 的配置 + .env.example
│   ├── constants/                  # 环境、日志、错误、端点常量
│   ├── datasources/
│   │   ├── caches/                 # Redis + Ristretto 两级缓存
│   │   ├── drivers/                # pgx (pgxpool) 连接 + RLS 可执行性启动守卫
│   │   ├── migration/              # SQL 迁移执行器
│   │   ├── records/                # pgx 记录结构体 + 记录↔领域映射
│   │   ├── rls/                    # 通过 context.Context 携带的按请求 RLS 身份
│   │   └── repositories/
│   │       ├── interface/          # 网关抽象（包名 _interface）
│   │       └── postgres/*          # pgx 实现（手写 SQL、withRLS）
│   ├── http/
│   │   ├── auth/                   # 从请求 context 取 CurrentUser
│   │   ├── datatransfers/          # 请求 / 响应 DTO
│   │   ├── handlers/v1/            # HTTP handler（按模块）
│   │   ├── middlewares/            # 全局 + 分组中间件
│   │   └── routes/                 # 路由注册（按模块）
│   └── provider/                   # OIDC 提供方的运维能力面：
│       ├── adminapi/               #   /admin RP OAuth2 客户端管理
│       ├── adminkeys/ devapps/     #   管理 API key + 开发者应用存储
│       └── signrelay/              #   面向下游 RP 的 /rp/sign 中继
├── migrations/                     # 编号 SQL 迁移（N_name.up.sql + .down.sql）
├── pkg/                            # 与框架无关的客户端与工具（15 个包）
│   ├── eid/ google/ hydra/         # 身份：eID RP、Google OAuth、Hydra admin
│   ├── xyp/ gspace/ verify/        # XYP 组织登记、SFTP 存储、GeregeCloud Verify OTP
│   ├── gemini/                     # 免 SDK 的 Gemini REST（function calling、音频、PCM→WAV）
│   ├── jwt/ logger/ clock/         # JWT、Zap 日志、时间抽象
│   ├── helpers/ validators/        # 工具 + struct-tag 载荷校验
│   ├── audit/                      # 认证事件审计辅助
│   └── observability/              # OTel 追踪 + Prometheus 指标配置
└── internal/test/                  # mock、fixture、testcontainers 测试脚手架
```

## 依赖流向

依赖只能向内流动（整洁架构原则）：

```
HTTP → Usecase → Repository → Domain
  │        │          │
  ▼        ▼          ▼
 DTO     接口      pgx/SQL
```

- **HTTP 层**依赖 **Usecase** 接口（`auth.Usecase`、`users.Usecase` …）。
- **Usecase 层**依赖 **Repository** 接口（`_interface.UserRepository` …），
  绝不依赖 postgres 适配器。
- **Repository 层**依赖 **Domain** 实体。
- **Domain 层**只 import 标准库 + `golang.org/x/crypto/bcrypt` —
  绝不 import `internal/` 或 `pkg/`。

这一点在结构上是可验证的：`internal/business/**` 与
`internal/datasources/repositories/**` **不** import 任何 chi/net-http Web 包，
因此可以在不触碰业务代码的情况下更换交付框架。对「领域层不 import 任何内部包」
有一处刻意的例外：叶子包 `internal/datasources/rls` 只依赖标准库 `context`，
并被三层共享，用于在不产生 import 循环的前提下携带按请求的 RLS 身份。

## 关键组件

### 1. HTTP 层

**组合根：** `cmd/api/server/server.go` — 唯一的手动 DI 接线点。
通读它即可看到每一个挂载点。它负责：

- 初始化链路追踪、pgx 连接池（含 RLS 启动守卫）、Redis/Ristretto、JWT 服务，
  以及每一个外部客户端（eID、Google、XYP、OIDC/Hydra、Gemini、GeregeCloud Verify、
  Gerege Space、Gerege Core）。
- 手工把 repository → usecase → route 接线（无全局单例，无 DI 容器）。
- 构建 chi 路由，安装全局中间件栈，并把各路由模块挂载到 `/api/v1` 下。
- 仅当相关配置存在时，才有条件地挂载 OIDC 提供方能力面（`/admin`、`/rp/sign`）。
- 负责优雅停机（排空 HTTP、限流器、pgx 连接池、Redis、tracer）。

**路由：** `internal/http/routes/` — 每个模块一个文件（`route_auth.go`、
`route_gov.go`、`route_provider.go` …）。各自把 `/v1/<module>` 挂载在 `/api` 下。

**Handler：** `internal/http/handlers/v1/` — 每个模块一个包。Handler 签名为
`func(w http.ResponseWriter, r *http.Request) error`，由 `v1.Wrap` 包装；
请求体用 `v1.DecodeBody` 解码，DTO 由 `validators.ValidatePayloads` 校验，
响应通过 `v1.NewSuccessResponse` / `v1.RespondWithError` 返回。Handler 带有 swagger 注解。

### 2. 中间件栈

全局中间件在 `server.go` 中按此顺序应用（顺序很重要 — 追踪必须最先，
以便在 request-ID 日志之前就已存在 span/`trace_id`；Recoverer 紧随 Request ID，
以便捕获下游 panic 且恢复响应中带有 `request_id`）：

1. **Tracing**（`TracingMiddleware`）— 每请求一个 OTel span。
2. **Request ID**（`RequestIDMiddleware`）— 生成 / 传播 `X-Request-ID` 到 context + logger。
3. **Recoverer**（`RecovererMiddleware`）— 捕获下游 panic，返回干净的 500。
4. **Metrics**（`MetricsMiddleware`）— Prometheus 请求计数 + 延迟。
5. **安全响应头**（`SecurityHeadersMiddleware`）— HSTS、CSP、nosniff、frame options、referrer policy。
6. **CORS**（`CORSMiddleware`）— 来源取自 `ALLOWED_ORIGINS`（通配符仅限开发环境）。
7. **请求体大小限制**（`BodySizeLimitMiddleware`）— 全局上限（各路由另有更严格的限制）。
8. **访问日志**（`AccessLogMiddleware`）— 结构化的单行访问日志。
9. **超时**（`TimeoutMiddleware`）— 按请求的截止时间（服务器 `WriteTimeout` 设置得更长，
   以便它能先触发）。

**分组 / 单路由中间件：**

- **Auth**（`NewAuthMiddleware`）— 校验 JWT bearer 令牌，把 `CurrentUser` 放入 context，
  并**设置 RLS 身份**：管理员用 `rls.WithAdmin`，其余用 `rls.WithUser`（`middleware_auth.go`）。
- **服务 RLS 上下文**（`ServiceRLSContext`）— 安装在匿名 `/auth` 分组上，
  使认证前的流程（eID upsert、refresh 身份查询）以受信的 `service` RLS 角色运行
  （`middleware_rls.go`）。
- **RBAC**（`RequirePermission`、`RequireAdmin`、`RequireSuperAdmin`）— 认证之后的声明式授权；
  管理员绕过权限检查，`RequireSuperAdmin` 把守 `/superadmin` 能力面。解析器出错时 fail-closed。
- **可观测性门禁**（`ObservabilityGate`）— 守护 `/metrics` 与 `/swagger/doc.json`
  （参见[运维端点](#运维端点)）。
- **限流器** — 四个独立限流器：`/auth` 约 5 次/分钟、`/ai` 约 20 次/分钟
  （突发 10，用于翻译流）、`/eid/poll` 约 60 次/分钟（突发 30，用于 long-poll），
  以及 gov/assets/gspace/eID 档案的**写操作** 约 30 次/分钟（突发 15）。

`clientIP()`（`middleware_clientip.go`）是一个辅助函数 — 而非全局中间件 —
用于为限流和审计解析客户端 IP，且仅信任来自 `TRUSTED_PROXIES` 的
`X-Forwarded-For`（安全兜底：默认不信任）。

### 3. Usecase 层

**位置：** `internal/business/usecases/` — 每个限界上下文暴露一个接口 + 一个实现。
职责：业务规则校验、编排 repository + 缓存 + 外部客户端，并返回 `apperror.*` 值
（用 `apperror.InternalCause` 包装内部原因，使库层错误绝不到达客户端）。
Usecase 只依赖 `repositories/interface`，绝不依赖 postgres 适配器。

### 4. Repository 层

**位置：** `internal/datasources/repositories/` — `interface/` 包
（包名为 `_interface`，因为 `interface` 是关键字）存放网关抽象；
`postgres/*` 用 pgx 与手写 SQL 实现它们。关键特性：

- 查询直接接收 `ctx`；行用 `pgx.RowToStructByName` 扫描。
- 通过显式的 `deleted_at IS NULL` 谓词实现软删除。
- `Store` 使用单次往返的 `INSERT … RETURNING`。
- 通过 pgconn 错误码 `23505` 检测重复键 → `apperror.Conflict`。
- 按用户划分的仓储会把每条查询放在 **`withRLS` 事务**中执行，
  该事务以 `SET LOCAL` 作用域发布请求身份（参见[行级安全（RLS）](#行级安全rls)）。

### 5. Domain 层

**位置：** `internal/business/domain/` — 实体承载业务规则，不依赖任何内部包。
`domain_users.go` 定义角色模型和 eID 用户构造函数
（`NewEIDUser` — 无密码、`Active=true`、以 `civil_id` 为键）。
角色常量参见[授权](#授权)。

## 身份认证

平台签发 **JWT access + refresh 令牌**（`pkg/jwt`），但**没有密码登录、
没有邮箱/OTP 注册，也没有密码重置**。身份只来自外部提供方。端点形态记录在
[API_CONTRACT.md](API_CONTRACT.md)；路由在
`internal/http/routes/route_auth.go` 与 `route_eidprofile.go` 中注册。

**1. 使用 eID 登录（主要方式）。** 应用是 eID Mongolia 的依赖方
（`pkg/eid`、`EID_*` 配置）：

- `POST /api/v1/auth/eid/start` 开启会话并返回二维码 / 移动端 deep-link。
- `POST /api/v1/auth/eid/start-id` 按登记号（реестр）发起，向公民已登记的设备推送。
- `POST /api/v1/auth/eid/poll` 由前端**长轮询**（约每 2.5 秒一次；IdP 每次最多挂起 25 秒），
  直到 eID 会话进入 `COMPLETE`。完成时会 upsert 用户（以 `civil_id` 为键；
  公共 RP 收到的是 `civil_id` 而非 `national_id`）并签发一对令牌。

**2. Google OAuth 账户绑定**（`pkg/google`、`GOOGLE_*`）：
`POST /api/v1/auth/google` 交换 code，并绑定（或据此登录）附着在 eID 用户上的
Google 账户；`DELETE /api/v1/auth/google/link` 解绑。

**会话生命周期**（与登录方式无关）：

- `POST /api/v1/auth/refresh` 轮换令牌对；在凭据变更截止时间之前签发的令牌会被拒绝
  （`User.TokensRevokedBefore`）。`kind` claim 守卫可防止把 refresh 令牌当作 access 令牌使用。
- `POST /api/v1/auth/logout` 吊销 refresh 令牌。

> **注意。** `auth_login.go`、`auth_register.go`、`auth_send_otp.go`、
> `auth_forgot_password.go`、`auth_reset_password.go` 等 handler 文件仍存在于代码树中，
> 但**未接入任何路由** — `route_auth.go` 只注册上述 eID / Google / refresh / logout 端点。

## 授权

授权在两层强制执行：HTTP 边缘的 **JWT 角色/权限**，以及数据库层的 **RLS**。

**角色模型**（`domain_users.go`；迁移 `23_superadmin_role`）— 四个有序角色，`1` = 最高：

```go
RoleSuperAdmin = 1  // 管理管理员用户；由 RequireSuperAdmin 把守
RoleAdmin      = 2  // 完全访问权限；IsAdmin() 为 true
RoleManager    = 3
RoleUser        = 4  // 新 eID 用户的默认角色
```

`IsAdmin()` 对 `RoleAdmin` **和** `RoleSuperAdmin` 都返回 true
（超级管理员继承管理员的 JWT/RLS/权限路径）；`IsSuperAdmin()` 只对
`RoleSuperAdmin` 为 true。角色 ID `0` 是遗留的无 claim 令牌的哨兵值，
会被 RBAC 中间件降级为 `RoleUser`。

**动态 RBAC** — 在粗粒度角色等级之外，`rbac.Usecase` 从数据库解析角色的权限集
（迁移 `8_rbac_roles_permissions`）。`RequirePermission(resolver, perm)`
按具名权限把守路由；管理员绕过。超级管理员由 `SUPERADMIN_EMAIL`（或直接在数据库中）
引导创建，绝不通过 API 创建。

## 行级安全（RLS）

RLS 是平台承重的按用户隔离边界 — 位于仓储层已写的 `WHERE user_id = …`
条件之下的纵深防御。它确保即使查询存在缺陷，也无法返回他人的数据行。

**Context 上的身份**（`internal/datasources/rls/rls.go`）— 一个叶子包
（只依赖标准库 `context`）携带 `Identity{ UserID, Role }`，其中 `Role`
是三个字符串常量之一，且**必须**与 SQL 策略中的字面量一致：

- `service` — 受信的认证前 / 系统流程（eID upsert、refresh 身份查询、引导初始化）。
  由 `/auth` 上的 `ServiceRLSContext` 设置；完全访问权限。
- `admin` — 对每一行都有完全访问权限。由认证中间件针对管理员 JWT 通过 `rls.WithAdmin` 设置。
- `user` — 只能访问调用者自己的行。由认证中间件通过 `rls.WithUser` 设置。

**发布身份**（`…/postgres/users/users_postgres.go`，以及 `org`、`gov`、
`security`、`userintegrations` 中的对应实现）— `withRLS(ctx, fn)`
辅助函数把每条查询包在一个事务中并执行：

```go
SELECT set_config('app.user_id',   $1, true),   -- is_local = true ⇒ SET LOCAL 语义
       set_config('app.user_role',  $2, true)
```

`set_config(..., true)` 把值的作用域限定在事务内，因此身份不会跨连接池连接泄漏。
当 context **不携带**身份时，两个 GUC 都为空 — 空的 `app.user_role` 不匹配任何策略，
于是所有行被隐藏、所有写操作被拒绝（**fail-closed**）。`audit` 仓储使用只含角色的变体。

**按表策略** — 每张启用 RLS 的表都使用 `ENABLE` **且** `FORCE ROW LEVEL SECURITY`
（FORCE 使 RLS 对表属主也生效）。策略为宽松型（以 OR 组合），并识别同样的三种 GUC 角色。
`user` 策略以 `user_id = NULLIF(current_setting('app.user_id', true), '')::uuid` 把守
（`NULLIF` 把空 GUC 转成 `NULL`，使类型转换绝不报错，该行只是被排除）：

| 迁移 | 表 | RLS |
|-----------|----------|-----|
| `7_enable_rls_users`      | `users`                                                                     | ENABLE + FORCE；service / admin / 本人 |
| `14_organizations`        | `organizations`、`organization_memberships`                                 | ENABLE + FORCE；按**成员关系**可见 |
| `17_org_rls_recursion_fix`| （重建组织策略）                                                             | 使用 `SECURITY DEFINER` 的 `app_is_org_member()` 打破策略递归（SQLSTATE 42P17） |
| `20_gov_services`         | `gov_applications`、`gov_references`、`gov_notifications`、`gov_payments`、`gov_appointments` | ENABLE + FORCE；service / admin / 本人。（`gov_services` 目录为公开表，无 RLS） |
| `21_user_integrations`    | `user_integrations`                                                         | ENABLE + FORCE；service / admin / 本人 |

全局配置表刻意**不**受 RLS 保护；其数据库层兜底是针对 `app_user` 角色的表权限 `REVOKE`
（`17_least_privilege_config_grants` 针对 `permissions` / `role_permissions` /
`ai_prompts` / `ai_knowledge`；`27_site_appearance` 针对单例外观记录）。
提供方相关表（`26_sso_provider`：`developer_apps`、`admin_api_keys`、`login_events`）
以及 `org_stamps`（`25`）同样不启用 RLS，改在 usecase/handler 层把守。

**启动时的可执行性守卫** — RLS 会被 Postgres 超级用户和 `BYPASSRLS` 角色静默绕过，
因此 `guardRLSEnforceable`（`internal/datasources/drivers/driver_pgx.go`）
在启动时针对连接角色检查 `pg_roles`：

- 若角色具有 `rolsuper` 或 `rolbypassrls`：**生产环境 fail closed**
  （中止启动，关闭连接池）；**开发环境记录告警**并继续
  （migrate/测试可能以超级用户运行）。
- 因此在生产中 api 必须以最小权限的非超级用户角色（例如 `app_user`）连接。
  （compose 技术栈刻意以 `ENVIRONMENT=development` 运行，因此该守卫只在生产中硬失败。）

## OIDC 提供方（Ory Hydra）

平台自身可以充当**身份提供方**：其他依赖方应用通过 **Ory Hydra** 把登录委托给 dan。
只有当 `ProviderConfigured()` 为 true 时该能力面才会激活
（`HYDRA_ADMIN_URL` + `HYDRA_PUBLIC_URL` + `SSO_STATE_KEY ≥ 32 字节`）；
否则它处于惰性状态，其路由永不注册。

- **登录 / 授权 / 登出核心** — `usecases/provider` + `pkg/hydra` 处理 Hydra 的
  challenge；第一方客户端（`SSO_FIRSTPARTY_CLIENTS`）跳过 consent 界面。
  挂载在 `/api/v1/provider` 下。
- **Applications（统一客户端注册表）** — `usecases/applications`
  （挂载在 `/api/v1/applications`，由 `gateway.manage` 把守）是当前注册 OAuth2
  客户端的方式：RP「Login with DAN」应用（`web`/`spa`/`native` → `authorization_code`；
  `spa`/`native` 为公开客户端，使用 PKCE，无 secret）以及 m2m 客户端
  （`client_credentials`）。每个都是一个 Hydra OAuth2 客户端，其 scope 即允许的网关服务
  （`application_services` → `gateway_services.scope`）；机密型客户端的 `client_secret`
  只在创建/轮换时展示一次。
- **运维能力面（遗留）** — `internal/provider/adminapi` 挂载在 **`/admin`**
  （通过 `http.StripPrefix`），用于 RP OAuth2 客户端的注册/管理，
  由 `devapps`（`developer_apps`）存储和 `adminkeys`（来自 `SSO_ADMIN_API_KEYS`
  的引导密钥，按 SHA-256 匹配）支撑。这套 admin-API-key 运维面与 `developer_apps`
  覆盖层仍然存在，但**新工作应改用统一的 Applications 模型**。
- **签署中继** — `internal/provider/signrelay` 挂载在 **`/rp/sign/*`**，
  是一个反向代理，让下游 RP 能*通过* dan、使用 dan 的 eidmongolia RP 凭据完成
  eID PDF 签署（由 `SIGN_RELAY_TOKEN` + `EID_RP_SECRET` 启用）。

> **强制执行的注意事项。** 为应用分配服务只是设置该客户端的 OAuth **scope** —
> 这仅属于注册/配置。*运行时*的按请求强制执行需要一个网关代理，
> 对呈递的令牌做内省（`hydra.Admin.Introspect` 已存在）并与各路由的服务 scope 比对，
> 而该代理**尚不存在**。因此当下的服务分配并不是生效中的授权 —
> 请勿把它误当作已强制执行的授权。

## 数据库

- **驱动：** pgx v5（`github.com/jackc/pgx/v5` + pgxpool），手写 SQL — **无 ORM**。
- **数据库：** PostgreSQL，以**行级安全**作为按用户的边界。
- **迁移：** `migrations/` 中的编号 SQL 文件（`N_name.up.sql` + `.down.sql`），
  由 `migrate` compose 服务 / `cmd/migration` 应用。**没有 AutoMigrate** —
  数据库结构只来自 `*.up.sql` 文件（`cmd/migration/main.go`）。
- **追踪：** 通过 pgx 连接池插桩（`otelpgx`）接入 OpenTelemetry。

> **迁移编号冲突。** 有两个迁移共用前缀 `17_`：
> `17_least_privilege_config_grants` 与 `17_org_rls_recursion_fix`。
> 二者互相独立且都会被应用；执行器按编号排序文件，因此在添加 `18_` 及以上迁移
> 或推理应用顺序时请留意这一点。

### 连接管理

连接池按环境变量配置（`internal/datasources/drivers/driver_pgx.go`，
`SetupPgxPostgres`）：

```go
poolCfg.MaxConns        = cfg.MaxConns    // DB_MAX_OPEN_CONNS   （默认 25）
poolCfg.MinConns        = cfg.MinConns    // DB_MAX_IDLE_CONNS   （默认 5）
poolCfg.MaxConnLifetime = cfg.MaxLifetime // DB_CONN_MAX_LIFE_MINS（默认 15）
```

生产环境要求 TLS 校验过的 DSN（`sslmode=verify-full` 或 `verify-ca`）—
由配置守卫强制执行。

## 可观测性

### 日志

- **库：** Zap（结构化），经由 `pkg/logger`。生产环境输出 JSON，开发环境输出控制台格式。
  Request ID + trace ID 通过 `*WithContext` 辅助函数传播。

### 指标

- **库：** Prometheus，端点 `GET /metrics`（受门禁 — 参见[运维端点](#运维端点)）。
  包含 HTTP 请求计数/延迟、各层缓存命中/未命中/错误、OTP 发送结果，以及实时 pgx 连接池统计。

### 追踪

- **库：** OpenTelemetry；导出器由 `OTEL_EXPORTER` 选择（留空 = noop、`stdout` 或 `otlp`），
  采样率由 `OTEL_SAMPLE_RATIO` 控制。

## 运维端点

| 端点 | 访问权限 |
|----------|--------|
| `GET /health` | 公开 — 存活探针（供负载均衡器 / 编排系统使用）。 |
| `GET /ready`  | 公开 — 就绪探针：数据库 ping（pgx 连接池）+ Redis 探测。 |
| `GET /metrics` | 由 `ObservabilityGate` **把守**。 |
| `GET /swagger/doc.json` | 由 `ObservabilityGate` **把守**。 |

`ObservabilityGate`（`middleware_observability_gate.go`）保护这两个运维敏感端点：
在**开发环境**中它们始终开放；在**生产环境**中它们要求
`Authorization: Bearer <OBSERVABILITY_TOKEN>`（常数时间比较），
并在任何不匹配、或 `OBSERVABILITY_TOKEN` 未设置时返回 **404** — 而非 401，
从而让它们的存在本身也不会暴露给侦察行为。

## 安全特性

| 特性 | 实现 | 位置 |
|-------------------|-----------------------------------------|--------------------------------------------|
| 行级安全 | 按用户的数据库隔离 + 启动守卫 | `datasources/rls/`、`drivers/driver_pgx.go`、迁移 `7/14/20/21` |
| 认证（身份） | eID RP + Google OAuth | `usecases/auth`、`pkg/{eid,google}` |
| 授权 | 四角色模型 + 动态 RBAC | `domain_users.go`、`middlewares/middleware_rbac.go` |
| 安全响应头 | HSTS、CSP、nosniff、frame options | `middlewares/middleware_security.go` |
| CORS | 环境变量白名单，通配符仅限开发 | `middlewares/middleware_cors.go` |
| 限流 | 按 IP（auth / ai / poll / gov 写） | `middlewares/middleware_ratelimit.go` |
| 请求体大小限制 | 全局 + `/auth` 上更严格的上限 | `middlewares/middleware_bodysizelimit.go` |
| 运维端点门禁 | bearer 令牌，生产环境返回 404 | `middlewares/middleware_observability_gate.go` |
| 输入校验 | `validate:` struct 标签 | `internal/http/datatransfers/requests/` |
| 加密的密钥 | AES-256-GCM 的 OAuth 令牌 | `usecases/integrations`（`INTEGRATION_ENC_KEY`） |
| SQL 注入 | pgx（参数化查询） | `internal/datasources/repositories/` |
| PDF 签署 | 通过服务端 Document-Signer 证书的 PAdES | `usecases/sign`（`SIGN_SIGNER_*`） |

## API 设计

所有 API 路由都位于 `/api/v1` 之下；每个模块挂载 `/v1/<module>`：
`auth`、`users`、`users/me/eid`、`rbac`、`org`、`gov`、`integrations`、`assets`、
`gspace`、`gateway`、`core`、`sso`、`admin`、`superadmin`、`ai`、`audit`、
`security`、`site`、`sign`，以及（配置了 Hydra 时）`provider` + `applications`。
基础设施端点（`/health`、`/ready`、`/metrics`、`/swagger`）和提供方能力面
（`/admin`、`/rp/sign`）位于根路径。**完整的端点表见
[API_CONTRACT.md](API_CONTRACT.md)** 以及生成的 OpenAPI 规范（`/swagger`）。

### 响应格式

统一的信封（`internal/http/handlers/v1/handler_base_response.go`）：

**成功**

```json
{ "status": true, "message": "login success", "data": { }, "request_id": "…" }
```

**错误**

```json
{ "status": false, "message": "user not found", "request_id": "…" }
```

**校验错误（422）**

```json
{ "status": false, "message": "validation failed",
  "data": { "errors": { "national_id": "national_id is required" } }, "request_id": "…" }
```

领域错误（`internal/apperror`）映射到状态码：NotFound→404、Unauthorized→401、
Forbidden→403、Conflict→409、BadRequest→400、Internal→500。
5xx 的原因会被记入日志，并在响应体中替换为通用消息。

## 测试策略

- **单元测试** — 使用 mockery 生成的 mock（`internal/test/mocks/`）覆盖 usecase +
  handler 层。速度快，无需 Docker。`go test ./...`。
- **集成测试** — 通过 testcontainers-go（`internal/test/testenv/`）针对真实的
  Postgres + Redis 测试各仓储（含 RLS 策略）。`make test-integration`。
- **Mock** — 由 mockery 生成。`make mock interface=… dir=… filename=…`。
- **授权矩阵** — `routes/routes_authz_matrix_test.go` 断言每条路由上的认证/权限门禁。

## 配置

由 Viper 从 `.env` / 环境变量加载（`internal/config/config.go`；参见
`internal/config/.env.example`）。配置守卫强制执行生产不变量
（TLS DSN、`ALLOWED_ORIGINS`、`VERIFY_API_KEY`、JWT 密钥长度）。部分关键项：

| 分组 | 变量 |
|-------|-----------|
| **服务器** | `PORT`、`ENVIRONMENT`（`development`/`production`）、`DEBUG` |
| **数据库** | `DB_POSTGRE_DRIVER`、`DB_POSTGRE_DSN`（开发）、`DB_POSTGRE_URL`（生产；`sslmode=verify-full`/`verify-ca`）、`DB_MAX_OPEN_CONNS`（25）、`DB_MAX_IDLE_CONNS`（5）、`DB_CONN_MAX_LIFE_MINS`（15） |
| **JWT** | `JWT_SECRET`（≥32）、`JWT_EXPIRED`（小时，1–24）、`JWT_ISSUER`、`JWT_REFRESH_EXPIRED`（天，7） |
| **Redis** | `REDIS_HOST`、`REDIS_PASS`、`REDIS_EXPIRED`（分钟） |
| **加密** | `BCRYPT_COST`（12） |
| **Verify（OTP）** | `OTP_MAX_ATTEMPTS`（5）、`VERIFY_API_BASE`、`VERIFY_API_KEY`（生产必填）、`VERIFY_CHANNEL` |
| **eID** | `EID_BASE_URL`（`…/v3`）、`EID_RP_UUID`、`EID_RP_NAME`、`EID_RP_SECRET`、`EID_CERT_LEVEL`（ADVANCED）、`EID_CALLBACK_URL`、`EID_DISPLAY_TEXT`、`SIGN_RELAY_TOKEN` |
| **签署** | `SIGN_SIGNER_CERT_FILE`、`SIGN_SIGNER_KEY_FILE`（生产 fail-closed） |
| **Google OAuth** | `GOOGLE_CLIENT_ID`、`GOOGLE_CLIENT_SECRET` |
| **XYP** | `XYP_API_BASE`（`https://xyp.dgov.mn`）、`XYP_CLIENT_ID`、`XYP_CLIENT_SECRET` |
| **Gerege Space** | `GSPACE_HOST`、`GSPACE_PORT`（22）、`GSPACE_USER`、`GSPACE_PASSWORD`、`GSPACE_BASE_PATH`（gerege-space）、`GSPACE_QUOTA_BYTES`（2 MB） |
| **Gemini AI** | `GEMINI_API_KEY`、`GEMINI_MODEL`、`GEMINI_TTS_MODEL`、`GEMINI_VOICE`、`GEMINI_API_BASE`、`AI_SCOPE_PROMPT` |
| **Gerege Core** | `CORE_API_BASE`（`https://core.gerege.mn`）、`CORE_API_TOKEN` |
| **集成** | `INTEGRATION_ENC_KEY`（AES-256-GCM；生产必填） |
| **OIDC 提供方（Hydra）** | `HYDRA_ADMIN_URL`（`http://hydra:4445`）、`HYDRA_PUBLIC_URL`、`SSO_STATE_KEY`（≥32）、`SSO_FIRSTPARTY_CLIENTS`、`SSO_ADMIN_API_KEYS`、`SSO_ADMIN_SUBS` |
| **可观测性** | `OTEL_EXPORTER`（``/`stdout`/`otlp`）、`OTEL_SAMPLE_RATIO`、`OBSERVABILITY_TOKEN` |
| **网络** | `ALLOWED_ORIGINS`（生产必填）、`TRUSTED_PROXIES` |
| **引导初始化** | `SUPERADMIN_EMAIL` |

## 部署

```bash
go build ./...                 # 构建
docker compose up -d --build   # db + redis + migrate（一次性）+ api + web
```

健康检查：`curl http://localhost:8080/health`。部署拓扑参见 `docs/DEPLOYMENT.md`。

## 致谢与许可

本平台建立在开源工作之上：

| 项目 | 作者 | 许可 | 我们使用了什么 |
|---------|--------|---------|--------------|
| [snykk/go-rest-boilerplate](https://github.com/snykk/go-rest-boilerplate) | Najib Fikri | MIT | 整洁架构分层、缓存、可观测性与测试策略 |

交付层由 **Gin → chi (net/http)** 改写，数据层由 **sqlx → pgx (pgxpool)** 改写；
认证体系、RLS 安全模型、eID/SSO/OIDC 提供方集成与各功能模块均为本平台专门构建。
作为 MIT 衍生作品，上游版权声明被保留，本代码依 MIT 许可分发（见 `LICENSE`）。

---

**Gerege Template Platform V3.0** — 由 **Gerege Systems 开发团队**与 **Claude AI** 共同打造，2026。
