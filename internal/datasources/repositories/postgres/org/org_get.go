// Government Template Platform V3.0
// Gerege Systems Development Team болон Claude AI хамтран бүтээв, 2026.

package org

import (
	"context"
	"errors"

	"template/internal/apperror"
	"template/internal/business/domain"
	"template/internal/datasources/records"
	"template/pkg/logger"

	"github.com/jackc/pgx/v5"
)

// GetOrgByID нь primary key-ээр байгууллагыг хайна (soft-delete хийгдсэн
// мөрүүдийг ИЛ-ээр хасна). Дуудагчийн identity дор (RLS) ажиллана — энгийн
// хэрэглэгч зөвхөн гишүүн болсон org-оо хардаг тул гишүүн биш org дээр
// NotFound буцна.
func (r *orgRepository) GetOrgByID(ctx context.Context, id string) (domain.Organization, error) {
	const fileName = "org_get.go"
	var stored records.Organizations
	err := r.withRLS(ctx, func(tx pgx.Tx) error {
		rows, qErr := tx.Query(ctx,
			`SELECT `+records.OrgColumns+` FROM organizations WHERE id = $1 AND deleted_at IS NULL`, id)
		if qErr != nil {
			return qErr
		}
		var scanErr error
		stored, scanErr = pgx.CollectExactlyOneRow(rows, pgx.RowToStructByName[records.Organizations])
		return scanErr
	})
	if err == nil {
		return stored.ToV1Domain(), nil
	}
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.Organization{}, apperror.NotFound("organization not found")
	}
	logger.ErrorWithContext(ctx, "Failed to query organization by id", logger.Fields{
		"repository": "organizations", "method": "GetOrgByID", "file": fileName,
		"error": err.Error(), "table": "organizations", "org_id": id,
	})
	return domain.Organization{}, err
}

// GetOrgByRegNo нь reg_no-оор (case-insensitive) байгууллагыг хайна. Энэ нь
// "энэ дугаартай байгууллага бүртгэлтэй юу" лавлахад ашиглагдах тул RLS-ийн
// member харагдах байдалд захирагдана (гишүүн биш бол NotFound).
func (r *orgRepository) GetOrgByRegNo(ctx context.Context, regNo string) (domain.Organization, error) {
	const fileName = "org_get.go"
	var stored records.Organizations
	err := r.withRLS(ctx, func(tx pgx.Tx) error {
		rows, qErr := tx.Query(ctx,
			`SELECT `+records.OrgColumns+` FROM organizations WHERE lower(reg_no) = lower($1) AND deleted_at IS NULL`, regNo)
		if qErr != nil {
			return qErr
		}
		var scanErr error
		stored, scanErr = pgx.CollectExactlyOneRow(rows, pgx.RowToStructByName[records.Organizations])
		return scanErr
	})
	if err == nil {
		return stored.ToV1Domain(), nil
	}
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.Organization{}, apperror.NotFound("organization not found")
	}
	logger.ErrorWithContext(ctx, "Failed to query organization by reg_no", logger.Fields{
		"repository": "organizations", "method": "GetOrgByRegNo", "file": fileName,
		"error": err.Error(), "table": "organizations",
	})
	return domain.Organization{}, err
}

// ListOrgsForUser нь тухайн хэрэглэгч гишүүн болсон бүх (идэвхтэй) байгууллагыг
// буцаана. "service" GUC дор ажиллана — userID нь дуудагчтай заавал таарахгүй
// (admin өөр хэрэглэгчийн жагсаалт авч болзошгүй) тул membership-ийг JOIN-оор
// шууд шүүнэ.
func (r *orgRepository) ListOrgsForUser(ctx context.Context, userID string) ([]domain.Organization, error) {
	const fileName = "org_get.go"
	var stored []records.Organizations
	err := r.withService(ctx, func(tx pgx.Tx) error {
		rows, qErr := tx.Query(ctx, `
			SELECT `+orgColumnsPrefixed+`
			FROM organizations o
			JOIN organization_memberships m ON m.org_id = o.id
			WHERE m.user_id = $1 AND o.deleted_at IS NULL
			ORDER BY o.created_at DESC`, userID)
		if qErr != nil {
			return qErr
		}
		var scanErr error
		stored, scanErr = pgx.CollectRows(rows, pgx.RowToStructByName[records.Organizations])
		return scanErr
	})
	if err != nil {
		logger.ErrorWithContext(ctx, "Failed to list organizations for user", logger.Fields{
			"repository": "organizations", "method": "ListOrgsForUser", "file": fileName,
			"error": err.Error(), "table": "organizations", "user_id": userID,
		})
		return nil, err
	}
	return records.ToArrayOfOrganizationsV1Domain(&stored), nil
}

// orgColumnsPrefixed нь JOIN query-д баганануудыг "o." угтвартай нэрлэнэ —
// RowToStructByName нь угтваргүй нэрээр буудаг тул alias-аар буцаана.
const orgColumnsPrefixed = "o.id, o.reg_no, o.name, o.name_latin, o.created_by, o.created_at, o.updated_at, o.deleted_at"
