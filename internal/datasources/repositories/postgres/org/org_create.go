// Gerege Template Version 27.0
// Gerege Systems Development Team болон Claude AI хамтран бүтээв, 2026.

package org

import (
	"context"
	"errors"
	"fmt"

	"template/internal/apperror"
	"template/internal/business/domain"
	"template/internal/datasources/records"
	"template/pkg/logger"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

// CreateOrg нь байгууллага оруулж, үүсгэгчийг owner гишүүн болгож, нэг
// транзакцид хадгална. "service" GUC дор ажиллана — үүсгэгч хараахан гишүүн
// болоогүй тул user policy түүнийг хардаггүй; бизнесийн эрхийг usecase шалгасан.
func (r *orgRepository) CreateOrg(ctx context.Context, in *domain.Organization) (domain.Organization, error) {
	const (
		repositoryName = "organizations"
		funcName       = "CreateOrg"
		fileName       = "org_create.go"
	)
	var stored records.Organizations
	err := r.withService(ctx, func(tx pgx.Tx) error {
		rows, qErr := tx.Query(ctx, `
			INSERT INTO organizations (reg_no, name, name_latin, created_by)
			VALUES ($1, $2, $3, $4)
			RETURNING `+records.OrgColumns,
			in.RegNo, in.Name, in.NameLatin, in.CreatedBy)
		if qErr != nil {
			return qErr
		}
		var scanErr error
		stored, scanErr = pgx.CollectExactlyOneRow(rows, pgx.RowToStructByName[records.Organizations])
		if scanErr != nil {
			return scanErr
		}
		// Үүсгэгч автоматаар owner гишүүн болно.
		if _, mErr := tx.Exec(ctx, `
			INSERT INTO organization_memberships (org_id, user_id, role)
			VALUES ($1, $2, $3)`,
			stored.Id, in.CreatedBy, domain.OrgRoleOwner); mErr != nil {
			return mErr
		}
		return nil
	})
	if err == nil {
		return stored.ToV1Domain(), nil
	}

	// 23505 unique_violation (reg_no давхцал)-г 409 Conflict болгоно.
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == pgUniqueViolation {
		logger.ErrorWithContext(ctx, "Failed to insert organization: unique violation", logger.Fields{
			"repository": repositoryName, "method": funcName, "file": fileName,
			"error": err.Error(), "table": "organizations", "reg_no": in.RegNo,
		})
		return domain.Organization{}, apperror.Conflict("organization with this registration number already exists")
	}
	logger.ErrorWithContext(ctx, "Failed to insert organization", logger.Fields{
		"repository": repositoryName, "method": funcName, "file": fileName,
		"error": err.Error(), "table": "organizations",
	})
	return domain.Organization{}, fmt.Errorf("create org: %w", err)
}
