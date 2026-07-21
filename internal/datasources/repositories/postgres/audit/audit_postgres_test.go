//go:build integration

// Government Template Platform V3.0
// Gerege Systems Development Team болон Claude AI хамтран бүтээв, 2026.

// Audit repository-ийн integration тест (жинхэнэ Postgres): hash-chained
// append-only лог. Append нь prev_hash-аар гинжилдэг; VerifyChain нь бүрэн
// бүтэн гинжийг ok=true гэж баталж, мөр гараар засварлавал (tamper) ok=false +
// эвдэрсэн эхний мөрийн id-г буцаана. Энэ бол audit-ийн аюулгүй байдлын гол
// баталгаа.
package audit_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	repointerface "template/internal/datasources/repositories/interface"
	auditpg "template/internal/datasources/repositories/postgres/audit"
	"template/internal/test/testenv"
	pkgaudit "template/pkg/audit"
)

func TestAuditChain(t *testing.T) {
	pool := testenv.StartPostgres(t)
	repo := auditpg.NewAuditRepository(pool)
	ctx := context.Background()

	now := time.Now().UTC()
	entries := []pkgaudit.ChainEntry{
		{OccurredAt: now, Action: "auth.login", Category: "auth", Target: "u1"},
		{OccurredAt: now.Add(time.Second), Action: "role.update", Category: "rbac", Target: "role:3", Metadata: map[string]any{"perm": "users.manage"}},
		{OccurredAt: now.Add(2 * time.Second), Action: "user.delete", Category: "admin", Target: "u2"},
	}
	for _, e := range entries {
		_, err := repo.Append(ctx, e)
		require.NoError(t, err)
	}

	t.Run("intact chain verifies", func(t *testing.T) {
		ok, broken, err := repo.VerifyChain(ctx)
		require.NoError(t, err)
		assert.True(t, ok, "гэмтээгүй гинж ok=true байх ёстой (broken=%d)", broken)
	})

	t.Run("list returns entries", func(t *testing.T) {
		rows, err := repo.List(ctx, repointerface.AuditListFilter{}, 50, 0)
		require.NoError(t, err)
		assert.GreaterOrEqual(t, len(rows), 3)
	})

	t.Run("tampering breaks verification", func(t *testing.T) {
		// Хоёр дахь мөрийн action-ийг гараар өөрчилнө (superuser тул RLS bypass).
		var id int64
		require.NoError(t, pool.QueryRow(ctx,
			`SELECT id FROM audit_log ORDER BY id ASC OFFSET 1 LIMIT 1`).Scan(&id))
		_, err := pool.Exec(ctx, `UPDATE audit_log SET action = 'TAMPERED' WHERE id = $1`, id)
		require.NoError(t, err)

		ok, broken, err := repo.VerifyChain(ctx)
		require.NoError(t, err)
		assert.False(t, ok, "засварласан гинж ok=false байх ёстой")
		assert.Equal(t, id, broken, "эвдэрсэн эхний мөрийн id буцаах ёстой")
	})
}
