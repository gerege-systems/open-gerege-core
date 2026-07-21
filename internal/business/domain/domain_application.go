// Government Template Platform V3.0
// Gerege Systems Development Team болон Claude AI хамтран бүтээв, 2026.

package domain

import "time"

// Application нь API Gateway consumer + SSO RP (developer_apps)-ыг нэгтгэсэн
// бүртгэл. Мөр бүр нэг OAuth2 client-тэй (oauth_clients) тохирно: web/spa/native нь
// authorization_code (DAN-аар нэвтрэх RP), m2m нь client_credentials (API дуудагч).
// Аппад ямар gateway service ашиглаж болохыг ServiceIDs (application_services →
// scope) заана. Secret нь зөвхөн үүсгэх/эргүүлэх үед НЭГ удаа дүүрнэ (DB-д
// хадгалагдахгүй — зөвхөн hash хадгалагдана, pkg/secrethash).
type Application struct {
	ID           string
	ClientID     string // OAuth2 client_id
	Name         string
	AppType      string // web | spa | native | m2m
	Tags         []string
	RedirectURIs []string
	Enabled      bool
	CreatedBy    string
	ServiceIDs   []string // зөвшөөрсөн gateway service-ийн id-ууд (join-оор уншина)
	Secret       string   // зөвхөн create/rotate хариунд дүүрнэ; хадгалагдахгүй
	CreatedAt    time.Time
	UpdatedAt    *time.Time
}

// AppTypes нь зөвшөөрөгдсөн апп төрлүүд. web/spa/native = authorization_code RP,
// m2m = client_credentials (API-to-API).
var AppTypes = map[string]bool{"web": true, "spa": true, "native": true, "m2m": true}

// AppUsesRedirect нь тухайн төрөл redirect_uri шаарддаг эсэхийг (authorization_code).
func AppUsesRedirect(appType string) bool {
	return appType == "web" || appType == "spa" || appType == "native"
}

// AppIsPublic нь тухайн төрөл public (PKCE, secret-гүй) client эсэхийг заана.
// spa/native нь browser/төхөөрөмжид ажилладаг тул secret нууцлаж чадахгүй.
func AppIsPublic(appType string) bool {
	return appType == "spa" || appType == "native"
}
