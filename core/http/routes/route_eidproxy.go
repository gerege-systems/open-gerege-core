// Gerege Systems Development Team болон Claude AI хамтран бүтээв, 2026.

package routes

import (
	"net/http"

	authuc "github.com/gerege-systems/public-gerege-core/core/business/usecases/auth"
	gatewayuc "github.com/gerege-systems/public-gerege-core/core/business/usecases/gateway"
	v1 "github.com/gerege-systems/public-gerege-core/core/http/handlers/v1"
	eidprofilehandler "github.com/gerege-systems/public-gerege-core/core/http/handlers/v1/eidprofile"

	"github.com/go-chi/chi/v5"
)

// API gateway catalog дахь eID proxy service-үүдийн нэр. Admin gateway UI-аас энэ
// нэрсээр тус тусад нь enable/disable хийхэд route-ууд бодитоор нөлөөлнө.
const (
	// EIDProxyServiceName — хувь хүний eID (summary/certificates/devices/activity).
	EIDProxyServiceName = "eid-proxy"
	// EIDOrgProxyServiceName — байгууллагатай холбоотой eID (organizations/signers).
	EIDOrgProxyServiceName = "eid-org-proxy"
)

// eidProxyRoute нь бүртгэгдсэн апп (relying party)-уудад ЗОРИУЛСАН eID service
// proxy-г холбоно. Апп нь хэрэглэгчийнхээ Hydra access token-оор (Authorization:
// Bearer, "eid" scope) дуудна; SSO нь token-ий subject-ээр хэрэглэгчийг тогтоож,
// ӨӨРИЙН eidmongolia.mn RP creds-ээр өгөгдлийг татаж өгнө. Апп-д eID RP credential
// эзэмших шаардлагагүй. ЗӨВХӨН унших. Хоёр тусдаа gateway service-т хуваасан:
//   - /v1/eid/*      (eid-proxy)      — хувь хүний eID
//   - /v1/eid-org/*  (eid-org-proxy)  — байгууллагатай холбоотой eID
type eidProxyRoute struct {
	handler   eidprofilehandler.Handler
	router    chi.Router
	gatewayUC gatewayuc.Usecase
	// service тус бүрийн OAuth middleware — token active + subject + тухайн
	// service-т client олгогдсон эсэхийг (svc:<name> allowed scope) шалгана.
	eidMW    func(http.Handler) http.Handler
	eidOrgMW func(http.Handler) http.Handler
}

func NewEIDProxyRoute(router chi.Router, authUC authuc.Usecase, gatewayUC gatewayuc.Usecase, eidMW, eidOrgMW func(http.Handler) http.Handler) *eidProxyRoute {
	return &eidProxyRoute{
		handler:   eidprofilehandler.NewHandler(authUC),
		router:    router,
		gatewayUC: gatewayUC,
		eidMW:     eidMW,
		eidOrgMW:  eidOrgMW,
	}
}

// gate нь API gateway catalog дахь заасан service идэвхтэй эсэхийг шалгах
// middleware буцаана — admin gateway UI-аас унтраасан бол 503. Catalog-д
// байхгүй бол fail-open (код-default идэвхтэй). OAuth шалгалтаас ӨМНӨ ажиллана.
func (rt *eidProxyRoute) gate(serviceName string) func(http.Handler) http.Handler {
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

func (rt *eidProxyRoute) Routes() {
	// Хувь хүний eID (eid-proxy) — service enabled + client-д "svc:eid-proxy" олгогдсон
	rt.router.Route("/v1/eid", func(r chi.Router) {
		r.Use(rt.gate(EIDProxyServiceName))
		r.Use(rt.eidMW)
		r.Get("/summary", v1.Wrap(rt.handler.Summary))
		r.Get("/certificates", v1.Wrap(rt.handler.Certificates))
		r.Get("/devices", v1.Wrap(rt.handler.Devices))
		r.Get("/activity", v1.Wrap(rt.handler.Activity))
	})

	// Байгууллагатай холбоотой eID (eid-org-proxy) — тусад нь; "svc:eid-org-proxy" олгогдсон
	rt.router.Route("/v1/eid-org", func(r chi.Router) {
		r.Use(rt.gate(EIDOrgProxyServiceName))
		r.Use(rt.eidOrgMW)
		r.Get("/organizations", v1.Wrap(rt.handler.Organizations))
		r.Get("/organizations/{regNo}/signers", v1.Wrap(rt.handler.OrgSigners))
	})
}
