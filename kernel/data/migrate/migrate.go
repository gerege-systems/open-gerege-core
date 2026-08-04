// Gerege Systems Development Team болон Claude AI хамтран бүтээв, 2026.

// Package migrate нь МОДУЛЬ БҮРИЙН өөрийн migration-уудыг ажиллуулах
// runner (плангийн Phase 2). core/datasources/migration-ийн залгамжлагч
// боловч түүнийг СОЛИХГҮЙ: суурийн глобал migration-ууд тэндээ хэвээр
// ажиллана, модуль өөрийн SQL-ээ зарлаж эхэлсэн үедээ л энэ runner-т
// шилжинэ. Ингэснээр нүүлгэлт модуль тус бүрээр, буцаах боломжтойгоор
// явна.
//
// # Ялгаанууд
//
//   - Төлөв нь модуль тус бүрийн `mod_<id>_schema_migrations` хүснэгтэд
//     хадгалагдана — глобал `schema_migrations`-д ХҮРЭХГҮЙ. Модулийг
//     нэмэх/хасахад бусад модулийн бүртгэл хөндөгдөхгүй.
//   - Дугаарлалт нь модуль дотроо (1-ээс эхэлнэ) — репо даяарх нэг
//     дугаарын мужийн зөрчил (RANGE файл) алга болно.
//   - Adopt: аль хэдийн ажиллаж буй DB-д baseline-ыг ДАХИН ажиллуулахгүй,
//     зөвхөн "хэрэгжсэн" гэж тэмдэглэнэ (Runner.Adopt).
//
// # Яагаад SET SCHEMA хийгээгүй вэ
//
// Плангийн ноорогт модуль бүрийг Postgres-ийн ТУСДАА schema руу
// (`ALTER TABLE ... SET SCHEMA`) нүүлгэх санаа байсан. Үүнийг ЗОРИУД
// хийгээгүй: модуль хооронд гадаад түлхүүр олон (users, roles г.м.), 11
// файлд RLS policy байгаа бөгөөд Go давхаргын гар бичмэл SQL бүхэлдээ
// схемгүй (unqualified) нэр ашигладаг — өөрөөр хэлбэл холболт бүр дээр
// search_path удирдах шаардлагатай болно. Migration-ийн ТӨЛӨВИЙГ
// тусгаарлахад тэр физик тусгаарлалт шаардлагагүй. Хэрэв хожим хэрэгтэй
// бол тусдаа, болгоомжтой PR-аар.
package migrate

import (
	"cmp"
	"context"
	"fmt"
	"io/fs"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"
)

// AdvisoryLockID нь модулийн migration-д зориулсан advisory lock-ийн
// суурь ID. Модуль бүр өөрийн ID-тай (суурь + модулийн нэрийн hash) тул
// өөр өөр модуль зэрэг migrate хийж чадна, харин НЭГ модулийг хоёр
// процесс зэрэг migrate хийхгүй.
const AdvisoryLockID int64 = 947328461231

// idPattern нь модулийн ID-г хүснэгтийн нэрэнд оруулахын өмнө хязгаарлана.
// Хүснэгтийн нэрийг параметржүүлэх боломжгүй тул (identifier), ID-г
// хатуу шалгаж SQL injection-ий гадаргууг бүрэн хаана.
var idPattern = regexp.MustCompile(`^[a-z][a-z0-9]*(-[a-z0-9]+)*$`)

// Runner нь нэг модулийн migration-уудыг ажиллуулна.
type Runner struct {
	pool   *pgxpool.Pool
	id     string
	fsys   fs.FS
	table  string
	lockID int64
}

// New нь модулийн ID болон түүний migration FS-ээр runner үүсгэнэ.
// ID нь манифестийнхтэй ижил байх ёстой.
func New(pool *pgxpool.Pool, moduleID string, fsys fs.FS) (*Runner, error) {
	if !idPattern.MatchString(moduleID) {
		return nil, fmt.Errorf("migrate: модулийн ID %q буруу форматтай (a-z, 0-9, дефис)", moduleID)
	}
	return &Runner{
		pool:   pool,
		id:     moduleID,
		fsys:   fsys,
		table:  TableName(moduleID),
		lockID: lockIDFor(moduleID),
	}, nil
}

// TableName нь модулийн төлвийн хүснэгтийн нэрийг буцаана.
func TableName(moduleID string) string {
	return "mod_" + strings.ReplaceAll(moduleID, "-", "_") + "_schema_migrations"
}

// lockIDFor нь модуль бүрт тогтвортой, давхцахааргүй advisory lock ID
// гаргана (FNV-1a; санамсаргүй биш тул restart-д тогтвортой).
func lockIDFor(moduleID string) int64 {
	var h uint64 = 14695981039346656037
	for i := 0; i < len(moduleID); i++ {
		h ^= uint64(moduleID[i])
		h *= 1099511628211
	}
	// Тэмдэгт битийг цэвэрлэж эерэг утга болгоно — pg_advisory_lock нь
	// int64 авдаг ч сөрөг утга уншихад төвөгтэй.
	return AdvisoryLockID + int64(h>>1)%1_000_000_000
}

// Pending нь хараахан хэрэгжээгүй up-migration-уудын нэрсийг эрэмбээр нь
// буцаана (юу ажиллахыг урьдчилан харах, тестэд).
func (r *Runner) Pending(ctx context.Context) ([]string, error) {
	var out []string
	err := r.withLock(ctx, func(ctx context.Context, conn *pgxpool.Conn) error {
		if err := r.ensureTable(ctx, conn); err != nil {
			return err
		}
		applied, err := r.applied(ctx, conn)
		if err != nil {
			return err
		}
		files, err := r.list("up")
		if err != nil {
			return err
		}
		for _, f := range files {
			if !applied[f] {
				out = append(out, f)
			}
		}
		return nil
	})
	return out, err
}

// Up нь хэрэгжээгүй up-migration-уудыг дугаарын дарааллаар хэрэгжүүлнэ.
// Файл бүр өөрийн SQL болон бүртгэлийн мөрөө НЭГ транзакцид commit хийнэ
// — дунд нь гацвал хэсэгчилсэн бичлэг үлдэхгүй.
func (r *Runner) Up(ctx context.Context) error {
	return r.withLock(ctx, func(ctx context.Context, conn *pgxpool.Conn) error {
		if err := r.ensureTable(ctx, conn); err != nil {
			return err
		}
		applied, err := r.applied(ctx, conn)
		if err != nil {
			return err
		}
		files, err := r.list("up")
		if err != nil {
			return err
		}
		for _, name := range files {
			if applied[name] {
				continue
			}
			if err := r.exec(ctx, conn, name, name, true); err != nil {
				return err
			}
		}
		return nil
	})
}

// Adopt нь migration-уудыг АЖИЛЛУУЛАЛГҮЙ "хэрэгжсэн" гэж тэмдэглэнэ.
//
// Энэ нь ажиллаж буй DB-ийн re-baseline-д зориулагдсан: хүснэгтүүд нь
// глобал migration-аар аль хэдийн үүссэн байхад модулийн baseline-ыг
// дахин ажиллуулбал алдаа өгнө (эсвэл бүр дордуулна). Adopt нь зөвхөн
// бүртгэлийг гүйцээнэ.
//
// АНХААР: зөвхөн baseline нь одоо байгаа схемтэй ТЭНЦҮҮ үед дуудна.
// Дуудагч (kernel boot) үүнийг глобал schema_migrations-ийн ул мөрөөр
// шийднэ — ShouldAdopt-ыг үз.
func (r *Runner) Adopt(ctx context.Context, names []string) error {
	return r.withLock(ctx, func(ctx context.Context, conn *pgxpool.Conn) error {
		if err := r.ensureTable(ctx, conn); err != nil {
			return err
		}
		for _, n := range names {
			if _, err := conn.Exec(ctx,
				fmt.Sprintf(`INSERT INTO %s(name) VALUES ($1) ON CONFLICT DO NOTHING`, r.table), n); err != nil {
				return fmt.Errorf("migrate %s: adopt %s: %w", r.id, n, err)
			}
		}
		return nil
	})
}

// AdoptAll нь модулийн бүх up-migration-ыг adopt хийнэ.
func (r *Runner) AdoptAll(ctx context.Context) error {
	files, err := r.list("up")
	if err != nil {
		return err
	}
	return r.Adopt(ctx, files)
}

// Applied нь хэрэгжсэн гэж бүртгэгдсэн нэрсийг буцаана.
func (r *Runner) Applied(ctx context.Context) ([]string, error) {
	var out []string
	err := r.withLock(ctx, func(ctx context.Context, conn *pgxpool.Conn) error {
		if err := r.ensureTable(ctx, conn); err != nil {
			return err
		}
		m, err := r.applied(ctx, conn)
		if err != nil {
			return err
		}
		for n := range m {
			out = append(out, n)
		}
		slices.Sort(out)
		return nil
	})
	return out, err
}

// Down нь ХАМГИЙН СҮҮЛД хэрэгжсэн migration-ыг буцаана (нэгийг).
//
// Бодлого (плангийн Phase 5, энд урьдчилан баримтжуулав): шинэ
// migration-ууд **expand–contract** байх ёстой — эвдрэлтэй өөрчлөлтийг
// нэмэх (expand) ба хасах (contract) хоёр тусдаа release-д хуваана.
// Тиймээс production rollback нь down migration-аас БИШ, өмнөх хувилбарын
// binary-г буцаахаас хамаарна. Down нь зөвхөн хөгжүүлэлт/тестэд.
func (r *Runner) Down(ctx context.Context) error {
	return r.withLock(ctx, func(ctx context.Context, conn *pgxpool.Conn) error {
		if err := r.ensureTable(ctx, conn); err != nil {
			return err
		}
		applied, err := r.applied(ctx, conn)
		if err != nil {
			return err
		}
		downs, err := r.list("down")
		if err != nil {
			return err
		}
		slices.Reverse(downs)
		for _, d := range downs {
			up := strings.TrimSuffix(d, ".down.sql") + ".up.sql"
			if !applied[up] {
				continue
			}
			return r.exec(ctx, conn, d, up, false)
		}
		return nil
	})
}

// ── дотоод ────────────────────────────────────────────────────────────

func (r *Runner) withLock(ctx context.Context, fn func(context.Context, *pgxpool.Conn) error) error {
	conn, err := r.pool.Acquire(ctx)
	if err != nil {
		return fmt.Errorf("migrate %s: acquire conn: %w", r.id, err)
	}
	defer conn.Release()

	if _, err := conn.Exec(ctx, `SELECT pg_advisory_lock($1)`, r.lockID); err != nil {
		return fmt.Errorf("migrate %s: advisory lock: %w", r.id, err)
	}
	defer func() { _, _ = conn.Exec(ctx, `SELECT pg_advisory_unlock($1)`, r.lockID) }()
	return fn(ctx, conn)
}

func (r *Runner) ensureTable(ctx context.Context, conn *pgxpool.Conn) error {
	// r.table нь idPattern-ээр шалгагдсан ID-аас гарсан тул интерполяци
	// аюулгүй (identifier-ийг параметржүүлэх боломжгүй).
	_, err := conn.Exec(ctx, fmt.Sprintf(`
		CREATE TABLE IF NOT EXISTS %s (
			name        TEXT PRIMARY KEY,
			applied_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
		);`, r.table))
	if err != nil {
		return fmt.Errorf("migrate %s: create %s: %w", r.id, r.table, err)
	}
	return nil
}

func (r *Runner) applied(ctx context.Context, conn *pgxpool.Conn) (map[string]bool, error) {
	rows, err := conn.Query(ctx, fmt.Sprintf(`SELECT name FROM %s`, r.table))
	if err != nil {
		return nil, fmt.Errorf("migrate %s: load applied: %w", r.id, err)
	}
	defer rows.Close()
	out := map[string]bool{}
	for rows.Next() {
		var n string
		if err := rows.Scan(&n); err != nil {
			return nil, err
		}
		out[n] = true
	}
	return out, rows.Err()
}

// list нь модулийн FS доторх *.<action>.sql файлуудыг ДУГААРААР эрэмбэлж
// буцаана. Лексикограф эрэмбэ буруу ('10_' < '1_') тул тоон эрэмбэ.
func (r *Runner) list(action string) ([]string, error) {
	if r.fsys == nil {
		return nil, nil
	}
	names, err := fs.Glob(r.fsys, "*."+action+".sql")
	if err != nil {
		return nil, fmt.Errorf("migrate %s: glob: %w", r.id, err)
	}
	slices.SortFunc(names, func(a, b string) int {
		if c := cmp.Compare(number(a), number(b)); c != 0 {
			return c
		}
		return cmp.Compare(a, b)
	})
	return names, nil
}

func (r *Runner) exec(ctx context.Context, conn *pgxpool.Conn, file, upName string, isUp bool) error {
	data, err := fs.ReadFile(r.fsys, file)
	if err != nil {
		return fmt.Errorf("migrate %s: read %s: %w", r.id, file, err)
	}
	tx, err := conn.Begin(ctx)
	if err != nil {
		return fmt.Errorf("migrate %s: begin: %w", r.id, err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	if _, err := tx.Exec(ctx, string(data)); err != nil {
		return fmt.Errorf("migrate %s: exec %s: %w", r.id, filepath.Base(file), err)
	}
	if isUp {
		_, err = tx.Exec(ctx, fmt.Sprintf(`INSERT INTO %s(name) VALUES ($1) ON CONFLICT DO NOTHING`, r.table), upName)
	} else {
		_, err = tx.Exec(ctx, fmt.Sprintf(`DELETE FROM %s WHERE name = $1`, r.table), upName)
	}
	if err != nil {
		return fmt.Errorf("migrate %s: record %s: %w", r.id, upName, err)
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("migrate %s: commit %s: %w", r.id, filepath.Base(file), err)
	}
	return nil
}

// number нь "N_name.up.sql"-ээс N-г буцаана; дугааргүй бол хамгийн сүүлд.
func number(name string) int {
	base := filepath.Base(name)
	i := strings.IndexByte(base, '_')
	if i <= 0 {
		return int(^uint(0) >> 1)
	}
	n, err := strconv.Atoi(base[:i])
	if err != nil {
		return int(^uint(0) >> 1)
	}
	return n
}
