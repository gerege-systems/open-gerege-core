# open-gerege-core

> Gerege / dgov платформуудын **нээлттэй суурь Go модуль** —
> Clean Architecture backend, eID/SSO танилт, RBAC, API gateway, AI pipeline.

```
module github.com/gerege-systems/open-gerege-core
```

## 🧬 Удамшлын гинж

Энэ репо флотын удамшлын мод дотор дараах байрлалтай:

```
open-gerege-core            ← ЭНЭ РЕПО (Go модуль)
   ├─► private-gerege-core ──► gerege урсгалын 7 платформ
   └─► gov урсгалын 5 платформ (шууд)
```

**Механизм:** `go.mod` — git fork биш. Шинэчлэлт нэг мөрийн хувилбар солилтоор тарна, тиймээс merge conflict үүсэхгүй.

---

## Хоёр цөмийн загвар

Энэ репо нь **суурь (base)** цөм. Түүнээс удамшсан хаалттай цөм тусад нь байна:

| Цөм | Хэн хэрэглэдэг | Агуулга |
|---|---|---|
| **`open-gerege-core`** (энэ репо) | **gov урсгал** — `template-dgov-mn`, `ring-dgov-mn`, `hurdan-dgov-mn`, `sso-dgov-mn`, `developer-dgov-mn` | Суурь чадвар бүхэлдээ |
| [`private-gerege-core`](https://github.com/gerege-systems/private-gerege-core) | **gerege урсгал** — `template-gerege-mn`, `gerege-platform-mn`, `gerege-app-mn`, `gerege-pos-mn`, `gerege-kiosk-mn`, `wallet-gerege-mn`, `developer-gerege-mn` | Энэ цөмийг `go.mod`-оор **удамшиж**, дээр нь Gerege-ийн нэмэлт чадвар |

**Урсгалын чиг:** энэ репод өөрчлөлт орох бүрд `private-gerege-core` нь
өдөр бүр автоматаар шинэ хувилбар руу шилжинэ (тэнд `core-autosync.yml`).
Урвуу чиглэл байхгүй — private-ийн код энд **буцаж ирэхгүй**.

> Түүх: энэ репо нь `platform-core`-ийн шууд залгамжлагч (`template-dgov-mn/backend`-ээс
> `git subtree split`-ээр гарсан бүх түүхийг хадгалсан). Хуучин
> `gerege-systems/platform-core` архивлагдана.

Яагаад: өмнө нь суурь код 8 репод **хуулбарлагдаж**, шинэчлэлт бүрийг гараар
зөөдөг байсан (~85% давхардал). Одоо нэг эх сурвалж, хувилбартай.

## Бүтэц

| Хавтас | Юу вэ |
|---|---|
| `core/` | Суурийн давхаргууд — `config`, `constants`, `business/{domain,usecases}`, `datasources`, `http/{routes,handlers,middlewares}`, `provider` |
| `pkg/` | Бие даасан туслах сангууд — `eid`, `jwt`, `oidc`, `audit`, `gemini`, `xyp`, `logger` … |
| `cmd/` | Ажиллах хоёртын файлууд — `api`, `migration`, `seed`, `healthcheck` |
| `migrations/` | **Суурийн** SQL migration-ууд (дугаар `1–999`), хоёртын файлд шингээгдсэн |
| `docs/` | Swagger тодорхойлолт |

> `internal/` биш `core/` гэж нэрлэсэн шалтгаан: Go-ийн `internal` дүрмээр
> модулиас гадуур import хийх боломжгүй болно.

### Интерфейсийн хэл (v0.5.0-оос)

Super admin нь хэлийг **ажиллаж байх үед** нэмж/хасч, орчуулгыг гараар эсвэл
Gemini-ээр бөглөнө (`languages` + `translations`, migration 49).

Хариуцлагын хуваарь: **түлхүүрийн жагсаалт нь аппын өөрийнх** (frontend-д
багцлагдсан dictionary), платформ нь тэдгээрийн **утгыг** л хадгална. Иймд
суурь нь тухайн аппын түлхүүрийг мэдэхгүй ерөнхий хэвээр үлдэж, DB хоосон/
унасан үед апп багцлагдсан утгаараа ажилласаар байна. Автомат орчуулгын үед
апп эх dictionary-гаа хамт илгээнэ.

| Endpoint | Хандалт |
|---|---|
| `GET /v1/languages/enabled` | нийтийн |
| `GET /v1/languages/{code}/dictionary` | нийтийн |
| `GET`·`POST /v1/languages` | super admin |
| `PATCH`·`DELETE /v1/languages/{code}` | super admin |
| `PUT /v1/languages/{code}/translations` | super admin |
| `POST /v1/languages/{code}/translate` | super admin (Gemini) |

- `mn`/`en`/`zh`/`ru` нь `is_builtin` — устгах боломжгүй, зөвхөн унтраана.
- Шинэ хэл **унтраалттай** үүснэ; орчуулгыг бөглөсний дараа идэвхжүүлнэ.
- Гараар засагдсан (`manual`) утгыг автомат орчуулга хэзээ ч дарж бичихгүй.
- Байрлуулагч (`{name}`, `{0}`) алдагдсан AI орчуулга суухгүй.

## Апп хэрхэн хэрэглэх вэ

```go
package main

import "github.com/gerege-systems/open-gerege-core/cmd/api/server"

func main() {
    server.ServiceName = "ring-dgov" // telemetry-д харагдах нэр

    app, err := server.NewApp()
    if err != nil {
        panic(err)
    }

    // Аппын өөрийн маршрутыг суурийн router дээр нэмнэ.
    // app.Router().Route("/api/ring", ring.Routes(app.Pool()))

    if err := app.Run(); err != nil {
        panic(err)
    }
}
```

Заагууд (seam):

| Юм | Зориулалт |
|---|---|
| `server.ServiceName` | tracing/metrics дэх үйлчилгээний нэр (`NewApp`-аас өмнө тавина) |
| `app.Router()` | суурийн chi router — аппын маршрут нэмэх |
| `app.Pool()` | DB pool — аппын repository байгуулах |

## Migration

Runner нь **олон эх сурвалжийг** хүлээж авна: суурийн embed FS + аппын хавтас.
Бүгд нэг дараалалд, файлын эхний дугаараар эрэмбэлэгдэнэ.

```go
runner := migration.NewFS(pool, coremigrations.FS, os.DirFS("migrations"))
```

Дугаарын муж: **суурь `1–999`**, **апп `1000+`** — тиймээс суурийн бүх
migration аппынхаас өмнө ажиллана. Дэлгэрэнгүйг
[`migrations/README.md`](migrations/README.md)-ээс үзнэ үү.

`cmd/migration` нь суурийн embed FS-ийг үргэлж, ажлын директорт `migrations/`
хавтас байвал түүнийг нэмж уншина — өөрөөр хэлбэл контейнерт SQL файл хуулах
шаардлагагүй.

## Хувилбарлалт

Semver tag (`v0.x.y`). Апп бүр `go.mod`-д тодорхой хувилбар барина.
Breaking өөрчлөлт → minor bump + шилжих заавар.

Локал хөгжүүлэлтэд `go.work` ашиглавал core-ийн засвар аппуудад шууд харагдана:

```
go 1.26
use (
    ./platform-core
    ./gerege-platform-mn/backend
)
```

## Гарал үүсэл

Backend нь нээлттэй эх [snykk/go-rest-boilerplate](https://github.com/snykk/go-rest-boilerplate)
(MIT, Najib Fikri)-аас гаралтай; HTTP давхаргыг Gin → chi, өгөгдлийн давхаргыг
sqlx → pgx болгож хөрвүүлсэн. MIT лицензтэй — [LICENSE](LICENSE).
