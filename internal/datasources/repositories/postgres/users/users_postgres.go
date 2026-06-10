// Gerege Template Version 27.0
// Gerege Systems Development Team болон Claude AI хамтран бүтээв, 2026.

package postgres

import (
	repointerface "template/internal/datasources/repositories/interface"

	"github.com/jackc/pgx/v5/pgxpool"
)

// pgUniqueViolation нь Postgres-ийн unique_violation-ийн SQLSTATE код юм.
const pgUniqueViolation = "23505"

// postgreUserRepository нь pgx connection pool-г агуулна. Interface-ийн
// method бүр өөрийн файлд (users.store.go, users.get_by_email.go, ...)
// байрладаг тул нэг query-д хүрэх PR diff-үүд нарийн тодорхой хэвээр
// үлддэг.
//
// GORM-ийн автомат soft-delete байхгүй болсон тул query бүр уншихаасаа
// эсвэл бичихээсээ өмнө `deleted_at IS NULL`-г ИЛ-ээр нэмдэг.
type postgreUserRepository struct {
	pool *pgxpool.Pool
}

func NewUserRepository(pool *pgxpool.Pool) repointerface.UserRepository {
	return &postgreUserRepository{pool: pool}
}
