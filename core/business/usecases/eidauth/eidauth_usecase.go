// Gerege Systems Development Team болон Claude AI хамтран бүтээв, 2026.

// Package eidauth нь бүртгэлтэй апп (relying party)-д ЗОРИУЛСАН eID НЭВТРЭЛТИЙН
// proxy-ийн usecase. SSO нь өөрийн eidmongolia.mn RP креденшлээр eID session
// эхлүүлж, төлвийг нь дамжуулж өгдөг тул апп өөрөө RP креденшл эзэмших
// шаардлагагүй болно.
//
// ЯЛГАА (usecases/auth-аас): энд хэрэглэгч ҮҮСГЭХГҮЙ, session ОЛГОХГҮЙ —
// зөвхөн eID-ийн түүхий үр дүнг (төлөв + баталгаажсан identity) буцаана.
// Дуудагч апп өөрийн бодлогоор (нэвтрүүлэх, бүртгэл баталгаажуулах г.м.)
// шийднэ. Ингэснээр энэ proxy нь SSO-ийн хэрэглэгчийн санг хөндөхгүй.
package eidauth

import (
	"context"
)

// StartRequest — QR/device-link-ээр эхлүүлэх. CallbackURL хоосон бол
// CROSS-DEVICE (desktop QR); хоосон биш бол SAME-DEVICE (утасны browser).
type StartRequest struct {
	CallbackURL string
}

// StartByNationalIDRequest — иргэний РД-аар (бүртгэлтэй төхөөрөмж рүү push).
type StartByNationalIDRequest struct {
	NationalID  string
	CallbackURL string
}

// PollRequest — session-ийн төлөв асуух.
type PollRequest struct {
	SessionID string
}

// StartResponse — eID session эхлүүлэлтийн үр дүн.
type StartResponse struct {
	SessionID        string
	DeviceLinkURL    string
	VerificationCode string
	ExpiresAt        string
}

// Identity — eID-ээр баталгаажсан иргэний мэдээлэл (зөвхөн COMPLETE үед).
type Identity struct {
	CivilID        string
	NationalID     string
	GivenName      string
	Surname        string
	GivenNameEn    string
	SurnameEn      string
	FullName       string
	KYCLevel       string
	DocumentNumber string
}

// PollResponse — session-ийн төлөв; Identity нь зөвхөн COMPLETE үед дүүрэн.
type PollResponse struct {
	State    string
	Identity *Identity
}

// Usecase нь RP-д нээлттэй eID нэвтрэлтийн гурван үйлдэл.
type Usecase interface {
	Start(ctx context.Context, req StartRequest) (StartResponse, error)
	StartByNationalID(ctx context.Context, req StartByNationalIDRequest) (StartResponse, error)
	Poll(ctx context.Context, req PollRequest) (PollResponse, error)
}
