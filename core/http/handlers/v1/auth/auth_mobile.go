// Gerege Systems Development Team болон Claude AI хамтран бүтээв, 2026.

// Гар утасны түрийвчний апп-д зориулсан нэвтрэлтийн endpoint-ууд.
//
// Вэб BFF-ийн урсгал (/eid/start, /eid/poll) нь QR / device-link дээр
// тулгуурладаг бол апп нь регистрийн дугаараар эхлүүлж (утас руу push очно),
// дараа нь нэг session ID-г тогтмол асуудаг:
//
//	POST /api/v1/auth/initiate     {national_id}   → sid + verification_code
//	GET  /api/v1/auth/status/{sid}                 → RUNNING … COMPLETE + токен
//
// COMPLETE болох мөчид иргэний ТҮРИЙВЧ автоматаар нээгддэг (Fineract дээр
// харилцагч + данс) тул апп нэвтрэхийн зэрэгцээ IBAN-аа авна — тусад нь
// "данс нээх" алхам байхгүй.
//
// Хариулт нь ДУГТУЙГҮЙ (flat) JSON — v1.NewRawResponse-ийн тайлбарыг үз.
package auth

import (
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"

	authuc "github.com/gerege-systems/public-gerege-core/core/business/usecases/auth"
	v1 "github.com/gerege-systems/public-gerege-core/core/http/handlers/v1"
	"github.com/gerege-systems/public-gerege-core/pkg/audit"
	"github.com/gerege-systems/public-gerege-core/pkg/eid"
	"github.com/gerege-systems/public-gerege-core/pkg/logger"
	"github.com/gerege-systems/public-gerege-core/pkg/validators"

	"context"
)

// WalletProvisioner нь нэвтрэлт амжилттай болоход иргэний түрийвчийг
// нээх/олох нарийхан интерфэйс. Бүтэн wallet.Usecase-ийг auth handler руу
// чирэхгүйн тулд энд зөвхөн хэрэгтэй методыг зарлав.
// WalletAccount нь нэвтрэлтийн хариунд буцаах ХАМГИЙН БАГА талбарууд.
// Core нь түрийвчний domain-аас хамаарахгүйн тулд энд тусад нь тодорхойлов —
// апп нь өөрийн төрлөө энэ рүү хөрвүүлж өгнө.
type WalletAccount struct {
	IBAN      string
	AccountNo string
}

// WalletProvisioner нь гар утасны нэвтрэлт амжилттай болоход иргэний
// түрийвчийг нээх/олох сонголттой дэгээ. Суурь платформ түрийвчгүй ажиллах
// тул nil байж болно — тэр үед хариунд IBAN/AccountNo ирэхгүй.
type WalletProvisioner interface {
	EnsureAccount(ctx context.Context, userID string) (WalletAccount, error)
}

type mobileInitiateRequest struct {
	NationalID string `json:"national_id" validate:"required,min=6,max=32"`
}

type mobileInitiateResponse struct {
	SID              string `json:"sid"`
	State            string `json:"state"`
	VerificationCode string `json:"verification_code,omitempty"`
	ExpiresAt        string `json:"expires_at,omitempty"`
}

type mobileIdentity struct {
	NationalID string `json:"national_id"`
	FullName   string `json:"full_name"`
	KYCLevel   string `json:"kyc_level,omitempty"`
}

type mobileStatusResponse struct {
	SID      string          `json:"sid"`
	State    string          `json:"state"`
	Identity *mobileIdentity `json:"identity,omitempty"`
	UserID   string          `json:"user_id,omitempty"`
	// IBAN + AccountNo нь COMPLETE үед л ирнэ (түрийвч нээгдсэний дараа).
	IBAN             string `json:"iban,omitempty"`
	AccountNo        string `json:"account_no,omitempty"`
	AccessToken      string `json:"access_token,omitempty"`
	RefreshToken     string `json:"refresh_token,omitempty"`
	AccessExpiresAt  string `json:"access_expires_at,omitempty"`
	RefreshExpiresAt string `json:"refresh_expires_at,omitempty"`
}

// MobileInitiate godoc
// @Summary      Түрийвчний апп-аас нэвтрэлт эхлүүлэх
// @Description  Регистрийн дугаараар eID нэвтрэлт эхлүүлж, иргэний бүртгэлтэй төхөөрөмж рүү push илгээнэ. Буцаасан sid-ээр /auth/status/{sid}-г асууна.
// @Tags         auth
// @Accept       json
// @Produce      json
// @Param        payload  body  object  true  "national_id"
// @Success      200  {object}  map[string]interface{}  "sid + verification_code"
// @Failure      400  {object}  v1.BaseResponse  "РД буруу эсвэл олдсонгүй"
// @Router       /v1/auth/initiate [post]
func (h Handler) MobileInitiate(w http.ResponseWriter, r *http.Request) error {
	ctx := r.Context()
	var req mobileInitiateRequest
	if err := v1.DecodeBody(r, &req); err != nil {
		return v1.NewErrorResponse(w, r, http.StatusBadRequest, "invalid request body")
	}
	if err := validators.ValidatePayloads(req); err != nil {
		return v1.RespondWithError(w, r, err)
	}

	// callbackURL хоосон — апп нь өөрөө төлвийг асуудаг тул browser буцаах
	// шаардлагагүй. AppContext-ийг өгөгдмөлөөр (RP платформ) явуулна.
	result, err := h.usecase.EIDStartByNationalID(ctx, strings.TrimSpace(req.NationalID), "")
	if err != nil {
		return v1.RespondWithError(w, r, err)
	}
	return v1.NewRawResponse(w, http.StatusOK, mobileInitiateResponse{
		SID:              result.SessionID,
		State:            "RUNNING",
		VerificationCode: result.VerificationCode,
		ExpiresAt:        result.ExpiresAt,
	})
}

// MobileStatus godoc
// @Summary      Нэвтрэлтийн session-ы төлөв (түрийвчний апп)
// @Description  eID session-ыг long-poll хийж, COMPLETE болоход токен + иргэний танилт + түрийвчний IBAN-г буцаана. Иргэнд түрийвч байхгүй бол ЭНД автоматаар нээгдэнэ.
// @Tags         auth
// @Produce      json
// @Param        sid  path  string  true  "Session ID"
// @Success      200  {object}  map[string]interface{}  "Төлөв (+ COMPLETE үед токен)"
// @Failure      400  {object}  v1.BaseResponse  "sid буруу"
// @Router       /v1/auth/status/{sid} [get]
func (h Handler) MobileStatus(w http.ResponseWriter, r *http.Request) error {
	ctx := r.Context()
	sid := strings.TrimSpace(chi.URLParam(r, "sid"))
	if sid == "" {
		return v1.NewErrorResponse(w, r, http.StatusBadRequest, "sid is required")
	}

	result, err := h.usecase.EIDPoll(ctx, authuc.EIDPollRequest{SessionID: sid})
	if err != nil {
		return v1.RespondWithError(w, r, err)
	}

	// Терминал бус, эсвэл татгалзсан/хугацаа дууссан — зөвхөн төлвөө буцаана.
	if result.State != eid.StateComplete || result.AccessToken == "" {
		return v1.NewRawResponse(w, http.StatusOK, mobileStatusResponse{
			SID:   sid,
			State: appAuthState(result.State),
		})
	}

	user := result.User
	resp := mobileStatusResponse{
		SID:   sid,
		State: appStateConfirmed,
		Identity: &mobileIdentity{
			NationalID: user.NationalID,
			FullName:   user.FullName(),
			KYCLevel:   user.KYCLevel,
		},
		UserID:           user.ID,
		AccessToken:      result.AccessToken,
		RefreshToken:     result.RefreshToken,
		AccessExpiresAt:  formatTime(result.AccessExpiresAt),
		RefreshExpiresAt: formatTime(result.RefreshExpiresAt),
	}

	// Түрийвчийг нээх/олох. Fineract түр боломжгүй байвал НЭВТРЭЛТИЙГ
	// ЗОГСООХГҮЙ: иргэн апп руугаа орж, үлдэгдлээ дараа нь сэргээнэ. Апп нь
	// iban-г сонголттой талбар (optional) гэж задалдаг.
	if h.wallet != nil {
		account, walletErr := h.wallet.EnsureAccount(ctx, user.ID)
		if walletErr != nil {
			logger.ErrorWithContext(ctx, "auth: нэвтрэлтийн үед түрийвч нээгдсэнгүй", logger.Fields{
				"controller": "auth", "method": "MobileStatus",
				"user_id": user.ID, "error": walletErr.Error(),
			})
		} else {
			resp.IBAN = account.IBAN
			resp.AccountNo = account.AccountNo
		}
	}

	ev := auditFromRequest(r)
	ev.Type = audit.EventLoginSuccess
	ev.Success = true
	ev.UserID = user.ID
	ev.Email = user.Email
	audit.Record(ev)

	return v1.NewRawResponse(w, http.StatusOK, resp)
}

// Гар утасны апп-ын хүлээдэг төлвийн үгсийн сан.
//
// iOS/Android хоёулаа CONFIRMED / REFUSED / TIMEOUT гурвыг л терминал гэж
// үздэг (бусад бүх утгыг "үргэлжлүүлж асуу" гэж ойлгоно). Backend дотооддоо
// eID-ийн Smart-ID нэршлийг (COMPLETE / EXPIRED / …) хэрэглэдэг тул гадагшаа
// гарахдаа ЭНД буулгана — зураглал нэг газар байснаар хоёр апп-ын логик
// backend-ийн дотоод нэршилтэй хэзээ ч салахгүй.
const (
	appStateConfirmed = "CONFIRMED"
	appStateRefused   = "REFUSED"
	appStateTimeout   = "TIMEOUT"
	appStateRunning   = "RUNNING"
)

// appAuthState нь eID session-ы төлвийг апп-ын үгсийн сан руу буулгана.
func appAuthState(state string) string {
	switch state {
	case eid.StateComplete:
		return appStateConfirmed
	case eid.StateRefused:
		return appStateRefused
	case eid.StateExpired:
		return appStateTimeout
	default:
		return appStateRunning
	}
}

// formatTime нь тэг цагийг хоосон мөр болгож (omitempty ажиллана), бусдыг
// RFC3339 болгоно.
func formatTime(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.UTC().Format(time.RFC3339)
}
