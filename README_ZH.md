# Gerege Template Platform V3.0 — 后端（Go）

> _一套基础 — 承载所有公共与私营服务。_

> 🌐 **中文** · [English](README.md) · [Монгол](README_MN.md) · [Русский](README_RU.md)

[![Go](https://img.shields.io/badge/Go-1.26-blue.svg)](https://golang.org/)
[![chi](https://img.shields.io/badge/chi-v5-00ADD8.svg)](https://github.com/go-chi/chi)
[![pgx](https://img.shields.io/badge/pgx-v5-336791.svg)](https://github.com/jackc/pgx)
[![License](https://img.shields.io/badge/License-MIT-green.svg)](LICENSE)

**Gerege Template Platform V3.0** 的 Go 后端 — 一套可直接投入生产的基础平台，
*任何*公共部门或私营部门的数字服务都可在其上构建。它把严谨的**整洁架构**内核
与手写 **pgx SQL**（无 ORM）结合，并开箱附带一整套政务级能力：
**eID Mongolia** 认证、**Google** 账户绑定、**PAdES** 文件签署、**Gemini AI** 流水线，
以及纵深防御式的安全加固 — 全部支持多语言（mn/en/zh/ru），且从第一天起即可观测。
HTTP 层基于 **chi (net/http)**，数据层基于 **pgx (pgxpool) + PostgreSQL**，
缓存基于 **Redis + Ristretto**。

> **参考部署：** **Gerege Template Platform**（[template.gerege.mn](https://template.gerege.mn)）
> — 一个构建在本基础平台之上的政务服务平台，同时也是 Gerege SSO 的依赖方，
> 展示了 eID 单点登录以及面向其他应用的内置 OIDC 提供方。

## 📌 溯源与开源

> 本模板**基于并受启发于开源项目
> [snykk/go-rest-boilerplate](https://github.com/snykk/go-rest-boilerplate)**
> （作者：Najib Fikri，**MIT 许可**）。整洁架构结构、JWT/OTP 认证、审计、缓存、
> 可观测性与测试策略均承自该项目。
>
> 我们**移植**了以下两处：
> - HTTP 层：**Gin → chi (net/http)**
> - 数据层：**sqlx → pgx（pgxpool，手写 SQL）**
>
> 上游项目采用 MIT 许可，其版权与许可条款得到尊重并被保留
> （见下文[致谢](#-致谢与许可)一节）。本模板自身同样采用 **MIT 许可**。

## 功能特性

- **整洁架构** — `handler → usecase → repository → domain`，依赖向内，无反向 import
- **chi (net/http)** — 符合标准库习惯的 router
- **pgx (pgxpool)** — 手写 SQL，无 ORM；通过 `deleted_at IS NULL` 显式实现软删除
- **eID 认证** — 唯一的登录方式：eID Mongolia 依赖方（二维码 / 移动端 deep-link /
  按登记号推送）配合 long-poll 会话；签发 JWT access + refresh 令牌（带轮换、`kind` claim 守卫）
- **Google OAuth 绑定** — 把 Google 账户绑定到 eID 用户（code 交换仅在服务端完成），
  之后即可用它登录
- **OIDC 提供方（SSO）** — 可选的 Ory Hydra 前端，使 DAN 充当身份提供方；
  提供登录/授权/登出流程以及用于 RP 客户端注册的 `/admin` 能力面
  （仅在配置了 Hydra 时启用）
- **eID PKI 档案** — 已登录公民关联的组织与签署人、证书、设备与活动记录
- **组织与成员** — 组织创建/查询（Gerege Verify/XYP 国家登记查询）+ 成员/角色管理，
  按用户以 RLS 隔离
- **政务服务门户** — 目录、申请、证明、通知、缴费、预约
- **API 网关** — 服务 / 路由 / consumer / API key / 策略 + 请求遥测（管理员管理）
- **文件签署（PAdES）** — 通过 eID Mongolia `/v3` 的服务端 PDF 签署，
  使用常驻的 Document-Signer 证书；可选的面向第三方 RP 的签署中继
- **集成与存储** — 按用户的 OAuth 集成（Google Drive/Meet、Dropbox），令牌以
  AES-256-GCM 加密；Gerege Space 应用自有的 SFTP 存储
- **AI 流水线（Gemini）** — 免 SDK 的 REST 客户端 + function calling：
  文本/语音聊天、STT、TTS、实时翻译；分层提示词（硬编码防护规则 + 可从数据库配置的 scope）
  以及基于数据库的 `search_knowledge` 工具
- **RBAC 与超级管理员** — 动态角色 + 权限目录；四角色模型
  （超级管理员 → 管理员 → 经理 → 用户）
- **站点外观** — 管理员可配置的站点级外观（强调色/字体/密度/主题）+ 按用户覆盖
- **审计日志** — 哈希链式、仅追加的审计轨迹（仅管理员读取 + 完整性校验）
- **可观测性** — OpenTelemetry 追踪 + Prometheus 指标；生产环境中
  `/metrics` + `/swagger` 由 bearer 令牌把守
- **缓存** — Redis + Ristretto 两级缓存
- **集成测试** — testcontainers-go（真实 Postgres + Redis）
- **Swagger** — 由 godoc 注解自动生成 API 文档
- **结构化日志** — Zap，带 request ID 传播
- **安全** — 安全响应头、CORS、限流、请求体大小限制、完整的服务器超时、
  Postgres RLS + 启动时可执行性守卫、登出 access 令牌拒绝名单
- **优雅停机** — 依次排空 HTTP、数据库连接池、Redis、tracer

## 项目结构

```
.
├── cmd/
│   ├── api/main.go              # 应用入口
│   ├── api/server/server.go     # 组合根（手动 DI）
│   ├── migration/               # 迁移 CLI
│   ├── seed/                    # 播种 CLI
│   └── healthcheck/             # distroless 健康探针
├── internal/
│   ├── business/
│   │   ├── domain/              # 领域实体（最内层）
│   │   └── usecases/           # 业务逻辑（接口 + 实现），每个模块一个包：
│   │       #  auth · users · rbac · superadmin · ai · audit · security · site
│   │       #  org · gov · gateway · core · sso · provider · sign · assets
│   │       #  integrations · gspace
│   ├── datasources/
│   │   ├── drivers/             # pgx (pgxpool) Postgres 连接 (driver_pgx.go)
│   │   ├── caches/              # Redis + Ristretto
│   │   ├── migration/           # 迁移执行器
│   │   ├── records/             # pgx 记录结构体 + 记录↔领域映射
│   │   └── repositories/        # 接口 + postgres 实现
│   ├── http/
│   │   ├── handlers/v1/         # HTTP handler
│   │   ├── middlewares/         # 中间件栈
│   │   ├── routes/              # 路由注册
│   │   ├── datatransfers/       # 请求/响应 DTO
│   │   └── auth/                # 从 context 取 CurrentUser
│   └── config/ apperror/ constants/
├── migrations/                  # SQL 迁移
├── docs/                        # Swagger + ARCHITECTURE.md + DEVELOPMENT.md
└── pkg/                         # jwt、logger、clock、helpers、validators、
                                 # audit、observability、gemini、
                                 # eid、google、oidc、hydra、xyp、gspace、verify
```

## 快速开始

### 环境要求

- Go 1.26+
- PostgreSQL 15+
- Redis 7+
- Docker（用于集成测试 / 本地技术栈）
- Make

### 安装

```bash
# 1. 复制环境文件（它位于 internal/config/ 下）
cp internal/config/.env.example internal/config/.env
# 编辑 .env — JWT_SECRET 至少 32 个字符

# 2. 启动技术栈（Postgres + Redis + API）

# 3. 或在本地运行：先迁移 → 再启动服务
```

服务：`http://localhost:8080`，Swagger UI：`http://localhost:8080/swagger/`。

### Make 命令

```bash
make build              # 构建二进制
make test               # 单元测试（mock — 快速，无需 Docker）
make test-integration   # 集成测试（需要 Docker）
make swag               # 生成 Swagger 文档
make lint               # golangci-lint
make pre-push           # 在本地跑 CI 检查（lint+测试+swag+构建）
```

## 配置

来自 `internal/config/.env.example` 的关键变量：

```env
# 核心
PORT=8080
ENVIRONMENT=development          # development | production
JWT_SECRET=...                   # >= 32 个字符（HS256）
JWT_EXPIRED=5                    # access 令牌 TTL（小时，1..24）
JWT_REFRESH_EXPIRED=7            # refresh 令牌 TTL（天）
DB_POSTGRE_DSN=...               # 开发环境的 DSN
DB_POSTGRE_URL=...               # 生产环境的 URL（必须使用 sslmode=verify-full/verify-ca）
REDIS_HOST=localhost:6379
BCRYPT_COST=12                   # 10..31
ALLOWED_ORIGINS=                 # 生产必填（逗号分隔）
TRUSTED_PROXIES=                 # 可信任其 X-Forwarded-For 的反向代理 IP/CIDR
OBSERVABILITY_TOKEN=             # 生产环境把守 /metrics + /swagger 的 bearer 令牌

# eID Mongolia（依赖方）— 主要登录方式；默认值合理，保证启动不会中断
EID_BASE_URL=https://eidmongolia.mn/v3
EID_RP_UUID=                     # 在 IdP 处注册的 RP UUID
EID_RP_NAME=                     # RP 显示名称
EID_RP_SECRET=                   # RP API secret（也用于 /rp/sign 中继）
EID_CERT_LEVEL=ADVANCED          # ADVANCED | QUALIFIED | QSCD
EID_CALLBACK_URL=                # 必须在 IdP 处加入白名单
EID_DISPLAY_TEXT=

# Google OAuth — 把 Google 账户绑定到 eID 用户
GOOGLE_CLIENT_ID=
GOOGLE_CLIENT_SECRET=

# OIDC 提供方侧（平台自身即 issuer）— 未设置时提供方流程处于惰性状态
OAUTH_ISSUER=                    # issuer，例如 https://template.gerege.mn（留空 = 关闭提供方）
SSO_STATE_KEY=                   # >= 32 字节；登录/授权 state cookie 的 HMAC
SSO_FIRSTPARTY_CLIENTS=          # 跳过授权确认页的 client_id，逗号分隔
SSO_ADMIN_API_KEYS=              # /admin 能力面的引导密钥，逗号分隔

# 文件签署（PAdES）— 常驻的 Document-Signer 材料（生产必填）
SIGN_SIGNER_CERT_FILE=
SIGN_SIGNER_KEY_FILE=
SIGN_RELAY_TOKEN=                # 共享令牌，使第三方 RP 可借 DAN 的 eID 凭据签署

# Gerege 国家服务
XYP_API_BASE=https://xyp.dgov.mn # 组织查询（国家登记）；Basic 认证
XYP_CLIENT_ID=
XYP_CLIENT_SECRET=
CORE_API_BASE=https://core.gerege.mn  # Gerege Core 用户/组织查找
CORE_API_TOKEN=

# Gerege Space — 应用自有的 SFTP 存储（留空 = 停用该功能）
GSPACE_HOST=
GSPACE_PORT=22
GSPACE_USER=
GSPACE_PASSWORD=
GSPACE_BASE_PATH=gerege-space
GSPACE_QUOTA_BYTES=2097152       # 按用户配额（默认 2 MB）

# 集成令牌加密（AES-256-GCM）— 生产必填
INTEGRATION_ENC_KEY=

# GeregeCloud Verify (verify.gecloud.mn) — OTP 通道；生产必填
VERIFY_API_KEY=
VERIFY_API_BASE=https://verify.gecloud.mn/v1
VERIFY_CHANNEL=email

# AI 流水线 (/api/v1/ai/*)
GEMINI_API_KEY=                  # 留空 = 停用 AI（相关端点返回 500）
GEMINI_MODEL=gemini-2.5-flash    # 可选覆盖（聊天 / STT / 翻译）
GEMINI_TTS_MODEL=gemini-2.5-flash-preview-tts  # 可选覆盖（TTS）
GEMINI_VOICE=Kore                # 可选的预置 TTS 音色
GEMINI_API_BASE=                 # 可选覆盖（默认：Google generativelanguage v1beta）
AI_SCOPE_PROMPT=                 # 当数据库 'scope' 提示词层为空时的兜底值

# 可观测性 + 引导初始化
OTEL_EXPORTER=                   # 留空=关闭 | stdout | otlp
SUPERADMIN_EMAIL=                # 可选：启动时把该（已登录过的）用户提升为超级管理员
```

### 角色与超级管理员

角色按权限高低排序（id 1 = 最高）：**超级管理员=1、管理员=2、经理=3、用户=4**
（由迁移 `23_superadmin_role` 预置/重映射）。**超级管理员**位于管理员之上，
是唯一能够通过 `/api/v1/superadmin/*`（`RequireSuperAdmin`）管理管理员账户
（创建 / 授予 / 撤销）的角色；普通管理员无法访问该能力面。API 绝不创建超级管理员 —
可通过把 `SUPERADMIN_EMAIL` 设为某个已通过 eID 登录过的现有用户来引导
（下次启动时提升），或直接在数据库中更新 `role_id=1`。

> **破坏性变更（已有部署）：** 迁移 `23` 重新编号了角色，因此在此之前签发的 JWT
> 会被重新解释（旧的 `admin=1` → 超级管理员，`user=2` → 管理员）。
> 在已有数据库上应用时，请**轮换 `JWT_SECRET`**（或强制所有用户重新登录），
> 以免陈旧令牌获得错误的权限。全新安装不受影响。

### AI 提示词分层

AI 助手运行在分层系统提示词之上：**基础防护规则**
（硬编码 — 回复语言、范围约束、抗提示词注入）+ **scope**（助手协助的范围）
+ **instructions**（可选的语气/规则）。scope 与 instructions 存放在 `ai_prompts` 表中，
可通过 `GET/PUT /api/v1/admin/ai/prompts` 在运行时编辑
（需要 `settings.manage`；界面在 管理 → 设置）。助手会拒绝所配置范围之外的一切请求，
并通过 `search_knowledge` 工具检索 `ai_knowledge` 表来回答平台相关问题。

## API 端点

全部位于 `/api/v1` 之下（运维端点在根路径）。**不存在密码 / 邮箱 OTP /
注册 / 忘记密码-重置端点** — 认证方式只有 eID + Google。

### 公开（认证）

| 方法 | 路径 | 说明 |
|--------|------|---------|
| POST | `/api/v1/auth/eid/start` | 发起 eID 登录（二维码 / 移动端 deep-link） |
| POST | `/api/v1/auth/eid/start-id` | 按登记号发起 eID 登录（推送到已登记设备） |
| POST | `/api/v1/auth/eid/poll` | 长轮询 eID 会话直至完成 |
| POST | `/api/v1/auth/google` | Google OAuth 回调 — code 交换 + eID 绑定 / 登录 |
| POST | `/api/v1/auth/refresh` | 令牌轮换 |
| POST | `/api/v1/auth/logout` | 吊销 refresh + 把 access 令牌加入拒绝名单 |

### 受保护（需要 JWT）

| 方法 | 路径 | 说明 |
|--------|------|---------|
| GET | `/api/v1/users/me` | 用户档案 |
| GET | `/api/v1/rbac/me` | 当前用户的有效角色/权限 |
| DELETE | `/api/v1/auth/google/link` | 解绑已连接的 Google 账户 |
| GET | `/api/v1/me/*`、`/api/v1/users/me/eid/*` | eID PKI 档案 — 组织、签署人、证书、设备、活动 |
| CRUD | `/api/v1/org/*` | 组织 + 成员（国家登记查询、成员、角色） |
| GET/POST | `/api/v1/gov/*` | 政务服务门户 — 服务、申请、证明、通知、缴费、预约 |
| CRUD | `/api/v1/gateway/*` | API 网关 — 服务、路由、consumer、key、策略、日志 |
| GET | `/api/v1/core/users` · `/organizations` | Gerege Core 查找（用户/组织） |
| CRUD | `/api/v1/integrations/*` | 按用户的 OAuth 集成（加密令牌） |
| GET | `/api/v1/assets/*` | 签名图片 + 组织印章资产 |
| GET | `/api/v1/gspace/*` | Gerege Space SFTP 存储（列表 + 下载） |
| POST/GET | `/api/v1/sign/*` | 文件签署（PAdES）— 发起、状态、下载 |
| POST | `/api/v1/ai/chat` | AI 聊天（Gemini 流水线、function calling、文本/语音消息） |
| POST | `/api/v1/ai/stt` | 语音转文字（base64 音频 → 文本） |
| POST | `/api/v1/ai/tts` | 文字转语音（文本 → base64 WAV） |
| POST | `/api/v1/ai/translate` | 实时翻译（文本/音频 → 目标语言，可选 TTS） |
| GET | `/api/v1/site/appearance` | 站点级外观默认值（公开读取） |
| GET/PUT | `/api/v1/admin/ai/prompts` | AI 提示词分层 — scope/instructions（settings.manage） |
| GET | `/api/v1/audit` · `/audit/verify` | 读取审计日志 + 校验哈希链（管理员） |
| POST | `/api/v1/security/events` | 接收客户端安全事件 |
| GET | `/api/v1/superadmin/admins` | 列出管理员级账户（仅超级管理员） |
| POST | `/api/v1/superadmin/admins` | 创建新的管理员账户（仅超级管理员） |
| PUT | `/api/v1/superadmin/admins/{id}/grant` | 为已有用户授予管理员（仅超级管理员） |
| DELETE | `/api/v1/superadmin/admins/{id}` | 撤销管理员（仅超级管理员） |

### OIDC 提供方（仅在配置了 Hydra 时）

`GET /api/v1/provider/login` · `/consent`，以及登录/授权/登出的 accept/reject
（由 Hydra 驱动的登录/授权界面）。RP OAuth2 客户端注册位于挂载的 `/admin` 能力面下。

### 运维

`GET /health`（存活）· `GET /ready`（数据库+Redis）· `GET /metrics` · `GET /swagger/doc.json`
— 生产环境中 `/metrics` 与 `/swagger` 需要 `OBSERVABILITY_TOKEN` bearer（否则返回 404）。

### 响应格式

```json
{ "status": true, "message": "login success", "data": { }, "request_id": "…" }
```

出错时 `status:false`。校验错误 → `422`，每个字段位于 `data.errors` 下。

## 开发

详情请参见：

- **[docs/ARCHITECTURE.md](docs/ARCHITECTURE.md)** — 分层结构、依赖流向、安全
- **[docs/DEVELOPMENT.md](docs/DEVELOPMENT.md)** — 新增功能的 8 个步骤、测试、代码风格、故障排查
- **[docs/AI_PIPELINE.md](docs/AI_PIPELINE.md)** — AI 助手内部机制：流程、提示词分层、工具、语音、扩展方式

```bash
make test               # 单元测试
make test-integration   # 集成测试（Docker）
make test-cover         # 覆盖率
```

## Docker

```bash
make build              # 二进制
curl http://localhost:8080/health
```

## 🙏 致谢与许可

本模板站在开源工作的肩膀上：

| 项目 | 作者 | 许可 | 我们使用了什么 |
|-------|---------|--------|--------------|
| [snykk/go-rest-boilerplate](https://github.com/snykk/go-rest-boilerplate) | Najib Fikri | MIT | 基础架构、认证/OTP/审计、缓存、可观测性、测试 |
| [chi](https://github.com/go-chi/chi) · [pgx](https://github.com/jackc/pgx) | — | MIT | Router · Postgres 驱动 |

**我们的改动：** 把 HTTP 层由 **Gin → chi (net/http)** 移植，数据层由
**sqlx → pgx（pgxpool，手写 SQL）** 移植；其余部分忠实保留。
遵循 MIT 传统，上游项目的版权声明被保留，本模板自身同样采用
**MIT 许可**（见 `LICENSE` 文件）。

---

**Gerege Template Platform V3.0** — 由 **Gerege Systems 开发团队**与 **Claude AI** 共同打造，2026。
