//go:build integration

// Gerege Systems Development Team болон Claude AI хамтран бүтээв, 2026.

// Модулийн migration engine-ийн integration тестүүд — жинхэнэ Postgres.
//
// Хамгийн чухал нь TestAdoptDoesNotTouchExistingSchema: ажиллаж буй
// (глобал migration-аар босгогдсон) DB дээр шинэ runner ажиллахад НЭГ Ч
// хүснэгт, НЭГ Ч мөр алдагдахгүй байхыг баталдаг. Энэ тест унавал
// re-baseline-ыг production-д ойртуулж БОЛОХГҮЙ.
package migrate_test

import (
	"context"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"

	"github.com/gerege-systems/open-gerege-core/core/test/testenv"
	"github.com/gerege-systems/open-gerege-core/kernel/data/migrate"
)

// demoFS нь "модулийн baseline"-ыг дуурайна: аль хэдийн байгаа хүснэгтийг
// ДАХИН үүсгэх гэж оролддог SQL. Adopt зөв ажиллаж байвал энэ SQL хэзээ ч
// ажиллахгүй; буруу бол users хүснэгтийг сүйтгэх оролдлого болж тест унана.
var demoFS = fstest.MapFS{
	"1_baseline.up.sql": {Data: []byte(`
		CREATE TABLE demo_module_table (id BIGSERIAL PRIMARY KEY, note TEXT);
	`)},
	"1_baseline.down.sql": {Data: []byte(`DROP TABLE IF EXISTS demo_module_table;`)},
	"2_add_col.up.sql": {Data: []byte(`
		ALTER TABLE demo_module_table ADD COLUMN extra TEXT;
	`)},
	"2_add_col.down.sql": {Data: []byte(`
		ALTER TABLE demo_module_table DROP COLUMN extra;
	`)},
}

// hostileFS нь adopt буруу ажилласан тохиолдолд ЗОРИУД сүйрэл үүсгэнэ —
// "чимээгүй амжилт"-ыг тестээр барихын тулд.
var hostileFS = fstest.MapFS{
	"1_baseline.up.sql": {Data: []byte(`DROP TABLE users CASCADE;`)},
}

func tableNames(t *testing.T, pool *pgxpool.Pool) []string {
	t.Helper()
	rows, err := pool.Query(context.Background(),
		`SELECT tablename FROM pg_tables WHERE schemaname='public' ORDER BY tablename`)
	require.NoError(t, err)
	defer rows.Close()
	var out []string
	for rows.Next() {
		var n string
		require.NoError(t, rows.Scan(&n))
		out = append(out, n)
	}
	require.NoError(t, rows.Err())
	return out
}

// ЦӨМ ТЕСТ: хуучин schema-тай DB → шинэ runner → юу ч алдагдахгүй.
func TestAdoptDoesNotTouchExistingSchema(t *testing.T) {
	ctx := context.Background()
	pool := testenv.StartPostgres(t) // бүх глобал migration хэрэгжсэн

	// Бодит өгөгдөл — adopt нь өгөгдөлд хүрэхгүйг батлахын тулд.
	_, err := pool.Exec(ctx,
		`INSERT INTO users (id, username, email, password, active, role_id, created_at)
		 VALUES (gen_random_uuid(), 'adopt-probe', 'adopt-probe@example.com', 'x', true, 1, NOW())`)
	require.NoError(t, err, "туршилтын хэрэглэгч үүсгэх")

	before := tableNames(t, pool)
	var usersBefore int
	require.NoError(t, pool.QueryRow(ctx, `SELECT COUNT(*) FROM users`).Scan(&usersBefore))
	require.Positive(t, usersBefore)

	// Ажиллаж буй DB гэдгийг runner өөрөө таних ёстой.
	adopt, err := migrate.ShouldAdopt(ctx, pool)
	require.NoError(t, err)
	require.True(t, adopt, "глобал schema_migrations мөртэй DB нь adopt горимд байх ёстой")

	// hostileFS нь ажиллавал users хүснэгтийг устгана. Adopt зөв бол
	// SQL ажиллахгүй — зөвхөн бүртгэгдэнэ.
	r, err := migrate.New(pool, "hostile", hostileFS)
	require.NoError(t, err)
	require.NoError(t, r.AdoptAll(ctx))

	// Adopt нь өөрийн бүртгэлийн хүснэгтийг (mod_*_schema_migrations)
	// НЭМНЭ — тэр бол хүлээгдсэн цорын ганц өөрчлөлт. Бусад бүх хүснэгт
	// хэвээр байх ёстой: нэг ч алдагдаагүй, нэг ч дарагдаагүй.
	after := tableNames(t, pool)
	var afterExisting []string
	for _, n := range after {
		if strings.HasPrefix(n, "mod_") && strings.HasSuffix(n, "_schema_migrations") {
			continue
		}
		afterExisting = append(afterExisting, n)
	}
	require.Equal(t, before, afterExisting, "adopt дараа өмнөх хүснэгтүүд өөрчлөгдөх ёсгүй")
	require.Contains(t, after, migrate.TableName("hostile"), "бүртгэлийн хүснэгт үүссэн байх ёстой")

	var usersAfter int
	require.NoError(t, pool.QueryRow(ctx, `SELECT COUNT(*) FROM users`).Scan(&usersAfter))
	require.Equal(t, usersBefore, usersAfter, "adopt өгөгдөлд хүрэх ёсгүй")

	// Бүртгэл гүйцээгдсэн байх ёстой — дараагийн boot дахин оролдохгүй.
	applied, err := r.Applied(ctx)
	require.NoError(t, err)
	require.Equal(t, []string{"1_baseline.up.sql"}, applied)

	pending, err := r.Pending(ctx)
	require.NoError(t, err)
	require.Empty(t, pending, "adopt дараа pending үлдэх ёсгүй")
}

// Цэвэр DB дээр adopt хийхгүй — baseline ЖИНХЭНЭЭСЭЭ ажиллана.
func TestFreshDatabaseRunsMigrations(t *testing.T) {
	ctx := context.Background()
	pool := testenv.StartPostgresEmpty(t)

	adopt, err := migrate.ShouldAdopt(ctx, pool)
	require.NoError(t, err)
	require.False(t, adopt, "хоосон DB нь adopt горимд байх ЁСГҮЙ")

	r, err := migrate.New(pool, "demo", demoFS)
	require.NoError(t, err)

	pending, err := r.Pending(ctx)
	require.NoError(t, err)
	require.Len(t, pending, 2)

	require.NoError(t, r.Up(ctx))

	// Хоёр migration хоёулаа хэрэгжсэн эсэх (багана нэмэгдсэн үү).
	_, err = pool.Exec(ctx, `INSERT INTO demo_module_table (note, extra) VALUES ('a','b')`)
	require.NoError(t, err, "2_add_col хэрэгжсэн байх ёстой")

	applied, err := r.Applied(ctx)
	require.NoError(t, err)
	require.Equal(t, []string{"1_baseline.up.sql", "2_add_col.up.sql"}, applied)
}

// Дахин ажиллуулалт idempotent — boot бүрт дахин хэрэгжүүлэхгүй.
func TestUpIsIdempotent(t *testing.T) {
	ctx := context.Background()
	pool := testenv.StartPostgresEmpty(t)

	r, err := migrate.New(pool, "demo", demoFS)
	require.NoError(t, err)
	require.NoError(t, r.Up(ctx))
	// Хоёр дахь удаа — CREATE TABLE давхар ажиллавал алдаа өгнө.
	require.NoError(t, r.Up(ctx), "хоёр дахь Up нь no-op байх ёстой")

	pending, err := r.Pending(ctx)
	require.NoError(t, err)
	require.Empty(t, pending)
}

// Модуль бүр ТУСДАА төлвийн хүснэгттэй — нэгнийх нь бүртгэл нөгөөд
// нөлөөлөхгүй, глобал schema_migrations-д хүрэхгүй.
func TestPerModuleStateIsIsolated(t *testing.T) {
	ctx := context.Background()
	pool := testenv.StartPostgresEmpty(t)

	a, err := migrate.New(pool, "alpha", fstest.MapFS{
		"1_a.up.sql": {Data: []byte(`CREATE TABLE alpha_t (id INT);`)},
	})
	require.NoError(t, err)
	b, err := migrate.New(pool, "beta", fstest.MapFS{
		"1_b.up.sql": {Data: []byte(`CREATE TABLE beta_t (id INT);`)},
	})
	require.NoError(t, err)

	require.NoError(t, a.Up(ctx))

	bPending, err := b.Pending(ctx)
	require.NoError(t, err)
	require.Len(t, bPending, 1, "alpha-гийн Up нь beta-гийн pending-д нөлөөлөх ёсгүй")

	require.NoError(t, b.Up(ctx))

	// Хоёр тусдаа хүснэгт үүссэн байх ёстой.
	for _, tbl := range []string{
		migrate.TableName("alpha"), migrate.TableName("beta"),
	} {
		var exists bool
		require.NoError(t, pool.QueryRow(ctx, `SELECT to_regclass($1) IS NOT NULL`, tbl).Scan(&exists))
		require.True(t, exists, "%s байх ёстой", tbl)
	}

	// Глобал хүснэгтэд хүрээгүй байх ёстой.
	var legacyExists bool
	require.NoError(t, pool.QueryRow(ctx,
		`SELECT to_regclass($1) IS NOT NULL`, migrate.LegacyTable).Scan(&legacyExists))
	require.False(t, legacyExists, "модулийн runner глобал schema_migrations үүсгэх ёсгүй")
}

// Down нь сүүлийн нэг migration-ыг буцаана (зөвхөн хөгжүүлэлт/тестэд;
// production rollback нь expand–contract бодлогоор binary-аар явна).
func TestDownRevertsLastOnly(t *testing.T) {
	ctx := context.Background()
	pool := testenv.StartPostgresEmpty(t)

	r, err := migrate.New(pool, "demo", demoFS)
	require.NoError(t, err)
	require.NoError(t, r.Up(ctx))

	require.NoError(t, r.Down(ctx))
	applied, err := r.Applied(ctx)
	require.NoError(t, err)
	require.Equal(t, []string{"1_baseline.up.sql"}, applied, "зөвхөн сүүлийнх буцах ёстой")

	// extra багана алга болсон байх ёстой.
	_, err = pool.Exec(ctx, `INSERT INTO demo_module_table (note, extra) VALUES ('a','b')`)
	require.Error(t, err, "2_add_col буцсан тул extra багана байх ёсгүй")
}

// Схем боссон атлаа бүртгэл нь ХООСОН DB (pg_dump/restore, эсвэл
// migration-уудыг runner-ээр биш шууд ажиллуулсан тохиолдол) мөн adopt
// горимд орох ёстой — эс бөгөөс runner амьд хүснэгт дээр CREATE TABLE
// ажиллуулна.
func TestAdoptDetectsSchemaWithoutBookkeeping(t *testing.T) {
	ctx := context.Background()
	pool := testenv.StartPostgresEmpty(t)

	// Глобал бүртгэлгүйгээр sentinel хүснэгтийг гараар үүсгэнэ —
	// сэргээсэн dump-ийн ойролцоо загвар.
	_, err := pool.Exec(ctx, `CREATE TABLE users (id uuid PRIMARY KEY)`)
	require.NoError(t, err)

	var legacy bool
	require.NoError(t, pool.QueryRow(ctx,
		`SELECT to_regclass($1) IS NOT NULL`, migrate.LegacyTable).Scan(&legacy))
	require.False(t, legacy, "энэ хувилбарт глобал бүртгэл байхгүй")

	adopt, err := migrate.ShouldAdopt(ctx, pool)
	require.NoError(t, err)
	require.True(t, adopt, "схем боссон DB нь бүртгэлгүй ч adopt горимд байх ёстой")
}
