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

// GetMembership нь (orgID, userID) хосын гишүүнчлэлийг буцаана. usecase нь
// дуудагчийн эрхийг (owner/admin эсэх) шалгахад үүнийг ашигладаг тул "service"
// GUC дор ажиллана (дуудагч өөрийн membership-ээ найдвартай уншихын тулд).
func (r *orgRepository) GetMembership(ctx context.Context, orgID, userID string) (domain.OrganizationMembership, error) {
	const fileName = "org_members.go"
	var stored records.OrganizationMemberships
	err := r.withService(ctx, func(tx pgx.Tx) error {
		rows, qErr := tx.Query(ctx,
			`SELECT `+records.OrgMembershipColumns+` FROM organization_memberships WHERE org_id = $1 AND user_id = $2`,
			orgID, userID)
		if qErr != nil {
			return qErr
		}
		var scanErr error
		stored, scanErr = pgx.CollectExactlyOneRow(rows, pgx.RowToStructByName[records.OrganizationMemberships])
		return scanErr
	})
	if err == nil {
		return stored.ToV1Domain(), nil
	}
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.OrganizationMembership{}, apperror.NotFound("membership not found")
	}
	logger.ErrorWithContext(ctx, "Failed to query membership", logger.Fields{
		"repository": "organization_memberships", "method": "GetMembership", "file": fileName,
		"error": err.Error(), "table": "organization_memberships", "org_id": orgID, "user_id": userID,
	})
	return domain.OrganizationMembership{}, err
}

// ListMembers нь тухайн байгууллагын бүх гишүүнийг буцаана. Дуудагчийн identity
// дор (RLS) ажиллана — зөвхөн дуудагч өөрөө гишүүн болсон org-ийн гишүүдийг
// харна.
func (r *orgRepository) ListMembers(ctx context.Context, orgID string) ([]domain.OrganizationMembership, error) {
	const fileName = "org_members.go"
	var stored []records.OrganizationMemberships
	err := r.withRLS(ctx, func(tx pgx.Tx) error {
		rows, qErr := tx.Query(ctx,
			`SELECT `+records.OrgMembershipColumns+` FROM organization_memberships WHERE org_id = $1 ORDER BY created_at`, orgID)
		if qErr != nil {
			return qErr
		}
		var scanErr error
		stored, scanErr = pgx.CollectRows(rows, pgx.RowToStructByName[records.OrganizationMemberships])
		return scanErr
	})
	if err != nil {
		logger.ErrorWithContext(ctx, "Failed to list members", logger.Fields{
			"repository": "organization_memberships", "method": "ListMembers", "file": fileName,
			"error": err.Error(), "table": "organization_memberships", "org_id": orgID,
		})
		return nil, err
	}
	return records.ToArrayOfOrgMembershipsV1Domain(&stored), nil
}

// AddMember нь гишүүн нэмнэ. "service" GUC дор ажиллана (usecase нь дуудагч
// owner/admin эсэхийг шалгасан). Аль хэдийн гишүүн бол apperror.Conflict.
func (r *orgRepository) AddMember(ctx context.Context, in *domain.OrganizationMembership) (domain.OrganizationMembership, error) {
	const fileName = "org_members.go"
	var stored records.OrganizationMemberships
	err := r.withService(ctx, func(tx pgx.Tx) error {
		rows, qErr := tx.Query(ctx, `
			INSERT INTO organization_memberships (org_id, user_id, role)
			VALUES ($1, $2, $3)
			RETURNING `+records.OrgMembershipColumns,
			in.OrgID, in.UserID, in.Role)
		if qErr != nil {
			return qErr
		}
		var scanErr error
		stored, scanErr = pgx.CollectExactlyOneRow(rows, pgx.RowToStructByName[records.OrganizationMemberships])
		return scanErr
	})
	if err == nil {
		return stored.ToV1Domain(), nil
	}
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == pgUniqueViolation {
		return domain.OrganizationMembership{}, apperror.Conflict("user is already a member of this organization")
	}
	logger.ErrorWithContext(ctx, "Failed to add member", logger.Fields{
		"repository": "organization_memberships", "method": "AddMember", "file": fileName,
		"error": err.Error(), "table": "organization_memberships", "org_id": in.OrgID, "user_id": in.UserID,
	})
	return domain.OrganizationMembership{}, fmt.Errorf("add member: %w", err)
}

// UpdateMemberRole нь гишүүний дүрийг солино. "service" GUC дор ажиллана.
// Гишүүн биш бол apperror.NotFound.
func (r *orgRepository) UpdateMemberRole(ctx context.Context, orgID, userID, role string) error {
	const fileName = "org_members.go"
	err := r.withService(ctx, func(tx pgx.Tx) error {
		tag, qErr := tx.Exec(ctx,
			`UPDATE organization_memberships SET role = $3 WHERE org_id = $1 AND user_id = $2`,
			orgID, userID, role)
		if qErr != nil {
			return qErr
		}
		if tag.RowsAffected() == 0 {
			return apperror.NotFound("membership not found")
		}
		return nil
	})
	if err == nil {
		return nil
	}
	if _, ok := err.(*apperror.DomainError); ok {
		return err
	}
	logger.ErrorWithContext(ctx, "Failed to update member role", logger.Fields{
		"repository": "organization_memberships", "method": "UpdateMemberRole", "file": fileName,
		"error": err.Error(), "table": "organization_memberships", "org_id": orgID, "user_id": userID,
	})
	return fmt.Errorf("update member role: %w", err)
}

// RemoveMember нь гишүүнийг хасна. "service" GUC дор ажиллана. Гишүүн биш бол
// apperror.NotFound.
func (r *orgRepository) RemoveMember(ctx context.Context, orgID, userID string) error {
	const fileName = "org_members.go"
	err := r.withService(ctx, func(tx pgx.Tx) error {
		tag, qErr := tx.Exec(ctx,
			`DELETE FROM organization_memberships WHERE org_id = $1 AND user_id = $2`,
			orgID, userID)
		if qErr != nil {
			return qErr
		}
		if tag.RowsAffected() == 0 {
			return apperror.NotFound("membership not found")
		}
		return nil
	})
	if err == nil {
		return nil
	}
	if _, ok := err.(*apperror.DomainError); ok {
		return err
	}
	logger.ErrorWithContext(ctx, "Failed to remove member", logger.Fields{
		"repository": "organization_memberships", "method": "RemoveMember", "file": fileName,
		"error": err.Error(), "table": "organization_memberships", "org_id": orgID, "user_id": userID,
	})
	return fmt.Errorf("remove member: %w", err)
}
