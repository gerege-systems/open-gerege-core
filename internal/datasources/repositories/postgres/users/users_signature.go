// Government Template Platform V3.0
// Gerege Systems Development Team болон Claude AI хамтран бүтээв, 2026.

package postgres

import (
	"context"
	"errors"
	"strings"

	"template/internal/apperror"

	"github.com/jackc/pgx/v5"
)

// GetSignature нь хэрэглэгчийн гарын үсгийн зургийг (data-URL) буцаана. Тавиагүй
// бол хоосон мөр. withRLS дор ажилладаг тул зөвхөн өөрийн мөрөнд хандана.
func (r *postgreUserRepository) GetSignature(ctx context.Context, userID string) (string, error) {
	var img string
	err := r.withRLS(ctx, func(tx pgx.Tx) error {
		return tx.QueryRow(ctx,
			`SELECT COALESCE(signature_image,'') FROM users WHERE id = $1 AND deleted_at IS NULL`, userID).Scan(&img)
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return "", apperror.NotFound("user not found")
	}
	if err != nil {
		return "", apperror.InternalCause(err)
	}
	return img, nil
}

// SetLatinName нь хэрэглэгчийн латин нэрийг (first_name_en/last_name_en) гараар
// засна. eID-ийн автомат галиглалт заримдаа буруу гардаг тул засварлах боломж;
// UpsertFromEID нь дараа нь дарж бичихгүй (COALESCE).
func (r *postgreUserRepository) SetLatinName(ctx context.Context, userID, firstEn, lastEn string) error {
	err := r.withRLS(ctx, func(tx pgx.Tx) error {
		ct, execErr := tx.Exec(ctx,
			`UPDATE users SET first_name_en = NULLIF($1,''), last_name_en = NULLIF($2,''), updated_at = now() WHERE id = $3 AND deleted_at IS NULL`,
			strings.TrimSpace(firstEn), strings.TrimSpace(lastEn), userID)
		if execErr != nil {
			return execErr
		}
		if ct.RowsAffected() == 0 {
			return apperror.NotFound("user not found")
		}
		return nil
	})
	if err != nil {
		var de *apperror.DomainError
		if errors.As(err, &de) {
			return err
		}
		return apperror.InternalCause(err)
	}
	return nil
}

// SetSignature нь хэрэглэгчийн гарын үсгийн зургийг тавина/шинэчилнэ. Хоосон img
// нь баганыг NULL болгож (устгаж) байна.
func (r *postgreUserRepository) SetSignature(ctx context.Context, userID, img string) error {
	err := r.withRLS(ctx, func(tx pgx.Tx) error {
		ct, execErr := tx.Exec(ctx,
			`UPDATE users SET signature_image = NULLIF($1,''), updated_at = now() WHERE id = $2 AND deleted_at IS NULL`,
			img, userID)
		if execErr != nil {
			return execErr
		}
		if ct.RowsAffected() == 0 {
			return apperror.NotFound("user not found")
		}
		return nil
	})
	if err != nil {
		var de *apperror.DomainError
		if errors.As(err, &de) {
			return err
		}
		return apperror.InternalCause(err)
	}
	return nil
}
