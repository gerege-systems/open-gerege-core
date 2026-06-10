//go:build integration

// Gerege Template Version 27.0
// Gerege Systems Development Team болон Claude AI хамтран бүтээв, 2026.

package migration_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"template/internal/datasources/migration"
	"template/internal/test/testenv"
	"template/pkg/logger"
)

// writeMigration нь dir дотор зохиомол migration хосыг үүсгэдэг жижиг
// туслах функц бөгөөд тест нь жинхэнэ cmd/migration файлуудаас
// (хувьсан өөрчлөгддөг бөгөөд тестийг schema өөрчлөлттэй холбох болно)
// хамааралгүй байх боломжийг олгоно.
func writeMigration(t *testing.T, dir, num, body, downBody string) {
	t.Helper()
	require.NoError(t, os.WriteFile(filepath.Join(dir, num+"_test.up.sql"), []byte(body), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(dir, num+"_test.down.sql"), []byte(downBody), 0o600))
}

func newRunner(t *testing.T) (*migration.Runner, string) {
	t.Helper()
	pool := testenv.StartPostgresEmpty(t)
	dir := t.TempDir()
	r := migration.New(pool, dir)
	// runner-ийн чимээг намжаа — testing.T нь юу амжилтгүй болсныг аль
	// хэдийн харуулдаг.
	r.SetLogger(func(string, logger.Fields) {})
	return r, dir
}

func TestRunner_UpIsIdempotent(t *testing.T) {
	r, dir := newRunner(t)
	ctx := context.Background()

	writeMigration(t, dir, "1",
		`CREATE TABLE widgets (id SERIAL PRIMARY KEY, name TEXT NOT NULL);`,
		`DROP TABLE widgets;`,
	)

	require.NoError(t, r.Up(ctx))
	// Хоёр дахь дуудалт no-op байх ёстой — schema_migrations нь файлыг
	// богино холбоодог. Хяналтын хүснэгтгүйгээр энэ нь "relation widgets
	// already exists" гэж алдаа өгөх байсан.
	require.NoError(t, r.Up(ctx), "second Up should be a no-op")
}

func TestRunner_DownThenUpRoundTrip(t *testing.T) {
	r, dir := newRunner(t)
	ctx := context.Background()

	writeMigration(t, dir, "1",
		`CREATE TABLE widgets (id SERIAL PRIMARY KEY);`,
		`DROP TABLE widgets;`,
	)

	require.NoError(t, r.Up(ctx))
	require.NoError(t, r.Down(ctx))
	// Down-ийн дараа schema_migrations дахь мөр алга болсон байх ёстой
	// тул дараагийн Up нь migration-г цэвэрхэн хэрэгжүүлнэ.
	require.NoError(t, r.Up(ctx))
}

func TestRunner_PartialFailureRollsBack(t *testing.T) {
	r, dir := newRunner(t)
	ctx := context.Background()

	// DDL амжилттай болно; хоёр дахь statement нь файлын дунд алдаа
	// гаргуулахын тулд зориуд буруу SQL юм. Нэг файлд нэг транзакцтай
	// үед хүснэгт үүсгэх БОЛОН schema_migrations-ийн бүртгэл хоёулаа
	// буцах ёстой — өгөгдлийн санг яг хэвээр нь үлдээнэ.
	writeMigration(t, dir, "1",
		`CREATE TABLE half_done (id SERIAL PRIMARY KEY); SELECT this_function_does_not_exist();`,
		`DROP TABLE half_done;`,
	)

	err := r.Up(ctx)
	require.Error(t, err, "Up must surface the SQL error")

	// Амжилтгүй болсон migration-ийн нэр schema_migrations-д ГАРАХ
	// ёсгүй.
	pool := r.Pool()
	var count int
	require.NoError(t, pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM schema_migrations WHERE name = $1`, "1_test.up.sql").Scan(&count))
	assert.Equal(t, 0, count, "schema_migrations must not record a failed migration")

	// Мөн хүснэгт өөрөө байх ёсгүй (rollback нь DDL-г барьсан).
	var exists bool
	require.NoError(t, pool.QueryRow(ctx,
		`SELECT EXISTS (SELECT 1 FROM information_schema.tables WHERE table_name = 'half_done')`).Scan(&exists))
	assert.False(t, exists, "half-applied DDL must roll back with the tx")
}

func TestRunner_AppliesMultipleFilesInOrder(t *testing.T) {
	r, dir := newRunner(t)
	ctx := context.Background()

	// Хамааралтай хоёр migration: #2 нь #1-ээр үүсгэгдсэн хүснэгтийг
	// лавладаг. Хэрэв файлууд буруу дарааллаар хэрэгжвэл #2 амжилтгүй
	// болно.
	writeMigration(t, dir, "1",
		`CREATE TABLE a (id SERIAL PRIMARY KEY);`,
		`DROP TABLE a;`,
	)
	writeMigration(t, dir, "2",
		`CREATE TABLE b (id SERIAL PRIMARY KEY, a_id INTEGER REFERENCES a(id));`,
		`DROP TABLE b;`,
	)

	require.NoError(t, r.Up(ctx))

	// schema_migrations-ийн хоёр мөр хоёулаа байх ёстой.
	pool := r.Pool()
	var count int
	require.NoError(t, pool.QueryRow(ctx, `SELECT COUNT(*) FROM schema_migrations`).Scan(&count))
	assert.Equal(t, 2, count)
}
