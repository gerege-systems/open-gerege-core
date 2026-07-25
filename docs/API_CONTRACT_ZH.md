# API 契约

> 🌐 **中文** · [English](API_CONTRACT.md) · [Монгол](API_CONTRACT_MN.md) · [Русский](API_CONTRACT_RU.md)

**Gerege Template Platform V3.0**（Цахим үйлчилгээг бүтээх суурь）的 REST API 参考 —
一套用于构建数字服务的生产就绪基础平台（整洁架构 Go 后端 + Next.js BFF + Gemini AI）。
本契约跟踪其参考部署 **Gerege Template Platform**（template.gerege.mn），
一个基于 eID 的公共与私营服务平台。实时自动生成的规范由 `GET /swagger/` 提供
（来源：`docs/swagger.json`）。

> **关于路径的说明。** 下文的每个模块都挂载在 `/api` 分组下，且每个路由组会再加一个
> `/v1` 前缀，因此真实请求路径是 `/api/v1/<group>/…`，尽管 swagger 的 `@Router`
> 注解是相对写法（例如注解 `/auth/eid/start` → 真实路径 `/api/v1/auth/eid/start`）。
> 本文档的表格使用**完整**路径。

## 约定

- **基础 URL：** `http://localhost:8080/api/v1`
- **Content-Type：** `application/json`
- **认证：** 受保护端点需要 `Authorization: Bearer <access_token>`
  （令牌由下文的 eID / Google 登录流程签发）
- **限流（按 IP）：** `/auth/*` 约 5 次/分钟，`/auth/eid/poll` 单独放宽为约 60 次/分钟
  （长轮询不能把自己限成 429），`/ai/*` 约 20 次/分钟，
  以及 `/gov`、`/gspace`、`/me`、`/users/me/eid` 的**写操作**端点约 30 次/分钟（超出返回 429）
- **请求体上限：** `/auth/*` 与 `/provider/*` 限制为 4 KiB；其余为 1 MiB

### 响应信封

每个响应都使用统一的信封：

```json
{
  "status": true,
  "message": "human-readable summary",
  "data": { },
  "request_id": "b1d2…"
}
```

- `status` — 成功为 `true`，错误为 `false`
- `data` — 成功时存在（错误时省略/为 null）
- `request_id` — 关联 id（同时在 `X-Request-ID` 响应头中回显）

### 状态码

| 码 | 含义 | 何时出现 |
|------|---------|------|
| 200 | OK | 读取 / 操作成功 |
| 201 | Created | 资源已创建 |
| 400 | Bad Request | 请求体格式错误 |
| 401 | Unauthorized | 令牌缺失/无效/过期 |
| 403 | Forbidden | 已认证但缺少所需角色/权限 |
| 404 | Not Found | 资源不存在 |
| 409 | Conflict | 重复 / 状态冲突 |
| 422 | Unprocessable Entity | 校验失败（见下） |
| 429 | Too Many Requests | 超出限流 |
| 500 | Internal Server Error | 意外故障（原因记入日志，返回通用消息） |

### 校验错误（422）

字段级细节返回在 `data.errors` 下，它是一个由 `{ field, tag, message }`
对象组成的**数组**。`field` 是 JSON 标签名：

```json
{
  "status": false,
  "message": "validation failed",
  "data": { "errors": [ { "field": "target_lang", "tag": "required", "message": "target_lang is required" } ] },
  "request_id": "b1d2…"
}
```

### 图例

- 🔒 — 需要 `Authorization: Bearer <access_token>`
- 🛡️ `perm` — 另需具名 RBAC 权限（**管理员**角色会自动解析出完整权限目录；
  标注处需要**超级管理员**）。路径参数以 `{花括号}` 表示。

---

## 身份认证（`/api/v1/auth`）

**唯一**的登录方式是 **eID 登录**（eID Mongolia 依赖方），
外加 **Google OAuth** 账户绑定。不存在密码、邮箱/OTP 或注册面。
本分组受限流与请求体上限（4 KiB）约束；登录前的流程以 service RLS 身份运行。

| 方法 | 路径 | 认证 | 说明 |
|--------|------|------|-------------|
| POST | `/auth/eid/start` | — | 发起 eID 登录；返回二维码 / 移动端 deep-link 以及用于轮询的会话令牌。 |
| POST | `/auth/eid/start-id` | — | 按登记号发起 eID 登录；向公民已登记的设备推送批准请求。 |
| POST | `/auth/eid/poll` | — | 对待处理的 eID 会话长轮询（约挂起 25 秒）；返回 `PENDING`，或在批准后返回 access + refresh 令牌对。使用单独放宽的限流器。 |
| POST | `/auth/google` | — | Google OAuth 回调 — 交换授权 `code`，随后把 Google 账户绑定到（或据此登录）eID 用户。 |
| DELETE | `/auth/google/link` | 🔒 | 解绑当前已认证用户的 Google 账户（绑定只通过登录流程完成）。 |
| POST | `/auth/refresh` | — | 用有效的 refresh 令牌轮换令牌对。refresh 会**轮换**令牌，因此旧的 refresh 令牌失效。 |
| POST | `/auth/logout` | — | 吊销提交的 refresh 令牌；若同时提交 `access_token`，其 jti 会被加入 Redis 拒绝名单并立即失效。 |

成功时，登录/刷新流程会在 `data` 中返回令牌对
（`token` = access JWT，`refresh_token` = refresh JWT），
以及用户身份信息（`id`、`role_id`、姓名字段）。

---

## 用户（`/api/v1/users`）

| 方法 | 路径 | 认证 | 说明 |
|--------|------|------|-------------|
| GET | `/users/me` | 🔒 | 返回当前已认证用户的档案（`id`、`username`、`email`、`role_id`、时间戳）。 |

## eID 档案（`/api/v1/users/me/eid`）🔒

已登录公民的扩展 eID 数据。写操作（`POST`/`DELETE`）端点适用约 30 次/分钟的写限流器；
读取不受限。

| 方法 | 路径 | 说明 |
|--------|------|-------------|
| GET | `/users/me/eid/organizations` | 该公民所代表的组织。 |
| POST | `/users/me/eid/organizations` | 绑定组织（按登记号，经 XYP 核验）。 |
| DELETE | `/users/me/eid/organizations/{regNo}` | 解绑组织。 |
| GET | `/users/me/eid/organizations/{regNo}/signers` | 列出该组织的授权签署人。 |
| POST | `/users/me/eid/organizations/{regNo}/signers` | 添加组织签署人。 |
| POST | `/users/me/eid/organizations/{regNo}/signers/resend` | 重新发送签署人邀请。 |
| DELETE | `/users/me/eid/organizations/{regNo}/signers` | 移除组织签署人。 |
| GET | `/users/me/eid/summary` | eID 档案摘要。 |
| GET | `/users/me/eid/certificates` | 公民的 eID 证书。 |
| GET | `/users/me/eid/devices` | 已登记的 eID 设备。 |
| GET | `/users/me/eid/activity` | 近期 eID 活动。 |

---

## RBAC（`/api/v1/rbac`）🔒

动态角色 + 权限。`/rbac/me` 对任何已认证用户开放；其余需要 🛡️ `roles.manage`。

| 方法 | 路径 | 门禁 | 说明 |
|--------|------|-------|-------------|
| GET | `/rbac/me` | 🔒 | 调用者的有效权限（用于过滤 UI 菜单）。 |
| GET | `/rbac/roles` | 🛡️ `roles.manage` | 列出角色。 |
| GET | `/rbac/permissions` | 🛡️ `roles.manage` | 列出权限目录。 |
| POST | `/rbac/roles` | 🛡️ `roles.manage` | 创建角色。 |
| PUT | `/rbac/roles/{id}` | 🛡️ `roles.manage` | 重命名/更新角色。 |
| PUT | `/rbac/roles/{id}/permissions` | 🛡️ `roles.manage` | 替换角色的权限集。 |
| DELETE | `/rbac/roles/{id}` | 🛡️ `roles.manage` | 删除角色。 |

## 组织（`/api/v1/org`）🔒

组织 + 成员管理。归属/管理员校验在 usecase 层强制执行。

| 方法 | 路径 | 说明 |
|--------|------|-------------|
| POST | `/org/` | 创建组织。 |
| GET | `/org/` | 列出调用者的组织。 |
| GET | `/org/lookup/{regNo}` | 按登记号查询组织。 |
| GET | `/org/{id}` | 获取单个组织。 |
| GET | `/org/{id}/members` | 列出成员。 |
| POST | `/org/{id}/members` | 添加成员。 |
| PUT | `/org/{id}/members/{userID}` | 修改成员角色。 |
| DELETE | `/org/{id}/members/{userID}` | 移除成员。 |

---

## 政务服务门户（`/api/v1/gov`）🔒

面向公民的「Төрийн үйлчилгээ」门户。所有数据均按用户划分（userID 取自令牌）。
写操作端点适用约 30 次/分钟的写限流器。

| 方法 | 路径 | 说明 |
|--------|------|-------------|
| GET | `/gov/services` | 服务目录。 |
| GET | `/gov/overview` | 仪表板概览。 |
| GET | `/gov/applications` | 列出该公民的申请。 |
| POST | `/gov/applications` | 提交新申请。 |
| POST | `/gov/applications/{id}/cancel` | 取消申请。 |
| GET | `/gov/references` | 列出证明（лавлагаа）申请。 |
| POST | `/gov/references` | 申请证明。 |
| GET | `/gov/notifications` | 列出通知。 |
| POST | `/gov/notifications/read-all` | 将所有通知标记为已读。 |
| POST | `/gov/notifications/{id}/read` | 将一条通知标记为已读。 |
| GET | `/gov/payments` | 列出缴费项。 |
| POST | `/gov/payments/{id}/pay` | 支付一笔待缴费用。 |
| GET | `/gov/appointments` | 列出预约。 |
| POST | `/gov/appointments` | 预约。 |
| POST | `/gov/appointments/{id}/cancel` | 取消预约。 |

---

## 第三方集成（`/api/v1/integrations`）🔒

管理用户的第三方 OAuth 连接（Google Drive/Meet、Dropbox）。
令牌按用户加密存储（RLS）。

| 方法 | 路径 | 说明 |
|--------|------|-------------|
| GET | `/integrations/` | 列出已连接的提供方。 |
| POST | `/integrations/` | 连接某个提供方（OAuth）。 |
| GET | `/integrations/{provider}/token` | 获取某个已连接提供方的可用 access 令牌。 |
| DELETE | `/integrations/{provider}` | 断开某个提供方。 |

## 资产 — 签名 / 拉丁姓名 / 组织印章（`/api/v1/me`）🔒

挂载在 `/api/v1/me`（而非 `/users/me`）下，以免遮蔽 who-am-I 路由。
写操作适用约 30 次/分钟的写限流器。组织印章只有组织**管理员**可写。

| 方法 | 路径 | 说明 |
|--------|------|-------------|
| GET | `/me/signature` | 获取个人签名图片 URL。 |
| PUT | `/me/signature` | 设置个人签名图片。 |
| DELETE | `/me/signature` | 删除个人签名。 |
| PUT | `/me/latin-name` | 修正公民的拉丁（转写）姓名。 |
| PUT | `/me/org-name-latin/{regNo}` | 修正组织的拉丁名称。 |
| GET | `/me/orgstamp/{regNo}` | 获取组织印章图片。 |
| PUT | `/me/orgstamp/{regNo}` | 设置组织印章图片（仅组织管理员）。 |
| DELETE | `/me/orgstamp/{regNo}` | 删除组织印章图片（仅组织管理员）。 |

## Gerege Space（`/api/v1/gspace`）🔒

应用自有的按用户 SFTP 存储。在配置 `GSPACE_*` 之前返回 500。
写操作适用写限流器。

| 方法 | 路径 | 说明 |
|--------|------|-------------|
| GET | `/gspace/` | 存储概览（用量/配额、文件列表）。 |
| GET | `/gspace/download` | 下载文件。 |
| POST | `/gspace/upload` | 上传文件。 |
| DELETE | `/gspace/` | 删除文件。 |

---

## API 网关（`/api/v1/gateway`）🛡️ `gateway.manage`

上游**服务**注册表以及遥测。每个端点都需要 🔒 + 🛡️ `gateway.manage`。
网关**客户端**（原「consumers + API keys」）现已迁至下文的 **Applications** 分组；
每个服务携带一个 `scope`（应用为访问它而申请的 OAuth scope）。
旧的 Kong 风格 **routes** 与 **policies** 已移除（没有运行时代理消费它们）。
**请求日志**现在是真实的：一个中间件记录每一次实际的 `/api` 请求
（方法/路径/状态码/延迟/client_ip）— 概览由此聚合。

| 方法 | 路径 | 说明 |
|--------|------|-------------|
| GET | `/gateway/overview` | 遥测概览（服务/应用计数 + 来自真实流量的 24 小时请求统计）。 |
| GET | `/gateway/logs` | 真实请求日志（方法/路径/状态码/延迟/client_ip）。 |
| GET | `/gateway/services` | 列出服务。 |
| POST | `/gateway/services` | 创建服务（其 OAuth `scope` = `svc:`+名称）。 |
| PUT | `/gateway/services/{id}` | 更新服务。 |
| DELETE | `/gateway/services/{id}` | 删除服务。 |

## Applications（`/api/v1/applications`）🛡️ `gateway.manage`

统一的 OAuth2 **客户端注册表** — 合并取代了旧的网关「consumers + API keys」
与独立的 SSO RP 注册。每个应用都是一个 **Ory Hydra OAuth2 客户端**；
其按服务的访问权限以 OAuth **scope** 表达（`application_services` →
`gateway_services.scope`）。每个端点都需要 🔒 + 🛡️ `gateway.manage`，
且该分组**仅在配置了 Hydra 时才注册**（`ProviderConfigured()`）。

`app_type` 决定授权类型 + 认证方式：

| `app_type` | 授权类型 | 客户端 | 用途 |
|------------|-------|--------|-----|
| `web` | `authorization_code`（+ `refresh_token`） | 机密型（带 secret） | RP「Login with DAN」— 服务端 Web 应用 |
| `spa` | `authorization_code`（+ `refresh_token`） | **公开型**（PKCE，无 secret） | 浏览器 SPA |
| `native` | `authorization_code`（+ `refresh_token`） | **公开型**（PKCE，无 secret） | 移动端 / 原生应用 |
| `m2m` | `client_credentials` | 机密型（带 secret） | 服务器间调用 |

OAuth2 的 **`client_secret`**（仅机密型）**只展示一次** —
在创建 / 轮换的响应中 — 此后不再展示。

| 方法 | 路径 | 说明 |
|--------|------|-------------|
| GET | `/applications` | 列出应用。 |
| POST | `/applications` | 创建；会开通一个 Hydra OAuth2 客户端，并返回应用信息（含一次性的 `secret`，机密型）。 |
| GET | `/applications/{id}` | 获取单个应用。 |
| PUT | `/applications/{id}` | 更新覆盖层 + Hydra 客户端的目标状态。 |
| DELETE | `/applications/{id}` | 删除 Hydra 客户端 + 覆盖层。 |
| POST | `/applications/{id}/rotate-secret` | 签发新的客户端 secret，只返回一次（仅机密型）。 |
| PUT | `/applications/{id}/services` | 替换允许的网关服务 — 它们会成为该客户端的 OAuth scope。 |

**创建/更新请求体** — `{ name, app_type (web\|spa\|native\|m2m), redirect_uris[], tags[], service_ids[], enabled }`；
**设置服务请求体** — `{ service_ids[] }`。

## Gerege Core（`/api/v1/core`）🔒

对 Gerege Core（core.gerege.mn）的检索封装；服务令牌始终保留在后端。

| 方法 | 路径 | 说明 |
|--------|------|-------------|
| GET | `/core/users` | 查找用户。 |
| GET | `/core/organizations` | 查找组织。 |

---

## OIDC 提供方 — 登录/授权/登出（`/api/v1/provider`）

**仅在提供方已配置时**激活（`ProviderConfigured()`）。这是平台作为 OIDC **提供方**
运行的部分（其内置 Go provider）；Next.js BFF 的 `/login`、`/consent`、`/logout`
页面会调用这些端点。请求体上限 4 KiB。`get`/`reject`/`logout-accept`
端点以 challenge 认证（不需要 bearer）；`accept` 端点要求已登录的公民
（subject = dan 用户 ID）。

| 方法 | 路径 | 认证 | 说明 |
|--------|------|------|-------------|
| GET | `/provider/login` | challenge | 获取登录 challenge 的详情。 |
| GET | `/provider/consent` | challenge | 获取授权 challenge 的详情。 |
| POST | `/provider/login/reject` | challenge | 拒绝登录 challenge。 |
| POST | `/provider/consent/reject` | challenge | 拒绝授权 challenge。 |
| POST | `/provider/logout/accept` | challenge | 接受登出 challenge。 |
| POST | `/provider/login/accept` | 🔒 | 为已登录公民接受登录 challenge。 |
| POST | `/provider/consent/accept` | 🔒 | 接受授权 challenge。 |

---

## 管理 — 用户与 AI 提示词（`/api/v1/admin`）🔒

| 方法 | 路径 | 门禁 | 说明 |
|--------|------|-------|-------------|
| GET | `/admin/users` | 🛡️ `users.manage` | 列出用户。 |
| PUT | `/admin/users/{id}/role` | 🛡️ `users.manage` | 修改用户角色。 |
| PUT | `/admin/users/{id}/active` | 🛡️ `users.manage` | 启用/停用用户。 |
| DELETE | `/admin/users/{id}` | 🛡️ `users.manage` | 删除用户。 |
| GET | `/admin/ai/prompts` | 🛡️ `settings.manage` | 列出可配置的 AI 提示词分层。 |
| PUT | `/admin/ai/prompts/{key}` | 🛡️ `settings.manage` | 更新某个提示词层（`key` ∈ `scope` \| `instructions`）。 |
| POST | `/admin/ai/knowledge/reindex` | 🛡️ `settings.manage` | 为向量缺失或过期的知识库条目重新生成向量；返回 `{ "embedded": n }`。未配置 `GEMINI_API_KEY` 时为空操作。 |

> **命名提示。** 这个应用内的 `/api/v1/admin` 分组与下文*非 `/api` 挂载*中记录的
> 顶层 `/admin` Hydra 运维面无关 — 同一个词，不同的挂载点。

## 超级管理员（`/api/v1/superadmin`）🔒

由 `RequireSuperAdmin` 把守 — 只有 `RoleSuperAdmin` 可进入；普通管理员不可。
每次变更都会写入审计日志。

| 方法 | 路径 | 说明 |
|--------|------|-------------|
| GET | `/superadmin/admins` | 列出管理员。 |
| POST | `/superadmin/admins` | 创建管理员。 |
| PUT | `/superadmin/admins/{id}/grant` | 为已有用户授予管理员权限。 |
| DELETE | `/superadmin/admins/{id}` | 撤销管理员权限。 |

---

## 审计日志（`/api/v1/audit`）🔒 管理员

哈希链式、仅追加的审计日志；仅管理员可用（`RequireAdmin`）。

| 方法 | 路径 | 说明 |
|--------|------|-------------|
| GET | `/audit/` | 列出审计记录。 |
| GET | `/audit/verify` | 校验哈希链完整性。 |

## 安全事件（`/api/v1/security`）🔒

RASP 风格的客户端遥测。接收对任何已认证用户开放（RLS 打上 `user_id`）；
列表仅管理员可用。

| 方法 | 路径 | 门禁 | 说明 |
|--------|------|-------|-------------|
| POST | `/security/events` | 🔒 | 接收一条安全事件。 |
| GET | `/security/events` | 🔒 管理员 | 列出安全事件（`RequireAdmin`）。 |

## 站点外观（`/api/v1/site`）

站点级默认外观（强调色/字体/密度/主题）。

| 方法 | 路径 | 门禁 | 说明 |
|--------|------|-------|-------------|
| GET | `/site/appearance` | —（公开） | 读取公开的外观默认值（首页/匿名）。 |
| PUT | `/site/appearance` | 🛡️ `settings.manage` | 更新外观默认值。 |

## PDF 签署 — PAdES（`/api/v1/sign`）🔒

通过 eID Mongolia `/v3` 的服务端辅助 PAdES 签署。

| 方法 | 路径 | 说明 |
|--------|------|-------------|
| POST | `/sign/init` | 发起签署会话（返回 `id` + 批准提示）。 |
| GET | `/sign/{id}` | 轮询签署会话状态。 |
| GET | `/sign/{id}/download` | 下载已签署的 PDF。 |

---

## AI（Gemini 流水线）（`/api/v1/ai`）🔒

所有 `/ai/*` 端点都需要 bearer 令牌，并共用一个专用限流（每 IP 约 20 次/分钟）。
在配置 `GEMINI_API_KEY` 之前它们返回 500。助手运行在分层系统提示词之上 —
硬编码的防护规则 + 管理员可配置的 **scope**（范围之外一律拒绝）+ 可选的
**instructions** — 并通过 `search_knowledge` 工具把平台相关回答落到
`ai_knowledge` 表的数据上。

### POST `/ai/chat` 🔒

与助手聊天。可发送文本、语音（模型可直接理解的 base64 音频）或二者。
无状态 — 请在 `history` 中传入此前的轮次。

**请求**

```json
{ "message": "what time is it?",
  "audio": { "mime": "audio/webm", "data": "<base64>" },
  "history": [ { "role": "user", "text": "…" }, { "role": "model", "text": "…" } ] }
```

| 字段 | 规则 |
|-------|-------|
| `message` | 可选（若无 `audio` 则必填），≤ 4000 字符 |
| `audio` | 可选；`mime` ∈ webm/ogg/wav/mpeg/mp3/mp4/m4a/aac/flac，`data` base64 ≤ 约 700 KB |
| `history` | 可选，≤ 20 轮 |
| `lang` | 可选的界面语言 — `mn` \| `en` \| `zh` \| `ru`；助手以该语言回复（留空 ⇒ `mn`） |

**响应 `200`**

```json
{ "status": true, "message": "ai reply generated", "data": {
  "reply": "Одоо 12:30 цаг болж байна.",
  "steps": [ { "tool": "get_server_time", "args": {}, "result": { } } ],
  "degraded": false }, "request_id": "…" }
```

`steps` 列出模型执行过的 function call（流水线轨迹）。当 Gemini 暂时不可用时，
该端点仍返回 `200`，携带以用户语言呈现的兜底 `reply` 和 `degraded: true`。

### POST `/ai/stt` 🔒

语音转文字。**请求** `{ "audio": { "mime": "audio/webm", "data": "<base64>" } }`
**响应 `200`** — `data: { "text": "…" }`（未检测到语音时为空）。

### POST `/ai/tts` 🔒

文字转语音。**请求** `{ "text": "Сайн байна уу", "voice": "Kore" }`（`voice` 可选）
**响应 `200`** — `data: { "mime": "audio/wav", "data": "<base64 WAV>" }` — 可在浏览器中直接播放。

### POST `/ai/translate` 🔒

实时翻译。提供 `text` **或** `audio`（音频会先经过内部的 STT 步骤）；
`speak: true` 会额外返回朗读（TTS）版本。静音音频分片返回空字段 —
实时翻译界面会把录制的短片段流式发送到这里。

**请求** `{ "audio": { … }, "target_lang": "en", "speak": false }`
（`target_lang`：必填，例如 `mn|en|ru|zh|ja|ko|de`）
**响应 `200`** — `data: { "source_text": "Сайн уу", "translated": "Hello", "audio": { … } }`。

### POST `/public/ai/chat` 🌐
**无需认证。** 为首页右下角的浮动聊天挂件提供支持，访客在登录前即可询问平台相关问题。
与 `/ai/chat` 使用同一条流水线，但针对公开入口做了收紧：

| 防护 | 取值 |
|------|------|
| 限流 | 每个 IP 约 6 次/分钟（突发 3）— 与 `/ai/*` 的限流器相互独立 |
| `message` | 必填，≤ 1000 字符 |
| `history` | 可选，≤ 6 轮，每轮 ≤ 1000 字符 |
| `lang` | 可选 — `mn` \| `en` \| `zh` \| `ru` |
| `audio` | 可选的按住说话片段 — mime 白名单与 `/ai/chat` 相同，但 `data` base64 ≤ 约 250 KB（≈ 15 秒 opus）。`message` 与 `audio` 至少提供其一 |
| 工具 | 仅知识库检索 — 该 usecase 绑定的是受限工具集，读取用户数据的工具无法触及 |
| 提示词 | 额外的硬编码防护层：绝不索取个人信息，绝不声称可以访问账户 |

**响应 `200`** — 在 `/ai/chat` 的结构上增加 `transcript`。语音消息会先转写（STT），
再用转写文本进行对话，因此挂件可以显示「实际听到的内容」，而不是「语音消息」占位符。
未识别到语音时返回 `200`，`reply` 为空且 `degraded: true`（不会再调用一次 Gemini）。

### POST `/public/ai/chat/stream` 🌐
**无需认证。** 请求体相同，但以 **Server-Sent Events**（`text/event-stream`）
逐块返回 — 首页挂件使用的正是这个接口。语音只需**一次** Gemini 调用：
模型在同一轮里完成转写与回答，无需单独的 STT 往返。

| 事件 | 负载 | 含义 |
|------|------|------|
| `transcript` | `{"text": "…"}` | 语音消息的转写（一次，位于回答之前） |
| `delta` | `{"text": "…"}` | 回答的下一片段 |
| `reset` | `{}` | 丢弃此前收到的 delta（模型在调用工具前先说了话） |
| `done` | `{"degraded": false, "transcript": "…"}` | 流结束 |
| `error` | `{"message": "…"}` | 响应头发出后流失败 |

响应会带上 `X-Accel-Buffering: no`，以免反向代理把整个流缓冲成一块。
限流、负载上限、工具集与防护规则均与 `/public/ai/chat` 相同。

### POST `/public/ai/tts` 🌐
**无需认证。** 首页挂件中的「朗读」按钮 — 把助手的一条回复合成为语音。
拥有**独立**的限流（每个 IP 约 20 次/分钟，突发 8）— 语音回复是逐句合成的，
一次回答会产生多次短调用，用聊天的限流（约 6 次/分钟）会把回答拦腰截断。`text` ≤ 800 字符，
音色由服务端决定（调用方无法指定模型或音色）。

**请求** `{ "text": "…" }` · **响应 `200`** — `data: { "mime": "audio/wav", "data": "<base64 WAV>" }`

> 提示词分层的配置见上文**管理 — 用户与 AI 提示词**
> （`GET`/`PUT /api/v1/admin/ai/prompts`）。基础防护层为硬编码，且从不对外暴露。

---

## 非 `/api` 挂载

### OIDC 提供方管理面 — `/admin`（运维）

**仅在配置了 Hydra 时**激活（`ProviderConfigured()`）。这是一个挂载在 `/admin`
的普通 `http.ServeMux`（通过 `StripPrefix`，因此其内部模式写作 `/api/v1/…`）。
它管理 **RP OAuth2 客户端注册**与**管理 API key**，并以**管理 API key** 认证 —
`Authorization: Bearer <key>` 或 `X-API-Key: <key>` — 而**不是**用户 JWT。

> ⚠️ **命名冲突。** 这个顶层 `/admin` 运维面与上文应用内的 `/api/v1/admin`
> 分组是两回事。它自身的路由在 strip 之后也恰好写作 `/api/v1/…`，
> 但实际访问路径是 `/admin/api/v1/…`。

| 方法 | 路径（位于 `/admin` 下） | 说明 |
|--------|-----------------------|-------------|
| GET | `/api/v1/me` | 识别调用方的管理 key。 |
| GET | `/api/v1/clients` | 列出已注册的 RP OAuth2 客户端。 |
| POST | `/api/v1/clients` | 注册新的 RP 客户端。 |
| GET | `/api/v1/clients/{client_id}` | 获取单个 RP 客户端。 |
| PATCH | `/api/v1/clients/{client_id}` | 更新 RP 客户端。 |
| DELETE | `/api/v1/clients/{client_id}` | 删除 RP 客户端。 |
| POST | `/api/v1/clients/{client_id}/rotate-secret` | 轮换 RP 客户端 secret。 |
| GET | `/api/v1/keys` | 列出管理 API key。 |
| POST | `/api/v1/keys` | 创建管理 API key。 |
| DELETE | `/api/v1/keys/{id}` | 吊销管理 API key。 |

### 签署中继 — `/rp/sign/*`（RP 代理）

**仅在 `SIGN_RELAY_TOKEN` 与 `EID_RP_SECRET` 同时设置时**激活。
这是一个反向代理，让第三方 RP 能借助 dan 的 eID Mongolia 凭据完成签署：
调用方以 `Authorization: Bearer <token>` 提交共享中继令牌；
中继将其替换为 dan 真实的 eID RP secret 并转发到 eID Mongolia。
`/rp/sign` 与 `/rp/sign/*` 都会被处理。

---

## 运维（无 `/api/v1` 前缀）

| 方法 | 路径 | 门禁 | 说明 |
|--------|------|------|-------------|
| GET | `/health` | 公开 | 存活探针 — 进程存活即返回 200。 |
| GET | `/ready` | 公开 | 就绪探针 — ping Postgres（pgx 连接池）+ Redis。 |
| GET | `/metrics` | ObservabilityGate | Prometheus 指标输出（生产中受 bearer 把守且以 404 隐藏）。 |
| GET | `/swagger/*` · `/swagger/doc.json` | ObservabilityGate | Swagger UI + 规范（生产中受把守）。 |
| GET | `/api/` | 公开 | 根 "alive" JSON。 |

`ObservabilityGate` 要求可观测性 bearer 令牌，未认证时在生产中返回 404（而非 401）。

---

🔒 = 需要 `Authorization: Bearer <access_token>`；🛡️ = 另需具名 RBAC 权限。
用 `make swag` 从 handler 注解重新生成 swagger 规范。
（仍有七个遗留的 `auth_*` handler 带着面向密码/OTP 端点的 `@Router` 注解，
而这些端点**并未**注册 — 上文的认证面反映的是权威来源 `route_auth.go`。）

---

**Gerege Template Platform V3.0** — 由 **Gerege Systems 开发团队**与 **Claude AI** 共同打造，2026。
