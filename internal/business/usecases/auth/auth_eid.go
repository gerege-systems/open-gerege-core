// Gerege Template Version 27.0
// Gerege Systems Development Team болон Claude AI хамтран бүтээв, 2026.

package auth

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"

	"template/internal/apperror"
	"template/internal/business/domain"
	"template/internal/business/usecases/users"
	"template/pkg/eid"
	"template/pkg/logger"
)

// eidPollTimeoutMs нь IdP-ийн session long-poll-ийн хүлээх дээд хугацаа (мс).
// eid client-ийн HTTP timeout (30с) үүнээс урт тул сүлжээ дуусахаас өмнө IdP
// хариу буцаах зайтай.
const eidPollTimeoutMs = 25000

// EIDStart нь eID QR/deep-link нэвтрэлтийг IdP дээр эхлүүлнэ. Config-оос
// display text + callback URL, crypto/rand-аар nonce ашиглана.
func (uc *usecase) EIDStart(ctx context.Context) (resp EIDStartResponse, err error) {
	const (
		usecaseName = "auth"
		funcName    = "EIDStart"
		fileName    = "auth_eid.go"
	)
	startTime := time.Now()

	logger.InfoWithContext(ctx, fmt.Sprintf("Upper %s", funcName), logger.Fields{
		"usecase": usecaseName, "method": funcName, "file": fileName,
	})
	defer func() {
		fields := logger.Fields{
			"usecase": usecaseName, "method": funcName, "file": fileName,
			"duration": time.Since(startTime).Milliseconds(),
		}
		if err == nil {
			fields["response"] = logger.Fields{"session_id": resp.SessionID}
		}
		logger.InfoWithContext(ctx, fmt.Sprintf("Lower %s", funcName), fields)
	}()

	nonce, nonceErr := randomNonce()
	if nonceErr != nil {
		err = apperror.InternalCause(fmt.Errorf("generate nonce: %w", nonceErr))
		logger.ErrorWithContext(ctx, "EIDStart failed: nonce generation error", logger.Fields{
			"usecase": usecaseName, "method": funcName, "file": fileName,
			"step": "random_nonce", "error": nonceErr.Error(),
		})
		return EIDStartResponse{}, err
	}

	// Smart-ID v3 Web2App CROSS-DEVICE: callbackUrl-г ХООСОН дамжуулна. Desktop
	// дээрх browser нь QR-аа гар утсаар уншуулаад, иргэн eID app-д баталгаажуулсны
	// дараа device_link_url дотор callbackUrl байхгүй тул утасны browser callback
	// руу redirect хийхгүй — эх browser зүгээр /eid/poll-оор төлвийг хүлээж нэвтэрнэ.
	// EID_CALLBACK_URL config нь зөвхөн ирээдүйн same-device (tap-through) урсгалд
	// зориулагдсан тул энд ашиглахгүй.
	start, initErr := uc.eid.QRInitiate(ctx, uc.cfg.EIDDisplayText, "", nonce)
	if initErr != nil {
		err = mapInitiateErr(initErr, "eID session эхлүүлэх боломжгүй байна")
		logger.ErrorWithContext(ctx, "EIDStart failed: initiate error", logger.Fields{
			"usecase": usecaseName, "method": funcName, "file": fileName,
			"step": "eid_qr_initiate", "error": initErr.Error(),
		})
		return EIDStartResponse{}, err
	}

	resp = EIDStartResponse{
		SessionID:        start.SessionID,
		DeviceLinkURL:    start.DeviceLinkURL,
		VerificationCode: start.VerificationCode,
		ExpiresAt:        start.ExpiresAt,
	}
	return resp, nil
}

// EIDStartByNationalID нь иргэний РД (national_id)-аар нэвтрэлтийг IdP дээр
// эхлүүлнэ (gerege.mn-ийн "РД оруулах → утас руу push" урсгал). IdP нь тухайн
// РД-тэй холбоотой бүртгэлтэй төхөөрөмж(үүд) рүү баталгаажуулах prompt шууд push
// хийдэг тул device_link / QR шаардлагагүй — зөвхөн session_id, verification_code,
// expires_at буцна. Дуусгахдаа QR урсгалтай ижил EIDPoll-ийг ашиглана.
func (uc *usecase) EIDStartByNationalID(ctx context.Context, nationalID string) (resp EIDStartResponse, err error) {
	const (
		usecaseName = "auth"
		funcName    = "EIDStartByNationalID"
		fileName    = "auth_eid.go"
	)
	startTime := time.Now()

	// РД-г лог-д бичихгүй (хувийн мэдээлэл) — зөвхөн утга байгаа эсэхийг тэмдэглэнэ.
	logger.InfoWithContext(ctx, fmt.Sprintf("Upper %s", funcName), logger.Fields{
		"usecase": usecaseName, "method": funcName, "file": fileName,
		"request": logger.Fields{"has_national_id": nationalID != ""},
	})
	defer func() {
		fields := logger.Fields{
			"usecase": usecaseName, "method": funcName, "file": fileName,
			"duration": time.Since(startTime).Milliseconds(),
		}
		if err == nil {
			fields["response"] = logger.Fields{"session_id": resp.SessionID}
		}
		logger.InfoWithContext(ctx, fmt.Sprintf("Lower %s", funcName), fields)
	}()

	nationalID = strings.TrimSpace(nationalID)
	if nationalID == "" {
		err = apperror.BadRequest("national_id is required")
		return EIDStartResponse{}, err
	}

	nonce, nonceErr := randomNonce()
	if nonceErr != nil {
		err = apperror.InternalCause(fmt.Errorf("generate nonce: %w", nonceErr))
		logger.ErrorWithContext(ctx, "EIDStartByNationalID failed: nonce generation error", logger.Fields{
			"usecase": usecaseName, "method": funcName, "file": fileName,
			"step": "random_nonce", "error": nonceErr.Error(),
		})
		return EIDStartResponse{}, err
	}

	start, initErr := uc.eid.Initiate(ctx, nationalID, uc.cfg.EIDDisplayText, nonce)
	if initErr != nil {
		err = mapInitiateErr(initErr, "Регистрийн дугаар олдсонгүй эсвэл буруу байна")
		logger.ErrorWithContext(ctx, "EIDStartByNationalID failed: initiate error", logger.Fields{
			"usecase": usecaseName, "method": funcName, "file": fileName,
			"step": "eid_initiate", "error": initErr.Error(),
		})
		return EIDStartResponse{}, err
	}

	// Push урсгалд device_link шаардлагагүй тул орхино.
	resp = EIDStartResponse{
		SessionID:        start.SessionID,
		VerificationCode: start.VerificationCode,
		ExpiresAt:        start.ExpiresAt,
	}
	return resp, nil
}

// EIDPoll нь session төлвийг IdP-ээс long-poll-оор асууна. COMPLETE болоход
// identity-аар хэрэглэгчийг upsert хийж, токен хос олгоно. RUNNING/EXPIRED/
// REFUSED үед зөвхөн State буцаана (handler цэвэр мессеж рүү буулгана).
func (uc *usecase) EIDPoll(ctx context.Context, req EIDPollRequest) (resp EIDPollResponse, err error) {
	const (
		usecaseName = "auth"
		funcName    = "EIDPoll"
		fileName    = "auth_eid.go"
	)
	startTime := time.Now()

	logger.InfoWithContext(ctx, fmt.Sprintf("Upper %s", funcName), logger.Fields{
		"usecase": usecaseName, "method": funcName, "file": fileName,
		"request": logger.Fields{"has_session_id": req.SessionID != ""},
	})
	defer func() {
		fields := logger.Fields{
			"usecase": usecaseName, "method": funcName, "file": fileName,
			"duration": time.Since(startTime).Milliseconds(),
		}
		if err == nil {
			fields["response"] = logger.Fields{"state": resp.State, "user_id": resp.User.ID}
		}
		logger.InfoWithContext(ctx, fmt.Sprintf("Lower %s", funcName), fields)
	}()

	if req.SessionID == "" {
		err = apperror.BadRequest("session_id is required")
		return EIDPollResponse{}, err
	}

	sess, pollErr := uc.eid.Session(ctx, req.SessionID, eidPollTimeoutMs)
	if pollErr != nil {
		err = apperror.InternalCause(fmt.Errorf("eid session: %w", pollErr))
		logger.ErrorWithContext(ctx, "EIDPoll failed: session error", logger.Fields{
			"usecase": usecaseName, "method": funcName, "file": fileName,
			"step": "eid_session", "error": pollErr.Error(),
		})
		return EIDPollResponse{}, err
	}

	// Terminal биш (RUNNING г.м.) болон terminal-fail (EXPIRED/REFUSED) үед
	// зөвхөн төлвийг буцаана — клиент дахин асуух эсвэл мессеж харуулна.
	if sess.State != eid.StateComplete {
		return EIDPollResponse{State: sess.State}, nil
	}

	// Subject нь хэрэглэгчийн давтагдашгүй түлхүүр. Public RP (энэ template)-д IdP
	// нь national_id-г илчлэхгүй, зөвхөн civil_id өгдөг тул civil_id-г түлхүүр
	// болгоно; эрх бүхий RP-ийн ховор тохиолдолд national_id руу fallback хийнэ.
	// Хоёулаа хоосон үед л identity дутуу гэж үзэж татгалзана. РД/civil_id-г лог-д
	// бичихгүй — зөвхөн identity байгаа эсэхийг (boolean) тэмдэглэнэ.
	var subject string
	if sess.Identity != nil {
		subject = sess.Identity.CivilID
		if subject == "" {
			subject = sess.Identity.NationalID
		}
	}
	if subject == "" {
		err = apperror.InternalCause(fmt.Errorf("eid complete without identity"))
		logger.ErrorWithContext(ctx, "EIDPoll failed: complete without identity", logger.Fields{
			"usecase": usecaseName, "method": funcName, "file": fileName,
			"step": "check_identity", "has_identity": sess.Identity != nil,
		})
		return EIDPollResponse{}, err
	}

	// АНХААР: IdP нь TLS-ээр хамгаалагдсан, эрх бүхий эх сурвалж тул COMPLETE
	// хариунд итгэнэ. Ирээдүйн сонголттой сайжруулалт: sess.signature-ийг
	// sess.certificate-ийн эсрэг шалгах (одоогоор хатуу татгалздаггүй).
	// Түлхүүр болгож subject (civil_id, эс бөгөөс national_id)-г дамжуулна.
	id := sess.Identity
	newUser, buildErr := domain.NewEIDUser(
		subject, id.GivenName, id.Surname, id.GivenNameEn, id.SurnameEn, id.NationalID, id.KYCLevel,
	)
	if buildErr != nil {
		err = apperror.InternalCause(fmt.Errorf("build eid user: %w", buildErr))
		logger.ErrorWithContext(ctx, "EIDPoll failed: build user error", logger.Fields{
			"usecase": usecaseName, "method": funcName, "file": fileName,
			"step": "domain_new_eid_user", "error": buildErr.Error(),
		})
		return EIDPollResponse{}, err
	}
	// login COMPLETE-ийн cert.value-ээс задалсан сертификатын дэлгэрэнгүйг
	// (+ documentNumber) хэрэглэгчид хадгална — Profile хуудсанд харуулна.
	newUser.DocumentNumber = id.DocumentNumber
	if id.Certificate != nil {
		newUser.CertSerial = id.Certificate.Serial
		newUser.CertNotBefore = &id.Certificate.NotBefore
		newUser.CertNotAfter = &id.Certificate.NotAfter
		newUser.CertIssuer = id.Certificate.Issuer
		newUser.CertKeyType = id.Certificate.KeyType
	}

	upserted, upsertErr := uc.users.UpsertFromEID(ctx, users.UpsertFromEIDRequest{User: newUser})
	if upsertErr != nil {
		err = upsertErr
		logger.ErrorWithContext(ctx, "EIDPoll failed: upsert user error", logger.Fields{
			"usecase": usecaseName, "method": funcName, "file": fileName,
			"step": "users_upsert_from_eid", "error": upsertErr.Error(),
		})
		return EIDPollResponse{}, err
	}
	user := upserted.User

	pair, mintErr := uc.jwtService.GenerateTokenPair(user.ID, user.IsAdmin(), user.RoleID, user.Email)
	if mintErr != nil {
		err = apperror.InternalCause(fmt.Errorf("generate token: %w", mintErr))
		logger.ErrorWithContext(ctx, "EIDPoll failed: token generation error", logger.Fields{
			"usecase": usecaseName, "method": funcName, "file": fileName,
			"step": "generate_token_pair", "error": mintErr.Error(), "user_id": user.ID,
		})
		return EIDPollResponse{}, err
	}

	if persistErr := uc.rememberRefresh(ctx, pair); persistErr != nil {
		err = apperror.InternalCause(fmt.Errorf("persist refresh: %w", persistErr))
		logger.ErrorWithContext(ctx, "EIDPoll failed: persist refresh error", logger.Fields{
			"usecase": usecaseName, "method": funcName, "file": fileName,
			"step": "persist_refresh", "error": persistErr.Error(), "user_id": user.ID,
		})
		return EIDPollResponse{}, err
	}

	resp = EIDPollResponse{
		State:        eid.StateComplete,
		User:         user,
		AccessToken:  pair.AccessToken,
		RefreshToken: pair.RefreshToken,
	}
	return resp, nil
}

// mapInitiateErr нь eID initiate-ийн алдааг HTTP статус руу буулгана: IdP-ийн
// 4xx (РД олдсонгүй / scope / формат) бол цэвэр BadRequest (clientMsg),
// бусад (сүлжээ / 5xx) бол дотоод 5xx алдаа.
func mapInitiateErr(initErr error, clientMsg string) error {
	if errors.Is(initErr, eid.ErrInitiateRejected) {
		return apperror.BadRequest(clientMsg)
	}
	return apperror.InternalCause(fmt.Errorf("eid initiate: %w", initErr))
}

// randomNonce нь IdP-ийн replay-аас хамгаалах 32 hex тэмдэгтийн (16 байт)
// crypto/rand nonce үүсгэнэ.
func randomNonce() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}
