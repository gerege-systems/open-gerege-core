// Government Template Platform V3.0
// Gerege Systems Development Team болон Claude AI хамтран бүтээв, 2026.

// Package oidc нь өөрийн OAuth2/OIDC provider-ийн НИЙТИЙН endpoint-уудыг
// үйлчилнэ (`/oauth2/*`, `/userinfo`, `/.well-known/*`).
//
// АНХААР: эдгээр нь OAuth2/OIDC-ийн стандарт гэрээ тул платформын ердийн
// `v1.BaseResponse` бүрхүүлийг ХЭРЭГЛЭХГҮЙ — RP-ийн сангууд задлахгүй. Хариу нь
// RFC-ийн заасан JSON биетэй, алдаа нь RFC 6749 §5.2-ийн `{"error": ...}` хэлбэртэй.
package oidc

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/url"
	"strings"

	"template/internal/apperror"
	"template/internal/business/domain"
	oidcuc "template/internal/business/usecases/oidc"
	"template/pkg/logger"
)

type Handler struct {
	keys   *oidcuc.KeyManager
	svc    *oidcuc.Service
	issuer string
}

func NewHandler(keys *oidcuc.KeyManager, svc *oidcuc.Service, issuer string) Handler {
	return Handler{keys: keys, svc: svc, issuer: strings.TrimRight(issuer, "/")}
}

// Discovery godoc
// @Summary      OpenID Connect discovery баримт
// @Tags         oidc
// @Produce      json
// @Success      200  {object}  map[string]any
// @Router       /.well-known/openid-configuration [get]
func (h Handler) Discovery(w http.ResponseWriter, r *http.Request) {
	// Discovery нь ховор өөрчлөгддөг ба RP-үүд кэшилдэг.
	w.Header().Set("Cache-Control", "public, max-age=3600")
	writeJSON(w, r, http.StatusOK, oidcuc.BuildDiscovery(h.issuer))
}

// JWKS godoc
// @Summary      id_token шалгах нийтийн түлхүүрүүд (JWK Set)
// @Tags         oidc
// @Produce      json
// @Success      200  {object}  map[string]any
// @Router       /.well-known/jwks.json [get]
func (h Handler) JWKS(w http.ResponseWriter, r *http.Request) {
	set, err := h.keys.JWKS(r.Context())
	if err != nil {
		logger.ErrorWithContext(r.Context(), "OIDC: JWKS-ийг уншиж чадсангүй", logger.Fields{"error": err.Error()})
		writeError(w, r, http.StatusInternalServerError, "server_error", "could not load signing keys")
		return
	}
	// Түлхүүр эргэлт нь шинэ kid авчирдаг тул RP-үүд удаан кэшлэх ёсгүй.
	w.Header().Set("Cache-Control", "public, max-age=300")
	writeJSON(w, r, http.StatusOK, set)
}

func writeJSON(w http.ResponseWriter, r *http.Request, status int, body any) {
	w.Header().Set("Content-Type", "application/json;charset=UTF-8")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(body); err != nil {
		logger.ErrorWithContext(r.Context(), "OIDC: хариу бичихэд алдаа", logger.Fields{"error": err.Error()})
	}
}

// writeError нь RFC 6749 §5.2-ийн алдааны биетийг буцаана.
func writeError(w http.ResponseWriter, r *http.Request, status int, code, description string) {
	writeJSON(w, r, status, map[string]string{
		"error":             code,
		"error_description": description,
	})
}

// Authorize godoc
// @Summary      OAuth2 authorization endpoint
// @Tags         oidc
// @Param        client_id      query  string  true   "Client ID"
// @Param        redirect_uri   query  string  true   "Registered redirect URI"
// @Param        response_type  query  string  true   "code"
// @Param        scope          query  string  false  "Space-separated scopes"
// @Param        state          query  string  false  "Opaque RP state"
// @Success      302
// @Router       /oauth2/auth [get]
func (h Handler) Authorize(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	req := oidcuc.AuthorizeRequest{
		ClientID:            q.Get("client_id"),
		RedirectURI:         q.Get("redirect_uri"),
		ResponseType:        q.Get("response_type"),
		Scope:               q.Get("scope"),
		State:               q.Get("state"),
		Nonce:               q.Get("nonce"),
		CodeChallenge:       q.Get("code_challenge"),
		CodeChallengeMethod: q.Get("code_challenge_method"),
		Prompt:              q.Get("prompt"),
	}

	challenge, _, err := h.svc.Authorize(r.Context(), req)
	if err != nil {
		var authErr *oidcuc.AuthorizeError
		if errors.As(err, &authErr) {
			// Зөвхөн service-ийн БАТАЛГААЖУУЛСАН хаяг руу чиглүүлнэ. Хүсэлтээс
			// ирсэн түүхий redirect_uri-г энд огт ашиглахгүй — client эсвэл
			// redirect_uri буруу бол алдааг шууд харуулна.
			if !authErr.CanRedirect() {
				writeError(w, r, http.StatusBadRequest, authErr.Code, authErr.Description)
				return
			}
			http.Redirect(w, r, authErr.RedirectURL(), http.StatusFound)
			return
		}
		logger.ErrorWithContext(r.Context(), "OIDC: authorize амжилтгүй", logger.Fields{"error": err.Error()})
		writeError(w, r, http.StatusInternalServerError, "server_error", "could not start the authorization request")
		return
	}

	// Нэвтрэх хуудас руу. Session байвал тэр хуудас шууд accept руу шилжинэ.
	http.Redirect(w, r, h.issuer+"/oauth/login?login_challenge="+url.QueryEscape(challenge), http.StatusFound)
}

// Token godoc
// @Summary      OAuth2 token endpoint
// @Tags         oidc
// @Accept       x-www-form-urlencoded
// @Produce      json
// @Param        grant_type  formData  string  true  "authorization_code | refresh_token | client_credentials"
// @Success      200  {object}  map[string]any
// @Router       /oauth2/token [post]
func (h Handler) Token(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		writeError(w, r, http.StatusBadRequest, "invalid_request", "malformed request body")
		return
	}

	req := oidcuc.TokenRequest{
		GrantType:    r.PostFormValue("grant_type"),
		Code:         r.PostFormValue("code"),
		RedirectURI:  r.PostFormValue("redirect_uri"),
		CodeVerifier: r.PostFormValue("code_verifier"),
		RefreshToken: r.PostFormValue("refresh_token"),
		Scope:        r.PostFormValue("scope"),
		ClientID:     r.PostFormValue("client_id"),
		ClientSecret: r.PostFormValue("client_secret"),
	}
	// HTTP Basic нь биетээс давуу — хоёулаа ирвэл Basic-ыг авна (RFC 6749 §2.3.1).
	if id, secret, ok := basicClientAuth(r); ok {
		req.ClientID, req.ClientSecret, req.SecretFromBasic = id, secret, true
	}

	// Token нь ХЭЗЭЭ Ч кэшлэгдэх ёсгүй.
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Pragma", "no-cache")

	resp, err := h.svc.Token(r.Context(), req)
	if err != nil {
		var tokErr *oidcuc.TokenError
		if errors.As(err, &tokErr) {
			if tokErr.Code == "invalid_client" {
				// RFC 6749 §5.2 — Basic ашигласан бол WWW-Authenticate буцаана.
				w.Header().Set("WWW-Authenticate", `Basic realm="oauth2"`)
			}
			writeError(w, r, tokErr.Status, tokErr.Code, tokErr.Description)
			return
		}
		logger.ErrorWithContext(r.Context(), "OIDC: token гаргаж чадсангүй", logger.Fields{
			"error": err.Error(), "grant_type": req.GrantType,
		})
		writeError(w, r, http.StatusInternalServerError, "server_error", "could not issue a token")
		return
	}
	writeJSON(w, r, http.StatusOK, resp)
}

// basicClientAuth нь Authorization: Basic-аас client итгэмжлэлийг задална.
//
// RFC 6749 §2.3.1 нь client_id/secret-ыг base64-ийн ӨМНӨ form-urlencode хийхийг
// шаарддаг — тусгай тэмдэгттэй secret зөв ажиллахын тулд буцааж decode хийнэ.
func basicClientAuth(r *http.Request) (clientID, clientSecret string, ok bool) {
	id, secret, ok := r.BasicAuth()
	if !ok {
		return "", "", false
	}
	decodedID, err := url.QueryUnescape(id)
	if err != nil {
		decodedID = id
	}
	decodedSecret, err := url.QueryUnescape(secret)
	if err != nil {
		decodedSecret = secret
	}
	return decodedID, decodedSecret, true
}

// Userinfo godoc
// @Summary      OIDC userinfo
// @Tags         oidc
// @Produce      json
// @Success      200  {object}  map[string]any
// @Router       /userinfo [get]
func (h Handler) Userinfo(w http.ResponseWriter, r *http.Request) {
	token := bearerToken(r)
	if token == "" {
		w.Header().Set("WWW-Authenticate", `Bearer realm="userinfo"`)
		writeError(w, r, http.StatusUnauthorized, "invalid_token", "a bearer access token is required")
		return
	}
	claims, err := h.svc.Userinfo(r.Context(), token)
	if err != nil {
		w.Header().Set("WWW-Authenticate", `Bearer realm="userinfo", error="invalid_token"`)
		writeError(w, r, http.StatusUnauthorized, "invalid_token", "the access token is not valid for userinfo")
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	writeJSON(w, r, http.StatusOK, claims)
}

// Introspect godoc
// @Summary      OAuth2 token introspection (RFC 7662)
// @Tags         oidc
// @Accept       x-www-form-urlencoded
// @Produce      json
// @Success      200  {object}  map[string]any
// @Router       /oauth2/introspect [post]
func (h Handler) Introspect(w http.ResponseWriter, r *http.Request) {
	client, ok := h.authenticateCaller(w, r)
	if !ok {
		return
	}
	// Зөвхөн ӨӨРИЙН token-ыг шалгана — өөр client-ийнх бол active:false.
	w.Header().Set("Cache-Control", "no-store")
	writeJSON(w, r, http.StatusOK, h.svc.Introspect(r.Context(), client.ClientID, r.PostFormValue("token")))
}

// Revoke godoc
// @Summary      OAuth2 token revocation (RFC 7009)
// @Tags         oidc
// @Accept       x-www-form-urlencoded
// @Success      200
// @Router       /oauth2/revoke [post]
func (h Handler) Revoke(w http.ResponseWriter, r *http.Request) {
	client, ok := h.authenticateCaller(w, r)
	if !ok {
		return
	}
	if err := h.svc.Revoke(r.Context(), client, r.PostFormValue("token"), r.PostFormValue("token_type_hint")); err != nil {
		logger.ErrorWithContext(r.Context(), "OIDC: revoke амжилтгүй", logger.Fields{"error": err.Error()})
		writeError(w, r, http.StatusInternalServerError, "server_error", "could not revoke the token")
		return
	}
	// RFC 7009 §2.2 — амжилттай үед хоосон 200.
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(http.StatusOK)
}

// EndSession godoc
// @Summary      RP-initiated logout
// @Tags         oidc
// @Success      302
// @Router       /oauth2/sessions/logout [get]
func (h Handler) EndSession(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	challenge, err := h.svc.StartLogout(r.Context(),
		q.Get("client_id"), q.Get("id_token_hint"),
		q.Get("post_logout_redirect_uri"), q.Get("state"))
	if err != nil {
		// Яагаад болохгүйг RP-д хэлнэ — "could not start logout" гэдэг нь
		// интеграц хийж буй хүнд юу ч хэлдэггүй байсан.
		desc := "could not start logout"
		var domErr *apperror.DomainError
		if errors.As(err, &domErr) && domErr.Type == apperror.ErrTypeBadRequest {
			desc = domErr.Message
		}
		logger.ErrorWithContext(r.Context(), "OIDC: logout эхлүүлж чадсангүй", logger.Fields{"error": err.Error()})
		writeError(w, r, http.StatusBadRequest, "invalid_request", desc)
		return
	}
	http.Redirect(w, r, h.issuer+"/oauth/logout?logout_challenge="+url.QueryEscape(challenge), http.StatusFound)
}

// authenticateCaller нь introspect/revoke-ийг дуудаж буй client-ийг
// баталгаажуулна. Эдгээр endpoint нээлттэй байвал дурын хүн token-ийн төлөвийг
// шалгах (эсвэл цуцлах) боломжтой болно.
func (h Handler) authenticateCaller(w http.ResponseWriter, r *http.Request) (domain.OAuthClient, bool) {
	if err := r.ParseForm(); err != nil {
		writeError(w, r, http.StatusBadRequest, "invalid_request", "malformed request body")
		return domain.OAuthClient{}, false
	}
	req := oidcuc.TokenRequest{
		ClientID:     r.PostFormValue("client_id"),
		ClientSecret: r.PostFormValue("client_secret"),
	}
	if id, secret, ok := basicClientAuth(r); ok {
		req.ClientID, req.ClientSecret, req.SecretFromBasic = id, secret, true
	}

	client, err := h.svc.AuthenticateClient(r.Context(), req)
	if err != nil {
		w.Header().Set("WWW-Authenticate", `Basic realm="oauth2"`)
		writeError(w, r, http.StatusUnauthorized, "invalid_client", "client authentication failed")
		return domain.OAuthClient{}, false
	}
	// Public client (auth method = none) нь ЮУ Ч батлаагүй — түүний client_id нь
	// SPA/мобайл багцад ил байдаг. Token endpoint дээр PKCE нөхдөг ч энд нөхөх
	// зүйл байхгүй тул introspect/revoke-д хүлээж авахгүй.
	if client.IsPublic() {
		w.Header().Set("WWW-Authenticate", `Basic realm="oauth2"`)
		writeError(w, r, http.StatusUnauthorized, "invalid_client",
			"this endpoint requires a client that authenticates with a secret")
		return domain.OAuthClient{}, false
	}
	return client, true
}

// bearerToken нь Authorization: Bearer <token>-ыг гаргаж авна.
func bearerToken(r *http.Request) string {
	const prefix = "Bearer "
	h := r.Header.Get("Authorization")
	if len(h) > len(prefix) && strings.EqualFold(h[:len(prefix)], prefix) {
		return strings.TrimSpace(h[len(prefix):])
	}
	return ""
}
