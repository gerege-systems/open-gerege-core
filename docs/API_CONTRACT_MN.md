# API Гэрээ (Contract)

> 🌐 [English](API_CONTRACT.md) · **Монгол**

**Gerege Backend Template v27**-ийн REST API лавлагаа. Шууд, автоматаар үүсэх
бүрэн тодорхойлолтыг `GET /swagger/` дээр үзнэ (эх: `docs/swagger.json`).
Англи хувилбар: [API_CONTRACT.md](./API_CONTRACT.md).

> **Эх сурвалж.** Нээлттэй эх
> [snykk/go-rest-boilerplate](https://github.com/snykk/go-rest-boilerplate)
> (MIT, Najib Fikri)-аас гаралтай; HTTP давхаргыг **Gin → chi (net/http)**,
> өгөгдлийн давхаргыг **sqlx → pgx (pgxpool)** болгосон.

## Дүрэм

- **Үндсэн URL:** `http://localhost:8080/api/v1`
- **Content-Type:** `application/json`
- **Танилт:** хамгаалагдсан endpoint-д `Authorization: Bearer <access_token>` шаардана
- **Rate limit:** `/auth/*` нь IP тус бүр ~5 хүсэлт/минут (хэтэрвэл `429`)

### Хариуны бүтэц (envelope)

```json
{ "status": true, "message": "...", "data": { }, "request_id": "..." }
```
- `status` — амжилтад `true`, алдаанд `false`
- `data` — амжилтад л байна
- `request_id` — корреляцийн ID (`X-Request-ID` header-т мөн)

### Статус кодууд

| Код | Утга |
|-----|------|
| 200 / 201 | Амжилттай / Үүсгэсэн |
| 400 | Буруу body |
| 401 | Токен/нэвтрэлт буруу |
| 403 | Хориглосон (lockout) |
| 404 | Олдсонгүй |
| 409 | Давхцал (username/email) |
| 422 | Validation алдаа (`data.errors` нь `{field, tag, message}` объектуудын массив) |
| 429 | Хэт олон хүсэлт |
| 500 | Дотоод алдаа |

`strongpassword` дүрэм: том/жижиг үсэг, тоо, тусгай тэмдэгт + доод тал нь 12 тэмдэгт.

---

## Танилт (Authentication)

| Method | Path | Body | Амжилт (200/201) |
|--------|------|------|------------------|
| POST | `/auth/register` | `username`(3–25), `email`(≤50), `password`(12–72, strong) | `201` "registration user success" + user |
| POST | `/auth/login` | `email`, `password` | `200` "login success" + user + `token` + `refresh_token` |
| POST | `/auth/send-otp` | `email` | `200` "otp code has been send to …" |
| POST | `/auth/verify-otp` | `email`, `code`(numeric) | `200` "otp verification success" |
| POST | `/auth/refresh` | `refresh_token` | `200` "token refreshed" + шинэ token pair |
| POST | `/auth/logout` | `refresh_token` | `200` "logout success" |
| POST | `/auth/password/forgot` | `email` | `200` "if the email is registered, a reset code has been sent" (6 оронтой OTP илгээнэ) |
| POST | `/auth/password/reset` | `email`, `code`, `new_password`(strong) | `200` "password reset"; `401` код буруу/хүчингүй |
| PUT 🔒 | `/auth/password/change` | `current_password`, `new_password`(strong) | `200` "password changed" |

### Жишээ: нэвтрэх

**Хүсэлт** `POST /api/v1/auth/login`
```json
{ "email": "john@example.com", "password": "Str0ng!Passw0rd" }
```
**Хариу `200`**
```json
{ "status": true, "message": "login success", "data": {
  "id": "…", "username": "johndoe", "email": "john@example.com", "role_id": 2,
  "token": "<access_jwt>", "refresh_token": "<refresh_jwt>",
  "created_at": "…", "updated_at": null }, "request_id": "…" }
```
Алдаа: `401` нэвтрэлт буруу, `403` дараалсан амжилтгүйн дараа lockout.

---

## Хэрэглэгч (Users)

| Method | Path | Тайлбар |
|--------|------|---------|
| GET 🔒 | `/users/me` | Нэвтэрсэн хэрэглэгчийн профайл — `200` "user data fetched successfully" |

---

## Үйлдлийн endpoint-ууд (`/api/v1` угтваргүй)

`GET /health` (liveness) · `GET /ready` (Postgres + Redis шалгана) ·
`GET /metrics` (Prometheus) · `GET /swagger/*` (Swagger UI)

---

🔒 = `Authorization: Bearer <access_token>` шаардана. Энэ тодорхойлолтыг handler
annotation-аас `make swag`-аар дахин үүсгэнэ.

---

**Gerege Template Version 27.0** — **Gerege Systems Development Team** болон **Claude AI** хамтран бүтээв, 2026.
