// Gerege Template Version 27.0
// Gerege Systems Development Team болон Claude AI хамтран бүтээв, 2026.

// Эрэмбийн unit тест — DB шаардлагагүй тул integration tag-гүй.
// "10_" нь лексикографоор "1_"-ээс өмнө ордог ('0' < '_') байсан
// регрессийг хамгаална: шинэ хоосон DB дээр 10-р migration 1-ээс
// түрүүлж ажиллаад унадаг байв.
package migration

import (
	"os"
	"path/filepath"
	"testing"
)

func TestListFilesNumericOrder(t *testing.T) {
	dir := t.TempDir()
	names := []string{
		"10_users_name_en", "1_create_tables_users", "2_create_extensions_uuid",
		"9_users_name", "11_ai_prompts_knowledge", "15_audit_log",
	}
	for _, n := range names {
		if err := os.WriteFile(filepath.Join(dir, n+".up.sql"), []byte("SELECT 1;"), 0o600); err != nil {
			t.Fatal(err)
		}
	}

	r := &Runner{dir: dir}
	files, err := r.listFiles("up")
	if err != nil {
		t.Fatal(err)
	}

	want := []string{
		"1_create_tables_users.up.sql", "2_create_extensions_uuid.up.sql",
		"9_users_name.up.sql", "10_users_name_en.up.sql",
		"11_ai_prompts_knowledge.up.sql", "15_audit_log.up.sql",
	}
	if len(files) != len(want) {
		t.Fatalf("файлын тоо: авсан %d, хүлээсэн %d", len(files), len(want))
	}
	for i, w := range want {
		if got := filepath.Base(files[i]); got != w {
			t.Errorf("байрлал %d: авсан %s, хүлээсэн %s", i, got, w)
		}
	}
}

func TestMigrationNumber(t *testing.T) {
	cases := map[string]int{
		"1_a.up.sql":   1,
		"10_b.up.sql":  10,
		"007_c.up.sql": 7,
	}
	for name, want := range cases {
		if got := migrationNumber(name); got != want {
			t.Errorf("%s: авсан %d, хүлээсэн %d", name, got, want)
		}
	}
	// Дугааргүй файл хамгийн сүүлд эрэмбэлэгдэнэ.
	if got := migrationNumber("no_number_here.up.sql"); got != int(^uint(0)>>1) {
		t.Errorf("дугааргүй нэр MaxInt байх ёстой, авсан %d", got)
	}
}
