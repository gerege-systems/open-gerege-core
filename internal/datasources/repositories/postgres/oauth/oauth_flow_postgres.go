// Government Template Platform V3.0
// Gerege Systems Development Team болон Claude AI хамтран бүтээв, 2026.

package oauth

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"template/internal/apperror"
	"template/internal/business/domain"
	"template/internal/datasources/rls"
)

// flowRepository нь authorize урсгалын түр төлөвийг (challenge, санагдсан
// consent, authorization code) хадгална.
//
// Эдгээр хүснэгтүүд RLS-тэй бөгөөд протоколын endpoint-ууд нэвтрэхээс ӨМНӨ
// ажилладаг тул query бүр withRLS транзакцаар (ихэвчлэн RoleService) явна.
type flowRepository struct {
	pool *pgxpool.Pool
}

func NewFlowRepository(pool *pgxpool.Pool) *flowRepository {
	return &flowRepository{pool: pool}
}

// withRLS нь context дахь Identity-г SET LOCAL болгож тавина (users repository-
// тэй ижил загвар). Identity байхгүй бол GUC хоосон → бодлого бүх мөрийг хаана.
func (r *flowRepository) withRLS(ctx context.Context, fn func(tx pgx.Tx) error) error {
	id, _ := rls.FromContext(ctx)
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx) //nolint:errcheck // commit хийсний дараах rollback нь ErrTxClosed — хүлээгдсэн
	if _, err := tx.Exec(ctx,
		`SELECT set_config('app.user_id',$1,true), set_config('app.user_role',$2,true)`,
		id.UserID, string(id.Role),
	); err != nil {
		return fmt.Errorf("set rls session context: %w", err)
	}
	if err := fn(tx); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

// ── Challenge ────────────────────────────────────────────────────────────────

const challengeColumns = ` challenge, kind, client_id, subject, requested_scopes, granted_scopes,
	redirect_uri, state, nonce, response_type, code_challenge, code_challenge_method,
	prompt, post_logout_redirect_uri, skip, decided_at, expires_at, created_at`

// scanChallenge нь challenge мөрийг уншина. client_id болон subject нь NULL
// байж БОЛНО (login challenge-д subject хараахан мэдэгдээгүй; logout challenge-д
// client_id байхгүй байж болно) тул тэдгээрийг заагчаар уншаад хоосон мөр болгоно.
func scanChallenge(row pgx.Row) (domain.OAuthChallenge, error) {
	var c domain.OAuthChallenge
	var clientID, subject *string
	err := row.Scan(&c.Challenge, &c.Kind, &clientID, &subject, &c.RequestedScopes, &c.GrantedScopes,
		&c.RedirectURI, &c.State, &c.Nonce, &c.ResponseType, &c.CodeChallenge, &c.CodeChallengeMethod,
		&c.Prompt, &c.PostLogoutRedirectURI, &c.Skip, &c.DecidedAt, &c.ExpiresAt, &c.CreatedAt)
	if clientID != nil {
		c.ClientID = *clientID
	}
	if subject != nil {
		c.Subject = *subject
	}
	return c, err
}

// CreateChallenge нь шинэ login/consent/logout challenge бичнэ.
func (r *flowRepository) CreateChallenge(ctx context.Context, c domain.OAuthChallenge) error {
	return r.withRLS(ctx, func(tx pgx.Tx) error {
		_, err := tx.Exec(ctx, `
			INSERT INTO oauth_challenges (
				challenge, kind, client_id, subject, requested_scopes, granted_scopes,
				redirect_uri, state, nonce, response_type, code_challenge, code_challenge_method,
				prompt, post_logout_redirect_uri, skip, expires_at)
			VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16)`,
			c.Challenge, c.Kind, nullableText(c.ClientID), nullableUUID(c.Subject),
			strList(c.RequestedScopes), strList(c.GrantedScopes),
			c.RedirectURI, c.State, c.Nonce, c.ResponseType, c.CodeChallenge, c.CodeChallengeMethod,
			c.Prompt, c.PostLogoutRedirectURI, c.Skip, c.ExpiresAt)
		if err != nil {
			return fmt.Errorf("insert oauth challenge: %w", err)
		}
		return nil
	})
}

// Challenge нь ХҮЧИНТЭЙ (хугацаа дуусаагүй, хараахан шийдэгдээгүй) challenge-ыг
// буцаана. Дуусал/шийдэгдсэнийг NotFound-той ижилхэн үзнэ — дахин ашиглахаас
// сэргийлнэ.
func (r *flowRepository) Challenge(ctx context.Context, kind, challenge string) (domain.OAuthChallenge, error) {
	var out domain.OAuthChallenge
	err := r.withRLS(ctx, func(tx pgx.Tx) error {
		c, scanErr := scanChallenge(tx.QueryRow(ctx,
			`SELECT`+challengeColumns+` FROM oauth_challenges
			 WHERE challenge = $1 AND kind = $2 AND decided_at IS NULL AND expires_at > now()`,
			challenge, kind))
		if errors.Is(scanErr, pgx.ErrNoRows) {
			return apperror.NotFound("challenge not found or already used")
		}
		if scanErr != nil {
			return fmt.Errorf("get oauth challenge: %w", scanErr)
		}
		out = c
		return nil
	})
	return out, err
}

// DecideChallenge нь challenge-ыг шийдэгдсэн гэж тэмдэглэнэ (нэг удаагийн).
// Хэрэв өөр хүсэлт аль хэдийн шийдсэн бол NotFound — давхар зарцуулалт болохгүй.
func (r *flowRepository) DecideChallenge(ctx context.Context, challenge, subject string, granted []string) error {
	return r.withRLS(ctx, func(tx pgx.Tx) error {
		tag, err := tx.Exec(ctx, `
			UPDATE oauth_challenges
			   SET decided_at = now(), subject = COALESCE($2, subject), granted_scopes = $3
			 WHERE challenge = $1 AND decided_at IS NULL AND expires_at > now()`,
			challenge, nullableUUID(subject), strList(granted))
		if err != nil {
			return fmt.Errorf("decide oauth challenge: %w", err)
		}
		if tag.RowsAffected() == 0 {
			return apperror.NotFound("challenge not found or already used")
		}
		return nil
	})
}

// ── Санагдсан consent ────────────────────────────────────────────────────────

// Consent нь тухайн иргэн тухайн апп-д өмнө нь олгосон, хүчинтэй scope-уудыг
// буцаана. Байхгүй бол хоосон (алдаа биш) — consent UI харуулна.
func (r *flowRepository) Consent(ctx context.Context, subject, clientID string) ([]string, error) {
	var scopes []string
	err := r.withRLS(ctx, func(tx pgx.Tx) error {
		row := tx.QueryRow(ctx,
			`SELECT scopes FROM oauth_consents
			  WHERE subject = $1 AND client_id = $2 AND expires_at > now()`,
			subject, clientID)
		if scanErr := row.Scan(&scopes); scanErr != nil {
			if errors.Is(scanErr, pgx.ErrNoRows) {
				scopes = nil
				return nil
			}
			return fmt.Errorf("get oauth consent: %w", scanErr)
		}
		return nil
	})
	return scopes, err
}

// SaveConsent нь олгосон scope-уудыг санана (дараагийн удаа UI алгасна).
func (r *flowRepository) SaveConsent(ctx context.Context, subject, clientID string, scopes []string, ttl time.Duration) error {
	return r.withRLS(ctx, func(tx pgx.Tx) error {
		_, err := tx.Exec(ctx, `
			INSERT INTO oauth_consents (subject, client_id, scopes, expires_at)
			VALUES ($1, $2, $3, now() + $4::interval)
			ON CONFLICT (subject, client_id) DO UPDATE
			   SET scopes = EXCLUDED.scopes, expires_at = EXCLUDED.expires_at, updated_at = now()`,
			subject, clientID, strList(scopes), ttl.String())
		if err != nil {
			return fmt.Errorf("save oauth consent: %w", err)
		}
		return nil
	})
}

// RevokeConsent нь тухайн апп-д олгосон зөвшөөрлийг устгана.
func (r *flowRepository) RevokeConsent(ctx context.Context, subject, clientID string) error {
	return r.withRLS(ctx, func(tx pgx.Tx) error {
		_, err := tx.Exec(ctx,
			`DELETE FROM oauth_consents WHERE subject = $1 AND client_id = $2`, subject, clientID)
		if err != nil {
			return fmt.Errorf("revoke oauth consent: %w", err)
		}
		return nil
	})
}

// ── Authorization code ───────────────────────────────────────────────────────

// CreateCode нь authorization code-ыг (hash хэлбэрээр) бичнэ.
func (r *flowRepository) CreateCode(ctx context.Context, c domain.OAuthAuthCode) error {
	return r.withRLS(ctx, func(tx pgx.Tx) error {
		_, err := tx.Exec(ctx, `
			INSERT INTO oauth_auth_codes (
				code_hash, client_id, subject, scopes, redirect_uri, nonce,
				code_challenge, code_challenge_method, auth_time, expires_at)
			VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10)`,
			c.CodeHash, c.ClientID, c.Subject, strList(c.Scopes), c.RedirectURI, c.Nonce,
			c.CodeChallenge, c.CodeChallengeMethod, c.AuthTime, c.ExpiresAt)
		if err != nil {
			return fmt.Errorf("insert authorization code: %w", err)
		}
		return nil
	})
}

// ConsumeCode нь code-ыг АТОМААР нэг удаа зарцуулна: хугацаа дуусаагүй бөгөөд
// хараахан ашиглагдаагүй бол used_at тавьж мөрийг буцаана.
//
// Хоёр дахь удаа ирвэл (`used` = true) дуудагч энэ нь дахин ашиглалт гэдгийг
// мэдэж, тухайн session-ий бүх token-ыг цуцлах ёстой (RFC 6749 §4.1.2).
func (r *flowRepository) ConsumeCode(ctx context.Context, codeHash []byte) (code domain.OAuthAuthCode, alreadyUsed bool, err error) {
	err = r.withRLS(ctx, func(tx pgx.Tx) error {
		// Эхлээд мөрийг түгжиж уншина — өрсөлдөөнт солилцоо давхар амжилтгүй болно.
		var c domain.OAuthAuthCode
		var usedAt *time.Time
		scanErr := tx.QueryRow(ctx, `
			SELECT code_hash, client_id, subject, scopes, redirect_uri, nonce,
			       code_challenge, code_challenge_method, auth_time, expires_at, used_at
			  FROM oauth_auth_codes
			 WHERE code_hash = $1
			 FOR UPDATE`, codeHash).Scan(
			&c.CodeHash, &c.ClientID, &c.Subject, &c.Scopes, &c.RedirectURI, &c.Nonce,
			&c.CodeChallenge, &c.CodeChallengeMethod, &c.AuthTime, &c.ExpiresAt, &usedAt)
		if errors.Is(scanErr, pgx.ErrNoRows) {
			return apperror.NotFound("authorization code not found")
		}
		if scanErr != nil {
			return fmt.Errorf("get authorization code: %w", scanErr)
		}

		code = c
		if usedAt != nil {
			alreadyUsed = true
			return nil // дуудагч бүлгийг цуцална
		}
		if time.Now().After(c.ExpiresAt) {
			return apperror.BadRequest("authorization code expired")
		}

		if _, execErr := tx.Exec(ctx,
			`UPDATE oauth_auth_codes SET used_at = now() WHERE code_hash = $1`, codeHash); execErr != nil {
			return fmt.Errorf("consume authorization code: %w", execErr)
		}
		return nil
	})
	return code, alreadyUsed, err
}

// DeleteExpired нь хугацаа дууссан түр мөрүүдийг цэвэрлэнэ (тогтмол ажил).
// Ашиглагдсан code-ыг ХЭСЭГ ХУГАЦААНД үлдээнэ — дахин ашиглалтыг илрүүлэхэд
// хэрэгтэй; зөвхөн эрс хуучирсныг нь хаяна.
func (r *flowRepository) DeleteExpired(ctx context.Context) error {
	return r.withRLS(ctx, func(tx pgx.Tx) error {
		for _, q := range []string{
			`DELETE FROM oauth_challenges WHERE expires_at < now() - interval '1 day'`,
			`DELETE FROM oauth_auth_codes WHERE expires_at < now() - interval '1 day'`,
			`DELETE FROM oauth_access_tokens WHERE expires_at < now() - interval '7 days'`,
			`DELETE FROM oauth_refresh_tokens WHERE expires_at < now() - interval '7 days'`,
			`DELETE FROM oauth_consents WHERE expires_at < now()`,
		} {
			if _, err := tx.Exec(ctx, q); err != nil {
				return fmt.Errorf("cleanup oauth state: %w", err)
			}
		}
		return nil
	})
}

func nullableText(s string) any {
	if s == "" {
		return nil
	}
	return s
}

func nullableUUID(s string) any {
	if s == "" {
		return nil
	}
	return s
}

// ── Access / refresh token ───────────────────────────────────────────────────

// StoreTokens нь нэг гүйлгээнд access + (сонголтоор) refresh token-ыг бичнэ —
// хагас гаргасан хос үлдэхээс сэргийлнэ.
func (r *flowRepository) StoreTokens(ctx context.Context, at domain.OAuthAccessToken, rt *domain.OAuthRefreshToken) error {
	return r.withRLS(ctx, func(tx pgx.Tx) error {
		if _, err := tx.Exec(ctx, `
			INSERT INTO oauth_access_tokens (token_hash, client_id, subject, scopes, refresh_family, expires_at)
			VALUES ($1,$2,$3,$4,$5,$6)`,
			at.TokenHash, at.ClientID, nullableUUID(at.Subject), strList(at.Scopes),
			nullableUUID(at.RefreshFamily), at.ExpiresAt); err != nil {
			return fmt.Errorf("insert access token: %w", err)
		}
		if rt == nil {
			return nil
		}
		if _, err := tx.Exec(ctx, `
			INSERT INTO oauth_refresh_tokens (
				token_hash, family_id, rotated_from, client_id, subject, scopes, nonce, auth_time, expires_at)
			VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9)`,
			rt.TokenHash, rt.FamilyID, rt.RotatedFrom, rt.ClientID, rt.Subject,
			strList(rt.Scopes), rt.Nonce, rt.AuthTime, rt.ExpiresAt); err != nil {
			return fmt.Errorf("insert refresh token: %w", err)
		}
		return nil
	})
}

// AccessToken нь ХҮЧИНТЭЙ access token-ыг буцаана (хугацаа дуусаагүй, цуцлагдаагүй).
func (r *flowRepository) AccessToken(ctx context.Context, tokenHash []byte) (domain.OAuthAccessToken, error) {
	var out domain.OAuthAccessToken
	err := r.withRLS(ctx, func(tx pgx.Tx) error {
		var subject *string
		scanErr := tx.QueryRow(ctx, `
			SELECT token_hash, client_id, subject, scopes, expires_at
			  FROM oauth_access_tokens
			 WHERE token_hash = $1 AND revoked_at IS NULL AND expires_at > now()`, tokenHash).
			Scan(&out.TokenHash, &out.ClientID, &subject, &out.Scopes, &out.ExpiresAt)
		if errors.Is(scanErr, pgx.ErrNoRows) {
			return apperror.NotFound("token not found")
		}
		if scanErr != nil {
			return fmt.Errorf("get access token: %w", scanErr)
		}
		if subject != nil {
			out.Subject = *subject
		}
		return nil
	})
	return out, err
}

// ConsumeRefreshToken нь refresh token-ыг АТОМААР зарцуулна. Аль хэдийн
// хэрэглэгдсэн/цуцлагдсан бол reused=true — дуудагч БҮХ гэр бүлийг цуцлана.
func (r *flowRepository) ConsumeRefreshToken(ctx context.Context, tokenHash []byte) (rt domain.OAuthRefreshToken, reused bool, err error) {
	err = r.withRLS(ctx, func(tx pgx.Tx) error {
		var consumedAt, revokedAt *time.Time
		scanErr := tx.QueryRow(ctx, `
			SELECT token_hash, family_id, client_id, subject, scopes, nonce, auth_time, expires_at, consumed_at, revoked_at
			  FROM oauth_refresh_tokens
			 WHERE token_hash = $1
			 FOR UPDATE`, tokenHash).Scan(
			&rt.TokenHash, &rt.FamilyID, &rt.ClientID, &rt.Subject, &rt.Scopes,
			&rt.Nonce, &rt.AuthTime, &rt.ExpiresAt, &consumedAt, &revokedAt)
		if errors.Is(scanErr, pgx.ErrNoRows) {
			return apperror.NotFound("refresh token not found")
		}
		if scanErr != nil {
			return fmt.Errorf("get refresh token: %w", scanErr)
		}

		// Аль хэдийн хэрэглэгдсэн/цуцлагдсан token дахин ирэх нь хулгайн шинж.
		if consumedAt != nil || revokedAt != nil {
			reused = true
			return nil
		}
		if time.Now().After(rt.ExpiresAt) {
			return apperror.BadRequest("refresh token expired")
		}
		if _, execErr := tx.Exec(ctx,
			`UPDATE oauth_refresh_tokens SET consumed_at = now() WHERE token_hash = $1`, tokenHash); execErr != nil {
			return fmt.Errorf("consume refresh token: %w", execErr)
		}
		return nil
	})
	return rt, reused, err
}

// RevokeFamily нь нэг эргэлтийн гэр бүлийн БҮХ refresh болон access token-ыг
// цуцална — дахин ашиглалт илэрсэн үед хулгайлагдсан session-ыг бүхэлд нь хаана.
func (r *flowRepository) RevokeFamily(ctx context.Context, familyID string) error {
	return r.withRLS(ctx, func(tx pgx.Tx) error {
		if _, err := tx.Exec(ctx,
			`UPDATE oauth_refresh_tokens SET revoked_at = now()
			  WHERE family_id = $1 AND revoked_at IS NULL`, familyID); err != nil {
			return fmt.Errorf("revoke refresh family: %w", err)
		}
		if _, err := tx.Exec(ctx,
			`UPDATE oauth_access_tokens SET revoked_at = now()
			  WHERE refresh_family = $1 AND revoked_at IS NULL`, familyID); err != nil {
			return fmt.Errorf("revoke access tokens of family: %w", err)
		}
		return nil
	})
}

// RevokeAccessToken / RevokeRefreshToken нь /oauth2/revoke-д ашиглагдана.
func (r *flowRepository) RevokeAccessToken(ctx context.Context, tokenHash []byte, clientID string) (bool, error) {
	var found bool
	err := r.withRLS(ctx, func(tx pgx.Tx) error {
		tag, execErr := tx.Exec(ctx,
			`UPDATE oauth_access_tokens SET revoked_at = now()
			  WHERE token_hash = $1 AND client_id = $2 AND revoked_at IS NULL`, tokenHash, clientID)
		if execErr != nil {
			return fmt.Errorf("revoke access token: %w", execErr)
		}
		found = tag.RowsAffected() > 0
		return nil
	})
	return found, err
}

func (r *flowRepository) RevokeRefreshToken(ctx context.Context, tokenHash []byte, clientID string) (bool, error) {
	var found bool
	err := r.withRLS(ctx, func(tx pgx.Tx) error {
		var familyID string
		scanErr := tx.QueryRow(ctx,
			`SELECT family_id FROM oauth_refresh_tokens WHERE token_hash = $1 AND client_id = $2`,
			tokenHash, clientID).Scan(&familyID)
		if errors.Is(scanErr, pgx.ErrNoRows) {
			return nil
		}
		if scanErr != nil {
			return fmt.Errorf("find refresh token: %w", scanErr)
		}
		found = true
		// Нэг refresh token цуцлах нь тухайн session-ий БҮХ эргэлтийг цуцална —
		// RP "гарлаа" гэж хэлж байгаа тул хагас цуцлалт утгагүй.
		if _, execErr := tx.Exec(ctx,
			`UPDATE oauth_refresh_tokens SET revoked_at = now() WHERE family_id = $1 AND revoked_at IS NULL`,
			familyID); execErr != nil {
			return fmt.Errorf("revoke refresh family: %w", execErr)
		}
		if _, execErr := tx.Exec(ctx,
			`UPDATE oauth_access_tokens SET revoked_at = now() WHERE refresh_family = $1 AND revoked_at IS NULL`,
			familyID); execErr != nil {
			return fmt.Errorf("revoke access tokens of family: %w", execErr)
		}
		return nil
	})
	return found, err
}

// RevokeForSubjectClient нь тухайн иргэний тухайн апп дахь БҮХ идэвхтэй
// token-ыг цуцална.
//
// Authorization code дахин ашиглагдсан үед хэрэглэнэ: тухайн код ямар token
// гаргасныг холбосон бичлэг байдаггүй тул (эргэлтийн гэр бүл нь token гаргах
// мөчид үүсдэг) хамгийн ойрын аюулгүй хүрээ болох subject+client-ээр цуцална.
// Илүү өргөн боловч алдааны талд аюулгүй — RFC 6749 §4.1.2 нь тухайн кодоос
// гаргасан бүх token-ыг цуцлахыг шаарддаг.
func (r *flowRepository) RevokeForSubjectClient(ctx context.Context, subject, clientID string) error {
	return r.withRLS(ctx, func(tx pgx.Tx) error {
		if _, err := tx.Exec(ctx,
			`UPDATE oauth_refresh_tokens SET revoked_at = now()
			  WHERE subject = $1 AND client_id = $2 AND revoked_at IS NULL`, subject, clientID); err != nil {
			return fmt.Errorf("revoke refresh tokens for subject: %w", err)
		}
		if _, err := tx.Exec(ctx,
			`UPDATE oauth_access_tokens SET revoked_at = now()
			  WHERE subject = $1 AND client_id = $2 AND revoked_at IS NULL`, subject, clientID); err != nil {
			return fmt.Errorf("revoke access tokens for subject: %w", err)
		}
		return nil
	})
}
