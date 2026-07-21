// Government Template Platform V3.0
// Gerege Systems Development Team болон Claude AI хамтран бүтээв, 2026.

package requests

// ApplicationRequest нь Application (нэгдсэн Gateway consumer + SSO RP) үүсгэх/
// шинэчлэх админ хүсэлт. app_type нь grant/auth-method-ыг тодорхойлно
// (web/spa/native = authorization_code RP, m2m = client_credentials). redirect_uris
// нь зөвхөн RP төрөлд шаардлагатай; нарийн шалгалт usecase давхаргад.
type ApplicationRequest struct {
	Name         string   `json:"name" validate:"required,min=1,max=128"`
	AppType      string   `json:"app_type" validate:"required,oneof=web spa native m2m"`
	RedirectURIs []string `json:"redirect_uris" validate:"omitempty,dive,max=400"`
	Tags         []string `json:"tags" validate:"omitempty,dive,max=40"`
	ServiceIDs   []string `json:"service_ids" validate:"omitempty,dive,uuid"`
	Enabled      bool     `json:"enabled"`
}

// ApplicationSecretRequest нь апп-ын Hydra client_secret-ыг гараар оноох хүсэлт
// (санамсаргүй rotate биш — тодорхой утга).
type ApplicationSecretRequest struct {
	Secret string `json:"secret" validate:"required,min=16,max=128"`
}

// ApplicationServicesRequest нь апп-ын зөвшөөрсөн gateway service-үүдийг солих хүсэлт.
type ApplicationServicesRequest struct {
	ServiceIDs []string `json:"service_ids" validate:"omitempty,dive,uuid"`
}
