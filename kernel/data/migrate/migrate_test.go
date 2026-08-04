// Gerege Systems Development Team болон Claude AI хамтран бүтээв, 2026.

package migrate

import (
	"testing"
	"testing/fstest"
)

func TestTableName(t *testing.T) {
	cases := map[string]string{
		"users":           "mod_users_schema_migrations",
		"gov":             "mod_gov_schema_migrations",
		"gateway-console": "mod_gateway_console_schema_migrations",
	}
	for id, want := range cases {
		if got := TableName(id); got != want {
			t.Errorf("TableName(%q) = %q, хүлээсэн %q", id, got, want)
		}
	}
}

// Хүснэгтийн нэрийг параметржүүлэх боломжгүй тул ID-г хатуу шалгах нь
// цорын ганц хамгаалалт — энэ тест тэр хаалгыг онгойлгохоос сэргийлнэ.
func TestNewRejectsUnsafeID(t *testing.T) {
	bad := []string{
		"", "Users", "users;DROP TABLE x", "users schema", "1users",
		"users_x", "-users", "users-", "users--x", "users\"x",
	}
	for _, id := range bad {
		if _, err := New(nil, id, nil); err == nil {
			t.Errorf("New(%q) алдаа буцаах ёстой байсан", id)
		}
	}
	for _, id := range []string{"users", "gov", "gateway-console", "core-find", "ai"} {
		if _, err := New(nil, id, nil); err != nil {
			t.Errorf("New(%q) зөвшөөрөгдөх ёстой: %v", id, err)
		}
	}
}

// Дугаарын эрэмбэ нь ЛЕКСИКОГРАФ БИШ байх ёстой: "10_" нь "2_"-оос
// хойно ажиллана. Энэ буруу байсан бол шинэ DB буруу дарааллаар босно.
func TestListOrdersNumerically(t *testing.T) {
	fsys := fstest.MapFS{
		"1_a.up.sql":   {Data: []byte("")},
		"2_b.up.sql":   {Data: []byte("")},
		"10_c.up.sql":  {Data: []byte("")},
		"11_d.up.sql":  {Data: []byte("")},
		"3_e.up.sql":   {Data: []byte("")},
		"1_a.down.sql": {Data: []byte("")},
	}
	r, err := New(nil, "demo", fsys)
	if err != nil {
		t.Fatal(err)
	}
	got, err := r.list("up")
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"1_a.up.sql", "2_b.up.sql", "3_e.up.sql", "10_c.up.sql", "11_d.up.sql"}
	if len(got) != len(want) {
		t.Fatalf("got %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("got %v, want %v", got, want)
		}
	}
}

// Модуль бүр өөрийн lock ID-тай байх ёстой — эс бөгөөс өөр модулиуд
// зэрэг migrate хийхдээ бие биенээ хүлээнэ.
func TestLockIDDistinctAndStable(t *testing.T) {
	ids := []string{"users", "auth", "gov", "ai", "site", "audit", "rbac", "org"}
	seen := map[int64]string{}
	for _, id := range ids {
		l := lockIDFor(id)
		if prev, dup := seen[l]; dup {
			t.Errorf("%q болон %q ижил lock ID (%d)", id, prev, l)
		}
		seen[l] = id
		if l != lockIDFor(id) {
			t.Errorf("%q lock ID тогтворгүй", id)
		}
		if l < 0 {
			t.Errorf("%q lock ID сөрөг (%d)", id, l)
		}
	}
}

func TestListNilFSIsEmpty(t *testing.T) {
	r, err := New(nil, "demo", nil)
	if err != nil {
		t.Fatal(err)
	}
	got, err := r.list("up")
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 0 {
		t.Fatalf("nil FS дээр %v буцаав", got)
	}
}
