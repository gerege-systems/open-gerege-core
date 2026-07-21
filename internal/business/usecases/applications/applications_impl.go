// Government Template Platform V3.0
// Gerege Systems Development Team болон Claude AI хамтран бүтээв, 2026.

package applications

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"net/url"
	"strings"

	"template/internal/apperror"
	"template/internal/business/domain"
	"template/pkg/secrethash"
)

// clientStore нь OAuth2 client бүртгэлийн хадгалалт (oauth_clients хүснэгт).
// Апп бүр яг нэг client — client_id нь танигч. Өмнө нь энэ бүртгэл Hydra-д
// амьдардаг байсан бөгөөд overlay (tags/enabled/app_type)-ыг client metadata-д
// шахаж хадгалдаг байсан; одоо тэдгээр нь жинхэнэ багана.
type clientStore interface {
	List(ctx context.Context) ([]domain.OAuthClient, error)
	Get(ctx context.Context, clientID string) (domain.OAuthClient, error)
	Create(ctx context.Context, c domain.OAuthClient) (domain.OAuthClient, error)
	Update(ctx context.Context, c domain.OAuthClient) (domain.OAuthClient, error)
	SetSecretHash(ctx context.Context, clientID, hash string) error
	Delete(ctx context.Context, clientID string) error
}

// serviceScopeResolver нь gateway service id ↔ OAuth scope хооронд хөрвүүлнэ
// (gateway_services хүснэгт).
type serviceScopeResolver interface {
	ServiceScopes(ctx context.Context, serviceIDs []string) ([]string, error)
	ServiceIDsForScopes(ctx context.Context, scopes []string) ([]string, error)
}

// Гараар оноох client secret-ийн зөвшөөрөгдөх урт — сул secret-ыг хүлээж авахгүй.
const (
	minSecretLen = 16
	maxSecretLen = 128
)

type usecase struct {
	svc     serviceScopeResolver
	clients clientStore
}

// NewUsecase нь applications usecase-ыг буцаана. clients нь oauth_clients
// хадгалалт; svc нь gateway service scope resolver.
func NewUsecase(svc serviceScopeResolver, clients clientStore) Usecase {
	return &usecase{svc: svc, clients: clients}
}

func (u *usecase) List(ctx context.Context) ([]domain.Application, error) {
	clients, err := u.clients.List(ctx)
	if err != nil {
		return nil, apperror.InternalCause(fmt.Errorf("list oauth clients: %w", err))
	}
	out := make([]domain.Application, 0, len(clients))
	for i := range clients {
		app, err := u.clientToApp(ctx, clients[i])
		if err != nil {
			return nil, err
		}
		out = append(out, app)
	}
	return out, nil
}

func (u *usecase) Get(ctx context.Context, id string) (domain.Application, error) {
	c, err := u.clients.Get(ctx, id)
	if err != nil {
		return domain.Application{}, err // repo нь NotFound-ыг аль хэдийн төрөлжүүлсэн
	}
	return u.clientToApp(ctx, c)
}

func (u *usecase) Create(ctx context.Context, in Input) (domain.Application, error) {
	app, err := validate(in)
	if err != nil {
		return domain.Application{}, err
	}
	app.ClientID = "app-" + randomHex(8)
	app.ID = app.ClientID // тусдаа UUID байхгүй — client_id нь танигч.

	scopes, err := u.scopesFor(ctx, app.AppType, app.ServiceIDs)
	if err != nil {
		return domain.Application{}, err
	}

	// Public (spa/native) нь secret нууцалж чадахгүй тул огт үүсгэхгүй.
	secret, hash := "", ""
	if !domain.AppIsPublic(app.AppType) {
		secret = randomToken(40)
		if hash, err = secrethash.Hash(secret); err != nil {
			return domain.Application{}, apperror.InternalCause(fmt.Errorf("hash client secret: %w", err))
		}
	}

	client := buildClient(app, scopes)
	client.SecretHash = hash
	created, err := u.clients.Create(ctx, client)
	if err != nil {
		return domain.Application{}, err
	}

	out, err := u.clientToApp(ctx, created)
	if err != nil {
		return domain.Application{}, err
	}
	// Түүхий secret нь ЗӨВХӨН энэ хариунд, нэг удаа — хадгалагдсан нь hash.
	out.Secret = secret
	return out, nil
}

func (u *usecase) Update(ctx context.Context, id string, in Input) (domain.Application, error) {
	app, err := validate(in)
	if err != nil {
		return domain.Application{}, err
	}
	app.ID = id
	app.ClientID = id

	scopes, err := u.scopesFor(ctx, app.AppType, app.ServiceIDs)
	if err != nil {
		return domain.Application{}, err
	}
	// Update нь secret_hash-д хүрэхгүй (repo-ийн баталгаа) — rotate биш.
	updated, err := u.clients.Update(ctx, buildClient(app, scopes))
	if err != nil {
		return domain.Application{}, err
	}
	return u.clientToApp(ctx, updated)
}

func (u *usecase) Delete(ctx context.Context, id string) error {
	// Аль хэдийн байхгүй бол амжилттай гэж үзнэ (идемпотент устгалт).
	if err := u.clients.Delete(ctx, id); err != nil && !apperror.IsNotFound(err) {
		return err
	}
	return nil
}

func (u *usecase) RotateSecret(ctx context.Context, id string) (domain.Application, error) {
	return u.applySecret(ctx, id, randomToken(40))
}

func (u *usecase) SetSecret(ctx context.Context, id, secret string) (domain.Application, error) {
	secret = strings.TrimSpace(secret)
	if len(secret) < minSecretLen {
		return domain.Application{}, apperror.BadRequest(fmt.Sprintf("client secret must be at least %d characters", minSecretLen))
	}
	if len(secret) > maxSecretLen {
		return domain.Application{}, apperror.BadRequest(fmt.Sprintf("client secret too long (max %d)", maxSecretLen))
	}
	return u.applySecret(ctx, id, secret)
}

// applySecret нь confidential апп-ын client secret-ыг өгөгдсөн утгаар сольж,
// шинэ secret-ыг хариунд НЭГ удаа буцаана (rotate ба set нийтлэг зам). DB-д
// зөвхөн hash хадгалагдана.
func (u *usecase) applySecret(ctx context.Context, id, secret string) (domain.Application, error) {
	c, err := u.clients.Get(ctx, id)
	if err != nil {
		return domain.Application{}, err
	}
	app, err := u.clientToApp(ctx, c)
	if err != nil {
		return domain.Application{}, err
	}
	if domain.AppIsPublic(app.AppType) {
		return domain.Application{}, apperror.BadRequest("public client (spa/native) has no secret to rotate")
	}
	hash, err := secrethash.Hash(secret)
	if err != nil {
		return domain.Application{}, apperror.InternalCause(fmt.Errorf("hash client secret: %w", err))
	}
	if err := u.clients.SetSecretHash(ctx, id, hash); err != nil {
		return domain.Application{}, err
	}
	app.Secret = secret
	return app, nil
}

func (u *usecase) SetServices(ctx context.Context, id string, serviceIDs []string) (domain.Application, error) {
	c, err := u.clients.Get(ctx, id)
	if err != nil {
		return domain.Application{}, err
	}
	app, err := u.clientToApp(ctx, c)
	if err != nil {
		return domain.Application{}, err
	}
	app.ServiceIDs = cleanList(serviceIDs)
	scopes, err := u.scopesFor(ctx, app.AppType, app.ServiceIDs)
	if err != nil {
		return domain.Application{}, err
	}
	updated, err := u.clients.Update(ctx, buildClient(app, scopes))
	if err != nil {
		return domain.Application{}, err
	}
	return u.clientToApp(ctx, updated)
}

// scopesFor нь base OIDC scope (RP төрөлд) + зөвшөөрсөн service-үүдийн scope-г нэгтгэнэ.
func (u *usecase) scopesFor(ctx context.Context, appType string, serviceIDs []string) ([]string, error) {
	var out []string
	if domain.AppUsesRedirect(appType) {
		out = append(out, "openid", "profile", "email")
	}
	svc, err := u.svc.ServiceScopes(ctx, serviceIDs)
	if err != nil {
		return nil, apperror.InternalCause(err)
	}
	return dedup(append(out, svc...)), nil
}

// clientToApp нь хадгалагдсан OAuth2 client-ыг админд харагдах домэйн
// Application болгоно. Service id-уудыг svc:* scope-оос сэргээнэ.
func (u *usecase) clientToApp(ctx context.Context, c domain.OAuthClient) (domain.Application, error) {
	serviceIDs, err := u.svc.ServiceIDsForScopes(ctx, filterSvcScopes(c.Scopes))
	if err != nil {
		return domain.Application{}, apperror.InternalCause(err)
	}
	return domain.Application{
		ID:           c.ClientID,
		ClientID:     c.ClientID,
		Name:         c.ClientName,
		AppType:      c.AppType,
		Tags:         c.Tags,
		RedirectURIs: c.RedirectURIs,
		Enabled:      c.Enabled,
		CreatedBy:    c.CreatedBy,
		ServiceIDs:   cleanList(serviceIDs),
		CreatedAt:    c.CreatedAt,
		UpdatedAt:    c.UpdatedAt,
	}, nil
}

// buildClient нь домэйн апп + шийдэгдсэн scope-оос хадгалах client мөрийг
// угсарна. SecretHash-ыг ЭНД тавихгүй — Create нь өөрөө нэмнэ, Update нь
// огт хүрэхгүй.
func buildClient(app domain.Application, scopes []string) domain.OAuthClient {
	grants, responseTypes, authMethod := grantsFor(app.AppType)
	c := domain.OAuthClient{
		ClientID:                app.ClientID,
		ClientName:              app.Name,
		TokenEndpointAuthMethod: authMethod,
		AppType:                 app.AppType,
		GrantTypes:              grants,
		ResponseTypes:           responseTypes,
		Scopes:                  scopes,
		Tags:                    arrStr(app.Tags),
		Enabled:                 app.Enabled,
		CreatedBy:               app.CreatedBy,
	}
	if domain.AppUsesRedirect(app.AppType) {
		c.RedirectURIs = app.RedirectURIs
		c.PostLogoutRedirectURIs = postLogoutFromRedirects(app.RedirectURIs)
	}
	return c
}

// postLogoutFromRedirects нь redirect_uri бүрийн гарал үүслээс (scheme://host/)
// logout-ийн дараах буцах хаягийг гаргана. RP-үүд ихэвчлэн үндсэн хаяг руугаа
// буцдаг ба бүртгэгдээгүй бол end-session endpoint 400 өгдөг.
func postLogoutFromRedirects(redirects []string) []string {
	out := make([]string, 0, len(redirects))
	seen := map[string]bool{}
	for _, raw := range redirects {
		u, err := url.Parse(raw)
		if err != nil || u.Scheme == "" || u.Host == "" {
			continue // native private-use scheme (myapp://) — origin гэж үзэхгүй
		}
		origin := u.Scheme + "://" + u.Host + "/"
		if !seen[origin] {
			seen[origin] = true
			out = append(out, origin)
		}
	}
	return out
}

// filterSvcScopes нь scope-уудаас зөвхөн gateway service scope-уудыг (svc:*) авна.
func filterSvcScopes(scopes []string) []string {
	var out []string
	for _, s := range scopes {
		if strings.HasPrefix(s, "svc:") {
			out = append(out, s)
		}
	}
	return out
}

// grantsFor нь апп төрлөөр grant_types / response_types / auth method-ыг өгнө.
func grantsFor(appType string) (grants, responseTypes []string, authMethod string) {
	switch appType {
	case "m2m":
		return []string{"client_credentials"}, []string{}, "client_secret_basic"
	case "spa", "native":
		return []string{"authorization_code", "refresh_token"}, []string{"code"}, "none"
	default: // web
		return []string{"authorization_code", "refresh_token"}, []string{"code"}, "client_secret_basic"
	}
}

// validate нь Input-ыг шалгаж нормчилсон домэйн апп болгоно.
func validate(in Input) (domain.Application, error) {
	name := strings.TrimSpace(in.Name)
	if name == "" {
		return domain.Application{}, apperror.BadRequest("application name is required")
	}
	if len(name) > 128 {
		return domain.Application{}, apperror.BadRequest("application name too long (max 128)")
	}
	appType := strings.TrimSpace(in.AppType)
	if appType == "" {
		appType = "m2m"
	}
	if !domain.AppTypes[appType] {
		return domain.Application{}, apperror.BadRequest("app_type must be web, spa, native or m2m")
	}

	redirects := cleanList(in.RedirectURIs)
	if domain.AppUsesRedirect(appType) {
		if len(redirects) == 0 {
			return domain.Application{}, apperror.BadRequest("at least one redirect_uri is required for this app type")
		}
		for _, u := range redirects {
			if err := validateRedirectURI(u, appType == "native"); err != nil {
				return domain.Application{}, apperror.BadRequest(err.Error())
			}
		}
	} else {
		redirects = nil // m2m нь redirect ашиглахгүй
	}

	return domain.Application{
		Name:         name,
		AppType:      appType,
		Tags:         cleanList(in.Tags),
		RedirectURIs: redirects,
		Enabled:      in.Enabled,
		ServiceIDs:   cleanList(in.ServiceIDs),
	}, nil
}

func validateRedirectURI(raw string, allowPrivateScheme bool) error {
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
	switch u.Scheme {
	case "https":
		return nil
	case "http":
		host := u.Hostname()
		if host != "localhost" && host != "127.0.0.1" && host != "::1" {
			return errors.New("redirect_uri: http only allowed on loopback")
		}
		return nil
	default:
		if allowPrivateScheme && u.Scheme != "" {
			return nil // native (RFC 8252) private-use scheme, e.g. geregetemp://oauth2/callback
		}
		return errors.New("redirect_uri: scheme must be https (or http on loopback)")
	}
}

// ── helpers ──────────────────────────────────────────────────────────────—

func randomHex(n int) string {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		panic("applications: crypto/rand failed: " + err.Error())
	}
	return hex.EncodeToString(b)
}

func randomToken(n int) string {
	const alphabet = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789"
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		panic("applications: crypto/rand failed: " + err.Error())
	}
	for i := range b {
		b[i] = alphabet[int(b[i])%len(alphabet)]
	}
	return string(b)
}

// cleanList нь trim + хоосон/давхардсаныг хасна (дараалал хадгална).
func cleanList(in []string) []string {
	seen := map[string]bool{}
	out := make([]string, 0, len(in))
	for _, s := range in {
		s = strings.TrimSpace(s)
		if s == "" || seen[s] {
			continue
		}
		seen[s] = true
		out = append(out, s)
	}
	return out
}

func dedup(in []string) []string { return cleanList(in) }

func arrStr(in []string) []string {
	if in == nil {
		return []string{}
	}
	return in
}
