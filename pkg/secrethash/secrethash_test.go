// Government Template Platform V3.0
// Gerege Systems Development Team болон Claude AI хамтран бүтээв, 2026.

package secrethash

import "testing"

// oryVectors нь ory/hydra v2.2.0-ийн admin API-аар БОДИТООР үүсгэсэн client
// secret hash-ууд (локал контейнер, 2026-07-20). Эдгээр нь production дахь
// client-уудтай ижил формат — Hydra-аас шилжүүлсэн secret-үүд ажиллах баталгаа.
var oryVectors = []struct {
	secret string
	hash   string
}{
	{"correct-horse-battery-staple", "$pbkdf2-sha256$i=25000,l=32$Xk3NjhYzw2vo0iHb0dENsw$85qvQUf5V71AmArvdGdczye399QcGfByVrEhTAIX4XU"},
	// salt-д '+' агуулсан — стандарт (URL биш) alphabet-ийг батална.
	{"secret-1", "$pbkdf2-sha256$i=25000,l=32$9+PhFISYk7T5oPha6LOx0A$BFZRvBe6tSexuJRIgSEbdb9rfeob/PMnnh6BFRyGJMo"},
	{"secret-2", "$pbkdf2-sha256$i=25000,l=32$Kn44x/395m0EQLy/l3ZhiQ$zwKukF725B990uDY4iZ1GTNCmsCHPJ9iAVFy3FImRrw"},
	{"secret-3", "$pbkdf2-sha256$i=25000,l=32$LqU+Lxu/8nBYkZDZxXCDiA$j3IT0xLkf4vvHVAxNbZ8uAqcTtyHIeEsOYY+YQ+I+rs"},
	// hash-д '/' агуулсан.
	{"secret-6", "$pbkdf2-sha256$i=25000,l=32$BYX6rANgjiXHzb2pY6N6+Q$cFPV/lghZ561lodjINtn/nBvFLbjMNMbNLIOmMry9a4"},
	{"secret-8", "$pbkdf2-sha256$i=25000,l=32$AnJdzQf4NBni/ILA0v8agA$4gYBgyukJJZ/CmB5Q6iUI83BAPsjuP8yJy0fUF2rcsw"},
}

func TestVerifyAcceptsRealHydraHashes(t *testing.T) {
	for _, v := range oryVectors {
		ok, err := Verify(v.hash, v.secret)
		if err != nil {
			t.Fatalf("Verify(%q): unexpected error %v", v.secret, err)
		}
		if !ok {
			t.Fatalf("Verify(%q): a genuine Hydra hash must validate — migrated clients would break", v.secret)
		}
	}
}

func TestVerifyRejectsWrongSecret(t *testing.T) {
	for _, v := range oryVectors {
		ok, err := Verify(v.hash, v.secret+"x")
		if err != nil {
			t.Fatalf("Verify: unexpected error %v", err)
		}
		if ok {
			t.Fatalf("Verify(%q + wrong suffix) must not validate", v.secret)
		}
	}
	// Хоосон secret ч мөн адил.
	if ok, _ := Verify(oryVectors[0].hash, ""); ok {
		t.Fatal("empty secret must never validate")
	}
}

func TestHashRoundTripsArgon2id(t *testing.T) {
	const secret = "a-manually-set-client-secret"
	h, err := Hash(secret)
	if err != nil {
		t.Fatalf("Hash: %v", err)
	}
	ok, err := Verify(h, secret)
	if err != nil || !ok {
		t.Fatalf("Verify(own hash) = %v, %v; want true, nil", ok, err)
	}
	if ok, _ := Verify(h, secret+"x"); ok {
		t.Fatal("wrong secret validated against argon2id hash")
	}
	if NeedsRehash(h) {
		t.Fatal("a freshly written argon2id hash must not be flagged for rehash")
	}
}

func TestHashIsSaltedPerCall(t *testing.T) {
	a, _ := Hash("same-secret")
	b, _ := Hash("same-secret")
	if a == b {
		t.Fatal("two hashes of the same secret must differ (per-call salt)")
	}
}

func TestNeedsRehashFlagsHydraFormat(t *testing.T) {
	if !NeedsRehash(oryVectors[0].hash) {
		t.Fatal("a PBKDF2 (Hydra) hash should be flagged for upgrade to argon2id")
	}
}

func TestVerifyRejectsMalformed(t *testing.T) {
	bad := []string{
		"",
		"not-a-hash",
		"$pbkdf2-sha256$i=25000,l=32$only-three-parts",
		"$pbkdf2-sha256$i=0,l=32$Xk3NjhYzw2vo0iHb0dENsw$85qvQUf5V71AmArvdGdczye399QcGfByVrEhTAIX4XU",
		"$pbkdf2-sha256$l=32$Xk3NjhYzw2vo0iHb0dENsw$85qvQUf5V71AmArvdGdczye399QcGfByVrEhTAIX4XU",
		// l= талбар нь бодит hash-ийн урттай зөрчилдөж байна.
		"$pbkdf2-sha256$i=25000,l=64$Xk3NjhYzw2vo0iHb0dENsw$85qvQUf5V71AmArvdGdczye399QcGfByVrEhTAIX4XU",
		"$argon2id$v=19$m=65536,t=3$only-five-parts$x",
		"$argon2id$v=99$m=65536,t=3,p=4$Xk3NjhYzw2vo0iHb0dENsw$85qvQUf5V71AmArvdGdczye399QcGfByVrEhTAIX4XU",
		"$scrypt$whatever",
	}
	for _, h := range bad {
		ok, err := Verify(h, "anything")
		if ok {
			t.Fatalf("Verify(%q) returned true for a malformed hash", h)
		}
		if err == nil {
			t.Fatalf("Verify(%q) should report an error for a malformed hash", h)
		}
	}
}

// Hash мөр нь өөрөө KDF-ийн зардлыг тодорхойлдог тул гэмтсэн/хорлонтой утга
// CPU эсвэл санах ойг шавхаж болзошгүй — хязгаараас хэтэрсэн бол ажиллуулахгүй
// шууд татгалзана.
func TestVerifyRejectsAbsurdCostParameters(t *testing.T) {
	cases := []string{
		// Тэрбум давталт — хүлээн авбал хүсэлт бүр CPU-г түгжинэ.
		"$pbkdf2-sha256$i=1000000000,l=32$Xk3NjhYzw2vo0iHb0dENsw$85qvQUf5V71AmArvdGdczye399QcGfByVrEhTAIX4XU",
		// Түлхүүрийн урт хэт богино (l нь бодит hash-тай таарсан ч).
		"$pbkdf2-sha256$i=25000,l=8$Xk3NjhYzw2vo0iHb0dENsw$AAAAAAAAAAA",
	}
	for _, h := range cases {
		ok, err := Verify(h, "anything")
		if ok || err == nil {
			t.Fatalf("Verify(%.40q…) must refuse absurd cost parameters", h)
		}
	}
}
