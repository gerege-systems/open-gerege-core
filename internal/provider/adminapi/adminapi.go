// Gerege Template Version 27.0
// Gerege Systems Development Team болон Claude AI хамтран бүтээв, 2026.

// Package adminapi нь dan-ийг OIDC provider болгосон /admin оператор гадаргуу:
// admin API key-ээр баталгаажиж, платформын АЛИВАА OAuth2 client (RP)-ыг
// бүртгэх/удирдах, admin key-үүдийг minted/цуцлах. Ory Hydra admin API (pkg/
// hydra) дээр суурилна; эзэмшлийн мэдээллийг devapps бүртгэлээс нэгтгэнэ.
// Дэлхийн стандарт "management API + secret key" загвар (Stripe/Auth0/Okta).
// sso-dgov-mn-ий internal/web/admin_handlers.go-оос шилжүүлэв.
//
// Router()-ийг dan-ийн chi router-т `Mount("/admin", ...)`-оор холбоно (chi нь
// "/admin" угтварыг хасдаг тул доорх pattern-ууд түүнгүй).
package adminapi

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"net/url"
	"regexp"
	"strings"

	"template/internal/provider/adminkeys"
	"template/internal/provider/devapps"
	"template/pkg/hydra"
)

// Handler нь /admin гадаргуугийн хараат байдлуудыг агуулна.
type Handler struct {
	hydra     *hydra.Admin
	devApps   *devapps.Store
	adminKeys *adminkeys.Store
}

func New(h *hydra.Admin, d *devapps.Store, a *adminkeys.Store) *Handler {
	return &Handler{hydra: h, devApps: d, adminKeys: a}
}

// Router нь /admin доорх зам (chi Mount-ийн дараах) бүхий stdlib mux-ийг буцаана.
func (h *Handler) Router() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/v1/me", h.me)
	mux.HandleFunc("GET /api/v1/clients", h.listClients)
	mux.HandleFunc("POST /api/v1/clients", h.createClient)
	mux.HandleFunc("GET /api/v1/clients/{client_id}", h.getClient)
	mux.HandleFunc("PATCH /api/v1/clients/{client_id}", h.updateClient)
	mux.HandleFunc("DELETE /api/v1/clients/{client_id}", h.deleteClient)
	mux.HandleFunc("POST /api/v1/clients/{client_id}/rotate-secret", h.rotateClientSecret)
	mux.HandleFunc("GET /api/v1/keys", h.listKeys)
	mux.HandleFunc("POST /api/v1/keys", h.createKey)
	mux.HandleFunc("DELETE /api/v1/keys/{id}", h.revokeKey)
	return mux
}

// requireAdminKey нь хүсэлтийг admin API key-ээр баталгаажуулна (`Authorization:
// Bearer <key>` эсвэл `X-API-Key: <key>`).
func (h *Handler) requireAdminKey(w http.ResponseWriter, r *http.Request) (*adminkeys.Key, bool) {
	presented := ""
	if a := r.Header.Get("Authorization"); strings.HasPrefix(a, "Bearer ") {
		presented = strings.TrimPrefix(a, "Bearer ")
	} else if k := r.Header.Get("X-API-Key"); k != "" {
		presented = k
	}
	if presented == "" {
		w.Header().Set("WWW-Authenticate", `Bearer realm="dan-admin"`)
		writeAPIError(w, http.StatusUnauthorized, "missing admin API key (Authorization: Bearer <key> or X-API-Key)")
		return nil, false
	}
	key, ok := h.adminKeys.Verify(r.Context(), presented)
	if !ok {
		writeAPIError(w, http.StatusUnauthorized, "invalid or revoked admin API key")
		return nil, false
	}
	return key, true
}

// --- client management ---

type ClientView struct {
	ClientID                string   `json:"client_id"`
	Name                    string   `json:"client_name"`
	RedirectURIs            []string `json:"redirect_uris"`
	PostLogoutRedirectURIs  []string `json:"post_logout_redirect_uris,omitempty"`
	Scopes                  []string `json:"scopes"`
	GrantTypes              []string `json:"grant_types,omitempty"`
	ResponseTypes           []string `json:"response_types,omitempty"`
	SubjectType             string   `json:"subject_type,omitempty"`
	TokenEndpointAuthMethod string   `json:"token_endpoint_auth_method,omitempty"`
	BackchannelLogoutURI    string   `json:"backchannel_logout_uri,omitempty"`
	FrontchannelLogoutURI   string   `json:"frontchannel_logout_uri,omitempty"`
	DPoPBoundAccessTokens   bool     `json:"dpop_bound_access_tokens,omitempty"`
	CreatedAt               string   `json:"created_at,omitempty"`
	UpdatedAt               string   `json:"updated_at,omitempty"`
	OwnerEIDSub             string   `json:"owner_eid_sub,omitempty"`
	// ClientSecret нь зөвхөн create + rotate-secret дээр буцна (нэг удаа).
	ClientSecret string `json:"client_secret,omitempty"`
}

func clientViewFromHydra(c *hydra.ClientListEntry) ClientView {
	return ClientView{
		ClientID:                c.ClientID,
		Name:                    c.ClientName,
		RedirectURIs:            c.RedirectURIs,
		PostLogoutRedirectURIs:  c.PostLogoutRedirectURIs,
		Scopes:                  strings.Fields(c.Scope),
		GrantTypes:              c.GrantTypes,
		ResponseTypes:           c.ResponseTypes,
		SubjectType:             c.SubjectType,
		TokenEndpointAuthMethod: c.TokenEndpointAuthMethod,
		CreatedAt:               c.CreatedAt,
		UpdatedAt:               c.UpdatedAt,
	}
}

func (h *Handler) listClients(w http.ResponseWriter, r *http.Request) {
	if _, ok := h.requireAdminKey(w, r); !ok {
		return
	}
	clients, err := h.hydra.ListClients(r.Context())
	if err != nil {
		log.Printf("admin: hydra list clients: %v", err)
		writeAPIError(w, http.StatusBadGateway, "hydra list failed")
		return
	}
	owners := map[string]string{}
	for _, a := range h.devApps.ListAll(r.Context()) {
		owners[a.ClientID] = a.OwnerEIDSub
	}
	out := make([]ClientView, 0, len(clients))
	for i := range clients {
		v := clientViewFromHydra(&clients[i])
		v.OwnerEIDSub = owners[clients[i].ClientID]
		out = append(out, v)
	}
	writeJSON(w, http.StatusOK, out)
}

type adminClientBody struct {
	ClientID               string   `json:"client_id,omitempty"`
	Name                   string   `json:"client_name"`
	RedirectURIs           []string `json:"redirect_uris"`
	PostLogoutRedirectURIs []string `json:"post_logout_redirect_uris,omitempty"`
	Scopes                 []string `json:"scopes,omitempty"`
	GrantTypes             []string `json:"grant_types,omitempty"`
	SubjectType            string   `json:"subject_type,omitempty"`
	// Public нь PKCE-enforced public client бүртгэнэ (RFC 9700): secret байхгүй,
	// token_endpoint_auth_method=none. Mobile/SPA RP-д.
	Public                bool   `json:"public,omitempty"`
	BackchannelLogoutURI  string `json:"backchannel_logout_uri,omitempty"`
	FrontchannelLogoutURI string `json:"frontchannel_logout_uri,omitempty"`
	DPoP                  bool   `json:"dpop_bound_access_tokens,omitempty"`
}

func (h *Handler) createClient(w http.ResponseWriter, r *http.Request) {
	if _, ok := h.requireAdminKey(w, r); !ok {
		return
	}
	var body adminClientBody
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 16<<10)).Decode(&body); err != nil {
		writeAPIError(w, http.StatusBadRequest, "invalid JSON: "+err.Error())
		return
	}
	if err := validateAdminClient(body, true); err != nil {
		writeAPIError(w, http.StatusBadRequest, err.Error())
		return
	}

	clientID := strings.TrimSpace(body.ClientID)
	if clientID == "" {
		clientID = "app-" + randomHex(8)
	} else if _, err := h.hydra.GetClient(r.Context(), clientID); err == nil {
		writeAPIError(w, http.StatusConflict, "client_id already exists: "+clientID)
		return
	}

	scopes := body.Scopes
	if len(scopes) == 0 {
		scopes = []string{"openid", "profile", "email"}
	}
	grants := body.GrantTypes
	if len(grants) == 0 {
		grants = []string{"authorization_code", "refresh_token"}
	}
	subjectType := body.SubjectType
	if subjectType == "" {
		subjectType = "public"
	}
	postLogout := body.PostLogoutRedirectURIs
	if postLogout == nil {
		postLogout = postLogoutFromRedirects(body.RedirectURIs)
	}
	// Public (PKCE) vs confidential. Public client-д secret байхгүй, Hydra PKCE
	// шаардана (RFC 9700).
	authMethod := "client_secret_basic"
	clientSecret := randomURL(40)
	if body.Public {
		authMethod = "none"
		clientSecret = ""
	}

	hc := hydra.ClientCreate{
		ClientID:                clientID,
		ClientName:              body.Name,
		ClientSecret:            clientSecret,
		GrantTypes:              grants,
		ResponseTypes:           responseTypesFor(grants),
		Scope:                   strings.Join(scopes, " "),
		RedirectURIs:            body.RedirectURIs,
		PostLogoutRedirectURIs:  postLogout,
		TokenEndpointAuthMethod: authMethod,
		SubjectType:             subjectType,
		BackchannelLogoutURI:    body.BackchannelLogoutURI,
		FrontchannelLogoutURI:   body.FrontchannelLogoutURI,
		DPoPBoundAccessTokens:   body.DPoP,
	}
	if body.BackchannelLogoutURI != "" {
		hc.BackchannelLogoutSessionRequired = true
	}
	if _, err := h.hydra.CreateClient(r.Context(), hc); err != nil {
		log.Printf("admin: hydra create client: %v", err)
		writeAPIError(w, http.StatusBadGateway, "hydra client create failed: "+err.Error())
		return
	}

	view := ClientView{
		ClientID:                clientID,
		Name:                    body.Name,
		RedirectURIs:            body.RedirectURIs,
		PostLogoutRedirectURIs:  postLogout,
		Scopes:                  scopes,
		GrantTypes:              grants,
		ResponseTypes:           responseTypesFor(grants),
		SubjectType:             subjectType,
		TokenEndpointAuthMethod: authMethod,
		BackchannelLogoutURI:    body.BackchannelLogoutURI,
		FrontchannelLogoutURI:   body.FrontchannelLogoutURI,
		DPoPBoundAccessTokens:   body.DPoP,
		ClientSecret:            clientSecret, // нэг удаа (public-д хоосон)
	}
	writeJSON(w, http.StatusCreated, view)
}

func (h *Handler) getClient(w http.ResponseWriter, r *http.Request) {
	if _, ok := h.requireAdminKey(w, r); !ok {
		return
	}
	id := r.PathValue("client_id")
	c, err := h.hydra.GetClient(r.Context(), id)
	if err != nil {
		writeAPIError(w, http.StatusNotFound, "client not found")
		return
	}
	v := clientViewFromHydra(c)
	if a, ok := h.devApps.Get(r.Context(), id); ok {
		v.OwnerEIDSub = a.OwnerEIDSub
	}
	writeJSON(w, http.StatusOK, v)
}

// PATCH — өгсөн талбаруудыг одоогийн record дээр давхарлаж бүрэн PUT хийнэ
// (Hydra-ийн PUT нь replace-style).
func (h *Handler) updateClient(w http.ResponseWriter, r *http.Request) {
	if _, ok := h.requireAdminKey(w, r); !ok {
		return
	}
	id := r.PathValue("client_id")
	current, err := h.hydra.GetClient(r.Context(), id)
	if err != nil {
		writeAPIError(w, http.StatusNotFound, "client not found")
		return
	}
	var body adminClientBody
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 16<<10)).Decode(&body); err != nil {
		writeAPIError(w, http.StatusBadRequest, "invalid JSON: "+err.Error())
		return
	}
	if err := validateAdminClient(body, false); err != nil {
		writeAPIError(w, http.StatusBadRequest, err.Error())
		return
	}

	name := current.ClientName
	if body.Name != "" {
		name = body.Name
	}
	redirects := current.RedirectURIs
	if body.RedirectURIs != nil {
		redirects = body.RedirectURIs
	}
	scope := current.Scope
	if len(body.Scopes) > 0 {
		scope = strings.Join(body.Scopes, " ")
	}
	grants := defaultIfEmpty(current.GrantTypes, []string{"authorization_code", "refresh_token"})
	if len(body.GrantTypes) > 0 {
		grants = body.GrantTypes
	}
	subjectType := current.SubjectType
	if subjectType == "" {
		subjectType = "public"
	}
	if body.SubjectType != "" {
		subjectType = body.SubjectType
	}
	postLogout := defaultIfEmpty(current.PostLogoutRedirectURIs, postLogoutFromRedirects(redirects))
	if body.PostLogoutRedirectURIs != nil {
		postLogout = body.PostLogoutRedirectURIs
	}

	hu := hydra.ClientUpdate{
		ClientID:                id,
		ClientName:              name,
		GrantTypes:              grants,
		ResponseTypes:           responseTypesFor(grants),
		Scope:                   scope,
		RedirectURIs:            redirects,
		PostLogoutRedirectURIs:  postLogout,
		TokenEndpointAuthMethod: "client_secret_basic",
		SubjectType:             subjectType,
	}
	if _, err := h.hydra.UpdateClient(r.Context(), id, hu); err != nil {
		log.Printf("admin: hydra update client: %v", err)
		writeAPIError(w, http.StatusBadGateway, "hydra update failed: "+err.Error())
		return
	}
	if _, ok := h.devApps.Get(r.Context(), id); ok {
		if _, err := h.devApps.Update(r.Context(), id, name, redirects); err != nil {
			log.Printf("admin: devapps sync update failed: %v", err)
		}
	}
	updated, _ := h.hydra.GetClient(r.Context(), id)
	writeJSON(w, http.StatusOK, clientViewFromHydra(updated))
}

func (h *Handler) deleteClient(w http.ResponseWriter, r *http.Request) {
	if _, ok := h.requireAdminKey(w, r); !ok {
		return
	}
	id := r.PathValue("client_id")
	if err := h.hydra.DeleteClient(r.Context(), id); err != nil {
		log.Printf("admin: hydra delete client: %v", err)
		writeAPIError(w, http.StatusBadGateway, "hydra delete failed: "+err.Error())
		return
	}
	if _, ok := h.devApps.Get(r.Context(), id); ok {
		if err := h.devApps.Delete(r.Context(), id); err != nil {
			log.Printf("admin: devapps delete after hydra delete: %v — possible orphan row", err)
		}
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) rotateClientSecret(w http.ResponseWriter, r *http.Request) {
	if _, ok := h.requireAdminKey(w, r); !ok {
		return
	}
	id := r.PathValue("client_id")
	current, err := h.hydra.GetClient(r.Context(), id)
	if err != nil {
		writeAPIError(w, http.StatusNotFound, "client not found")
		return
	}
	newSecret := randomURL(40)
	subjectType := current.SubjectType
	if subjectType == "" {
		subjectType = "public"
	}
	hu := hydra.ClientUpdate{
		ClientID:                id,
		ClientName:              current.ClientName,
		ClientSecret:            newSecret,
		GrantTypes:              defaultIfEmpty(current.GrantTypes, []string{"authorization_code", "refresh_token"}),
		ResponseTypes:           defaultIfEmpty(current.ResponseTypes, responseTypesFor(current.GrantTypes)),
		Scope:                   current.Scope,
		RedirectURIs:            current.RedirectURIs,
		PostLogoutRedirectURIs:  defaultIfEmpty(current.PostLogoutRedirectURIs, postLogoutFromRedirects(current.RedirectURIs)),
		TokenEndpointAuthMethod: "client_secret_basic",
		SubjectType:             subjectType,
	}
	if _, err := h.hydra.UpdateClient(r.Context(), id, hu); err != nil {
		log.Printf("admin: hydra rotate secret: %v", err)
		writeAPIError(w, http.StatusBadGateway, "hydra rotate failed: "+err.Error())
		return
	}
	v := clientViewFromHydra(current)
	v.ClientSecret = newSecret // нэг удаа
	writeJSON(w, http.StatusOK, v)
}

// --- API key self-management ---

type keyView struct {
	ID         string `json:"id"`
	Name       string `json:"name"`
	Display    string `json:"display"`
	CreatedAt  string `json:"created_at"`
	LastUsedAt string `json:"last_used_at,omitempty"`
	Disabled   bool   `json:"disabled,omitempty"`
	Secret     string `json:"secret,omitempty"`
}

func keyToView(k adminkeys.Key) keyView {
	v := keyView{
		ID:        k.ID,
		Name:      k.Name,
		Display:   k.Display,
		CreatedAt: k.CreatedAt.Format("2006-01-02T15:04:05Z"),
		Disabled:  k.Disabled,
	}
	if !k.LastUsedAt.IsZero() {
		v.LastUsedAt = k.LastUsedAt.Format("2006-01-02T15:04:05Z")
	}
	return v
}

func (h *Handler) me(w http.ResponseWriter, r *http.Request) {
	key, ok := h.requireAdminKey(w, r)
	if !ok {
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"name": key.Name, "env": key.Env})
}

func (h *Handler) listKeys(w http.ResponseWriter, r *http.Request) {
	if _, ok := h.requireAdminKey(w, r); !ok {
		return
	}
	keys := h.adminKeys.List(r.Context())
	out := make([]keyView, 0, len(keys))
	for _, k := range keys {
		out = append(out, keyToView(k))
	}
	writeJSON(w, http.StatusOK, out)
}

func (h *Handler) createKey(w http.ResponseWriter, r *http.Request) {
	if _, ok := h.requireAdminKey(w, r); !ok {
		return
	}
	var body struct {
		Name string `json:"name"`
	}
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 4<<10)).Decode(&body); err != nil {
		writeAPIError(w, http.StatusBadRequest, "invalid JSON: "+err.Error())
		return
	}
	body.Name = strings.TrimSpace(body.Name)
	if body.Name == "" {
		writeAPIError(w, http.StatusBadRequest, "name is required")
		return
	}
	if len(body.Name) > 128 {
		writeAPIError(w, http.StatusBadRequest, "name too long (max 128)")
		return
	}
	secret, key, err := h.adminKeys.Mint(r.Context(), body.Name)
	if err != nil {
		log.Printf("admin: mint key: %v", err)
		writeAPIError(w, http.StatusInternalServerError, "store error")
		return
	}
	v := keyToView(*key)
	v.Secret = secret // нэг удаа
	writeJSON(w, http.StatusCreated, v)
}

func (h *Handler) revokeKey(w http.ResponseWriter, r *http.Request) {
	if _, ok := h.requireAdminKey(w, r); !ok {
		return
	}
	id := r.PathValue("id")
	if err := h.adminKeys.Revoke(r.Context(), id); err != nil {
		if errors.Is(err, adminkeys.ErrNotFound) {
			writeAPIError(w, http.StatusNotFound, "key not found (env bootstrap keys are removed from the operator env, not here)")
			return
		}
		log.Printf("admin: revoke key: %v", err)
		writeAPIError(w, http.StatusInternalServerError, "store error")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// --- validation ---

var clientIDRe = regexp.MustCompile(`^[a-zA-Z0-9._-]{3,64}$`)
var scopeRe = regexp.MustCompile(`^[a-zA-Z0-9._:-]+$`)

// schemeRe нь синтакс-зөв URI scheme (RFC 3986 §3.1). Go-ийн url.Parse scheme-ыг
// жижиг үсэг болгодог.
var schemeRe = regexp.MustCompile(`^[a-z][a-z0-9+.-]*$`)

var allowedGrants = map[string]bool{
	"authorization_code": true,
	"refresh_token":      true,
	"client_credentials": true,
	// Device Authorization Grant (RFC 8628) — TV, CLI, limited-input төхөөрөмж.
	"urn:ietf:params:oauth:grant-type:device_code": true,
}

func validateAdminClient(b adminClientBody, create bool) error {
	if b.ClientID != "" && !clientIDRe.MatchString(b.ClientID) {
		return errors.New("client_id: 3-64 chars of letters, digits, dot, underscore, dash")
	}
	if create && strings.TrimSpace(b.Name) == "" {
		return errors.New("client_name is required")
	}
	if len(b.Name) > 128 {
		return errors.New("client_name too long (max 128)")
	}
	usesRedirect := len(b.GrantTypes) == 0 || containsStr(b.GrantTypes, "authorization_code")
	if create && len(b.RedirectURIs) == 0 && usesRedirect {
		return errors.New("at least one redirect_uri is required")
	}
	for _, u := range b.RedirectURIs {
		// Native/mobile public (PKCE) client нь RFC 8252 private-use scheme
		// callback (жишээ geregetemp://oauth2/callback) ашиглаж болно; web client
		// https-only. post_logout / logout URI үргэлж https.
		if err := validateRedirectURI(u, b.Public); err != nil {
			return err
		}
	}
	for _, u := range b.PostLogoutRedirectURIs {
		if err := validateRedirectURI(u, false); err != nil {
			return errors.New("post_logout_redirect_uri: " + err.Error())
		}
	}
	for _, sc := range b.Scopes {
		if !scopeRe.MatchString(sc) {
			return errors.New("scope: invalid characters in " + sc)
		}
	}
	for _, g := range b.GrantTypes {
		if !allowedGrants[g] {
			return errors.New("grant_type not supported: " + g)
		}
	}
	if b.SubjectType != "" && b.SubjectType != "public" && b.SubjectType != "pairwise" {
		return errors.New("subject_type must be public or pairwise")
	}
	if b.BackchannelLogoutURI != "" {
		if err := validateRedirectURI(b.BackchannelLogoutURI, false); err != nil {
			return errors.New("backchannel_logout_uri: " + err.Error())
		}
	}
	if b.FrontchannelLogoutURI != "" {
		if err := validateRedirectURI(b.FrontchannelLogoutURI, false); err != nil {
			return errors.New("frontchannel_logout_uri: " + err.Error())
		}
	}
	return nil
}

// validateRedirectURI нь parse хийгдэх absolute URL шаардана; web client-д https
// (эсвэл loopback дээр http). allowPrivateScheme=true үед (native/mobile PUBLIC
// PKCE client) RFC 8252 §7.1 private-use scheme (geregetemp://...) зөвшөөрнө.
func validateRedirectURI(raw string, allowPrivateScheme bool) error {
	if raw == "" {
		return errors.New("redirect_uri: empty")
	}
	u, err := url.Parse(raw)
	if err != nil {
		return errors.New("redirect_uri: invalid URL")
	}
	if !u.IsAbs() {
		return errors.New("redirect_uri: must be absolute")
	}
	if u.Fragment != "" {
		return errors.New("redirect_uri: fragments not allowed (RFC 6749 §3.1.2)")
	}
	host := u.Hostname()
	switch u.Scheme {
	case "https":
		// OK
	case "http":
		if host != "localhost" && host != "127.0.0.1" && host != "::1" {
			return errors.New("redirect_uri: http only allowed on loopback")
		}
	default:
		if allowPrivateScheme && isPrivateUseScheme(u.Scheme) {
			if u.Opaque == "" && u.Path == "" && u.Host == "" {
				return errors.New("redirect_uri: private-use scheme needs a path or host")
			}
			return nil
		}
		return errors.New("redirect_uri: scheme must be https (or http on loopback)")
	}
	return nil
}

func isPrivateUseScheme(scheme string) bool {
	switch scheme {
	case "", "https", "http", "ws", "wss", "ftp", "file", "data", "blob",
		"javascript", "vbscript", "mailto":
		return false
	}
	return schemeRe.MatchString(scheme)
}

func containsStr(list []string, want string) bool {
	for _, s := range list {
		if s == want {
			return true
		}
	}
	return false
}

// responseTypesFor нь grant-аас Hydra-д хэрэгтэй response_types-г гаргана.
func responseTypesFor(grants []string) []string {
	for _, g := range grants {
		if g == "authorization_code" {
			return []string{"code"}
		}
	}
	return []string{}
}

func defaultIfEmpty(v, def []string) []string {
	if len(v) == 0 {
		return def
	}
	return v
}

// postLogoutFromRedirects нь redirect callback-уудын host-уудаас (path хассан)
// post-logout олонлогийг гаргана.
func postLogoutFromRedirects(redirects []string) []string {
	seen := map[string]struct{}{}
	out := make([]string, 0, len(redirects))
	for _, r := range redirects {
		u, err := url.Parse(r)
		if err != nil {
			continue
		}
		u.Path = "/"
		u.RawQuery = ""
		root := u.String()
		if _, ok := seen[root]; ok {
			continue
		}
		seen[root] = struct{}{}
		out = append(out, root)
	}
	return out
}

func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(v)
}

func writeAPIError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}

func randomHex(n int) string {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		panic("adminapi: crypto/rand failed: " + err.Error())
	}
	return hex.EncodeToString(b)
}

func randomURL(n int) string {
	const alphabet = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789"
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		panic("adminapi: crypto/rand failed: " + err.Error())
	}
	for i := range b {
		b[i] = alphabet[int(b[i])%len(alphabet)]
	}
	return string(b)
}
