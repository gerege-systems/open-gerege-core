// Government Template Platform V3.0
// Gerege Systems Development Team болон Claude AI хамтран бүтээв, 2026.

package domain

import "time"

// OAuthClient нь бүртгэгдсэн relying party (OAuth2 client). Өмнө нь энэ бүртгэл
// Hydra-д амьдардаг байсан; одоо oauth_clients хүснэгтэд.
//
// SecretHash нь ХЭЗЭЭ Ч энэ процессоос гарахгүй — API хариунд зөвхөн шинээр
// үүсгэсэн/эргүүлсэн түүхий secret нэг удаа буцна.
type OAuthClient struct {
	ClientID                string
	ClientName              string
	SecretHash              string
	TokenEndpointAuthMethod string // client_secret_basic | client_secret_post | none
	AppType                 string // web | spa | native | m2m
	GrantTypes              []string
	ResponseTypes           []string
	Scopes                  []string
	RedirectURIs            []string
	PostLogoutRedirectURIs  []string
	Tags                    []string
	Enabled                 bool
	CreatedBy               string
	CreatedAt               time.Time
	UpdatedAt               *time.Time
}

// SigningKey нь id_token-д гарын үсэг зурах түлхүүр. PrivateKeyEnc нь
// AES-256-GCM-ээр шифрлэгдсэн PKCS#8 (pkg/crypto); PublicJWK нь JWKS-д
// нийтлэгдэх нийтийн хэсэг.
//
// Идэвхтэй нь ЯГ нэг байна, харин JWKS нь тэтгэвэрт гарсныг ч нийтэлнэ —
// эс бөгөөс тэдгээрээр зурсан, хараахан хүчинтэй id_token-ууд шалгагдахгүй болно.
type SigningKey struct {
	KID           string
	Alg           string
	PrivateKeyEnc string
	PublicJWK     []byte // jsonb
	Active        bool
	CreatedAt     time.Time
	RetiredAt     *time.Time
}

// Grant / auth method-ийн зөвшөөрөгдсөн утгууд.
const (
	GrantAuthorizationCode = "authorization_code"
	GrantRefreshToken      = "refresh_token"
	GrantClientCredentials = "client_credentials"

	AuthMethodBasic = "client_secret_basic"
	AuthMethodPost  = "client_secret_post"
	AuthMethodNone  = "none"
)

// IsPublic нь client secret нууцалж чаддаггүй (spa/native) эсэхийг заана.
// Public client-д PKCE ЗААВАЛ шаардагдана.
func (c OAuthClient) IsPublic() bool {
	return c.TokenEndpointAuthMethod == AuthMethodNone
}

// HasGrant нь тухайн grant type зөвшөөрөгдсөн эсэхийг шалгана.
func (c OAuthClient) HasGrant(grant string) bool {
	return containsExact(c.GrantTypes, grant)
}

// AllowsScope нь тухайн scope client-д олгогдсон эсэхийг шалгана.
func (c OAuthClient) AllowsScope(scope string) bool {
	return containsExact(c.Scopes, scope)
}

// FilterAllowedScopes нь хүссэн scope-уудаас client-д ОЛГОГДСОНЫГ нь л үлдээнэ
// (эрх өсгөх боломжгүй). Дараалал нь хүсэлтийнхээр хадгалагдана.
func (c OAuthClient) FilterAllowedScopes(requested []string) []string {
	out := make([]string, 0, len(requested))
	seen := make(map[string]bool, len(requested))
	for _, s := range requested {
		if s == "" || seen[s] || !c.AllowsScope(s) {
			continue
		}
		seen[s] = true
		out = append(out, s)
	}
	return out
}

// MatchRedirectURI нь redirect_uri-г бүртгэгдсэн жагсаалттай ЯГ (яг тэмдэгт
// бүрээр) тулгана.
//
// АЮУЛГҮЙ БАЙДАЛ: энд prefix/substring/wildcard тулгалт ХЭЗЭЭ Ч хийж болохгүй.
// Сул тулгалт нь authorization code-ыг халдагчийн хаяг руу дамжуулах сонгодог
// эмзэг байдал (open redirect → code хулгай). RFC 6749 §3.1.2.3, RFC 9700 §2.1.
func (c OAuthClient) MatchRedirectURI(uri string) bool {
	return containsExact(c.RedirectURIs, uri)
}

// MatchPostLogoutRedirectURI нь logout-ийн дараах буцах хаягийг ЯГ тулгана
// (OIDC RP-Initiated Logout §3). Тулгалтын шалтгаан нь MatchRedirectURI-тай ижил.
func (c OAuthClient) MatchPostLogoutRedirectURI(uri string) bool {
	return containsExact(c.PostLogoutRedirectURIs, uri)
}

// containsExact нь яг тэнцүү мөр байгаа эсэхийг шалгана.
func containsExact(list []string, v string) bool {
	for _, s := range list {
		if s == v {
			return true
		}
	}
	return false
}

// OAuthChallenge нь authorize урсгалын түр төлөв: RP-ийн анхны хүсэлтийн
// параметрүүдийг хадгалж, нэвтрэх/зөвшөөрөх UI-аас буцаж ирэхэд сэргээнэ.
//
// Нэг удаагийн: шийдэгдмэгц (DecidedAt) дахин ашиглагдахгүй.
type OAuthChallenge struct {
	Challenge             string
	Kind                  string // login | consent | logout
	ClientID              string
	Subject               string
	RequestedScopes       []string
	GrantedScopes         []string
	RedirectURI           string
	State                 string
	Nonce                 string
	ResponseType          string
	CodeChallenge         string
	CodeChallengeMethod   string
	Prompt                string
	PostLogoutRedirectURI string
	Skip                  bool
	DecidedAt             *time.Time
	ExpiresAt             time.Time
	CreatedAt             time.Time
}

// Challenge-ийн төрлүүд.
const (
	ChallengeLogin   = "login"
	ChallengeConsent = "consent"
	ChallengeLogout  = "logout"
)

// OAuthAuthCode нь authorization code-ийн хадгалагдсан хэлбэр. CodeHash нь
// sha256(code) — түүхий code хэзээ ч хадгалагдахгүй.
type OAuthAuthCode struct {
	CodeHash            []byte
	ClientID            string
	Subject             string
	Scopes              []string
	RedirectURI         string
	Nonce               string
	CodeChallenge       string
	CodeChallengeMethod string
	AuthTime            time.Time
	ExpiresAt           time.Time
}

// OAuthAccessToken нь гаргасан access token (opaque). TokenHash нь sha256.
// Subject нь client_credentials grant-д хоосон (хэрэглэгчгүй).
type OAuthAccessToken struct {
	TokenHash     []byte
	ClientID      string
	Subject       string
	Scopes        []string
	RefreshFamily string
	ExpiresAt     time.Time
}

// OAuthRefreshToken нь эргэлттэй refresh token. FamilyID нь эргэлтийн бүх үеийг
// нэгтгэдэг тул хулгайлагдсаныг илрүүлэхэд бүлгээр нь цуцлах боломж өгнө.
type OAuthRefreshToken struct {
	TokenHash   []byte
	FamilyID    string
	RotatedFrom []byte
	ClientID    string
	Subject     string
	Scopes      []string
	Nonce       string
	AuthTime    time.Time
	ExpiresAt   time.Time
}
