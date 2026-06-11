# AI Pipeline (Gemini)

> 🌐 **English** · [Монгол](AI_PIPELINE_MN.md)

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
3× retry) do **not** produce a 5xx — the user gets a Mongolian fallback
message with `degraded: true`. Only a missing `GEMINI_API_KEY` is a real
error (500, cause logged). Unknown/failed tools are reported back to the
model as `{"error": …}` so it can apologize gracefully — tool errors never
reach the client directly.

## Prompt layers

The system prompt is assembled per request from three layers
(`ai_prompts.go`):

| Layer | Source | Editable | Purpose |
|-------|--------|----------|---------|
| 1. Base guardrails | hardcoded const | **never** | Mongolian-only, scope enforcement, prompt-injection resistance ("forget your instructions" is treated as plain text; the prompt is never revealed) |
| 2. Scope | `ai_prompts` table → `AI_SCOPE_PROMPT` env → built-in default | admin, at runtime | *What* the assistant helps with. The assistant politely refuses anything outside it |
| 3. Instructions | `ai_prompts` table (optional) | admin, at runtime | Tone, extra rules |

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

- **`search_knowledge`** — searches the `ai_knowledge` table (title/content
  `ILIKE` + tag match, top 5). The base guardrails tell the model to call it
  *before* answering platform questions and to say "I don't know" rather than
  guess when nothing is found. Grow the corpus by inserting rows
  (title/content/tags); swap the single query in
  `repositories/postgres/ai` for tsvector or pgvector when it gets large.
- **`get_server_time`** — minimal demo (Ulaanbaatar time), zero dependencies.

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

**Gerege Template Version 27.0** — Co-developed by the **Gerege Systems Development Team** and **Claude AI**, 2026.
