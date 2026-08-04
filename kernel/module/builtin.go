// Gerege Systems Development Team болон Claude AI хамтран бүтээв, 2026.

package module

// Builtin нь платформын одоогийн (in-tree) модулиудын бүртгэлийг үүсгэнэ.
// Энэ бол MODULAR_REFACTOR_PLAN §3-ын ангиллын кодон дахь эх сурвалж:
// core/business хуваарь, хамаарлын граф, route-ийн эзэмшил бүгд эндээс.
//
// Угтварын дүрэм: илүү урт угтвар түрүүлж таарна, тиймээс жишээ нь
// /api/v1/admin/ai/ нь ai модульд, үлдсэн /api/v1/admin/ нь users-т очно.
// routes_golden_test нь бүртгэгдсэн бүх route яг нэг модульд харьяалагдахыг
// баталгаажуулдаг — route нэмэхдээ эзэн модулийг нь энд зарлана.
func Builtin() *Registry {
	return MustNew(
		// ── Core модулиуд ──────────────────────────────────────────────
		Manifest{
			ID: "auth", Name: "Танилт (eID · Google · SSO)", Kind: KindCore,
			RoutePrefixes: []string{"/api/v1/auth/", "/api/v1/sso/"},
		},
		Manifest{
			ID: "users", Name: "Хэрэглэгч ба eID профайл", Kind: KindCore,
			DependsOn:     []string{"auth"},
			RoutePrefixes: []string{"/api/v1/users/", "/api/v1/admin/"},
		},
		Manifest{
			ID: "rbac", Name: "Эрхийн удирдлага (RBAC)", Kind: KindCore,
			DependsOn:     []string{"users"},
			RoutePrefixes: []string{"/api/v1/rbac/"},
		},
		Manifest{
			ID: "org", Name: "Байгууллага ба гишүүнчлэл", Kind: KindCore,
			DependsOn:     []string{"users"},
			RoutePrefixes: []string{"/api/v1/org/"},
		},
		Manifest{
			ID: "audit", Name: "Audit ба аюулгүй байдлын үйл явдал", Kind: KindCore,
			RoutePrefixes: []string{"/api/v1/audit/", "/api/v1/security/"},
		},
		Manifest{
			ID: "superadmin", Name: "Super admin", Kind: KindCore,
			DependsOn:     []string{"users", "rbac"},
			RoutePrefixes: []string{"/api/v1/superadmin/", "/api/v1/auth/superadmin/"},
		},
		Manifest{
			ID: "site", Name: "Сайтын харагдац · theme · хэл", Kind: KindCore,
			RoutePrefixes: []string{"/api/v1/site/", "/api/v1/themes/", "/api/v1/languages/"},
		},
		Manifest{
			ID: "assets", Name: "Гарын үсэг/тамгын asset", Kind: KindCore,
			DependsOn:     []string{"users"},
			RoutePrefixes: []string{"/api/v1/me/"},
		},
		Manifest{
			ID: "platform", Name: "Модулийн удирдлага", Kind: KindCore,
			RoutePrefixes: []string{"/api/v1/platform/"},
		},

		// ── Business модулиуд ──────────────────────────────────────────
		Manifest{
			ID: "gov", Name: "Төрийн үйлчилгээний портал", Kind: KindBusiness,
			DependsOn:     []string{"auth", "users", "rbac", "org"},
			RoutePrefixes: []string{"/api/v1/gov/"},
			UIPrefixes:    []string{"/me/services/", "/me/applications/", "/me/references/", "/me/appointments/", "/me/payments/", "/me/notifications/"},
		},
		Manifest{
			ID: "ai", Name: "AI туслах (Gemini)", Kind: KindBusiness,
			DependsOn:     []string{"auth"},
			RoutePrefixes: []string{"/api/v1/ai/", "/api/v1/public/ai/", "/api/v1/admin/ai/"},
			UIPrefixes:    []string{"/me/ai/", "/me/translate/"},
		},
		Manifest{
			ID: "sign", Name: "Баримт бичгийн гарын үсэг (PAdES)", Kind: KindBusiness,
			DependsOn:     []string{"auth", "users", "assets"},
			RoutePrefixes: []string{"/api/v1/sign/"},
			UIPrefixes:    []string{"/me/eid/sign"},
		},
		Manifest{
			ID: "relay", Name: "Sign relay ба платформ хоорондын дамжуулалт", Kind: KindBusiness,
			DependsOn:     []string{"sign", "rbac"},
			RoutePrefixes: []string{"/api/v1/relay/"},
			UIPrefixes:    []string{"/admin/relay/"},
		},
		Manifest{
			ID: "integrations", Name: "Гуравдагч интеграци (Drive · Meet · Dropbox)", Kind: KindBusiness,
			DependsOn:     []string{"auth"},
			RoutePrefixes: []string{"/api/v1/integrations/"},
			UIPrefixes:    []string{"/me/integrations/"},
		},
		Manifest{
			ID: "gspace", Name: "Gerege Space (SFTP хадгалалт)", Kind: KindBusiness,
			DependsOn:     []string{"auth"},
			RoutePrefixes: []string{"/api/v1/gspace/"},
			UIPrefixes:    []string{"/me/files/"},
		},
		Manifest{
			ID: "gateway-console", Name: "API gateway удирдлага", Kind: KindBusiness,
			DependsOn:     []string{"rbac"},
			RoutePrefixes: []string{"/api/v1/gateway/"},
			UIPrefixes:    []string{"/admin/gateway/"},
		},
		Manifest{
			ID: "registry", Name: "Үйлчилгээний регистр ба каталог", Kind: KindBusiness,
			DependsOn:     []string{"rbac"},
			RoutePrefixes: []string{"/api/v1/registry/", "/api/v1/catalog/"},
			UIPrefixes:    []string{"/admin/registry/"},
		},
		Manifest{
			ID: "provider", Name: "OIDC provider (SSO issuer)", Kind: KindBusiness,
			DependsOn: []string{"auth"},
			RoutePrefixes: []string{
				"/api/v1/provider/",
				// OIDC стандартын нийтийн endpoint-ууд /api-ийн ГАДНА, үндэс дээр.
				"/.well-known/", "/oauth2/", "/userinfo",
			},
		},
		Manifest{
			ID: "applications", Name: "OAuth клиент апп-ууд", Kind: KindBusiness,
			DependsOn:     []string{"provider"},
			RoutePrefixes: []string{"/api/v1/applications/"},
			UIPrefixes:    []string{"/admin/applications/"},
		},
		Manifest{
			ID: "core-find", Name: "Gerege Core лавлагаа", Kind: KindBusiness,
			DependsOn:     []string{"users", "org"},
			RoutePrefixes: []string{"/api/v1/core/"},
		},
		Manifest{
			ID: "eidproxy", Name: "eID service proxy (RP дамжуулалт)", Kind: KindBusiness,
			DependsOn:     []string{"auth", "provider"},
			RoutePrefixes: []string{"/api/v1/eid/", "/api/v1/eid-org/"},
			UIPrefixes:    []string{"/me/eid/"},
		},
	)
}
