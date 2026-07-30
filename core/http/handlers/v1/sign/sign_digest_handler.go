// Gerege Systems Development Team болон Claude AI хамтран бүтээв, 2026.

// Гар утасны түрийвчний апп-д зориулсан hash-signing endpoint-ууд.
//
// PDF-ийн урсгалаас (Init/Poll/Download) ялгаатай нь баримт байхгүй: апп нь
// шилжүүлгийн канон агуулгын SHA-256 хэшийг илгээж, иргэн утсан дээрээ
// display_text-ийг хараад PIN2-оор баталдаг.
//
// Хариулт нь ДУГТУЙГҮЙ (flat) JSON — апп-ууд ингэж хүлээдэг
// (v1.NewRawResponse-ийн тайлбарыг үз).
package sign

import (
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/gerege-systems/public-gerege-core/core/business/usecases/users"
	httpauth "github.com/gerege-systems/public-gerege-core/core/http/auth"
	v1 "github.com/gerege-systems/public-gerege-core/core/http/handlers/v1"
	"github.com/gerege-systems/public-gerege-core/pkg/validators"
)

type digestInitRequest struct {
	DocumentName    string `json:"document_name"    validate:"max=160"`
	DocumentHashHex string `json:"document_hash_hex" validate:"required,len=64,hexadecimal"`
	DisplayText     string `json:"display_text"     validate:"max=160"`
}

type digestInitResponse struct {
	SID              string `json:"sid"`
	State            string `json:"state"`
	VerificationCode string `json:"verification_code"`
	// SessionSecret нь Smart-ID v3 device-link-ийн нууц. Одоогийн eID Mongolia
	// backend үүнийг буцаадаггүй тул хоосон явна — апп үүнийг хүлээж авдаг
	// (хоосон бол гар аргаар апп сэлгэх горим руу шилжинэ).
	SessionSecret string `json:"session_secret,omitempty"`
}

type digestStatusResponse struct {
	SID   string `json:"sid"`
	State string `json:"state"`
}

// InitiateDigest godoc
// @Summary      Хэшид гарын үсэг эхлүүлэх (гар утасны түрийвч)
// @Description  Шилжүүлгийн канон агуулгын SHA-256 хэшид eID PIN2 гарын үсэг эхлүүлнэ. Иргэн утсан дээрээ display_text-ийг хараад баталдаг тул түүнд бодит дүн, хүлээн авагч тусгагдсан байх ёстой.
// @Tags         sign
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        payload  body  object  true  "document_hash_hex + display_text (+ document_name)"
// @Success      200  {object}  map[string]interface{}  "sid + verification_code"
// @Failure      400  {object}  v1.BaseResponse  "хэш формат буруу / регистр олдсонгүй"
// @Failure      401  {object}  v1.BaseResponse  "unauthorized"
// @Router       /v1/sign/initiate [post]
func (h Handler) InitiateDigest(w http.ResponseWriter, r *http.Request) error {
	cu, err := httpauth.CurrentUserFromContext(r)
	if err != nil {
		return v1.NewErrorResponse(w, r, http.StatusUnauthorized, "unauthorized")
	}
	ures, err := h.users.GetByID(r.Context(), users.GetByIDRequest{ID: cu.ID})
	if err != nil {
		return v1.RespondWithError(w, r, err)
	}
	regNo, err := h.currentRegNo(r)
	if err != nil {
		return v1.RespondWithError(w, r, err)
	}

	var req digestInitRequest
	if err := v1.DecodeBody(r, &req); err != nil {
		return v1.NewErrorResponse(w, r, http.StatusBadRequest, "invalid request body")
	}
	if err := validators.ValidatePayloads(req); err != nil {
		return v1.RespondWithError(w, r, err)
	}

	// document_name нь ХҮСЭЛТЭД аль хэдийн байсан ч хаана ч дамждаггүй байв —
	// RP илгээсэн ч verify хуудсанд хүрдэггүй байсны шалтгаан яг энэ.
	result, err := h.sign.InitDigest(r.Context(), regNo, ures.User.FullName(),
		req.DocumentHashHex, req.DisplayText, req.DocumentName)
	if err != nil {
		return v1.RespondWithError(w, r, err)
	}
	return v1.NewRawResponse(w, http.StatusOK, digestInitResponse{
		SID:              result.SessionID,
		State:            "RUNNING",
		VerificationCode: result.VerificationCode,
	})
}

// DigestStatus godoc
// @Summary      Гарын үсгийн session-ы төлөв (гар утасны түрийвч)
// @Tags         sign
// @Produce      json
// @Security     BearerAuth
// @Param        sid  path  string  true  "Session ID"
// @Success      200  {object}  map[string]interface{}  "sid + state"
// @Failure      404  {object}  v1.BaseResponse  "session олдсонгүй"
// @Router       /v1/sign/status/{sid} [get]
func (h Handler) DigestStatus(w http.ResponseWriter, r *http.Request) error {
	regNo, err := h.currentRegNo(r)
	if err != nil {
		return v1.RespondWithError(w, r, err)
	}
	sid := chi.URLParam(r, "sid")
	state, err := h.sign.Poll(r.Context(), regNo, sid)
	if err != nil {
		return v1.RespondWithError(w, r, err)
	}
	return v1.NewRawResponse(w, http.StatusOK, digestStatusResponse{
		SID:   sid,
		State: toAppState(state),
	})
}

// toAppState нь sign session-ы дотоод төлвийг гар утасны апп-ын хүлээдэг
// үгсийн сан руу буулгана.
//
// Апп нь CONFIRMED / REFUSED / TIMEOUT гурвыг л терминал гэж үздэг; бусдыг
// "үргэлжлүүлж асуу" гэж ойлгоод хугацаа дуустал давтдаг. Иймд "failed"-ыг
// TIMEOUT рүү буулгана — эс бөгөөс гарын үсэг техникийн шалтгаанаар унасан
// үед иргэн 3 минут хүлээж байж л алдаа хардаг.
func toAppState(state string) string {
	switch state {
	case "completed":
		return "CONFIRMED"
	case "rejected":
		return "REFUSED"
	case "expired", "failed":
		return "TIMEOUT"
	default:
		return "RUNNING"
	}
}
