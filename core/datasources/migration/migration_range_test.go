// Migration-ий дугаарлалтын хамгаалалт — DB шаардлагагүй тул integration tag-гүй.
//
// Юуг хамгаалж байна вэ: runner нь файлуудыг эхний дугаараар эрэмбэлж
// ажиллуулдаг тул нэг дугаарыг хоёр файл эзэмбэл эрэмбэ нь файлын нэрээр
// санамсаргүй шийдэгддэг. Мөн суурь (core) болон апп өөрийн migration-ууд
// нэг дугаарын мужид орвол repo хооронд merge хийхэд эрэмбэ будлиантана.
//
// Дүрмийг migrations/README.md-д тайлбарлав. Муж нь migrations/RANGE-д,
// конвенцоос өмнөх хуучин файлууд migrations/LEGACY-д байрлана.
package migration

import (
	"bufio"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"testing"
)

const migrationsDir = "../../../migrations"

// readLines нь тайлбар (#) болон хоосон мөрийг алгасаж мөрүүдийг буцаана.
func readLines(t *testing.T, path string) []string {
	t.Helper()
	f, err := os.Open(path) // #nosec G304 — тестийн тогтмол зам
	if err != nil {
		t.Fatalf("%s уншиж чадсангүй: %v", path, err)
	}
	defer func() { _ = f.Close() }()

	var out []string
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		out = append(out, line)
	}
	if err := sc.Err(); err != nil {
		t.Fatalf("%s scan: %v", path, err)
	}
	return out
}

// readRange нь "эхлэл төгсгөл" хэлбэрийн RANGE файлыг задлана.
func readRange(t *testing.T) (lo, hi int) {
	t.Helper()
	lines := readLines(t, filepath.Join(migrationsDir, "RANGE"))
	if len(lines) != 1 {
		t.Fatalf("RANGE файлд яг нэг утгын мөр байх ёстой, авсан %d", len(lines))
	}
	parts := strings.Fields(lines[0])
	if len(parts) != 2 {
		t.Fatalf("RANGE формат нь \"эхлэл төгсгөл\" байх ёстой, авсан %q", lines[0])
	}
	lo, err := strconv.Atoi(parts[0])
	if err != nil {
		t.Fatalf("RANGE эхлэл тоо биш: %v", err)
	}
	hi, err = strconv.Atoi(parts[1])
	if err != nil {
		t.Fatalf("RANGE төгсгөл тоо биш: %v", err)
	}
	if lo > hi {
		t.Fatalf("RANGE эхлэл (%d) төгсгөлөөс (%d) их байна", lo, hi)
	}
	return lo, hi
}

func upFiles(t *testing.T) []string {
	t.Helper()
	files, err := filepath.Glob(filepath.Join(migrationsDir, "*.up.sql"))
	if err != nil {
		t.Fatalf("migration файл олдсонгүй: %v", err)
	}
	if len(files) == 0 {
		t.Fatal("migrations хавтас хоосон байна — зам буруу байж болзошгүй")
	}
	names := make([]string, 0, len(files))
	for _, f := range files {
		names = append(names, filepath.Base(f))
	}
	sort.Strings(names)
	return names
}

// legacySet нь конвенцоос чөлөөлөгдсөн хуучин migration-уудыг буцаана.
func legacySet(t *testing.T) map[string]bool {
	t.Helper()
	set := map[string]bool{}
	for _, l := range readLines(t, filepath.Join(migrationsDir, "LEGACY")) {
		set[l] = true
	}
	return set
}

// TestMigrationNumbersUnique — LEGACY-д ороогүй migration-ууд дугаараа
// хуваалцаж болохгүй. Хуваалцвал ажиллах эрэмбэ нь файлын нэрээр
// санамсаргүй шийдэгдэнэ.
// Шинэ файл нь LEGACY доторх дугаартай мөргөлдөх нь бас алдаа тул
// бүлэглэхдээ бүх файлыг тооцож, дор хаяж нэг шинэ файл оролцсон
// бүлгийг л мэдээлнэ.
func TestMigrationNumbersUnique(t *testing.T) {
	legacy := legacySet(t)
	byNum := map[int][]string{}
	for _, name := range upFiles(t) {
		byNum[migrationNumber(name)] = append(byNum[migrationNumber(name)], name)
	}
	for num, names := range byNum {
		if len(names) < 2 {
			continue
		}
		fresh := false
		for _, n := range names {
			if !legacy[n] {
				fresh = true
				break
			}
		}
		if !fresh {
			continue // бүгд хуучин — production-д хэрэгжсэн тул хэвээр үлдээнэ
		}
		sort.Strings(names)
		t.Errorf("%d дугаарыг %d файл эзэмшиж байна: %v — шинэ migration-д сул дугаар өг",
			num, len(names), names)
	}
}

// TestMigrationNumbersInRange — шинэ migration нь энэ репод хуваарилагдсан
// мужид байх ёстой (migrations/RANGE).
func TestMigrationNumbersInRange(t *testing.T) {
	lo, hi := readRange(t)
	legacy := legacySet(t)
	for _, name := range upFiles(t) {
		if legacy[name] {
			continue
		}
		n := migrationNumber(name)
		if n < lo || n > hi {
			t.Errorf("%s: дугаар %d нь энэ репогийн муж [%d..%d]-аас гадуур байна "+
				"(migrations/README.md харна уу)", name, n, lo, hi)
		}
	}
}

// TestMigrationsHaveDownPair — up бүрд down хос байх ёстой, эс тэгвээс
// буцаах боломжгүй migration production-д үлдэнэ. Анхны схем үүсгэдэг
// хуучин файлууд (LEGACY) чөлөөлөгдөнө — тэдгээрийг буцаах нь DB-г
// бүхэлд нь устгахтай адил тул зориуд хос үлдээгээгүй.
func TestMigrationsHaveDownPair(t *testing.T) {
	legacy := legacySet(t)
	for _, up := range upFiles(t) {
		if legacy[up] {
			continue
		}
		down := strings.TrimSuffix(up, ".up.sql") + ".down.sql"
		if _, err := os.Stat(filepath.Join(migrationsDir, down)); err != nil {
			t.Errorf("%s: хос %s файл алга", up, down)
		}
	}
}

// TestLegacyEntriesExist — LEGACY нь бодит файлуудыг заана; устсан migration
// жагсаалтад үлдвэл хамгаалалт чимээгүйхэн суларна.
func TestLegacyEntriesExist(t *testing.T) {
	present := map[string]bool{}
	for _, n := range upFiles(t) {
		present[n] = true
	}
	for name := range legacySet(t) {
		if !present[name] {
			t.Errorf("LEGACY дахь %q файл байхгүй — мөрийг устга", name)
		}
	}
}
