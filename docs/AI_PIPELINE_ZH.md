# AI 流水线（Gemini）

> 🌐 **中文** · [English](AI_PIPELINE.md) · [Монгол](AI_PIPELINE_MN.md) · [Русский](AI_PIPELINE_RU.md)

本文说明 AI 助手端到端的工作方式，以及如何扩展它。该流水线**不使用 SDK** —
`pkg/gemini` 直接调用 Gemini REST API — 并与后端其余部分遵循同样的整洁架构分层。

## 总体图景

```
浏览器 (/me/ai, /me/translate)
   │  同源 fetch（带 CSRF 请求头）
   ▼
Next.js BFF  /api/ai/{chat,stt,tts,translate}     ← 校验数据结构，附加 JWT
   │  服务器→服务器
   ▼
Go API  /api/v1/ai/*  （JWT + 限流约 20 次/分钟）
   │
   ▼
usecases/ai ──────────────► pkg/gemini ──────► Gemini REST API
   │   ▲                      （429/5xx/网络错误时 3 次退避重试）
   │   └─ functionResponse
   ▼
ToolDef.Execute()  ← 在后端以请求上下文运行
   ├─ search_knowledge → repositories/postgres/ai → ai_knowledge 表
   └─ get_server_time  → 演示工具
```

核心原则：**由模型决定调用哪个工具，由后端负责执行。** 模型永远不会运行代码；
工具在服务端以请求的 context 运行（因此它们触碰的一切都受 RLS 和超时约束）。

## 聊天流程（function-calling 循环）

`usecases/ai.Run()`（见 `ai_impl.go`）：

1. 用历史消息（≤ 20 轮）和新提示词构建 `contents`。语音消息以内联 base64 音频片段
   传入 — Gemini 可直接理解，无需 STT 步骤。
2. 携带分层的 system instruction 和工具声明调用 Gemini。
3. 若回复包含 **function call**：逐个执行工具，追加模型轮次和一条 `functionResponse`
   轮次，然后继续循环（最多 `MaxSteps` 次，默认 4）。每次执行都会记录为
   `Step{Tool, Args, Result}` — 返回给客户端，便于界面展示“AI 做了什么”。
4. 若回复为**文本**：直接返回。

**失败语义：** Gemini 的暂时性故障（在客户端自身 3 次重试之后）**不会**产生 5xx —
用户会收到带 `degraded: true` 的、以其自身语言呈现的兜底消息。只有缺少 `GEMINI_API_KEY` 才算真正的
错误（500，原因记入日志）。未知/执行失败的工具会以 `{"error": …}` 回报给模型，
以便它得体地致歉 — 工具错误绝不会直接抵达客户端。

## 提示词分层

系统提示词按请求由三层组装而成（`ai_prompts.go`）：

| 层 | 来源 | 可编辑性 | 用途 |
|-------|--------|----------|---------|
| 1. 基础防护规则 | 硬编码常量 | **永不可编辑** | 回复语言（取自请求的 `lang`）、范围约束、抵御提示词注入（“忘掉你的指令”被当作普通文本处理；提示词绝不外泄） |
| 2. 适用范围 scope | `ai_prompts` 表 → `AI_SCOPE_PROMPT` 环境变量 → 内置默认值 | 管理员可在运行时修改 | 助手*协助什么*。范围之外的问题会被礼貌拒绝 |
| 3. 补充指令 instructions | `ai_prompts` 表（可选） | 管理员可在运行时修改 | 语气、额外规则 |

- **回复语言**取自请求的 `lang`（前端发送界面语言：`mn`/`en`/`zh`/`ru`；
  未知或留空 ⇒ `mn`）。若用户以其他语言书写，模型会跟随用户。
  `degraded` 兜底消息同样按该语言本地化。该规则还会在提示词末尾以
  `[ХЭЛ / LANGUAGE]` 小节、*用该语言本身*再写一遍 — 整个提示词是蒙古语，
  末尾的母语指令可防止模型漂移回蒙古语。
- 管理界面：**管理 → 设置**；API：`GET/PUT /api/v1/admin/ai/prompts/{key}`
  （需要 `settings.manage` 权限）。
- 提示词缓存 60 秒；`SetPrompt` 会清除缓存，因此修改会在接收写入的那个实例上立即生效。
- `SetPrompt` **只做 UPDATE**，且仅针对迁移 11 预置的键（`scope`、`instructions`）—
  提示词面无法通过 API 扩张。
- 数据库读取失败时**放行式降级**（fail open）到环境变量/默认 scope
  （提示词查询绝不能拖垮聊天功能）。

## 工具

一个工具就是一个 `ai.ToolDef`：Gemini 函数声明 + 一个 Go 函数：

```go
ai.ToolDef{
    Declaration: gemini.FunctionDeclaration{
        Name:        "my_tool",
        Description: "模型应在何时调用此工具…",
        Parameters:  map[string]any{ /* JSON Schema */ },
    },
    Execute: func(ctx context.Context, args map[string]any) (map[string]any, error) {
        // 在后端运行；ctx 携带请求的身份信息（RLS 生效）
        return map[string]any{"result": "…"}, nil
    },
}
```

在 `cmd/api/server/server.go` 中注册：

```go
aiTools := append(ai.DefaultTools(), ai.KnowledgeSearchTool(aiRepo), myTool)
```

随平台附带的工具：

- **`search_knowledge`** — 检索 `ai_knowledge` 表（标题/内容 `ILIKE` 加标签匹配，
  取前 5 条）。基础防护规则要求模型在回答平台相关问题*之前*先调用它，
  并在检索不到内容时回答“我不知道”，而不是猜测。通过插入数据行
  （title/content/tags）扩充语料库；当数据量增大时，把
  `repositories/postgres/ai` 中那条查询换成 tsvector 或 pgvector。
- **`get_server_time`** — 一个最小演示（乌兰巴托时间），零依赖。

## 语音

| 能力 | 端点 | 工作方式 |
|------------|----------|--------------|
| 语音聊天消息 | 带 `audio` 的 `POST /ai/chat` | 音频作为内联数据直接进入用户轮次 — 聊天模型本身是多模态的 |
| 语音转文字 | `POST /ai/stt` | 一次性调用 Gemini，附带严格的“逐字转写”指令；文本为空 = 没有语音 |
| 文字转语音 | `POST /ai/tts` | 使用单独的 TTS 模型（`GEMINI_TTS_MODEL`），`responseModalities: ["AUDIO"]`；原始 PCM（L16/24kHz）会被包上 WAV 头（`pkg/gemini/wav.go`），以便浏览器直接播放 |
| 实时翻译 | `POST /ai/translate` | 文本 → 直接翻译；音频 → **两步**：STT→翻译（可靠，无需解析结构化输出）；`speak: true` 会附加译文的 TTS 音频。TTS 失败时静默降级（文本照常返回） |

**实时翻译的交互设计**（前端 `LiveTranslateView`）：麦克风以约 7 秒为一段录制 —
**每段都使用全新的 `MediaRecorder`**，使每个分片都是有效的独立容器
（timeslice 分块只有第一块携带容器头）— 并逐段推送到 `/ai/translate`。
静音片段返回空字段并被丢弃，而不是报错。

音频输入按 mime 白名单校验（webm/ogg/wav/mpeg/mp3/mp4/m4a/aac/flac），
并在 BFF（`lib/aiBff.ts`）与后端 DTO 两处都限制在约 700 KB base64（约 30 秒的 opus）以内。

## 配置

```env
GEMINI_API_KEY=     # AI 功能必填；留空则相关端点返回 500
GEMINI_MODEL=gemini-2.5-flash                  # 聊天 / STT / 翻译
GEMINI_TTS_MODEL=gemini-2.5-flash-preview-tts  # TTS（具备音频能力的模型）
GEMINI_VOICE=Kore   # 预置 TTS 音色
GEMINI_API_BASE=    # 代理/测试时可覆盖
AI_SCOPE_PROMPT=    # 数据库层为空时的 scope 兜底值
```

限流：`/ai/*` 共用一个专用的按 IP 限流器（约 20 次/分钟，突发 5），
其额度足以让实时翻译（约 8 个分片/分钟）留有余量。

## 测试

一切都可以在没有 Gemini 的情况下测试：

- `gemini.Generator` 是接口 — usecase 测试使用返回脚本化响应的 `fakeGenerator`
  （`ai_impl_test.go`、`ai_speech_test.go`）。
- `repointerface.AIRepository` 在提示词/工具测试中被伪造（`ai_prompts_test.go`）。
- HTTP 客户端本身针对 `httptest` 服务器测试
  （重试/退避、4xx 不重试、function-call 解析 — `pkg/gemini/gemini_test.go`）。

## 故障排查

| 现象 | 原因 / 处理 |
|---------|-------------|
| 所有 AI 调用都返回 500 “internal server error” | 未设置 `GEMINI_API_KEY`（原因见日志） |
| 出现 `degraded: true` + 兜底回复 | 重试后 Gemini 仍不可达 / 429 / 5xx — 属于暂时性问题；查看 api 日志（`category=ai`） |
| TTS 失败但聊天正常 | `GEMINI_TTS_MODEL` 是**预览版**模型 — 若 Google 更名，请覆盖该环境变量 |
| 助手拒绝了本应受理的问题 | `scope` 提示词层过窄 — 在 管理 → 设置 中修改 |
| `search_knowledge` 检索不到内容 | `ai_knowledge` 表中只有 3 条预置演示数据 — 请插入您自己的内容 |
| 实时翻译出现 429 | 分片节奏与 `/ai` 限流冲突 — 调高 `server.go` 中的限流器或延长 `SEGMENT_MS` |

---

**Gerege Template Platform V3.0** — 由 **Gerege Systems 开发团队**与 **Claude AI** 共同打造，2026。
