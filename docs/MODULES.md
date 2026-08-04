# Модульчлагдсан Framework — хөгжүүлэгчийн гарын авлага

> V4.0 Modular Platform. Архитектурын үндэслэл, үе шатууд:
> open-gerege-mn репогийн `docs/MODULAR_REFACTOR_PLAN.md`.

## Ойлголтууд

| Ойлголт | Юу | Хаана |
|---|---|---|
| **Kernel** | Framework runtime: модулийн бүртгэл, route gate, Host гэрээ, update цөм | `kernel/module`, `kernel/update` |
| **Манифест** | Модулийн зарлал: ID, нэр, kind (core\|business), хамаарал, route угтварууд | `kernel/module/builtin.go` |
| **Module** | Ажиллагааны гэрээ: `ID()` + `Register(ctx, host)` — өөрийн repo→usecase→route wiring | `modules/<id>/module.go` |
| **Host** | Kernel-ээс модульд олгох орчин: `APIRouter()`, `Pool()`, `AuthMiddleware()`, `Service()` | `kernel/module/host.go` |
| **Gate** | Идэвхгүй модулийн бүх route-ыг 404 болгодог middleware | `kernel/module/gate.go` |
| **Registry** | Enable/disable төлөв + дүрэм (core унтрахгүй, хамаарлын дараалал) | `kernel/module/registry.go` |
| **platformd** | Өөрийгөө шинэчлэгч supervisor: суваг, цонх, авто-rollback | `cmd/platformd`, `deploy/platformd` |

## Модулийн идэвхийг удирдах

```bash
# Boot үед (env):
MODULES_DISABLED=gspace,relay

# Ажиллаж байхад (админ, restart-гүй):
curl -X PUT https://.../api/v1/platform/admin/modules/gspace \
  -H "Authorization: Bearer $ADMIN_TOKEN" \
  -d '{"enabled": false}'

# Төлөв харах:
curl https://.../api/v1/platform/modules            # нийтийн (id, kind, enabled)
curl https://.../api/v1/platform/admin/modules      # админ (нэр, хамаарал, route)
```

Дүрмүүд (кодоор мөрдөгдөнө): core модуль унтрахгүй; идэвхтэй хамаарагчтай
модуль унтрахгүй (эхлээд хамаарагчийг); хамаарал нь унтарсан модуль асахгүй.
Runtime төлөв `platform_modules` хүснэгтэд хадгалагдаж дараагийн boot-д
сэргэнэ. Идэвхгүй модулийн route-ууд 404 (модуль байхгүйтэй ижил гадаргуу).

## Шинэ business модуль нэмэх

1. **Манифест зарлана** — `kernel/module/builtin.go`:

```go
Manifest{
    ID: "ring", Name: "Хонхны модуль", Kind: KindBusiness,
    DependsOn:     []string{"auth"},
    RoutePrefixes: []string{"/api/v1/ring/"},
},
```

2. **Домэйн кодоо бичнэ** — Clean Architecture хэвээр:
   usecase (`core/business/usecases/ring`), repo interface + postgres adapter,
   handler + route (`core/http/routes/route_ring.go`), migration.

3. **Wiring-ээ модульдоо хийнэ** — `modules/ring/module.go`:

```go
type Module struct{}
func New() *Module        { return &Module{} }
func (*Module) ID() string { return "ring" }

func (m *Module) Register(_ context.Context, host module.Host) error {
    repo := ringpostgres.NewRingRepository(host.Pool())
    uc := ring.NewUsecase(repo)
    routes.NewRingRoute(host.APIRouter(), uc, host.AuthMiddleware()).Routes()
    return nil
}
```

   Модуль-дамнасан хамаарал (rbac, audit, users, write limiter) —
   `module.ServiceAs[T](host, module.ServiceRBAC)` г.м. Дутуу бол Register
   алдаа буцааж boot-ыг унагаана (чимээгүй үргэлжлэхгүй).

4. **Жагсаалтад нэмнэ** — `cmd/api/server/server.go` `platformModules()`.

5. **Тестээ шинэчилнэ**:
   - `go test ./core/http/routes -run TestGoldenRoutes -update` (route
     inventory) — эзэн модуль зарлагдаагүй route-ыг
     `TestEveryRouteHasOwnerModule` барьдаг;
   - модулийн wiring тест (`modules/gspace/module_test.go`-г жишээ болго).

Downstream нимгэн апп өөрийн модулиа `app.Modules()`-ээр шалгаж,
`app.Router()`-т суулгасаар байж болно (хуучин зам хэвээр).

## platformd — авто-шинэчлэлт

```
release pipeline → manifest.json ({"channels":{"stable":"v1.7.0"}})
platformd: татах → semver харьцуулах → цонх/major шалгах →
update.sh (git checkout таг + compose build) → health хүлээх →
OK: VERSION файл шинэчлэх | FAIL: rollback.sh (өмнөх commit)
```

- Суваг: `PLATFORMD_CHANNEL=stable|beta`; major өсөлт default-оор хаалттай
  (`PLATFORMD_ALLOW_MAJOR=true` үед л автомат).
- Maintenance цонх: `PLATFORMD_WINDOW=03:00-05:00` (шөнө дамнасан болно).
- platformd платформыг шинэчилдэг, ӨӨРИЙГӨӨ шинэчилдэггүй — supervisor
  update урсгалын гадна байх нь rollback-ийн баталгаа.
- Migration-ууд **expand–contract** байх ёстой: rollback нь зөвхөн кодыг
  буцаадаг, DB-г доош буулгадаггүй.

Суулгах: `deploy/platformd/platformd.service` (systemd) + `update.sh` /
`rollback.sh` (репогийн root-д ажиллана, замыг орчиндоо тааруулна).

## Frontend

`GET /api/v1/platform/modules` → нимгэн апп-ын BFF proxy
(`/api/platform/modules`) → `useModules()` hook + `<ModuleGuard module="ai">`
(open-gerege-mn репод). UI нуулт нь UX — аюулгүй байдлын хил нь backend gate.

## Юу хийхгүй вэ

- Go `plugin` (.so) / runtime code loading — үгүй. Суулгах = build + solih.
- Uninstall үед автомат `DROP` — хэзээ ч (archive-first).
- Kernel-ээс business package import — үгүй (Host service locator-оор).
- AI guardrail, RLS, security header-ийг модульд тохируулагдахуйц болгох — үгүй.
