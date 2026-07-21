//go:build integration

// Government Template Platform V3.0
// Gerege Systems Development Team болон Claude AI хамтран бүтээв, 2026.

// Security event repository-ийн integration тест (жинхэнэ Postgres + RLS):
// хэрэглэгч зөвхөн ӨӨРИЙНХӨӨ тухай event INSERT хийж чадна (RLS policy
// user_id = app.user_id), admin бүгдийг received_at буурахаар (DESC)
// хуудаслан уншина.
package security_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	repointerface "template/internal/datasources/repositories/interface"
	securitypg "template/internal/datasources/repositories/postgres/security"
	"template/internal/datasources/rls"
	"template/internal/test/testenv"
)

const testUserID = "11111111-1111-4111-8111-111111111111"

func TestSecurityRepository(t *testing.T) {
	admin := testenv.StartPostgres(t)    // superuser — migration + RLS суулгана
	app := testenv.AppUserPool(t, admin) // non-superuser — RLS хүчинтэй (production шиг)
	// AppUserPool нь зөвхөн users-т эрх олгодог; production initdb script шиг
	// security_events-т эрх нэмнэ (RLS policy эрхийн ДЭЭР ажилладаг).
	for _, g := range []string{
		`GRANT SELECT, INSERT, UPDATE, DELETE ON security_events TO app_user`,
		`GRANT USAGE, SELECT ON SEQUENCE security_events_id_seq TO app_user`, // SERIAL id
	} {
		_, err := admin.Exec(context.Background(), g)
		require.NoError(t, err)
	}
	repo := securitypg.NewSecurityEventRepository(app)

	userCtx := rls.WithUser(context.Background(), testUserID)

	t.Run("user ingests own events under RLS", func(t *testing.T) {
		for _, kind := range []string{"jailbreak_detected", "integrity_fail", "anomaly"} {
			err := repo.Ingest(userCtx, repointerface.SecurityEventRecord{
				UserID:   testUserID,
				Kind:     kind,
				Severity: "high",
				Source:   "web",
				IP:       "203.0.113.9",
				Detail:   map[string]any{"note": kind},
			})
			require.NoError(t, err, "kind=%s", kind)
		}
	})

	t.Run("admin lists newest-first with pagination", func(t *testing.T) {
		rows, err := repo.List(context.Background(), 50, 0)
		require.NoError(t, err)
		require.GreaterOrEqual(t, len(rows), 3)
		// received_at DESC (id DESC) — хамгийн сүүлд ингест хийсэн нь эхэнд.
		assert.Equal(t, "anomaly", rows[0].Kind)

		// Хуудаслалт: limit=1 нь нэг мөр.
		page, err := repo.List(context.Background(), 1, 0)
		require.NoError(t, err)
		assert.Len(t, page, 1)

		// offset=1 нь дараагийн мөр рүү шилжинэ.
		page2, err := repo.List(context.Background(), 1, 1)
		require.NoError(t, err)
		require.Len(t, page2, 1)
		assert.NotEqual(t, page[0].ID, page2[0].ID)
	})

	t.Run("RLS blocks ingesting an event about another user", func(t *testing.T) {
		// app.user_id (testUserID) ба record.UserID зөрвөл user_insert policy
		// WITH CHECK-т унана.
		err := repo.Ingest(userCtx, repointerface.SecurityEventRecord{
			UserID: "22222222-2222-4222-8222-222222222222",
			Kind:   "spoofed",
		})
		require.Error(t, err, "өөр хэрэглэгчийн нэрээр ингест хийхийг RLS хориглох ёстой")
	})
}
