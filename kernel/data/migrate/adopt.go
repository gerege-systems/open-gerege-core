// Gerege Systems Development Team болон Claude AI хамтран бүтээв, 2026.

package migrate

import (
	"context"
	"fmt"
	"io/fs"

	"github.com/jackc/pgx/v5/pgxpool"
)

// LegacyTable нь суурийн глобал migration-уудын төлвийн хүснэгт.
const LegacyTable = "schema_migrations"

// SentinelTable нь "энэ DB-д схем аль хэдийн босчихсон" гэдгийн заагч.
// `users` нь 1-р migration-аар үүсдэг платформын хамгийн эртний хүснэгт
// тул бодит суулгац бүрт байдаг; цоо шинэ DB-д байхгүй.
const SentinelTable = "users"

// ShouldAdopt нь ЭНЭ DB-д схем аль хэдийн босгогдсон эсэхийг тогтооно.
// Үнэн бол модулийн baseline-уудыг АЖИЛЛУУЛАХГҮЙ, зөвхөн бүртгэнэ.
//
// Хоёр дохиог ЭСВЭЛ-ээр шалгана:
//
//  1. глобал `schema_migrations` хүснэгт байгаа БА мөртэй, эсвэл
//  2. sentinel хүснэгт (`users`) байгаа.
//
// ЯАГААД ЗӨВХӨН (1) ХАНГАЛТГҮЙ ВЭ: схем нь бүрэн боссон атлаа бүртгэл нь
// хоосон DB бодитоор оршдог — pg_dump/restore нь өгөгдлийг сэргээхдээ
// бүртгэлийн мөрийг авчрахгүй байж болно, мөн integration harness нь
// .up.sql файлуудыг runner-ээр биш шууд ажиллуулдаг. Зөвхөн бүртгэл дээр
// шийдвэл эдгээр DB "шинэ" гэж андуурагдаж, runner нь АМЬД хүснэгтүүд
// дээр CREATE TABLE ажиллуулах оролдлого хийнэ. Sentinel нь схемийн
// БОДИТ байдлыг хардаг тул тэр алдааг хаана.
//
// Шийдвэр нь модуль бүрт биш, DB-д НЭГ УДАА тавигдана — хагас шилжсэн
// (зарим модуль adopt, зарим нь run) төлөв үүсгэхгүй.
func ShouldAdopt(ctx context.Context, pool *pgxpool.Pool) (bool, error) {
	var sentinel bool
	if err := pool.QueryRow(ctx, `SELECT to_regclass($1) IS NOT NULL`, SentinelTable).Scan(&sentinel); err != nil {
		return false, fmt.Errorf("migrate: sentinel хүснэгт шалгах: %w", err)
	}
	return sentinel, nil
}

// Source нь өөрийн migration-тай модулийг илэрхийлнэ. Модуль энэ
// interface-ийг хэрэгжүүлбэл kernel нь boot дээр түүний migration-уудыг
// (эсвэл adopt-ыг) гүйцэтгэнэ. Хэрэгжүүлээгүй модуль нь суурийн глобал
// migration дээрээ хэвээр — нүүлгэлт модуль тус бүрээр явна.
type Source interface {
	// ID нь манифестийн ID-тай тохирно.
	ID() string
	// Migrations нь модулийн *.up.sql / *.down.sql файлуудыг агуулна
	// (ихэвчлэн //go:embed migrations/*.sql).
	Migrations() MigrationFS
}

// MigrationFS нь fs.FS-ийн нэр өгсөн хувилбар (модулийн гэрээг уншихад
// ойлгомжтой байлгах үүднээс).
type MigrationFS = fs.FS

// RunAll нь өгөгдсөн эх сурвалжуудыг дарааллаар нь гүйцэтгэнэ. adopt үнэн
// бол SQL ажиллуулахгүй, зөвхөн бүртгэнэ.
//
// Дараалал нь дуудагчийнх (kernel нь модулийн бүртгэлийн дарааллыг
// өгнө) — модуль хоорондын гадаад түлхүүр тэр дарааллыг шаарддаг.
func RunAll(ctx context.Context, pool *pgxpool.Pool, srcs []Source, adopt bool) error {
	for _, s := range srcs {
		r, err := New(pool, s.ID(), s.Migrations())
		if err != nil {
			return err
		}
		if adopt {
			if err := r.AdoptAll(ctx); err != nil {
				return err
			}
			continue
		}
		if err := r.Up(ctx); err != nil {
			return err
		}
	}
	return nil
}
