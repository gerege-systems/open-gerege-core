// Gerege Systems Development Team болон Claude AI хамтран бүтээв, 2026.

package middlewares

import (
	"context"
	"net/http"
	"strings"

	"github.com/gerege-systems/platform-core/core/business/domain"
	oidcuc "github.com/gerege-systems/platform-core/core/business/usecases/oidc"
	"github.com/gerege-systems/platform-core/core/constants"
	"github.com/gerege-systems/platform-core/core/datasources/rls"
	V1Handler "github.com/gerege-systems/platform-core/core/http/handlers/v1"
	"github.com/gerege-systems/platform-core/pkg/jwt"
	"github.com/gerege-systems/platform-core/pkg/logger"
)

// OAuthBearerMiddleware нь бүртгэгдсэн апп (relying party)-ийн access token-оор
// хамгаалдаг middleware — тухайн апп хэрэглэгчийнхээ өмнөөс энэ SSO-ий
// service-үүдийг ДАМЖУУЛАН (proxy) дуудна. Token-ий `sub` нь энэ SSO-ий
// тогтвортой user ID тул downstream handler-ууд session-ий адил
// CurrentUserFromContext-оор хэрэглэгчийг олно.
//
// ЗӨВШӨӨРӨЛ: requiredServiceScope тавьсан бол апп нь тухайн gateway service-т
// ЗӨВШӨӨРӨГДСӨН байх ёстой. Зөвшөөрөл нь client-ийн ОДООГИЙН allowed scope дахь
// service scope (жишээ "svc:eid-proxy")-ээр илэрхийлэгдэнэ — admin UI-аас
// service олгох/цуцлах нь аль хэдийн гаргасан token-д ч ШУУД хүчинтэй болно.
// Тиймээс token-ий scope биш, client-ийн одоогийн олголтыг шалгана.
//
// Өмнө нь энэ шалгалт Hydra руу хүсэлт БҮРД хоёр HTTP дуудалт (introspect +
// get client) хийдэг байсан; одоо хоёулаа локал DB унших болсон.
type OAuthBearerMiddleware struct {
	oidc                 *oidcuc.Service
	clients              clientLookup
	requiredServiceScope string // жишээ "svc:eid-proxy"; хоосон бол зөвхөн active token
}

// clientLookup нь client-ийн одоогийн олголтыг уншина.
type clientLookup interface {
	Get(ctx context.Context, clientID string) (domain.OAuthClient, error)
}

// NewOAuthBearerMiddleware нь token шалгах + client олголт шалгах chi middleware
// буцаана. requiredServiceScope хоосон бол зөвхөн active token шаардана.
func NewOAuthBearerMiddleware(svc *oidcuc.Service, clients clientLookup, requiredServiceScope string) func(http.Handler) http.Handler {
	m := &OAuthBearerMiddleware{
		oidc:                 svc,
		clients:              clients,
		requiredServiceScope: strings.TrimSpace(requiredServiceScope),
	}
	return m.Handle
}

func (m *OAuthBearerMiddleware) Handle(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()

		token := bearerToken(r)
		if token == "" {
			_ = V1Handler.NewAbortResponse(w, r, "missing bearer token")
			return
		}

		// Token хайлт нь нэвтрэхээс ӨМНӨ явагдана — RLS-ийн service identity дор.
		info := m.oidc.Introspect(ctx, "", token)
		if !info.Active {
			_ = V1Handler.NewAbortResponse(w, r, "invalid or expired token")
			return
		}
		// Subject-гүй token (client_credentials) нь иргэний өгөгдөлд хандахгүй.
		if info.Subject == "" {
			_ = V1Handler.NewAbortResponse(w, r, "token has no subject")
			return
		}

		if m.requiredServiceScope != "" {
			client, err := m.clients.Get(ctx, info.ClientID)
			if err != nil {
				logger.ErrorWithContext(ctx, "OAuth: client-ийн олголтыг уншиж чадсангүй", logger.Fields{
					"middleware": "OAuthBearerMiddleware", "error": err.Error(), "client_id": info.ClientID,
				})
				_ = V1Handler.NewErrorResponse(w, r, http.StatusServiceUnavailable, "authorization temporarily unavailable")
				return
			}
			if !client.Enabled || !client.AllowsScope(m.requiredServiceScope) {
				_ = V1Handler.NewErrorResponse(w, r, http.StatusForbidden, "application is not granted this service")
				return
			}
		}

		// Downstream handler-ууд OAuth дуудагчийг session дуудагчаас ялгахгүй.
		claims := jwt.JwtCustomClaim{UserID: info.Subject, Kind: "access"}
		ctx = context.WithValue(ctx, constants.CtxAuthenticatedUserKey, claims)
		ctx = rls.WithUser(ctx, info.Subject)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// bearerToken нь `Authorization: Bearer <token>` толгойгоос token-ыг гаргана.
func bearerToken(r *http.Request) string {
	const prefix = "Bearer "
	h := r.Header.Get("Authorization")
	if len(h) > len(prefix) && strings.EqualFold(h[:len(prefix)], prefix) {
		return strings.TrimSpace(h[len(prefix):])
	}
	return ""
}
