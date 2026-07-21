// eID based AI enabled Government Template Platform V3.0
// Gerege Systems Development Team болон Claude AI хамтран бүтээв, 2026.

package oidc

import (
	"testing"

	"template/internal/business/domain"
)

func TestClaimsForScopes(t *testing.T) {
	u := domain.User{
		FirstName:  "Бат",
		LastName:   "Дорж",
		Email:      "bat@example.mn",
		NationalID: "УБ12345678",
		CivilID:    "9999",
	}

	id := ClaimsForScopes([]string{"profile", "email", "nationalid"}, u)
	if id["name"] != "Дорж Бат" {
		t.Fatalf("name claim = %v", id["name"])
	}
	if id["email"] != "bat@example.mn" || id["email_verified"] != true {
		t.Fatalf("email claims = %v / %v", id["email"], id["email_verified"])
	}
	if id["national_id"] != "УБ12345678" || id["register_number"] != "9999" {
		t.Fatalf("nationalid claims = %v / %v", id["national_id"], id["register_number"])
	}
	// sub is never set here — Hydra sets it from the login subject.
	if _, ok := id["sub"]; ok {
		t.Fatal("sub must not be set in claims")
	}

	// scope-gated: without nationalid scope, no national_id claim.
	id2 := ClaimsForScopes([]string{"profile"}, u)
	if _, ok := id2["national_id"]; ok {
		t.Fatal("national_id leaked without nationalid scope")
	}
	if _, ok := id2["email"]; ok {
		t.Fatal("email leaked without email scope")
	}
}

func TestIntersect(t *testing.T) {
	// Дараалал нь ЭХНИЙ (хүсэгдсэн) жагсаалтынхаар хадгалагдана — RP-д буцаах
	// scope мөр хүсэлтийнхтэй ижил дарааллаар харагдана.
	got := intersect([]string{"openid", "profile", "email"}, []string{"email", "phone", "openid"})
	want := []string{"openid", "email"}
	if len(got) != len(want) || got[0] != want[0] || got[1] != want[1] {
		t.Fatalf("intersect = %v, want %v", got, want)
	}
	// Хүсээгүй зүйлийг "олгосон" гэж дамжуулсан ч гарч ирэхгүй.
	if len(intersect(nil, []string{"x"})) != 0 {
		t.Fatal("intersect with nothing requested must stay empty")
	}
	if len(intersect([]string{"openid"}, nil)) != 0 {
		t.Fatal("intersect with nothing granted must stay empty")
	}
}
