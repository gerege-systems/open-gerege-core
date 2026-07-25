# 安全态势 — Gerege Template Platform V3.0

> 🌐 **中文** · [English](SECURITY.md) · [Русский](SECURITY_RU.md) · 蒙古语说明请参见代码注释。漏洞上报流程见
> [`/SECURITY.md`](../../SECURITY.md)。

本文将后端已实现的各项控制映射到项目的安全标准 — 依据 **OWASP ASVS / API Top 10、
NIST SP 800-63B / 800-218 以及 CIS Controls**。它记录了代码中强制执行的内容、
本轮加固的内容，以及留待后续阶段处理的事项。要报告漏洞，请参阅仓库的
[安全策略](../../SECURITY.md)。

> **认证模型。** 唯一的交互式登录方式是 **eID（eID Mongolia 依赖方）** —
> 设备绑定 / 二维码 + 按登记号推送，配合 long-poll 会话 — 以及 **Google OAuth**
> 账户绑定。**没有任何路由接入密码 / 邮箱 OTP 登录路径**；历史遗留的
> `Login` / `Register` / OTP / 密码重置 usecase 仍存在于代码树中，但未被
> `route_auth.go` 暴露（后者只注册 eID / Google / refresh / logout）。
> 下列控制反映的是真实存在的攻击面。

## 已实现的控制（代码层面）

| 领域 | 控制 | 位置 | 指南 § |
|------|---------|-------|---------|
| 认证 | JWT access+refresh、轮换、`kind` claim 守卫 | `pkg/jwt`、`usecases/auth` | §1.3–1.4 |
| 认证 | eID 登录（Mongolia RP）— 设备绑定 / 二维码 + 按登记号推送、long-poll 会话；**唯一**的交互式登录 | `usecases/auth/auth_eid.go`、`pkg/eid`、`routes/route_auth.go` | §1 |
| 认证 | eID 公民证书（PKI）— 登录 COMPLETE 时返回公民证书（DER）；用 `crypto/x509` 解析，并持久化序列号 / 有效期 / 颁发者 / 公钥类型 | `pkg/eid`、`migrations/16_users_eid_certificate.up.sql` | §1 |
| 认证 | 联合身份 — Google OAuth 账户绑定，以稳定的 subject 列为键 | `usecases/auth`、`migrations/18_users_google_sub` | §1 |
| 加密 | 令牌 / 会话标识符使用 `crypto/rand`；拒绝采样避免取模偏差 | `pkg/helpers` | §13.2 |
| 加密 | 集成令牌静态加密 — 第三方 OAuth 令牌入库前用 **AES-256-GCM** 封装；密钥来自 `INTEGRATION_ENC_KEY` | `usecases/integrations/integrations_crypto.go`、`migrations/21_user_integrations` | §7.3 |
| 审计 | 哈希链式、仅追加的审计日志 — `chain_hash = SHA-256(prev_hash ‖ canonical-json(entry))`，写入由 `pg_advisory_xact_lock` 串行化；`VerifyChain` 可发现篡改 | `pkg/audit/chain.go`、`usecases/audit`、`migrations/15_audit_log` | §9 |
| 授权 | 动态 RBAC（角色 + 权限），超级管理员/管理员/经理/用户；`RequirePermission` / `RequireAdmin` 路由中间件；管理员自动解析出完整权限目录 | `middleware_rbac.go`、`domain_users.go`、`migrations/8_rbac_roles_permissions`、`migrations/23_superadmin_role` | §2 |
| 授权 | OIDC **提供方**面 — 平台自身即 OIDC 提供方（登录 / 授权 / 登出核心，内置 Go provider）；consent 决定每个 scope 释放哪些公民 claims；`developer_apps` RP 注册表是客户端归属的权威来源 | `usecases/provider`、`usecases/oidc`、`postgres/oauth`、`migrations/42_oauth_provider` | §2 |
| 数据库 | 只使用参数化查询（pgx） | `datasources/repositories/postgres` | §3.1 |
| 数据库 | `INSERT … RETURNING` 单次往返；pgconn 23505 → Conflict | `repositories/postgres/users`、`driver_pgx.go` | §3 |
| 数据库 | 每张按用户划分的表都启用行级安全（ENABLE + **FORCE**）：`users` 以及 `organizations` / `organization_memberships`、各 `gov_*` 公民表和 `user_integrations` — self/admin/service 策略由 `withRLS` 内每事务 `SET LOCAL` 设置的 `app.user_id`/`app.user_role` GUC 驱动；无身份 ⇒ 零行（fail-closed） | `migrations/7_enable_rls_users`、`migrations/14`、`migrations/20`、`migrations/21`、`datasources/rls`、`repositories/postgres/*` | §2.4/§3.3 |
| API | 防批量赋值（显式请求 DTO） | `http/datatransfers/requests` | API3 §5.1 |
| API | 请求体大小限制（全局 + `/auth` 上 4 KiB） | `middleware.bodysizelimit`、`routes` | §5.3 |
| Web | 安全响应头：CSP `default-src 'none'`、HSTS（生产）、nosniff、X-Frame DENY、Referrer-Policy、Permissions-Policy、COOP/CORP/COEP | `middleware_security.go` | §4.7 |
| Web | CORS 严格来源白名单，绝不 `*`+凭据 | `middleware.cors.go` | §4.8 |
| 运维 | 运维端点（`/metrics`、`/swagger/doc.json`）在生产中受控：bearer 令牌（常数时间比较）+ 未命中返回 404 | `middleware_observability_gate.go`、`cmd/api/server` | §4.7/§9 |
| 可观测性 | 带 request-id 的 Zap 结构化日志；不记录任何密钥 | `pkg/logger`、`handler_base_response.go` | §9.1–9.2 |
| 可观测性 | OpenTelemetry 链路追踪 + Prometheus 指标 | `pkg/observability`、`driver_pgx.go` | §9.4 |
| 运维 | 优雅停机（排空 HTTP、限流器、pgx 连接池、Redis、tracer） | `cmd/api/server` | §7 |
| 网络 | 完整的 HTTP 服务器超时（`ReadHeader` 10s、`Read` 30s、`Write` 60s、`Idle` 120s）+ `MaxHeaderBytes` 16 KiB — 防 slowloris / 超大请求头 | `cmd/api/server` | §5.3 / API4 |
| 认证 | 登出 access 令牌拒绝名单 — 登出时把 access jti 按剩余 TTL 写入 Redis；认证中间件在每次请求时拒绝名单内的令牌 | `usecases/auth.logout`、`middleware_auth.go` | §1.4 |
| 数据库 | RLS 启动守卫 — 启动时应用检查自身的数据库角色；生产环境中超级用户 / `BYPASSRLS` 会导致启动失败（否则 RLS 会静默失效），开发环境仅告警 | `datasources/drivers/driver_pgx.go` | §2.4/§3.4 |
| AI | 分层系统提示词：硬编码的防护规则（范围约束、抗提示词注入、绝不泄露提示词）+ 可从数据库配置的 scope/instructions；`SetPrompt` 仅对预置键做 UPDATE | `usecases/ai/ai_prompts.go`、`migrations/11` | §5.1 |
| AI | AI 输入卫生：音频 mime 白名单 + 约 700 KB base64 上限、消息/历史长度上限、专用 `/ai` 限流（约 20 次/分钟），工具错误只回报给模型 — 绝不返回客户端 | `requests_ai.go`、`routes/route_ai.go` | §5.1/§5.3 |

## 本轮实施的加固（对照指南）

1. **跨源隔离响应头** — 在 `middleware.security.go` 中加入
   `Cross-Origin-Opener-Policy: same-origin`、`Cross-Origin-Resource-Policy: same-site`、
   `Cross-Origin-Embedder-Policy: require-corp`（指南 §4.6/4.7）。*已在运行中的服务器上实测验证。*
2. **生产数据库 TLS 守卫** — 配置校验现在会拒绝未使用 `sslmode=verify-full`
   （或 `verify-ca`）的生产 `DB_POSTGRE_URL`；`.env.example` 中已记录
   （`internal/config/config.go`，指南 §3.5）。
3. **按请求超时** — `middleware.TimeoutMiddleware` 设置 30 秒的 context 截止时间，
   并传播到 pgx 查询，为卡住的 handler 划定边界
   （`middleware.timeout.go`，指南 §5.3 / API4）。
4. **由生成的 `docs` 包提供 Swagger 规范** — OpenAPI JSON 由 chi 路由上的生成
   `docs` 包在 `/swagger/doc.json` 提供（不涉及 Fiber）；可让静态 Swagger UI 指向它。
5. **运维端点门禁** — `/metrics` 与 `/swagger/doc.json` 不再公开提供。生产环境中
   `ObservabilityGate` 要求 `Authorization: Bearer <OBSERVABILITY_TOKEN>`
   （用 `crypto/subtle.ConstantTimeCompare` 比较），任何未命中都返回 **404**（而非 401），
   以便在侦察中隐藏这些端点。令牌为空 ⇒ 完全关闭。`/health` + `/ready` 仍对负载均衡器公开。
6. **Postgres RLS + 数据库角色分离** — `users` 现已 **ENABLE + FORCE** RLS，
   并配有 self/admin/service 策略。按请求的身份信息经由仓储层 `withRLS` 事务中的
   `SET LOCAL app.user_id`/`app.user_role` 从 context 流入每条查询；无身份 ⇒ 零行
   （fail-closed）。compose 中的 **api** 以非超级用户 `APP_DB_USER` 连接
   （由 `deploy/initdb/10-create-app-user.sh` 创建），使策略真正生效；
   **migrate** 仍保留超级用户以执行 DDL。由一项以非超级用户角色连接的集成测试证明
   （`users_rls_test.go`）。
7. **HTTP 服务器加固** — 除 `ReadHeaderTimeout` 外，服务器现已设置
   `ReadTimeout`/`WriteTimeout`/`IdleTimeout`，并将请求头限制为 16 KiB；
   `WriteTimeout` 由请求级超时预算推导（2 倍），确保进行中的 handler 不会先被服务器切断。
8. **登出吊销两种令牌** — refresh jti 被删除（与此前一致），access jti 则按令牌剩余
   生命周期加入 Redis 拒绝名单；认证中间件在每次请求时检查该名单
   （Redis 出错时 fail-open，与密码轮换截止策略一致）。
9. **启动时的 RLS 可执行性守卫** — 应用启动时查询 `pg_roles` 获取自身角色；
   生产环境中超级用户或 `BYPASSRLS` 角色会导致启动失败，开发环境记录告警，
   这样配置错误的 DSN 就再也无法悄悄地让 RLS 失效。
10. **AI 防护规则** — Gemini 助手运行在分层提示词之上，其基础层
    （仅蒙古语、范围约束、抗提示词注入）为硬编码；只有 scope/instructions 层可由管理员编辑
    （`settings.manage`，且仅对预置键做 UPDATE）。工具在服务端以请求 context 执行；
    工具失败以数据形式回报给模型，绝不泄露给客户端。

## ASVS 路线图状态（指南 §14）

- **阶段 1（ASVS L1）：** ✅ 支持 HTTPS + HSTS、仅 eID 登录（无密码面）、参数化查询、
  安全响应头、严格 CORS、输入校验、结构化日志、`.gitignore` 且无提交的密钥。
  ⏳ 在 CI 中接入容器扫描 / `govulncheck`（`.github/`）。
- **阶段 2（ASVS L2）：** ✅ 限流、refresh 令牌轮换、eID 设备绑定认证
  （抗钓鱼、硬件支撑的身份）、请求超时、加密的集成令牌、哈希链审计日志。
  ⏳ WAF、集中式 SIEM、加密备份恢复演练、应急响应预案。
- **阶段 3（ASVS L3）：** ◻ 字段级 PII 加密（KMS）、mTLS、SLSA L3 溯源、外部渗透测试。
  （超出模板范围。）

## 已知不足 / 后续事项

- **交互式 Swagger UI** — 目前仅在 `/swagger/doc.json` 提供原始规范
  （可载入 Swagger Editor / Postman，或让静态 Swagger UI 指向它）。
- **密码相关控制（HIBP / bcrypt / 泄露密码）** — **对现有攻击面不适用**：
  系统未接入任何密码登录路径（认证方式为 eID + Google OAuth）。历史遗留的
  密码/OTP usecase 仍在代码树中但无法触达；若将来重新暴露密码路径，
  务必在上线前接入泄露密码检查（HIBP k-匿名，§1.1）。
- **Postgres RLS**（指南 §2.4/§3.3）— ✅ 已在每张按用户划分的表上启用**并 FORCE**
  （`users`、`organizations` / `organization_memberships`、各 `gov_*` 公民表、
  `user_integrations`），self/admin/service 策略由 `app.user_id`/`app.user_role`
  会话 GUC 驱动（各仓储 `withRLS` 中的 `SET LOCAL`）。这是在仓储层已写的
  `deleted_at IS NULL` / WHERE 条件之上的纵深防御；无身份的请求会 fail-closed。
  公共参考表（例如 `gov_services` 目录）不启用 RLS，依赖表级授权。
  若要实现**多租户**，请为每张表添加 `tenant_id` 列 + 租户策略，并在
  `rls.Identity` 中携带租户信息。
- **密钥管理 / KMS**（指南 §7.3）— 生产环境请使用真正的密钥库；
  `.env` 仅用于本地开发且已加入 gitignore。
- **数据库角色分离**（指南 §3.4）— ✅ **已接入 compose 技术栈**（这是必需的：
  RLS 即使 FORCE，也会被超级用户 / BYPASSRLS 角色绕过，而 postgres 镜像会把
  `POSTGRES_USER` 设为超级用户）。数据库首次初始化时，
  `deploy/initdb/10-create-app-user.sh` 会创建**非超级用户**角色 `APP_DB_USER`
  （`NOSUPERUSER NOBYPASSRLS`），并通过默认权限授予其 DML 权限。**api** 以该角色连接
  （compose 用 `APP_DB_DSN` 覆盖 `DB_POSTGRE_DSN` — 该技术栈以开发模式运行，
  因此驱动读取关键字形式的 DSN），使 RLS 真正生效；**migrate** 容器继续使用
  `POSTGRES_USER`（`CREATE EXTENSION "uuid-ossp"` + RLS DDL 需要超级用户）。
  可从 api 的连接上做健全性检查：
  `SELECT rolsuper, rolbypassrls FROM pg_roles WHERE rolname = current_user;` —
  两者都必须为 `false`。若 `APP_DB_URL` 仍指向超级用户，RLS 将*不会*生效
  （它会静默变成空操作）。

---

**Gerege Template Platform V3.0** — 由 **Gerege Systems 开发团队**与 **Claude AI** 共同打造，2026。
