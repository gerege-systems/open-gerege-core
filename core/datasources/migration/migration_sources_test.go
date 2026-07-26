// Олон эх сурвалжийн (суурь + апп) нэгтгэлийн unit тест — DB шаардлагагүй.
//
// Юуг хамгаалж байна вэ: апп нь суурийн embed хийсэн migration дээр өөрийн
// хавтсыг нэмдэг. Хоёр эх сурвалж НЭГ дараалалд, файлын эхний дугаараар
// эрэмбэлэгдэх ёстой — эс тэгвээс аппын 1000+ migration нь суурийн 1..999-
// ээс өмнө ажиллаж, хараахан үүсээгүй хүснэгт рүү хандана.
package migration

import (
	"path/filepath"
	"testing"
	"testing/fstest"
)

func TestListFilesMergesSourcesInNumericOrder(t *testing.T) {
	core := fstest.MapFS{
		"1_create_tables_users.up.sql": {Data: []byte("SELECT 1;")},
		"9_users_name.up.sql":          {Data: []byte("SELECT 1;")},
		"10_users_name_en.up.sql":      {Data: []byte("SELECT 1;")},
	}
	app := fstest.MapFS{
		"1000_ring_registry.up.sql": {Data: []byte("SELECT 1;")},
		"1001_ring_process.up.sql":  {Data: []byte("SELECT 1;")},
	}

	r := NewFS(nil, core, app)
	files, err := r.listFiles("up")
	if err != nil {
		t.Fatal(err)
	}

	want := []string{
		"1_create_tables_users.up.sql",
		"9_users_name.up.sql",
		"10_users_name_en.up.sql",
		"1000_ring_registry.up.sql",
		"1001_ring_process.up.sql",
	}
	if len(files) != len(want) {
		t.Fatalf("файлын тоо: авсан %d, хүлээсэн %d", len(files), len(want))
	}
	for i, w := range want {
		if got := filepath.Base(files[i].name); got != w {
			t.Errorf("байрлал %d: авсан %s, хүлээсэн %s", i, got, w)
		}
	}
}

// Апп нь суурьтай ижил дугаар ашиглавал эрэмбэ нь файлын нэрээр
// шийдэгдэнэ — тогтвортой боловч санамсаргүй. Дугаарын мужийн дүрэм
// (migrations/README.md) яг үүнээс сэргийлдэг; энд зөвхөн зан төлөв нь
// тодорхой (deterministic) байхыг л батална.
func TestListFilesStableOnNumberClash(t *testing.T) {
	a := fstest.MapFS{"5_bbb.up.sql": {Data: []byte("SELECT 1;")}}
	b := fstest.MapFS{"5_aaa.up.sql": {Data: []byte("SELECT 1;")}}

	for i := 0; i < 5; i++ {
		files, err := NewFS(nil, a, b).listFiles("up")
		if err != nil {
			t.Fatal(err)
		}
		if len(files) != 2 || filepath.Base(files[0].name) != "5_aaa.up.sql" {
			t.Fatalf("эрэмбэ тогтворгүй: %v", files)
		}
	}
}
