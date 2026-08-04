// Gerege Systems Development Team болон Claude AI хамтран бүтээв, 2026.

package routes

import (
	"net/http"

	eidauthuc "github.com/gerege-systems/open-gerege-core/core/business/usecases/eidauth"
	gatewayuc "github.com/gerege-systems/open-gerege-core/core/business/usecases/gateway"
	v1 "github.com/gerege-systems/open-gerege-core/core/http/handlers/v1"
	eidauthhandler "github.com/gerege-systems/open-gerege-core/core/http/handlers/v1/eidauth"

	"github.com/go-chi/chi/v5"
)

// EIDAuthServiceName — API gateway catalog дахь eID НЭВТРЭЛТИЙН proxy service.
const EIDAuthServiceName = "eid-auth"

// eidAuthRoute нь бүртгэгдсэн апп (RP)-д eID НЭВТРЭЛТ (initiate + poll)-ийг
// прокси-оор олгоно. /v1/eid/* (уншилтын PKI proxy)-ээс ялгаатай нь: тэнд
// иргэн аль хэдийн танигдсан тул иргэний токеноор ажилладаг бол ЭНД иргэн
// хараахан танигдаагүй — тиймээс АППЫН токен (client_credentials) хэрэглэнэ.
//
// Ингэснээр eID RP креденшлгүй платформ (жишээ нь супер админ бүртгэлийн
// шидтэн) иргэнийг eID-ээр баталгаажуулж чадна: SSO нь өөрийн креденшлээр
// session эхлүүлж, түүхий төлөв + identity-г буцаана. Хэрэглэгч ҮҮСГЭХГҮЙ,
// session ОЛГОХГҮЙ — дуудагч апп өөрөө шийднэ.
type eidAuthRoute struct {
	handler   eidauthhandler.Handler
	router    chi.Router
	gatewayUC gatewayuc.Usecase
	appMW     func(http.Handler) http.Handler
	// pollMW — poll нь long-poll тул тусад нь хурдны хязгаартай.
	pollMW func(http.Handler) http.Handler
}

// NewEIDAuthRoute нь eID нэвтрэлтийн proxy-ийн route үүсгэнэ. pollMW nil байж
// болно (хязгааргүй).
func NewEIDAuthRoute(router chi.Router, uc eidauthuc.Usecase, gatewayUC gatewayuc.Usecase, appMW, pollMW func(http.Handler) http.Handler) *eidAuthRoute {
	return &eidAuthRoute{
		handler:   eidauthhandler.NewHandler(uc),
		router:    router,
		gatewayUC: gatewayUC,
		appMW:     appMW,
		pollMW:    pollMW,
	}
}

// gate нь gateway catalog дахь service идэвхтэй эсэхийг шалгана — унтраасан бол
// 503. Catalog-д байхгүй бол fail-open (код-default идэвхтэй).
func (rt *eidAuthRoute) gate(serviceName string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if on, err := rt.gatewayUC.ServiceEnabled(r.Context(), serviceName); err == nil && !on {
				_ = v1.NewErrorResponse(w, r, http.StatusServiceUnavailable, "service is disabled: "+serviceName)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

func (rt *eidAuthRoute) Routes() {
	rt.router.Route("/v1/eid-auth", func(r chi.Router) {
		r.Use(rt.gate(EIDAuthServiceName))
		r.Use(rt.appMW)
		if rt.pollMW != nil {
			r.Use(rt.pollMW)
		}
		r.Post("/start", v1.Wrap(rt.handler.Start))
		r.Post("/start-id", v1.Wrap(rt.handler.StartByNationalID))
		r.Post("/poll", v1.Wrap(rt.handler.Poll))
	})
}
