# AI Pipeline (Gemini)

> 🌐 **English** · [Монгол](AI_PIPELINE_MN.md) · [中文](AI_PIPELINE_ZH.md) · [Русский](AI_PIPELINE_RU.md)

How the AI assistant works end-to-end, and how to extend it. The pipeline is
**SDK-free** — `pkg/gemini` calls the Gemini REST API directly — and follows
the same Clean Architecture layering as the rest of the backend.

## Big picture

```
Browser (/me/ai, /me/translate)
   │  same-origin fetch (CSRF header)
   ▼
Next.js BFF  /api/ai/{chat,stt,tts,translate}     ← validates shape, attaches JWT
   │  server→server
   ▼
Go API  /api/v1/ai/*  (JWT + rate limit ~20/min)
   │
   ▼
usecases/ai ──────────────► pkg/gemini ──────► Gemini REST API
   │   ▲                      (retry 3× backoff on 429/5xx/network)
   │   └─ functionResponse
   ▼
ToolDef.Execute()  ← runs ON THE BACKEND with the request context
   ├─ search_knowledge → repositories/postgres/ai → ai_knowledge table
   └─ get_server_time  → demo tool
```

Key principle: **the model decides which tool to call; the backend executes
it.** The model never runs code; tools run server-side with the request's
context (so RLS and timeouts apply to anything they touch).

## Chat flow (function-calling loop)

`usecases/ai.Run()` (see `ai_impl.go`):

1. Build `contents` from history (≤ 20 turns) + the new prompt. A voice
   message arrives as an inline base64 audio part — Gemini understands it
   directly, no STT step needed.
2. Call Gemini with the layered system instruction + tool declarations.
3. If the reply contains **function calls**: execute each tool, append the
   model turn + a `functionResponse` turn, and loop (max `MaxSteps`, default
   4). Each executed call is recorded as a `Step{Tool, Args, Result}` —
   returned to the client so the UI can show "what the AI did".
4. If the reply is **text**: return it.

**Failure semantics:** transient Gemini failures (after the client's own
3× retry) do **not** produce a 5xx — the user gets a localized fallback
message with `degraded: true`. Only a missing `GEMINI_API_KEY` is a real
error (500, cause logged). Unknown/failed tools are reported back to the
model as `{"error": …}` so it can apologize gracefully — tool errors never
reach the client directly.

## Prompt layers

The system prompt is assembled per request from three layers
(`ai_prompts.go`):

| Layer | Source | Editable | Purpose |
|-------|--------|----------|---------|
| 1. Base guardrails | hardcoded const | **never** | Reply language (from the request's `lang`), scope enforcement, prompt-injection resistance ("forget your instructions" is treated as plain text; the prompt is never revealed) |
| 2. Scope | `ai_prompts` table → `AI_SCOPE_PROMPT` env → built-in default | admin, at runtime | *What* the assistant helps with. The assistant politely refuses anything outside it |
| 3. Instructions | `ai_prompts` table (optional) | admin, at runtime | Tone, extra rules |

- **Reply language** comes from the request's `lang` (the frontend sends the UI
  language: `mn`/`en`/`zh`/`ru`; unknown or empty ⇒ `mn`). **The UI language wins:**
  the language the user typed in, the conversation history, the knowledge base and
  tool results never change it — sources in another language are translated into the
  reply language. Only an explicit user request switches it. The `degraded` fallback
  message is localized the same way. The directive is written *in the target language*
  and appears twice — in the first rule and as the closing `[ХЭЛ / LANGUAGE]` section
  (the rest of the prompt is Mongolian, so primacy + recency in the native language
  keep the model from drifting). The frontend also drops history turns from a previous
  language, so switching languages mid-chat starts a clean context.
- Admin UI: **Admin → Settings**; API: `GET/PUT /api/v1/admin/ai/prompts/{key}`
  (`settings.manage` permission).
- Prompts are cached for 60s; `SetPrompt` invalidates the cache, so changes
  apply immediately on the instance that received the write.
- `SetPrompt` is **UPDATE-only** against the keys seeded by migration 11
  (`scope`, `instructions`) — the prompt surface cannot grow from the API.
- DB read failures fail **open** to the env/default scope (a prompt lookup
  must never take chat down).

## Tools

A tool is an `ai.ToolDef`: a Gemini function declaration + a Go func:

```go
ai.ToolDef{
    Declaration: gemini.FunctionDeclaration{
        Name:        "my_tool",
        Description: "When the model should call this…",
        Parameters:  map[string]any{ /* JSON Schema */ },
    },
    Execute: func(ctx context.Context, args map[string]any) (map[string]any, error) {
        // runs on the backend; ctx carries the request identity (RLS applies)
        return map[string]any{"result": "…"}, nil
    },
}
```

Register it in `cmd/api/server/server.go`:

```go
aiTools := append(ai.DefaultTools(), ai.KnowledgeSearchTool(aiRepo), myTool)
```

Shipped tools:

- **`search_knowledge`** — **semantic (vector) search** over `ai_knowledge`.
  The question is embedded (`gemini-embedding-001`, `RETRIEVAL_QUERY`) and
  matched by cosine distance in **pgvector** (`embedding <=> $1`, HNSW index),
  so a question phrased differently still finds the right chunk. The top-8
  candidates are then filtered **relative to the best hit**: anything further
  than `relativeScoreMargin` (0.03) below it is dropped, keeping between
  `minKnowledgeResults` (2) and `maxKnowledgeResults` (4) chunks. Why relative: measured on this corpus,
  even *unrelated* chunks sit at 0.64+ cosine similarity, so a fixed threshold
  (the old 0.55) filtered nothing; `minVectorScore` (0.35) is now only a garbage
  floor. It falls back to the `ILIKE` keyword query when no embedder is
  configured, the embedding call fails, or nothing survives the filter — and
  because the model is told to pass the full question, the fallback splits it
  into words and searches the longest ones by their 6-character stem. The tool
  result says which mode ran (`"mode": "vector" | "keyword"`); the log records
  mode, hit count, top score and slug (never the user's question text). The base
  guardrails tell the model to call it *before* answering platform questions.
- **`get_server_time`** — minimal demo (Ulaanbaatar time), zero dependencies.

## Public (anonymous) chat

The landing page carries a floating chat widget that works **without login**
(`POST /public/ai/chat`, no bearer token). It reuses the same pipeline, wired
as a **separate usecase instance** with a restricted tool set — knowledge-base
search only. That separation is the security boundary: a tool that reads user
data can be added to the authenticated assistant without ever becoming
reachable by an anonymous visitor.

Three more limits apply on this surface: a dedicated rate limiter (~6 req/min
per IP, burst 3), short payloads (message ≤ 1000 chars, history ≤ 6 turns), and
an extra hardcoded prompt layer that tells the assistant it is talking to a
visitor — never ask for personal data, never claim to see account records,
point to signing in when the question needs one.

The widget is voice-capable: **push-to-talk** (hold the mic, release to send)
posts a short clip in the same `/public/ai/chat` call — the chat model is
multimodal, so there is no separate STT step. Clips are capped at ~250 KB
base64 (≈ 15 s), a quarter of what the authenticated chat accepts. Replies are
text; a per-message "listen" button calls `POST /public/ai/tts` (text ≤ 800
chars, server-chosen voice), so speech synthesis only ever runs when a visitor
explicitly asks for it.

## Knowledge base (RAG)

The platform's own knowledge lives in `ai_knowledge` — ~58 chunks written from
the code and docs (migration `48_ai_knowledge_platform_corpus`). Each row has a
stable `slug` (the seed upserts on it), `source`, `lang` and a `vector(768)`
`embedding` (migration `47_ai_knowledge_vector`, pgvector + HNSW index).

- **Embedding backfill.** On boot the API embeds every row whose `embedding` is
  NULL or whose `content_hash` no longer matches the current text, in batches of
  20 (`EmbedKnowledge`). It runs in the background — boot never blocks on it, and
  without `GEMINI_API_KEY` it is a no-op (search stays on keywords).
- **Editing the corpus.** Add or change rows in a migration (keep the `slug`),
  then either restart or call `POST /api/v1/admin/ai/knowledge/reindex`
  (`settings.manage`, also a button under Admin → Settings). Changing `content`
  clears the stored embedding so the backfill recomputes it.
- **Model.** Leave `GEMINI_EMBED_MODEL` empty and the client picks a working
  model itself — `gemini-embedding-001` → `text-embedding-004` → `embedding-001`
  — because a given name may not exist for your API key/version (404). The first
  one that answers is cached for the process. Every request asks for
  `outputDimensionality: 768`, so the vector always fits `ai_knowledge.embedding`
  (`vector(768)`); changing that column's size needs a migration.

## Answer variety

The same question never gets a byte-identical answer twice:

- The system prompt carries a `[НАЙРУУЛГА]` section with a fixed anti-repetition
  rule plus one randomly chosen style hint per request (`styleHints`).
- Sampling: `temperature` 1.0, `topP` 0.95.

Only the *wording* varies. The rule states explicitly that facts, numbers, steps
and sources stay the same — grounding comes from the knowledge base and tools,
not from the phrasing.

## Voice

| Capability | Endpoint | How it works |
|------------|----------|--------------|
| Voice chat message | `POST /ai/chat` with `audio` | audio goes straight into the user turn as inline data — the chat model is multimodal |
| Speech-to-text | `POST /ai/stt` | one-shot Gemini call with a strict "transcribe verbatim" instruction; empty text = no speech |
| Text-to-speech | `POST /ai/tts` | separate TTS model (`GEMINI_TTS_MODEL`) with `responseModalities: ["AUDIO"]`; the raw PCM (L16/24kHz) is wrapped into a WAV header (`pkg/gemini/wav.go`) so browsers can play it directly |
| Live translation | `POST /ai/translate` | text → translate; audio → **two-step** STT→translate (reliable, no structured-output parsing); `speak: true` adds a TTS rendering of the translation. TTS failure degrades silently (text still returned) |

**Live translation UX** (frontend `LiveTranslateView`): the mic records ~7s
segments — a **fresh `MediaRecorder` per segment** so every chunk is a valid
standalone container (timeslice chunks only carry the header in the first
chunk) — and streams each one to `/ai/translate`. Silent segments return
empty fields and are dropped, not errored.

Audio input is whitelisted by mime (webm/ogg/wav/mpeg/mp3/mp4/m4a/aac/flac)
and capped at ~700 KB base64 (~30s of opus) in both the BFF (`lib/aiBff.ts`)
and the backend DTO.

## Configuration

```env
GEMINI_API_KEY=     # required for AI features; empty = endpoints return 500
GEMINI_MODEL=gemini-2.5-flash                  # chat / STT / translate
GEMINI_TTS_MODEL=gemini-2.5-flash-preview-tts  # TTS (audio-capable model)
GEMINI_VOICE=Kore   # prebuilt TTS voice
GEMINI_API_BASE=    # override for proxies/testing
AI_SCOPE_PROMPT=    # scope fallback when the DB layer is empty
```

Rate limit: `/ai/*` shares a dedicated per-IP limiter (~20 req/min, burst 5)
sized so live translation (~8 chunks/min) fits with headroom.

## Testing

Everything is testable without Gemini:

- `gemini.Generator` is an interface — usecase tests use a `fakeGenerator`
  returning scripted responses (`ai_impl_test.go`, `ai_speech_test.go`).
- `repointerface.AIRepository` is faked for prompt/tool tests
  (`ai_prompts_test.go`).
- The HTTP client itself is tested against `httptest` servers
  (retry/backoff, 4xx no-retry, function-call parsing — `pkg/gemini/gemini_test.go`).

## Troubleshooting

| Symptom | Cause / fix |
|---------|-------------|
| 500 "internal server error" on every AI call | `GEMINI_API_KEY` not set (cause is in the logs) |
| `degraded: true` + fallback reply | Gemini unreachable / 429 / 5xx after retries — transient; check api logs (`category=ai`) |
| TTS fails while chat works | `GEMINI_TTS_MODEL` is a **preview** model — if Google renames it, override the env var |
| Assistant refuses an on-topic question | The `scope` prompt layer is too narrow — edit it in Admin → Settings |
| `search_knowledge` finds nothing | The `ai_knowledge` table only has the 3 seeded demo rows — insert your own content |
| 429 on live translation | Segment cadence vs the `/ai` rate limit — raise the limiter in `server.go` or lengthen `SEGMENT_MS` |

---

**Gerege Template Platform V3.0** — Co-developed by the **Gerege Systems Development Team** and **Claude AI**, 2026.
