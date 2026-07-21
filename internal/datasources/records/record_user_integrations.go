// Government Template Platform V3.0
// Gerege Systems Development Team болон Claude AI хамтран бүтээв, 2026.

package records

import (
	"time"

	"template/internal/business/domain"
)

// UserIntegrations нь user_integrations хүснэгтийн pgx record. `db` tag-ууд нь
// snake_case баганатай тааруулагдсан бөгөөд nullable баганануудыг pointer-ээр
// илэрхийлсэн.
type UserIntegrations struct {
	Id           string     `db:"id"`
	UserId       string     `db:"user_id"`
	Provider     string     `db:"provider"`
	AccessToken  string     `db:"access_token"`
	RefreshToken *string    `db:"refresh_token"`
	ExpiresAt    *time.Time `db:"expires_at"`
	CreatedAt    time.Time  `db:"created_at"`
	UpdatedAt    *time.Time `db:"updated_at"`
}

// UserIntegrationsColumns нь SELECT/RETURNING-д DRY байлгахад ашиглагдана.
const UserIntegrationsColumns = "id, user_id, provider, access_token, refresh_token, expires_at, created_at, updated_at"

func (r UserIntegrations) ToV1Domain() domain.UserIntegration {
	refresh := ""
	if r.RefreshToken != nil {
		refresh = *r.RefreshToken
	}
	return domain.UserIntegration{
		ID:           r.Id,
		UserID:       r.UserId,
		Provider:     r.Provider,
		AccessToken:  r.AccessToken,
		RefreshToken: refresh,
		ExpiresAt:    r.ExpiresAt,
		CreatedAt:    r.CreatedAt,
		UpdatedAt:    r.UpdatedAt,
	}
}
