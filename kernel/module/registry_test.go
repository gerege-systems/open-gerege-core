// Gerege Systems Development Team болон Claude AI хамтран бүтээв, 2026.

package module

import (
	"strings"
	"testing"
)

func mustReg(t *testing.T, ms ...Manifest) *Registry {
	t.Helper()
	r, err := New(ms...)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	return r
}

func TestNewValidation(t *testing.T) {
	cases := []struct {
		name string
		ms   []Manifest
		want string // алдааны substring; хоосон бол амжилт
	}{
		{"ok", []Manifest{
			{ID: "a", Kind: KindCore},
			{ID: "b", Kind: KindBusiness, DependsOn: []string{"a"}},
		}, ""},
		{"давхардсан ID", []Manifest{
			{ID: "a", Kind: KindCore}, {ID: "a", Kind: KindCore},
		}, "давхардсан"},
		{"бүртгэлгүй хамаарал", []Manifest{
			{ID: "a", Kind: KindCore, DependsOn: []string{"ghost"}},
		}, "бүртгэлгүй"},
		{"цикл", []Manifest{
			{ID: "a", Kind: KindBusiness, DependsOn: []string{"b"}},
			{ID: "b", Kind: KindBusiness, DependsOn: []string{"a"}},
		}, "цикл"},
		{"давхардсан угтвар", []Manifest{
			{ID: "a", Kind: KindCore, RoutePrefixes: []string{"/api/v1/x/"}},
			{ID: "b", Kind: KindCore, RoutePrefixes: []string{"/api/v1/x/"}},
		}, "эзэмшиж"},
		{"буруу kind", []Manifest{{ID: "a", Kind: Kind("weird")}}, "Kind"},
		{"том үсэгтэй ID", []Manifest{{ID: "Gov", Kind: KindCore}}, "жижиг үсэг"},
		{"угтвар / -ээр эхлээгүй", []Manifest{
			{ID: "a", Kind: KindCore, RoutePrefixes: []string{"api/x"}},
		}, "/-ээр эхлэх"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := New(tc.ms...)
			if tc.want == "" {
				if err != nil {
					t.Fatalf("алдаа гарах ёсгүй байсан: %v", err)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("хүлээсэн алдаа %q, гарсан: %v", tc.want, err)
			}
		})
	}
}

func TestEnableDisable(t *testing.T) {
	r := mustReg(t,
		Manifest{ID: "auth", Kind: KindCore},
		Manifest{ID: "sign", Kind: KindBusiness, DependsOn: []string{"auth"}},
		Manifest{ID: "relay", Kind: KindBusiness, DependsOn: []string{"sign"}},
	)

	// Core модулийг унтраахыг хориглоно.
	if err := r.Disable("auth"); err == nil {
		t.Fatal("core модулийг унтраах ёсгүй")
	}
	// Хамаарагчтай business модулийг шууд унтраахыг хориглоно.
	if err := r.Disable("sign"); err == nil {
		t.Fatal("relay идэвхтэй байхад sign унтрах ёсгүй")
	}
	// Зөв дараалал: эхлээд relay, дараа нь sign.
	if err := r.Disable("relay"); err != nil {
		t.Fatalf("relay: %v", err)
	}
	if err := r.Disable("sign"); err != nil {
		t.Fatalf("sign: %v", err)
	}
	if r.Enabled("sign") || r.Enabled("relay") {
		t.Fatal("унтарсан модулиуд идэвхтэй харагдаж байна")
	}
	// Хамаарал нь унтраатай байхад асаахыг хориглоно.
	if err := r.Enable("relay"); err == nil {
		t.Fatal("sign унтраатай байхад relay асах ёсгүй")
	}
	if err := r.Enable("sign"); err != nil {
		t.Fatalf("sign асаах: %v", err)
	}
	if err := r.Enable("relay"); err != nil {
		t.Fatalf("relay асаах: %v", err)
	}
	// Бүртгэлгүй ID.
	if r.Enabled("ghost") {
		t.Fatal("бүртгэлгүй модуль идэвхтэй байж болохгүй")
	}
	if err := r.Disable("ghost"); err == nil {
		t.Fatal("бүртгэлгүй модулийг унтраахад алдаа гарах ёстой")
	}
}

func TestApplyDisabledList(t *testing.T) {
	r := mustReg(t,
		Manifest{ID: "auth", Kind: KindCore},
		Manifest{ID: "sign", Kind: KindBusiness},
		Manifest{ID: "relay", Kind: KindBusiness, DependsOn: []string{"sign"}},
	)
	// Дараалал буруу өгөгдсөн ч fixed-point давталт зөв шийднэ.
	if err := r.ApplyDisabledList(" sign , relay "); err != nil {
		t.Fatalf("ApplyDisabledList: %v", err)
	}
	if r.Enabled("sign") || r.Enabled("relay") {
		t.Fatal("хоёул унтарсан байх ёстой")
	}
	// Core модуль жагсаалтад орвол алдаа.
	r2 := mustReg(t, Manifest{ID: "auth", Kind: KindCore})
	if err := r2.ApplyDisabledList("auth"); err == nil {
		t.Fatal("core модулийг env-ээр унтраахад алдаа гарах ёстой")
	}
	// Хоосон жагсаалт OK.
	if err := r2.ApplyDisabledList(" , "); err != nil {
		t.Fatalf("хоосон жагсаалт: %v", err)
	}
}

func TestBuiltinIsValid(t *testing.T) {
	r := Builtin()
	list := r.List()
	if len(list) < 20 {
		t.Fatalf("builtin модулиуд дутуу: %d", len(list))
	}
	var core, business int
	for _, s := range list {
		if !s.Enabled {
			t.Fatalf("%s: эхэндээ идэвхтэй байх ёстой", s.Manifest.ID)
		}
		switch s.Manifest.Kind {
		case KindCore:
			core++
		case KindBusiness:
			business++
		}
	}
	if core < 8 || business < 10 {
		t.Fatalf("ангилал дутуу: core=%d business=%d", core, business)
	}
}
