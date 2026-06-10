// Gerege Template Version 27.0
// Gerege Systems Development Team болон Claude AI хамтран бүтээв, 2026.

package seeders

import (
	"context"
	"errors"
	"time"

	"template/internal/constants"
	"template/internal/datasources/records"
	"template/pkg/logger"

	"github.com/jackc/pgx/v5/pgxpool"
)

type Seeder interface {
	UserSeeder(userData []records.Users) (err error)
}

type seeder struct {
	pool *pgxpool.Pool
}

func NewSeeder(pool *pgxpool.Pool) Seeder {
	return &seeder{pool: pool}
}

func (s *seeder) UserSeeder(userData []records.Users) (err error) {
	if len(userData) == 0 {
		return errors.New("users data is empty")
	}

	logger.Info("inserting users data...", logger.Fields{constants.LoggerCategory: constants.LoggerCategorySeeder})

	ctx := context.Background()
	// Бүхэл багцад нэг транзакц — хагас дутуу seed нь огт байхгүйгээс дор.
	// Id-г INSERT-ээс хассан тул Postgres түүнийг uuid_generate_v4() баганы
	// анхдагч утгаар (migration-ууд бэлдсэн) дүүргэнэ.
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	for i := range userData {
		if _, createErr := tx.Exec(ctx, `
			INSERT INTO users(id, username, email, password, active, role_id, created_at)
			VALUES (uuid_generate_v4(), $1, $2, $3, $4, $5, $6)
		`, userData[i].Username, userData[i].Email, userData[i].Password,
			userData[i].Active, userData[i].RoleId, time.Now().UTC()); createErr != nil {
			return createErr
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return err
	}

	logger.Info("users data inserted successfully", logger.Fields{constants.LoggerCategory: constants.LoggerCategorySeeder})
	return nil
}
