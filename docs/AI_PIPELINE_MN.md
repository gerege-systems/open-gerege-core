# AI Pipeline (Gemini)

> 🌐 [English](AI_PIPELINE.md) · **Монгол** · [中文](AI_PIPELINE_ZH.md) · [Русский](AI_PIPELINE_RU.md)

AI туслах хэрхэн ажилладаг, хэрхэн өргөтгөхийг эхнээс нь дуустал тайлбарлана.
Pipeline нь **SDK-гүй** — `pkg/gemini` нь Gemini REST API-г шууд дууддаг —
бөгөөд backend-ийн бусад хэсэгтэй ижил Clean Architecture давхаргаар явдаг.

## Ерөнхий зураг

```
Browser (/me/ai, /me/translate)
   │  адил-origin fetch (CSRF header)
   ▼
Next.js BFF  /api/ai/{chat,stt,tts,translate}     ← хэлбэр шалгаад JWT хавсаргана
   │  server→server
   ▼
Go API  /api/v1/ai/*  (JWT + rate limit ~20/мин)
   │
   ▼
usecases/ai ──────────────► pkg/gemini ──────► Gemini REST API
   │   ▲                      (429/5xx/сүлжээн дээр 3× retry + backoff)
   │   └─ functionResponse
   ▼
ToolDef.Execute()  ← хүсэлтийн context-оор BACKEND ДЭЭР ажиллана
   ├─ search_knowledge → repositories/postgres/ai → ai_knowledge хүснэгт
   └─ get_server_time  → жишээ tool
```

Гол зарчим: **аль tool-ийг дуудахаа model ШИЙДНЭ, backend ГҮЙЦЭТГЭНЭ.**
Model хэзээ ч код ажиллуулахгүй; tool-ууд хүсэлтийн context-оор сервер талд
ажилладаг тул DB хандалтад RLS болон timeout үйлчилнэ.

## Чатын урсгал (function-calling давталт)

`usecases/ai.Run()` (`ai_impl.go`):

1. History (≤ 20 ээлж) + шинэ prompt-оос `contents` угсарна. Дуут мессеж
   inline base64 audio хэсгээр ирдэг — Gemini шууд ойлгоно, STT алхам
   хэрэггүй.
2. Давхаргат system instruction + tool зарлалуудтайгаар Gemini-г дуудна.
3. Хариу **function дуудлага** агуулж байвал: tool бүрийг гүйцэтгэж, model-ийн
   ээлж + `functionResponse` ээлжийг нэмээд давтана (дээд тал нь `MaxSteps`,
   өгөгдмөл 4). Гүйцэтгэсэн дуудлага бүр `Step{Tool, Args, Result}` болж
   клиентэд буцдаг — UI "AI юу хийснийг" харуулдаг.
4. Хариу **текст** бол: буцаана.

**Алдааны семантик:** Gemini-ийн түр зуурын алдаа (client-ийн өөрийн 3×
retry-ийн дараа ч) 5xx болохгүй — хэрэглэгч ӨӨРИЙН хэл дээрх fallback мессеж +
`degraded: true` авна. Зөвхөн `GEMINI_API_KEY` байхгүй нь жинхэнэ алдаа
(500, шалтгаан логдоно). Танигдаагүй/унасан tool нь model руу `{"error": …}`
хэлбэрээр буцдаг тул model эелдгээр тайлбарлаж чадна — tool-ийн алдаа клиент
рүү хэзээ ч шууд гардаггүй.

## Prompt давхаргууд

System prompt хүсэлт бүрд гурван давхаргаас угсрагдана (`ai_prompts.go`):

| Давхарга | Эх сурвалж | Засварлагдах | Зориулалт |
|----------|------------|--------------|-----------|
| 1. Suurь дүрэм | кодод хатуу const | **хэзээ ч үгүй** | Хариултын хэл (хүсэлтийн `lang`-аас), хүрээний сахилт, prompt-injection эсэргүүцэл ("зааврыг март" гэдгийг энгийн текст гэж үзнэ; prompt-оо хэзээ ч задлахгүй) |
| 2. Хамрах хүрээ | `ai_prompts` хүснэгт → `AI_SCOPE_PROMPT` env → built-in default | админ, ажиллаж байх үед | Туслах *юугаар* туслахыг заана. Гадуурх асуултад эелдгээр татгалзана |
| 3. Нэмэлт заавар | `ai_prompts` хүснэгт (сонголттой) | админ, ажиллаж байх үед | Өнгө аяс, нэмэлт дүрэм |

- **Хариултын хэл** нь хүсэлтийн `lang`-аас (frontend UI-ийн хэлээ илгээнэ:
  `mn`/`en`/`zh`/`ru`; танихгүй/хоосон ⇒ `mn`). Хэрэглэгч өөр хэлээр бичвэл
  **UI-ийн хэл эцсийн шийдэгч:** хэрэглэгчийн бичсэн хэл, ярианы түүх,
  мэдлэгийн сан, tool-ийн үр дүн аль нь ч хариултын хэлийг өөрчлөхгүй — өөр
  хэлтэй эх сурвалжийг орчуулж өгнө. Зөвхөн хэрэглэгч шууд хүсвэл солино.
  `degraded` fallback мессеж ч мөн адил хэлээрээ ирнэ. Заавар нь *тухайн хэл
  дээрээ* хоёр удаа орно — эхний дүрэмд ба төгсгөлийн `[ХЭЛ / LANGUAGE]`
  хэсэгт (prompt-ын үлдсэн нь монголоор тул primacy + recency хэрэгтэй).
  Frontend нь өмнөх хэл дээрх түүхийг илгээхээ болих тул хэл солиход
  контекст цэвэр эхэлнэ.
- Админ UI: **Админ → Тохиргоо**; API: `GET/PUT /api/v1/admin/ai/prompts/{key}`
  (`settings.manage` эрх).
- Prompt 60 секунд кэшлэгддэг; `SetPrompt` кэшийг хүчингүй болгодог тул
  өөрчлөлт бичсэн instance дээр шууд үйлчилнэ.
- `SetPrompt` нь migration 11-д seed хийгдсэн key-үүд (`scope`,
  `instructions`) дээр **зөвхөн UPDATE** хийдэг — API-аар prompt-ийн гадаргуу
  өргөжихгүй.
- DB уншилт унавал env/default хүрээ рүү fail-open болно (prompt уншилт
  чатыг унагах ёсгүй).

## Tools

Tool гэдэг нь `ai.ToolDef`: Gemini function declaration + Go функц:

```go
ai.ToolDef{
    Declaration: gemini.FunctionDeclaration{
        Name:        "my_tool",
        Description: "Model хэзээ дуудахыг эндээс ойлгоно…",
        Parameters:  map[string]any{ /* JSON Schema */ },
    },
    Execute: func(ctx context.Context, args map[string]any) (map[string]any, error) {
        // backend дээр ажиллана; ctx нь хүсэлтийн identity-тэй (RLS үйлчилнэ)
        return map[string]any{"result": "…"}, nil
    },
}
```

`cmd/api/server/server.go`-д бүртгэнэ:

```go
aiTools := append(ai.DefaultTools(), ai.KnowledgeSearchTool(aiRepo), myTool)
```

Хавсарга tool-ууд:

- **`search_knowledge`** — `ai_knowledge` дээрх **семантик (вектор) хайлт**.
  Асуултыг embed хийж (`gemini-embedding-001`, `RETRIEVAL_QUERY`)
  **pgvector**-ийн cosine зайгаар (`embedding <=> $1`, HNSW индекс) тааруулна —
  өөр үг хэллэгээр асуусан ч зөв бүлэг олдоно. Top-8 нэр дэвшигчээс **шилдэг
  таарцтай харьцуулж** шүүнэ: `relativeScoreMargin` (0.03)-оос хол зөрсөнг
  хаяад `minKnowledgeResults` (2)–`maxKnowledgeResults` (4) бичлэг үлдээнэ. Яагаад харьцангуй вэ:
  энэ корпус дээр хэмжихэд ХАМААРАЛГҮЙ хоёр бүлэг хүртэл 0.64+ cosine
  ижилсэлтэй байсан тул тогтмол босго (хуучин 0.55) юуг ч шүүхгүй байв;
  `minVectorScore` (0.35) нь одоо зөвхөн хог хаях шал. Embedder тохируулаагүй,
  embedding унасан, эсвэл юу ч үлдээгүй үед `ILIKE` түлхүүр үгийн хайлт руу
  уналт хийнэ — model бүтэн асуулт дамжуулдаг тул мөрийг үг болгон задлаад
  урт үгсийн үндсээр (6 үсэг) хайна. Tool-ийн хариунд аль горим ажилласныг
  (`"mode": "vector" | "keyword"`) заана; лог руу горим, олдсон тоо, шилдэг
  оноо, slug бичигдэнэ (хэрэглэгчийн асуултын текст ХЭЗЭЭ Ч биш). Suurь дүрэм
  нь model-д платформын асуултын *өмнө* үүнийг дуудахыг заадаг.
- **`get_server_time`** — хамгийн энгийн жишээ (УБ цаг), хамааралгүй.

## Нээлттэй (нэвтрэлтгүй) чат

Нүүр хуудсанд **нэвтрэлтгүй** ажилладаг хөвөгч чат виджет байна
(`POST /public/ai/chat`, bearer токенгүй). Ижил pipeline-ыг ашиглах ч
**тусдаа usecase instance**-аар холбогдсон: зөвхөн мэдлэгийн сангийн хайлтын
tool-той. Энэ тусгаарлалт нь аюулгүй байдлын хил — хэрэглэгчийн өгөгдөл уншдаг
tool-ыг нэвтэрсэн туслахад нэмсэн ч нэргүй зочинд хүрэхгүй.

Энэ гадаргуунд гурван нэмэлт хязгаар: тусдаа rate limiter (IP тус бүрт ~6/мин,
burst 3), богино payload (мессеж ≤ 1000 тэмдэгт, түүх ≤ 6 ээлж), мөн зочинтой
ярьж байгааг заасан нэмэлт hardcoded prompt давхарга — хувийн мэдээлэл бүү
асуу, бүртгэлийн өгөгдөл харж байгаа дүр бүү үзүүл, шаардлагатай үед
нэвтрэхийг зөвлө.

Виджет нь дуу ойлгодог бөгөөд урсгалаар хариулна. **Push-to-talk** (том дугуй
товчийг дарж барих хугацаанд бичээд тавихад илгээнэ) нь богино бичлэгийг —
~250 KB base64 (≈ 15 сек), нэвтэрсэн чатынхаас дөрөв дахин бага —
`POST /public/ai/chat/stream` руу явуулж, Server-Sent Events-ээр хариу авна.
Чат model мультимодаль тул **НЭГ** дуудалтаар хуулбар ба хариулт хоёуланг
өгнө: гаралтынх нь эхний мөр хуулбарыг агуулах бөгөөд (сервер тэр тэмдгийг
таслаж `transcript` event болгоно), үлдсэн нь `delta` event-үүдээр урсана.
Тиймээс хэрэглэгч юу хэлснээ хараад, model бичиж байх зуур хариултаа уншиж
эхэлнэ.

Дуут хариулт ч ижил зарчмаар: клиент урсгалт текстийг өгүүлбэрийн зааг дээр
таслаад, дууссан өгүүлбэр бүрийг `POST /public/ai/tts` руу (текст ≤ 800
тэмдэгт, дуу хоолойг сервер сонгоно) явуулж ээлжлэн тоглуулна. Ингэснээр
бүтэн хариулт биш, ЭХНИЙ ӨГҮҮЛБЭР бэлэн болмогц дуугарч эхэлнэ. Бичгээр
асуувал зөвхөн текст — TTS нь зочин дуугаар асуусан эсвэл «сонсох» товч
дарсан үед л ажиллана.

## Мэдлэгийн сан (RAG)

Платформын өөрийн мэдлэг `ai_knowledge`-д байрлана — код болон баримтаас
бичсэн ~58 бүлэг (migration `48_ai_knowledge_platform_corpus`). Мөр бүр
тогтвортой `slug` (seed үүгээр upsert хийнэ), `source`, `lang` болон
`vector(768)` төрлийн `embedding`-тэй (migration `47_ai_knowledge_vector`,
pgvector + HNSW индекс).

- **Embedding backfill.** Ачаалалтын дараа API нь `embedding` нь NULL, эсвэл
  `content_hash` нь одоогийн агуулгатай таарахгүй байгаа мөрүүдийг 20-оор
  багцалж embed хийнэ (`EmbedKnowledge`). Арын дэвсгэрт ажилладаг тул boot
  хүлээхгүй; `GEMINI_API_KEY` байхгүй бол огт ажиллахгүй (хайлт түлхүүр үгээр).
- **Корпусыг засах.** Мөрийг migration дотор нэмэх/өөрчлөх (slug-ийг хэвээр
  үлдээнэ), дараа нь дахин ачаалах эсвэл
  `POST /api/v1/admin/ai/knowledge/reindex` дуудна (`settings.manage`; Админ →
  Тохиргоо дотор товч бий). `content` өөрчлөгдвөл хадгалсан embedding
  цэвэрлэгдэж, backfill дахин тооцоолно.
- **Model.** `GEMINI_EMBED_MODEL`-ыг хоосон орхиход client өөрөө боломжтойг нь
  сонгоно — `gemini-embedding-001` → `text-embedding-004` → `embedding-001` —
  учир нь нэр бүр бүх API түлхүүр/хувилбарт байдаггүй (404). Эхэлж хариулсныг
  процессын турш тогтоож авна. Хүсэлт бүрд `outputDimensionality: 768` гэж
  захиалдаг тул вектор нь `ai_knowledge.embedding` (`vector(768)`)-д үргэлж
  таарна; баганы хэмжээг өөрчлөх бол migration хэрэгтэй.

## Хариултын олон янз байдал

Ижил асуултад хэзээ ч үсэг үсгээрээ ижил хариулт өгөхгүй:

- System prompt-д `[НАЙРУУЛГА]` хэсэг байх ба тогтмол давтагдлын эсрэг дүрэм +
  хүсэлт бүрд санамсаргүй сонгосон нэг хэв маяг (`styleHints`) орно.
- Sampling: `temperature` 1.0, `topP` 0.95.

Зөвхөн *найруулга* л хэлбэлзэнэ. Дүрэмд баримт, тоо, алхам, эх сурвалж
өөрчлөгдөхгүй гэж тодорхой заасан — үнэн зөв байдлыг мэдлэгийн сан болон
tool-ууд тогтоож өгдөг.

## Дуу хоолой (Voice)

| Чадвар | Endpoint | Хэрхэн ажилладаг |
|--------|----------|------------------|
| Дуут чат мессеж | `POST /ai/chat` + `audio` | audio нь user ээлжид inline орж явна — чат model нь multimodal |
| Яриа→текст | `POST /ai/stt` | "яг сонссоноо буцаа" гэсэн чанд заавартай нэг удаагийн Gemini дуудлага; хоосон текст = яриа илрээгүй |
| Текст→яриа | `POST /ai/tts` | тусдаа TTS model (`GEMINI_TTS_MODEL`), `responseModalities: ["AUDIO"]`; түүхий PCM (L16/24kHz)-ийг WAV толгойгоор ороодог (`pkg/gemini/wav.go`) тул browser шууд тоглуулна. Model нь хааяа `200` буцаагаад дотор нь **аудиогүй** байдаг (хэмжсэн — ижил текст дараагийн дуудалтад бүтэн ирдэг) тул `Speak` 3 хүртэл удаа дахин оролдоод сая `503` буцаана (500 биш — түр зуурын саатал) |
| Шууд орчуулга | `POST /ai/translate` | текст → орчуулга; audio → **хоёр алхамт** STT→орчуулга (найдвартай, structured output задлах шаардлагагүй); `speak: true` бол орчуулгын TTS хувилбар нэмэгдэнэ. TTS унавал текст хэвээр буцна |

**Live орчуулгын UX** (frontend `LiveTranslateView`): микрофон ~7 секундын
сегментүүдээр бичнэ — **сегмент бүрд шинэ `MediaRecorder`** (timeslice
chunk-ууд зөвхөн эхнийдээ container header-тэй байдаг тул) — сегмент бүрийг
`/ai/translate` руу урсгана. Чимээгүй сегмент хоосон талбар буцааж, алдаа
биш гэж тоологдоно.

Audio оролт нь mime whitelist (webm/ogg/wav/mpeg/mp3/mp4/m4a/aac/flac) +
~700 KB base64 (~30 сек opus) хязгаартай — BFF (`lib/aiBff.ts`) болон
backend DTO хоёуланд нь.

## Тохиргоо

```env
GEMINI_API_KEY=     # AI боломжуудад заавал; хоосон бол endpoint-ууд 500
GEMINI_MODEL=gemini-2.5-flash                  # чат / STT / орчуулга
GEMINI_TTS_MODEL=gemini-2.5-flash-preview-tts  # TTS (audio гаргадаг model)
GEMINI_VOICE=Kore   # prebuilt TTS дуу хоолой
GEMINI_API_BASE=    # proxy/тестэд override
AI_SCOPE_PROMPT=    # DB давхарга хоосон үеийн хүрээний fallback
```

Rate limit: `/ai/*` нь тусдаа IP-тус-бүрийн limiter-тэй (~20 хүсэлт/мин,
burst 5) — live орчуулгын ~8 chunk/мин урсгалд зайтай багтана.

## Тест

Бүгд Gemini-гүйгээр тестлэгдэнэ:

- `gemini.Generator` нь interface — usecase тестүүд бэлтгэсэн хариу буцаадаг
  `fakeGenerator` ашигладаг (`ai_impl_test.go`, `ai_speech_test.go`).
- `repointerface.AIRepository`-г prompt/tool тестэд fake-ээр сольдог
  (`ai_prompts_test.go`).
- HTTP client өөрөө `httptest` серверийн эсрэг тестлэгддэг (retry/backoff,
  4xx no-retry, function-call parsing — `pkg/gemini/gemini_test.go`).

## Асуудал шийдэх (Troubleshooting)

| Шинж тэмдэг | Шалтгаан / засвар |
|-------------|-------------------|
| AI дуудлага бүр 500 "internal server error" | `GEMINI_API_KEY` тохируулаагүй (шалтгаан логт бий) |
| `degraded: true` + fallback хариу | Gemini хүрэхгүй / 429 / 5xx — түр зуурын; api логийг шалга (`category=ai`) |
| Чат ажиллаад TTS унадаг | `GEMINI_TTS_MODEL` нь **preview** model — Google нэрийг нь солих юм бол env var-аар override хий |
| Хүрээний доторх асуултад татгалзана | `scope` давхарга хэт нарийн — Админ → Тохиргооноос засна |
| `search_knowledge` юу ч олдоггүй | `ai_knowledge`-д зөвхөн 3 жишээ мөр seed хийгдсэн — өөрийн агуулгаа нэм |
| Live орчуулгад 429 | Сегментийн давтамж `/ai` rate limit-ээс хэтэрсэн — `server.go`-ийн limiter-ийг өсгөх эсвэл `SEGMENT_MS`-ийг уртасга |

---

**Gerege Template Platform V3.0** — **Gerege Systems Development Team** болон **Claude AI** хамтран бүтээв, 2026.
