# API Contract

> 🌐 [English](API_CONTRACT.md) · **Монгол**

**Government Template Platform V3.0** (Цахим засаглалыг бүтээх суурь)-ийн REST API
лавлагаа — цахим засаглалын үйлчилгээг дээр нь босгох production-д бэлэн суурь
(Clean-Architecture Go backend + Next.js BFF + Gemini AI). Энэ лавлагаа нь түүний
жишиг deployment болох **Government Template Platform** (template.dgov.mn) — eID-д
суурилсан төрийн үйлчилгээний платформыг тусгана. Амьд, автоматаар
үүсгэгддэг spec-ийг `GET /swagger/`-ээр үзүүлнэ (эх сурвалж: `docs/swagger.json`).

> **Зам (path)-ын тухай тэмдэглэл.** Доорх модуль бүр `/api` бүлгийн дор
> суурьшдаг ба route бүлэг бүр `/v1` угтвар нэмдэг тул бодит хүсэлтийн зам нь
> `/api/v1/<бүлэг>/…` болно — swagger-ийн `@Router` annotation харьцангуйгаар
> бичигдсэн ч (ж: annotation `/auth/eid/start` → бодит зам
> `/api/v1/auth/eid/start`). Энэ баримтын хүснэгтүүд **бүтэн** замыг харуулна.

## Дүрэм (Conventions)

- **Base URL:** `http://localhost:8080/api/v1`
- **Content-Type:** `application/json`
- **Auth:** хамгаалагдсан endpoint-ууд `Authorization: Bearer <access_token>`
  шаардана (токеныг доорх eID / Google нэвтрэлтийн урсгалууд гаргана)
- **Rate limit (IP тус бүр):** `/auth/*` ~5 хүсэлт/мин, `/auth/eid/poll` нь
  тусдаа сул ~60 хүсэлт/мин (long-poll өөрөө 429 болохгүйн тулд), `/ai/*` ~20
  хүсэлт/мин, `/gov`, `/gspace`, `/me`, `/users/me/eid`-ийн **мутаци**
  endpoint-ууд ~30 хүсэлт/мин (хэтэрвэл 429)
- **Body хязгаар:** `/auth/*`, `/provider/*` bodies нь 4 KiB; бусад нь
  1 MiB

### Хариултын дугтуй (envelope)

Бүх хариулт нэг дугтуй ашиглана:

```json
{
  "status": true,
  "message": "human-readable summary",
  "data": { },
  "request_id": "b1d2…"
}
```

- `status` — амжилтад `true`, алдаанд `false`
- `data` — амжилтад байна (алдаанд орхигдоно/null)
- `request_id` — корреляцийн id (`X-Request-ID` header-т мөн давхардуулна)

### Статус кодууд

| Код | Утга | Хэзээ |
|------|---------|------|
| 200 | OK | Амжилттай унших / үйлдэл |
| 201 | Created | Ресурс үүсгэгдсэн |
| 400 | Bad Request | Гажуудсан body |
| 401 | Unauthorized | Токен байхгүй/буруу/хугацаа дууссан |
| 403 | Forbidden | Нэвтэрсэн ч шаардлагатай role/permission байхгүй |
| 404 | Not Found | Ресурс байхгүй |
| 409 | Conflict | Давхардал / төлөвийн зөрчил |
| 422 | Unprocessable Entity | Валидаци амжилтгүй (доор үз) |
| 429 | Too Many Requests | Rate limit хэтэрсэн |
| 500 | Internal Server Error | Санамсаргүй алдаа (шалтгааныг лог-д, ерөнхий мессеж буцаана) |

### Валидацийн алдаа (422)

Талбарын дэлгэрэнгүйг `data.errors`-т буцаана — энэ нь `{ field, tag, message }`
объектуудын **массив**. `field` нь JSON tag нэр:

```json
{
  "status": false,
  "message": "validation failed",
  "data": { "errors": [ { "field": "target_lang", "tag": "required", "message": "target_lang is required" } ] },
  "request_id": "b1d2…"
}
```

### Тэмдэглэгээ (Legend)

- 🔒 — `Authorization: Bearer <access_token>` шаардана
- 🛡️ `perm` — нэрлэсэн RBAC permission-ыг нэмж шаардана (**admin** role нь бүх
  permission каталогийг автоматаар давна; тэмдэглэсэн газар **super admin**
  шаардана). Path параметрийг `{хаалт}`-аар харуулна.

---

## Нэвтрэлт (`/api/v1/auth`)

**Цорын ганц** нэвтрэх арга бол **Login with eID** (eID Mongolia Relying Party),
дээр нь **Google OAuth** аккаунт холболт.
Нууц үг, имэйл/OTP, бүртгэлийн гадаргуу байхгүй. Энэ бүлэг rate-limit болон body
хязгаартай (4 KiB); нэвтрэхээс өмнөх урсгалууд service RLS identity-ээр ажиллана.

| Method | Path | Auth | Тайлбар |
|--------|------|------|-------------|
| POST | `/auth/eid/start` | — | eID нэвтрэлт эхлүүлнэ; QR код / мобайл deep-link болон poll хийх session токен буцаана. |
| POST | `/auth/eid/start-id` | — | Иргэний РД-аар eID нэвтрэлт эхлүүлж, бүртгэлтэй төхөөрөмж рүү зөвшөөрлийн push илгээнэ. |
| POST | `/auth/eid/poll` | — | Хүлээгдэж буй eID session-ийг long-poll хийнэ (~25 с барина); `PENDING` эсвэл зөвшөөрсний дараа access + refresh токен хос буцаана. Тусдаа сул limiter. |
| POST | `/auth/google` | — | Google OAuth callback — `code`-ыг солиод, Google аккаунтыг eID хэрэглэгчид холбоно (эсвэл нэвтрүүлнэ). |
| DELETE | `/auth/google/link` | 🔒 | Нэвтэрсэн хэрэглэгчийн Google аккаунтыг САЛГАХ (холбох нь зөвхөн login урсгалаар). |
| POST | `/auth/refresh` | — | Хүчинтэй refresh токеноор токен хосыг эргүүлнэ. Refresh нь токеныг **эргүүлдэг** тул хуучин refresh токен хүчингүй болно. |
| POST | `/auth/logout` | — | Өгсөн refresh токеныг цуцлана; `access_token`-ыг мөн өгвөл түүний jti-г Redis deny-list-д нэмж шууд ажиллагаагүй болгоно. |

Амжилтад login/refresh урсгалууд токен хосыг `data`-д буцаана (`token` = access
JWT, `refresh_token` = refresh JWT) хэрэглэгчийн identity-ийн хамт (`id`,
`role_id`, нэрийн талбарууд).

---

## Хэрэглэгч (`/api/v1/users`)

| Method | Path | Auth | Тайлбар |
|--------|------|------|-------------|
| GET | `/users/me` | 🔒 | Нэвтэрсэн хэрэглэгчийн профайл (`id`, `username`, `email`, `role_id`, timestamps). |

## eID профайл (`/api/v1/users/me/eid`) 🔒

Нэвтэрсэн иргэний eID нэмэлт мэдээлэл. Мутаци (`POST`/`DELETE`) endpoint-ууд ~30
хүсэлт/мин write limiter авна; уншилт хязгааргүй.

| Method | Path | Тайлбар |
|--------|------|-------------|
| GET | `/users/me/eid/organizations` | Иргэний төлөөлдөг байгууллагууд. |
| POST | `/users/me/eid/organizations` | Байгууллага холбох (РД-аар, XYP-ээр баталгаажуулна). |
| DELETE | `/users/me/eid/organizations/{regNo}` | Байгууллага салгах. |
| GET | `/users/me/eid/organizations/{regNo}/signers` | Байгууллагын эрх бүхий гарын үсэг зурагчид. |
| POST | `/users/me/eid/organizations/{regNo}/signers` | Гарын үсэг зурагч нэмэх. |
| POST | `/users/me/eid/organizations/{regNo}/signers/resend` | Гарын үсэг зурагчийн урилга дахин илгээх. |
| DELETE | `/users/me/eid/organizations/{regNo}/signers` | Гарын үсэг зурагч хасах. |
| GET | `/users/me/eid/summary` | eID профайлын хураангуй. |
| GET | `/users/me/eid/certificates` | Иргэний eID гэрчилгээнүүд. |
| GET | `/users/me/eid/devices` | Бүртгэлтэй eID төхөөрөмжүүд. |
| GET | `/users/me/eid/activity` | Сүүлийн eID үйл ажиллагаа. |

---

## RBAC (`/api/v1/rbac`) 🔒

Динамик role + permission. `/rbac/me` нь нэвтэрсэн хэрэглэгч бүрт нээлттэй; бусад
нь 🛡️ `roles.manage` шаардана.

| Method | Path | Guard | Тайлбар |
|--------|------|-------|-------------|
| GET | `/rbac/me` | 🔒 | Дуудагчийн үр дүнтэй permission-ууд (UI цэс шүүхэд). |
| GET | `/rbac/roles` | 🛡️ `roles.manage` | Role-уудыг жагсаах. |
| GET | `/rbac/permissions` | 🛡️ `roles.manage` | Permission каталог жагсаах. |
| POST | `/rbac/roles` | 🛡️ `roles.manage` | Role үүсгэх. |
| PUT | `/rbac/roles/{id}` | 🛡️ `roles.manage` | Role нэр солих/шинэчлэх. |
| PUT | `/rbac/roles/{id}/permissions` | 🛡️ `roles.manage` | Role-ийн permission багцыг солих. |
| DELETE | `/rbac/roles/{id}` | 🛡️ `roles.manage` | Role устгах. |

## Байгууллага (`/api/v1/org`) 🔒

Байгууллага + гишүүнчлэлийн удирдлага. Эзэмшигч/админ шалгалт usecase давхаргад
хэрэгжинэ.

| Method | Path | Тайлбар |
|--------|------|-------------|
| POST | `/org/` | Байгууллага үүсгэх. |
| GET | `/org/` | Дуудагчийн байгууллагуудыг жагсаах. |
| GET | `/org/lookup/{regNo}` | РД-аар байгууллага хайх. |
| GET | `/org/{id}` | Нэг байгууллага авах. |
| GET | `/org/{id}/members` | Гишүүд жагсаах. |
| POST | `/org/{id}/members` | Гишүүн нэмэх. |
| PUT | `/org/{id}/members/{userID}` | Гишүүний role солих. |
| DELETE | `/org/{id}/members/{userID}` | Гишүүн хасах. |

---

## Төрийн үйлчилгээний портал (`/api/v1/gov`) 🔒

Иргэний "Төрийн үйлчилгээ" портал. Бүх өгөгдөл per-user (userID токеноос).
Мутаци endpoint-ууд ~30 хүсэлт/мин write limiter авна.

| Method | Path | Тайлбар |
|--------|------|-------------|
| GET | `/gov/services` | Үйлчилгээний каталог. |
| GET | `/gov/overview` | Dashboard тойм. |
| GET | `/gov/applications` | Иргэний хүсэлтүүд жагсаах. |
| POST | `/gov/applications` | Шинэ хүсэлт илгээх. |
| POST | `/gov/applications/{id}/cancel` | Хүсэлт цуцлах. |
| GET | `/gov/references` | Лавлагааны хүсэлтүүд жагсаах. |
| POST | `/gov/references` | Лавлагаа хүсэх. |
| GET | `/gov/notifications` | Мэдэгдэл жагсаах. |
| POST | `/gov/notifications/read-all` | Бүх мэдэгдлийг уншсан гэж тэмдэглэх. |
| POST | `/gov/notifications/{id}/read` | Нэг мэдэгдлийг уншсан гэж тэмдэглэх. |
| GET | `/gov/payments` | Төлбөрүүд жагсаах. |
| POST | `/gov/payments/{id}/pay` | Хүлээгдэж буй төлбөр төлөх. |
| GET | `/gov/appointments` | Цаг захиалгууд жагсаах. |
| POST | `/gov/appointments` | Цаг захиалах. |
| POST | `/gov/appointments/{id}/cancel` | Цаг захиалга цуцлах. |

---

## Интеграциуд (`/api/v1/integrations`) 🔒

Хэрэглэгчийн гуравдагч этгээдийн OAuth холболтуудыг удирдах (Google Drive/Meet,
Dropbox). Токенийг per-user шифрлэн хадгална (RLS).

| Method | Path | Тайлбар |
|--------|------|-------------|
| GET | `/integrations/` | Холбогдсон провайдерууд жагсаах. |
| POST | `/integrations/` | Провайдер холбох (OAuth). |
| GET | `/integrations/{provider}/token` | Холбогдсон провайдерийн ашиглах боломжтой токен авах. |
| DELETE | `/integrations/{provider}` | Провайдер салгах. |

## Assets — гарын үсэг / латин нэр / тамга (`/api/v1/me`) 🔒

`/users/me` route-ийг шадовлахгүйн тулд `/api/v1/me`-д (биш `/users/me`)
суурьшсан. Мутациуд ~30 хүсэлт/мин write limiter авна. Байгууллагын тамгыг зөвхөн
байгууллагын **admin** бичнэ.

| Method | Path | Тайлбар |
|--------|------|-------------|
| GET | `/me/signature` | Хувь хүний гарын үсгийн зургийн URL авах. |
| PUT | `/me/signature` | Хувь хүний гарын үсгийн зураг тавих. |
| DELETE | `/me/signature` | Хувь хүний гарын үсэг устгах. |
| PUT | `/me/latin-name` | Иргэний латин (галиглалт) нэр засах. |
| PUT | `/me/org-name-latin/{regNo}` | Байгууллагын латин нэр засах. |
| GET | `/me/orgstamp/{regNo}` | Байгууллагын тамганы дардасын зураг авах. |
| PUT | `/me/orgstamp/{regNo}` | Байгууллагын тамганы дардас тавих (зөвхөн org admin). |
| DELETE | `/me/orgstamp/{regNo}` | Байгууллагын тамганы дардас устгах (зөвхөн org admin). |

## Gerege Space (`/api/v1/gspace`) 🔒

Апп-ын өөрийн per-user SFTP хадгалалт. `GSPACE_*` тохируулаагүй бол 500 буцаана.
Мутациуд write limiter авна.

| Method | Path | Тайлбар |
|--------|------|-------------|
| GET | `/gspace/` | Хадгалалтын тойм (ашиглалт/квот, файлын жагсаалт). |
| GET | `/gspace/download` | Файл татах. |
| POST | `/gspace/upload` | Файл байршуулах. |
| DELETE | `/gspace/` | Файл устгах. |

---

## API Gateway (`/api/v1/gateway`) 🛡️ `gateway.manage`

Upstream **service** бүртгэл + телеметр. Бүх endpoint 🔒 + 🛡️ `gateway.manage`
шаардана. Gateway **client**-ууд (хуучин "consumers + API keys") одоо доорх
**Applications** бүлэгт шилжсэн; service бүр `scope`-той. Хуучин Kong маягийн
**routes** ба **policies**-ийг хассан (runtime proxy тэдгээрийг ашигладаггүй
байсан). **Хүсэлтийн лог** одоо бодит: middleware бодит `/api` хүсэлт бүрийг
(method/path/status/latency/client_ip) бичдэг — тойм үүнээс нэгтгэнэ.

| Method | Path | Тайлбар |
|--------|------|-------------|
| GET | `/gateway/overview` | Телеметрийн тойм (service/апп тоо + 24ц бодит хүсэлтийн статистик). |
| GET | `/gateway/logs` | Бодит хүсэлтийн лог (method/path/status/latency/client_ip). |
| GET | `/gateway/services` | Service жагсаах. |
| POST | `/gateway/services` | Service үүсгэх (OAuth `scope` = `svc:`+нэр). |
| PUT | `/gateway/services/{id}` | Service шинэчлэх. |
| DELETE | `/gateway/services/{id}` | Service устгах. |

## Applications (`/api/v1/applications`) 🛡️ `gateway.manage`

Нэгдсэн OAuth2 **client бүртгэл** — хуучин gateway "consumers + API keys" болон
тусдаа SSO RP бүртгэлийг нэгтгэн орлуулсан. Application бүр нь **Ory Hydra OAuth2
client**; service тус бүрийн хандалтыг OAuth **scope**-оор илэрхийлнэ
(`application_services` → `gateway_services.scope`). Бүх endpoint 🔒 + 🛡️
`gateway.manage` шаардах ба энэ бүлэг нь **зөвхөн Hydra тохируулагдсан үед**
(`ProviderConfigured()`) бүртгэгдэнэ.

`app_type` нь grant + auth-method-ыг сонгоно:

| `app_type` | Grant | Client | Хэрэглээ |
|------------|-------|--------|-----|
| `web` | `authorization_code` (+ `refresh_token`) | confidential (secret) | RP "Login with DAN" — сервер талын web апп |
| `spa` | `authorization_code` (+ `refresh_token`) | **public** (PKCE, secret-гүй) | Браузер SPA |
| `native` | `authorization_code` (+ `refresh_token`) | **public** (PKCE, secret-гүй) | Мобайл / native апп |
| `m2m` | `client_credentials` | confidential (secret) | Server-to-server |

OAuth2 **`client_secret`** (зөвхөн confidential төрөл) нь create / rotate-ийн
хариунд **нэг удаа** харагдана — дахин хэзээ ч биш.

| Method | Path | Тайлбар |
|--------|------|-------------|
| GET | `/applications` | Application-уудыг жагсаах. |
| POST | `/applications` | Үүсгэх; Hydra OAuth2 client үүсгээд апп-ыг нэг удаагийн `secret`-ийн хамт буцаана (confidential төрөл). |
| GET | `/applications/{id}` | Нэг application авах. |
| PUT | `/applications/{id}` | Overlay + Hydra client-ийн хүссэн төлөвийг шинэчлэх. |
| DELETE | `/applications/{id}` | Hydra client + overlay-г устгах. |
| POST | `/applications/{id}/rotate-secret` | Шинэ client secret гаргаж нэг удаа буцаах (зөвхөн confidential). |
| PUT | `/applications/{id}/services` | Зөвшөөрсөн gateway service-үүдийг солих — тэдгээр нь client-ийн OAuth scope болно. |

**Create/update body** — `{ name, app_type (web\|spa\|native\|m2m), redirect_uris[], tags[], service_ids[], enabled }`; **set-services body** — `{ service_ids[] }`.

## Gerege Core (`/api/v1/core`) 🔒

Gerege Core (core.gerege.mn)-ийн хайлтын wrap; service токен backend-д нуугдана.

| Method | Path | Тайлбар |
|--------|------|-------------|
| GET | `/core/users` | Хэрэглэгч хайх. |
| GET | `/core/organizations` | Байгууллага хайх. |

---

## OIDC provider — login/consent/logout (`/api/v1/provider`)

Зөвхөн **provider тохируулагдсан үед** (`ProviderConfigured()`) идэвхжинэ. Энэ нь
платформ өөрөө OIDC **provider** болсон нь (өөрийн Go provider); Next.js BFF-ийн
`/login`, `/consent`, `/logout` хуудсууд дуудна. Body хязгаартай (4 KiB).
`get`/`reject`/`logout-accept` нь challenge-аар баталгаажна (bearer-гүй);
`accept` endpoint-ууд нэвтэрсэн иргэн шаардана (subject = dan user ID).

| Method | Path | Auth | Тайлбар |
|--------|------|------|-------------|
| GET | `/provider/login` | challenge | Login challenge-ийн мэдээлэл авах. |
| GET | `/provider/consent` | challenge | Consent challenge-ийн мэдээлэл авах. |
| POST | `/provider/login/reject` | challenge | Login challenge-ийг татгалзах. |
| POST | `/provider/consent/reject` | challenge | Consent challenge-ийг татгалзах. |
| POST | `/provider/logout/accept` | challenge | Logout challenge-ийг зөвшөөрөх. |
| POST | `/provider/login/accept` | 🔒 | Нэвтэрсэн иргэний login challenge-ийг зөвшөөрөх. |
| POST | `/provider/consent/accept` | 🔒 | Consent challenge-ийг зөвшөөрөх. |

---

## Админ — хэрэглэгч & AI prompt (`/api/v1/admin`) 🔒

| Method | Path | Guard | Тайлбар |
|--------|------|-------|-------------|
| GET | `/admin/users` | 🛡️ `users.manage` | Хэрэглэгч жагсаах. |
| PUT | `/admin/users/{id}/role` | 🛡️ `users.manage` | Хэрэглэгчийн role солих. |
| PUT | `/admin/users/{id}/active` | 🛡️ `users.manage` | Хэрэглэгч идэвхжүүлэх/идэвхгүйжүүлэх. |
| DELETE | `/admin/users/{id}` | 🛡️ `users.manage` | Хэрэглэгч устгах. |
| GET | `/admin/ai/prompts` | 🛡️ `settings.manage` | Тохируулж болох AI prompt давхаргуудыг жагсаах. |
| PUT | `/admin/ai/prompts/{key}` | 🛡️ `settings.manage` | Prompt давхарга шинэчлэх (`key` ∈ `scope` \| `instructions`). |

> **Нэрийн тэмдэглэл.** Апп доторх энэ `/api/v1/admin` бүлэг нь доор *Non-`/api`
> mounts*-д баримтжуулсан дээд түвшний `/admin` Hydra оператор гадаргуутай огт
> өөр — ижил үг, өөр mount.

## Super admin (`/api/v1/superadmin`) 🔒

`RequireSuperAdmin`-ээр хамгаалагдсан — зөвхөн `RoleSuperAdmin` орно; энгийн
admin ч болохгүй. Мутаци бүрийг audit log-д бичнэ.

| Method | Path | Тайлбар |
|--------|------|-------------|
| GET | `/superadmin/admins` | Админуудыг жагсаах. |
| POST | `/superadmin/admins` | Админ үүсгэх. |
| PUT | `/superadmin/admins/{id}/grant` | Одоо байгаа хэрэглэгчид админ эрх олгох. |
| DELETE | `/superadmin/admins/{id}` | Админ эрх хасах. |

---

## Audit log (`/api/v1/audit`) 🔒 admin

Hash-chained, append-only audit log; зөвхөн admin (`RequireAdmin`).

| Method | Path | Тайлбар |
|--------|------|-------------|
| GET | `/audit/` | Audit бичлэгүүд жагсаах. |
| GET | `/audit/verify` | Hash chain-ийн бүрэн бүтэн байдлыг шалгах. |

## Security events (`/api/v1/security`) 🔒

RASP маягийн клиент телеметр. Ингест нь нэвтэрсэн хэрэглэгч бүрт нээлттэй (RLS нь
`user_id` тавина); жагсаалт зөвхөн admin.

| Method | Path | Guard | Тайлбар |
|--------|------|-------|-------------|
| POST | `/security/events` | 🔒 | Security event ингест хийх. |
| GET | `/security/events` | 🔒 admin | Security event жагсаах (`RequireAdmin`). |

## Сайтын харагдац (`/api/v1/site`)

Сайт даяарх default харагдац (accent/font/density/theme).

| Method | Path | Guard | Тайлбар |
|--------|------|-------|-------------|
| GET | `/site/appearance` | — (нийтийн) | Нийтийн харагдацын default унших (landing/anon). |
| PUT | `/site/appearance` | 🛡️ `settings.manage` | Харагдацын default шинэчлэх. |

## PDF гарын үсэг — PAdES (`/api/v1/sign`) 🔒

eID Mongolia `/v3`-ээр серверийн туслалцаатай PAdES гарын үсэг.

| Method | Path | Тайлбар |
|--------|------|-------------|
| POST | `/sign/init` | Гарын үсгийн session эхлүүлэх (`id` + зөвшөөрлийн prompt буцаана). |
| GET | `/sign/{id}` | Гарын үсгийн session-ийн төлөв poll хийх. |
| GET | `/sign/{id}/download` | Гарын үсэг зурсан PDF татах. |

---

## AI (Gemini pipeline) (`/api/v1/ai`) 🔒

Бүх `/ai/*` endpoint bearer токен шаардах ба тусдаа rate limit (~20 хүсэлт/мин,
IP тус бүр) хуваалцана. `GEMINI_API_KEY` тохируулах хүртэл 500 буцаана.
Ассистент нь давхаргалсан system prompt дээр ажиллана — hardcoded guardrail +
админ-тохируулга бүхий **scope** (үүнээс гадуурхыг татгалзана) + сонголтот
**instructions** — ба платформын хариултыг `ai_knowledge` хүснэгтэд
`search_knowledge` tool-оор үндэслэнэ.

### POST `/ai/chat` 🔒
Ассистенттэй чатлах. Текст, дуу хоолой (model шууд ойлгодог base64 audio) эсвэл
хоёуланг илгээ. Stateless — өмнөх ээлжүүдийг `history`-д дамжуул.

**Хүсэлт**
```json
{ "message": "what time is it?",
  "audio": { "mime": "audio/webm", "data": "<base64>" },
  "history": [ { "role": "user", "text": "…" }, { "role": "model", "text": "…" } ] }
```
| Талбар | Дүрэм |
|-------|-------|
| `message` | сонголтот (`audio` байхгүй бол заавал), ≤ 4000 тэмдэгт |
| `audio` | сонголтот; `mime` ∈ webm/ogg/wav/mpeg/mp3/mp4/m4a/aac/flac, `data` base64 ≤ ~700 KB |
| `history` | сонголтот, ≤ 20 ээлж |

**Хариулт `200`**
```json
{ "status": true, "message": "ai reply generated", "data": {
  "reply": "Одоо 12:30 цаг болж байна.",
  "steps": [ { "tool": "get_server_time", "args": {}, "result": { } } ],
  "degraded": false }, "request_id": "…" }
```
`steps` нь model-ийн гүйцэтгэсэн функцийн дуудлагуудыг (pipeline trace) харуулна.
Gemini түр боломжгүй үед endpoint нь `200`-ыг монгол fallback `reply` болон
`degraded: true`-тэй буцаана.

### POST `/ai/stt` 🔒
Speech-to-text. **Хүсэлт** `{ "audio": { "mime": "audio/webm", "data": "<base64>" } }`
**Хариулт `200`** — `data: { "text": "…" }` (яриа илрээгүй бол хоосон).

### POST `/ai/tts` 🔒
Text-to-speech. **Хүсэлт** `{ "text": "Сайн байна уу", "voice": "Kore" }` (`voice` сонголтот)
**Хариулт `200`** — `data: { "mime": "audio/wav", "data": "<base64 WAV>" }` — браузерт шууд тоглоно.

### POST `/ai/translate` 🔒
Амьд орчуулга. `text` **эсвэл** `audio` өг (audio эхлээд дотоод STT-ээр орно);
`speak: true` нь орчуулгын дуутай (TTS) хувилбарыг нэмж буцаана. Чимээгүй audio
chunk хоосон талбар буцаана — амьд орчуулгын UI богино бичлэгийг энд стриминг
хийнэ.

**Хүсэлт** `{ "audio": { … }, "target_lang": "en", "speak": false }`
(`target_lang`: заавал, ж: `mn|en|ru|zh|ja|ko|de`)
**Хариулт `200`** — `data: { "source_text": "Сайн уу", "translated": "Hello", "audio": { … } }`.

> Prompt давхаргын тохиргоо нь дээрх **Админ — хэрэглэгч & AI prompt**-д байрлана
> (`GET`/`PUT /api/v1/admin/ai/prompts`). Суурь guardrail давхарга нь hardcoded
> бөгөөс хэзээ ч ил гарахгүй.

---

## Non-`/api` mounts

### OIDC provider админ гадаргуу — `/admin` (оператор)

Зөвхөн **Hydra тохируулагдсан үед** (`ProviderConfigured()`) идэвхжинэ. `/admin`-д
(`StripPrefix`-ээр, тул дотоод pattern нь `/api/v1/…` уншина) суурьшсан энгийн
`http.ServeMux`. Энэ нь **RP OAuth2 client бүртгэл** болон **admin API key**-ийг
удирдах ба **admin API key**-ээр баталгаажна — `Authorization: Bearer <key>`
эсвэл `X-API-Key: <key>` — хэрэглэгчийн JWT-ЭЭР БИШ.

> ⚠️ **Нэрийн давхцал.** Энэ дээд түвшний `/admin` оператор гадаргуу нь дээрх
> апп доторх `/api/v1/admin` бүлэгтэй өөр зүйл. Түүний өөрийн route-ууд ч strip-
> ийн дараа `/api/v1/…` уншдаг ч тэдгээрт `/admin/api/v1/…`-ээр хүрнэ.

| Method | Path (`/admin`-ийн доор) | Тайлбар |
|--------|-----------------------|-------------|
| GET | `/api/v1/me` | Дуудагч admin key-г таних. |
| GET | `/api/v1/clients` | Бүртгэлтэй RP OAuth2 client-уудыг жагсаах. |
| POST | `/api/v1/clients` | Шинэ RP client бүртгэх. |
| GET | `/api/v1/clients/{client_id}` | Нэг RP client авах. |
| PATCH | `/api/v1/clients/{client_id}` | RP client шинэчлэх. |
| DELETE | `/api/v1/clients/{client_id}` | RP client устгах. |
| POST | `/api/v1/clients/{client_id}/rotate-secret` | RP client secret эргүүлэх. |
| GET | `/api/v1/keys` | Admin API key жагсаах. |
| POST | `/api/v1/keys` | Admin API key үүсгэх. |
| DELETE | `/api/v1/keys/{id}` | Admin API key цуцлах. |

### Sign relay — `/rp/sign/*` (RP proxy)

Зөвхөн **`SIGN_RELAY_TOKEN` болон `EID_RP_SECRET` хоёул тохируулагдсан үед**
идэвхжинэ. Гуравдагч этгээдийн RP-үүд dan-ий eID Mongolia creds-ээр гарын үсэг
зурах боломжийг олгодог reverse proxy: дуудагч нь хуваалцсан relay токеныг
`Authorization: Bearer <token>`-оор танилцуулна; relay нь түүнийг dan-ий жинхэнэ
eID RP secret-ээр солиод eID Mongolia руу дамжуулна. `/rp/sign` болон
`/rp/sign/*` хоёулаа зохицуулагдана.

---

## Операциуд (`/api/v1` угтваргүй)

| Method | Path | Gate | Тайлбар |
|--------|------|------|-------------|
| GET | `/health` | нээлттэй | Liveness — процесс амьд бол үргэлж 200. |
| GET | `/ready` | нээлттэй | Readiness — Postgres (pgx pool) + Redis ping хийнэ. |
| GET | `/metrics` | ObservabilityGate | Prometheus exposition (production-д bearer-gated + 404-hidden). |
| GET | `/swagger/*` · `/swagger/doc.json` | ObservabilityGate | Swagger UI + spec (production-д gated). |
| GET | `/api/` | нээлттэй | Root "alive" JSON. |

`ObservabilityGate` нь observability bearer токен шаардах ба production-д
баталгаажаагүй үед 404 (401 биш) буцаана.

---

🔒 = `Authorization: Bearer <access_token>` шаардана; 🛡️ = дээр нь нэрлэсэн RBAC
permission шаардана. Swagger spec-ийг handler annotation-аас `make swag`-аар
дахин үүсгэ. (Долоон хуучирсан `auth_*` handler нь бүртгэгдээгүй password/OTP
endpoint-уудад `@Router` annotation-той хэвээр байгаа — дээрх auth гадаргуу нь
`route_auth.go`-г тусгасан бөгөөс энэ нь эрх мэдэлтэй.)

---

**Government Template Platform V3.0** — Co-developed by the **Gerege Systems Development Team** and **Claude AI**, 2026.
</content>
