// Government Template Platform V3.0
// Gerege Systems Development Team болон Claude AI хамтран бүтээв, 2026.

package routes

import (
	"github.com/go-chi/chi/v5"

	oidcuc "template/internal/business/usecases/oidc"
	oidchandler "template/internal/http/handlers/v1/oidc"
)

// oidcRoute нь өөрийн OAuth2/OIDC provider-ийн НИЙТИЙН endpoint-уудыг холбоно.
//
// Эдгээр нь `/api/v1` бүлгээс ГАДУУР, үндэс дээр сууна — учир нь тэдгээрийн зам
// нь OIDC стандартаар (`/.well-known/*`) болон одоо байгаа nginx-ийн Hydra руу
// заадаг дүрмүүдээр (`/oauth2/*`, `/userinfo`) тогтоогдсон. Cutover нь зөвхөн
// nginx-ийн upstream-ыг солих ажил болно.
//
// Бүгд нээлттэй (нэвтрэлт шаардахгүй) — client-ийн баталгаажуулалт нь endpoint
// бүрийн дотор, протоколын дагуу хийгдэнэ.
type oidcRoute struct {
	handler oidchandler.Handler
	router  chi.Router
}

func NewOIDCRoute(router chi.Router, keys *oidcuc.KeyManager, svc *oidcuc.Service, issuer string) *oidcRoute {
	return &oidcRoute{
		handler: oidchandler.NewHandler(keys, svc, issuer),
		router:  router,
	}
}

func (rt *oidcRoute) Routes() {
	rt.router.Get(oidcuc.PathDiscovery, rt.handler.Discovery)
	rt.router.Get(oidcuc.PathJWKS, rt.handler.JWKS)
	rt.router.Get(oidcuc.PathAuthorize, rt.handler.Authorize)
	rt.router.Post(oidcuc.PathToken, rt.handler.Token)
	rt.router.Post(oidcuc.PathIntrospect, rt.handler.Introspect)
	rt.router.Post(oidcuc.PathRevoke, rt.handler.Revoke)
	rt.router.Get(oidcuc.PathEndSession, rt.handler.EndSession)
	rt.router.Get(oidcuc.PathUserinfo, rt.handler.Userinfo)
	rt.router.Post(oidcuc.PathUserinfo, rt.handler.Userinfo)
}
