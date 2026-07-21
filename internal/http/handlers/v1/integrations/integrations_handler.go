// Government Template Platform V3.0
// Gerege Systems Development Team болон Claude AI хамтран бүтээв, 2026.

// Package integrations нь хэрэглэгчийн гуравдагч этгээдийн интеграцийн (Google
// Drive/Meet, Dropbox) токеныг удирдах HTTP handler-уудыг агуулна. Бүх endpoint
// auth-шаардлагатай — хэрэглэгч зөвхөн өөрийн холболтыг удирдана.
package integrations

import integrationsuc "template/internal/business/usecases/integrations"

type Handler struct {
	usecase integrationsuc.Usecase
}

func NewHandler(usecase integrationsuc.Usecase) Handler {
	return Handler{usecase: usecase}
}
