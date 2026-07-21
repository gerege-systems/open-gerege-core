// eID based AI enabled Government Template Platform V3.0
// Gerege Systems Development Team болон Claude AI хамтран бүтээв, 2026.

// Package signrelay нь 3 дагч RP (жишээ template.dgov.mn) dan-аар ДАМЖИН eID
// гарын үсэг зурах reverse-proxy. RP нь eidmongolia-ий signature RP creds-гүй
// (401) тул dan (creds-тэй) урдаа тавьж өгнө: /rp/sign/v3/* → eidmongolia /v3/*
// руу dan-ий жинхэнэ EID_RP_SECRET-ыг Bearer болгож дамжуулна. RP нь зөвхөн
// SIGN_RELAY_TOKEN-оор баталгаажина; eidmongolia-ий нууц secret RP-д ил болохгүй.
package signrelay

import (
	"crypto/subtle"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"
)

const prefix = "/rp/sign"

type Handler struct {
	token string
	proxy *httputil.ReverseProxy
}

// New нь eidmongolia base URL (https://eidmongolia.mn эсвэл .../v3), dan-ий
// eidmongolia RP Bearer secret, RP-ээс шаардах relay token-оос handler үүсгэнэ.
func New(eidBaseURL, rpSecret, token string) (*Handler, error) {
	base := strings.TrimSuffix(strings.TrimRight(strings.TrimSpace(eidBaseURL), "/"), "/v3")
	if base == "" {
		base = "https://eidmongolia.mn"
	}
	target, err := url.Parse(base)
	if err != nil {
		return nil, err
	}
	proxy := &httputil.ReverseProxy{
		Rewrite: func(pr *httputil.ProxyRequest) {
			pr.SetURL(target) // scheme/host + path-ыг target руу (path нэгтгэнэ)
			// /rp/sign/v3/... → /v3/... (relay угтварыг хасна)
			pr.Out.URL.Path = strings.TrimPrefix(pr.Out.URL.Path, prefix)
			if !strings.HasPrefix(pr.Out.URL.Path, "/") {
				pr.Out.URL.Path = "/" + pr.Out.URL.Path
			}
			pr.Out.Host = target.Host
			// RP-ийн relay token-ыг dan-ий ЖИНХЭНЭ eidmongolia RP Bearer-ээр солино.
			pr.Out.Header.Set("Authorization", "Bearer "+rpSecret)
		},
	}
	return &Handler{token: token, proxy: proxy}, nil
}

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	presented := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
	if h.token == "" || subtle.ConstantTimeCompare([]byte(presented), []byte(h.token)) != 1 {
		w.Header().Set("WWW-Authenticate", `Bearer realm="dan-sign-relay"`)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"error":"relay authentication required"}`))
		return
	}
	h.proxy.ServeHTTP(w, r)
}
