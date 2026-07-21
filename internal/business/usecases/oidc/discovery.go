// Government Template Platform V3.0
// Gerege Systems Development Team болон Claude AI хамтран бүтээв, 2026.

package oidc

// Discovery нь OpenID Connect Discovery 1.0-ийн баримт (`/.well-known/
// openid-configuration`).
//
// ЭНЭ БОЛ ГАДААД ГЭРЭЭ: RP-ийн сангууд болон iOS апп үүнийг татаж endpoint,
// дэмжигдэх алгоритм, scope-уудыг мэддэг. Талбар хасах нь RP-үүдийг эвдэж
// болзошгүй тул Hydra-гийн гаргаж байсан багцтай тэнцүү (эсвэл түүнээс өргөн)
// байлгана — ашиглагдаагүй боломжуудыг (device grant, DPoP, pairwise) л хассан.
type Discovery struct {
	Issuer                            string   `json:"issuer"`
	AuthorizationEndpoint             string   `json:"authorization_endpoint"`
	TokenEndpoint                     string   `json:"token_endpoint"`
	UserinfoEndpoint                  string   `json:"userinfo_endpoint"`
	JWKSURI                           string   `json:"jwks_uri"`
	RevocationEndpoint                string   `json:"revocation_endpoint"`
	IntrospectionEndpoint             string   `json:"introspection_endpoint"`
	EndSessionEndpoint                string   `json:"end_session_endpoint"`
	ScopesSupported                   []string `json:"scopes_supported"`
	ResponseTypesSupported            []string `json:"response_types_supported"`
	ResponseModesSupported            []string `json:"response_modes_supported"`
	GrantTypesSupported               []string `json:"grant_types_supported"`
	SubjectTypesSupported             []string `json:"subject_types_supported"`
	IDTokenSigningAlgValuesSupported  []string `json:"id_token_signing_alg_values_supported"`
	TokenEndpointAuthMethodsSupported []string `json:"token_endpoint_auth_methods_supported"`
	CodeChallengeMethodsSupported     []string `json:"code_challenge_methods_supported"`
	ClaimsSupported                   []string `json:"claims_supported"`
	UserinfoSigningAlgValuesSupported []string `json:"userinfo_signing_alg_values_supported"`
	ClaimsParameterSupported          bool     `json:"claims_parameter_supported"`
	RequestParameterSupported         bool     `json:"request_parameter_supported"`
	RequestURIParameterSupported      bool     `json:"request_uri_parameter_supported"`
	RequireRequestURIRegistration     bool     `json:"require_request_uri_registration"`
}

// Endpoint-уудын зам — nginx-ийн одоогийн proxy дүрмүүдтэй ЯГ таарна, тиймээс
// cutover нь зөвхөн upstream солих ажил болно.
const (
	PathAuthorize      = "/oauth2/auth"
	PathToken          = "/oauth2/token"
	PathRevoke         = "/oauth2/revoke"
	PathIntrospect     = "/oauth2/introspect"
	PathEndSession     = "/oauth2/sessions/logout"
	PathUserinfo       = "/userinfo"
	PathJWKS           = "/.well-known/jwks.json"
	PathDiscovery      = "/.well-known/openid-configuration"
	scopeOpenID        = "openid"
	scopeOfflineAccess = "offline_access"
)

// advertisedScopes нь discovery-д зарлах scope-ууд. САНААТАЙГААР статик:
// бүртгэгдсэн client-уудын scope-ийн нэгдлийг зарлавал дотоод gateway service-
// ийн нэрс (`svc:*`) нийтэд ил болно. Hydra ч мөн статик жагсаалт гаргадаг байв.
var advertisedScopes = []string{
	scopeOpenID, scopeOfflineAccess, "profile", "email", "nationalid",
}

// BuildDiscovery нь issuer-ээс discovery баримтыг угсарна.
func BuildDiscovery(issuer string) Discovery {
	return Discovery{
		Issuer:                issuer,
		AuthorizationEndpoint: issuer + PathAuthorize,
		TokenEndpoint:         issuer + PathToken,
		UserinfoEndpoint:      issuer + PathUserinfo,
		JWKSURI:               issuer + PathJWKS,
		RevocationEndpoint:    issuer + PathRevoke,
		IntrospectionEndpoint: issuer + PathIntrospect,
		EndSessionEndpoint:    issuer + PathEndSession,

		ScopesSupported:        advertisedScopes,
		ResponseTypesSupported: []string{"code"},
		ResponseModesSupported: []string{"query"},
		GrantTypesSupported: []string{
			"authorization_code", "refresh_token", "client_credentials",
		},
		// Зөвхөн public — production дахь бүх client `public` байсан тул
		// pairwise-ийг огт хэрэгжүүлээгүй.
		SubjectTypesSupported:            []string{"public"},
		IDTokenSigningAlgValuesSupported: []string{algRS256},
		TokenEndpointAuthMethodsSupported: []string{
			"client_secret_basic", "client_secret_post", "none",
		},
		// S256 ЗӨВХӨН — "plain" нь PKCE-ийн хамгаалалтыг утгагүй болгодог
		// (RFC 9700 §2.1.1), тиймээс огт зарлахгүй.
		CodeChallengeMethodsSupported: []string{"S256"},
		ClaimsSupported: []string{
			"sub", "iss", "aud", "exp", "iat", "auth_time", "nonce",
			"name", "given_name", "family_name", "given_name_en", "family_name_en",
			"email", "email_verified", "national_id", "register_number",
			"google_sub", "google_email", "google_name", "google_picture",
		},
		// `request` / `request_uri` (JAR) -ыг дэмжихгүй — хэрэглэгддэггүй бөгөөд
		// SSRF-ийн гадаргуу нэмдэг.
		UserinfoSigningAlgValuesSupported: []string{"none"},
		ClaimsParameterSupported:          false,
		RequestParameterSupported:         false,
		RequestURIParameterSupported:      false,
		RequireRequestURIRegistration:     true,
	}
}
