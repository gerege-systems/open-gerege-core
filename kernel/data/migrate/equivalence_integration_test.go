//go:build integration

// Gerege Systems Development Team болон Claude AI хамтран бүтээв, 2026.

// Схемийн ТЭНЦҮҮЛЭЛТИЙН тор — migration-уудыг модулиуд руу хуваах ажлын
// цорын ганц бодит хамгаалалт.
//
// Асуулт: "46 глобал migration-аар босгосон DB" ба "зарим нь модулиуд руу
// нүүсэн байдлаар босгосон DB" хоёр ЯГ ИЖИЛ схемтэй юу?
//
// Хэрэв тийм биш бол шинэ суулгац хуучнаасаа ялгаатай болно — тэр нь
// хамгийн аюултай төрлийн алдаа (нэг флотод хоёр өөр схем). Энэ тест
// баганын нэр/төрөл/null/default, индекс, хязгаарлалт, RLS policy-г бүхэлд
// нь харьцуулна.
//
// Migration нүүлгэх PR бүр ЭНЭ ТЕСТИЙГ ногоон байлгах ёстой.
package migrate_test

import (
	"context"
	"fmt"
	"io/fs"
	"sort"
	"strconv"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"

	"github.com/gerege-systems/open-gerege-core/core/test/testenv"
	"github.com/gerege-systems/open-gerege-core/kernel/data/migrate"
	coremigrations "github.com/gerege-systems/open-gerege-core/migrations"
	aimod "github.com/gerege-systems/open-gerege-core/modules/ai"
	govmod "github.com/gerege-systems/open-gerege-core/modules/gov"
	integrationsmod "github.com/gerege-systems/open-gerege-core/modules/integrations"
	platformmod "github.com/gerege-systems/open-gerege-core/modules/platform"
	registrymod "github.com/gerege-systems/open-gerege-core/modules/registry"
	relaymod "github.com/gerege-systems/open-gerege-core/modules/relay"
	sitemod "github.com/gerege-systems/open-gerege-core/modules/site"
)

// schemaFingerprint нь схемийн бүтцийг харьцуулж болохуйц мөр болгоно.
// Зөвхөн хүснэгтийн нэр биш — багана, төрөл, null, default, индекс,
// хязгаарлалт, RLS policy бүгд орно.
func schemaFingerprint(t *testing.T, pool *pgxpool.Pool) string {
	t.Helper()
	ctx := context.Background()
	var b strings.Builder

	q := func(header, sql string) {
		b.WriteString("== " + header + "\n")
		rows, err := pool.Query(ctx, sql)
		require.NoError(t, err, header)
		defer rows.Close()
		var lines []string
		for rows.Next() {
			vals, err := rows.Values()
			require.NoError(t, err)
			parts := make([]string, 0, len(vals))
			for _, v := range vals {
				parts = append(parts, strings.TrimSpace(strings.ToLower(toStr(v))))
			}
			lines = append(lines, strings.Join(parts, "|"))
		}
		require.NoError(t, rows.Err())
		sort.Strings(lines)
		for _, l := range lines {
			b.WriteString(l + "\n")
		}
	}

	// Модулийн бүртгэлийн хүснэгтүүд нь ЗӨРӨХ нь хүлээгдсэн (нэг тал нь
	// mod_*_schema_migrations-тэй, нөгөө нь schema_migrations-тэй) тул
	// хасна — бид ХЭРЭГЛЭЭНИЙ схемийг харьцуулж байна.
	const notBookkeeping = `
		AND c.relname NOT LIKE 'mod\_%\_schema\_migrations'
		AND c.relname <> 'schema_migrations'`

	q("columns", `
		SELECT c.relname, a.attname, format_type(a.atttypid, a.atttypmod),
		       a.attnotnull, COALESCE(pg_get_expr(d.adbin, d.adrelid), '')
		FROM pg_class c
		JOIN pg_namespace n ON n.oid = c.relnamespace AND n.nspname = 'public'
		JOIN pg_attribute a ON a.attrelid = c.oid AND a.attnum > 0 AND NOT a.attisdropped
		LEFT JOIN pg_attrdef d ON d.adrelid = c.oid AND d.adnum = a.attnum
		WHERE c.relkind = 'r'`+notBookkeeping)

	q("indexes", `
		SELECT c.relname, i.indexname, i.indexdef
		FROM pg_indexes i
		JOIN pg_class c ON c.relname = i.tablename
		JOIN pg_namespace n ON n.oid = c.relnamespace AND n.nspname = 'public'
		WHERE i.schemaname = 'public'`+notBookkeeping)

	q("constraints", `
		SELECT c.relname, con.conname, pg_get_constraintdef(con.oid)
		FROM pg_constraint con
		JOIN pg_class c ON c.oid = con.conrelid
		JOIN pg_namespace n ON n.oid = c.relnamespace AND n.nspname = 'public'`+notBookkeeping)

	q("rls", `
		SELECT c.relname, c.relrowsecurity, c.relforcerowsecurity
		FROM pg_class c
		JOIN pg_namespace n ON n.oid = c.relnamespace AND n.nspname = 'public'
		WHERE c.relkind = 'r'`+notBookkeeping)

	q("policies", `
		SELECT tablename, policyname, permissive, roles::text, cmd,
		       COALESCE(qual,''), COALESCE(with_check,'')
		FROM pg_policies WHERE schemaname = 'public'`)

	return b.String()
}

func toStr(v any) string {
	switch x := v.(type) {
	case nil:
		return ""
	case bool:
		if x {
			return "t"
		}
		return "f"
	default:
		// Хүснэгтийн тодорхойлолтууд олон мөртэй байж болно — нэг мөр
		// болгож хэвийн болгоно (зөвхөн зайны ялгаа зөрүү үүсгэхгүй).
		return strings.Join(strings.Fields(fmt.Sprintf("%v", x)), " ")
	}
}

// applyMerged нь ОЛОН эх сурвалжийн *.up.sql-ыг НЭГ глобал тоон
// дарааллаар ажиллуулна — өөрөөр хэлбэл файлууд нүүхээс ӨМНӨХ түүхэн
// дарааллыг яг давтана. Энэ бол лавлах (reference) хэрэгжилт.
func applyMerged(ctx context.Context, pool *pgxpool.Pool, sources ...fs.FS) error {
	type ref struct {
		fsys fs.FS
		name string
	}
	var all []ref
	for _, fsys := range sources {
		names, err := fs.Glob(fsys, "*.up.sql")
		if err != nil {
			return err
		}
		for _, n := range names {
			all = append(all, ref{fsys, n})
		}
	}
	sort.Slice(all, func(i, j int) bool {
		ni, nj := migNum(all[i].name), migNum(all[j].name)
		if ni != nj {
			return ni < nj
		}
		return all[i].name < all[j].name
	})
	for _, r := range all {
		data, err := fs.ReadFile(r.fsys, r.name)
		if err != nil {
			return err
		}
		if _, err := pool.Exec(ctx, string(data)); err != nil {
			return fmt.Errorf("%s: %w", r.name, err)
		}
	}
	return nil
}

func migNum(name string) int {
	i := strings.IndexByte(name, '_')
	if i <= 0 {
		return 1 << 30
	}
	n, err := strconv.Atoi(name[:i])
	if err != nil {
		return 1 << 30
	}
	return n
}

// requireAppRole нь `app_user` role-ыг үүсгэнэ.
//
// ЯАГААД ЧУХАЛ ВЭ: 17_least_privilege_config_grants дахь REVOKE-ууд нь
// `IF NOT EXISTS (app_user) THEN RETURN` хамгаалалтын дотор байдаг.
// Role-гүй тест контейнерт тэр блок БҮХЭЛДЭЭ алгасагддаг тул модуль руу
// нүүсэн хүснэгт дээрх REVOKE хэзээ ч ажиллахгүй — production-д ажилладаг
// атлаа тестэд ажиллахгүй бол энэ нь ХУУРАМЧ НОГООН болно. Role-ыг
// үүсгэснээр тест production-ы замыг жинхэнээсээ туулна.
func requireAppRole(t *testing.T, pool *pgxpool.Pool) {
	t.Helper()
	_, err := pool.Exec(context.Background(), `DO $$
		BEGIN
			IF NOT EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'app_user') THEN
				CREATE ROLE app_user;
			END IF;
		END $$;`)
	require.NoError(t, err)
}

// ЦӨМ ТЕСТ: нүүлгэлтийн дараах шинэ суулгац нь хуучин замтай ЯГ ИЖИЛ
// схем үүсгэх ёстой.
func TestModuleSplitProducesIdenticalSchema(t *testing.T) {
	ctx := context.Background()

	// Migration-аа эзэмшсэн БҮХ модуль — cmd/migration-ийн жагсаалттай
	// ижил дараалалтай байх ёстой.
	sources := []migrate.Source{
		aimod.New(),
		govmod.New(),
		integrationsmod.New(),
		registrymod.New(),
		relaymod.New(),
		sitemod.New(), // platform-ийн ӨМНӨ (43/52 → site_appearance)
		platformmod.New(),
	}
	moduleFS := make([]fs.FS, 0, len(sources))
	for _, s := range sources {
		moduleFS = append(moduleFS, s.Migrations())
	}

	// (A) ЛАВЛАХ: глобал + бүх модулийн файлууд НЭГ тоон дарааллаар —
	// нүүлгэхээс өмнөх түүхэн дараалал яг энэ байсан.
	poolA := testenv.StartPostgresEmpty(t)
	requireAppRole(t, poolA)
	require.NoError(t, applyMerged(ctx, poolA, append([]fs.FS{coremigrations.FS}, moduleFS...)...))
	want := schemaFingerprint(t, poolA)

	// (B) НҮҮЛГЭСЭН ЗАМ: глобалууд эхлээд, дараа нь модуль бүрийн runner
	// өөрийн дарааллаар.
	poolB := testenv.StartPostgresEmpty(t)
	requireAppRole(t, poolB)
	require.NoError(t, applyMerged(ctx, poolB, coremigrations.FS))
	for _, src := range sources {
		r, err := migrate.New(poolB, src.ID(), src.Migrations())
		require.NoError(t, err, src.ID())
		require.NoError(t, r.Up(ctx), src.ID())
	}

	got := schemaFingerprint(t, poolB)
	require.Equal(t, want, got,
		"модуль руу нүүсэн migration нь өөр схем үүсгэв — шинэ суулгац хуучнаасаа зөрнө")
}
