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
	"template/pkg/xyp"
)

// eidPollTimeoutMs нь IdP-ийн session long-poll-ийн хүлээх дээд хугацаа (мс).
// eid client-ийн HTTP timeout (30с) үүнээс урт тул сүлжээ дуусахаас өмнө IdP
// хариу буцаах зайтай.
const eidPollTimeoutMs = 25000

// EIDStart нь eID device-link нэвтрэлтийг IdP дээр эхлүүлнэ. callbackURL хоосон бол CROSS-DEVICE
// (desktop QR — browser өөрөө poll хийнэ); хоосон биш бол SAME-DEVICE (mobile browser App2App —
// утас approve-ийн дараа browser-ийг тэр URL руу буцаана). callbackURL нь frontend-ээс ирсэн
// <origin>/auth/eid/callback байх ба eID backend түүнийг стандарт зам руу force-normalize хийнэ.
func (uc *usecase) EIDStart(ctx context.Context, callbackURL string) (resp EIDStartResponse, err error) {
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

	// callbackURL хоосон (CROSS-DEVICE, desktop QR): eID backend утас руу callback дамжуулахгүй,
	// desktop browser device_link/QR-аа уншуулаад /eid/poll-оор нэвтэрнэ. callbackURL хоосон биш
	// (SAME-DEVICE, mobile browser App2App): утас approve хийсний дараа browser-ийг callback руу
	// буцаана. eID backend callbackURL-ийг стандарт зам (/auth/eid/callback) руу force-normalize хийнэ.
	start, initErr := uc.eid.QRInitiate(ctx, uc.cfg.EIDDisplayText, callbackURL, nonce)
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
// EIDStartByNationalID — РД-аар push нэвтрэлт. callbackURL хоосон бол CROSS-DEVICE (desktop +
// утас руу push); хоосон биш бол SAME-DEVICE (утасны browser — push ижил утас руу, approve-ийн
// дараа browser callback руу буцна). callbackURL нь frontend-ээс ирсэн <origin>/auth/eid/callback.
func (uc *usecase) EIDStartByNationalID(ctx context.Context, nationalID, callbackURL string) (resp EIDStartResponse, err error) {
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

	start, initErr := uc.eid.Initiate(ctx, nationalID, uc.cfg.EIDDisplayText, callbackURL)
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

	// Google-ээр эхний удаа нэвтэрч, eID-ээр баталгаажуулж байгаа бол тухайн
	// Google account-ийг энэ бодит хүнд холбоно (non-fatal).
	uc.linkGoogleIfPending(ctx, user.ID, req.GoogleLinkToken)

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

// eidPersonEtsi нь userID-аар хэрэглэгчийг олж, ETSI танигч
// (PNOMN-<civil_id>, томоор) буцаана. eID хэрэглэгч биш (civil_id хоосон) бол
// эхний утга "" (алдаагүй).
func (uc *usecase) eidPersonEtsi(ctx context.Context, userID string) (string, error) {
	got, err := uc.users.GetByID(ctx, users.GetByIDRequest{ID: userID})
	if err != nil {
		return "", err
	}
	civilID := strings.TrimSpace(got.User.CivilID)
	if civilID == "" {
		return "", nil
	}
	return "PNOMN-" + strings.ToUpper(civilID), nil
}

// mapPKIErr нь eID PKI дуудлагын алдааг HTTP-д буулгана: PKI_READ эрхгүй (403)
// бол Forbidden (frontend "эрх хүлээгдэж байна" харуулна), бусад бол Internal.
func mapPKIErr(err error) error {
	if errors.Is(err, eid.ErrPKINotPermitted) {
		return apperror.Forbidden("eID PKI хандах эрх (PKI_READ) олгогдоогүй байна")
	}
	return apperror.InternalCause(fmt.Errorf("eid pki: %w", err))
}

// EIDRepresentations нь нэвтэрсэн хэрэглэгчийн civil_id-аар ETSI танигч
// (PNOMN-<civil_id>) угсарч, eID-ээс төлөөлдөг байгууллагуудыг татна.
// Хэрэглэгч eID-ээр нэвтрээгүй (civil_id хоосон) бол алдаагүйгээр хоосон
// slice буцаана.
func (uc *usecase) EIDRepresentations(ctx context.Context, userID string) ([]eid.Representation, error) {
	etsi, err := uc.eidPersonEtsi(ctx, userID)
	if err != nil {
		return nil, err
	}
	if etsi == "" {
		return []eid.Representation{}, nil // eID хэрэглэгч биш
	}
	reps, repErr := uc.eid.Representations(ctx, etsi)
	if repErr != nil {
		return nil, apperror.InternalCause(fmt.Errorf("eid representations: %w", repErr))
	}
	return reps, nil
}

// RegisterEIDOrganization — regNo-гоор улсын бүртгэлээс (XYP) байгууллагыг хайж,
// нэвтэрсэн иргэнийг eidmongolia-д төлөөлөл болгон холбоно. Эрхийн шалгалт (иргэний
// РД нь тухайн байгууллагын ceo/founder/stakeholder мөн эсэх) нь eidmongolia талд
// (РД тэнд мэдэгддэг) хийгдэнэ — template нь зөвхөн XYP-ээс эрх бүхий этгээдийн РД
// жагсаалтыг дамжуулна. Иргэний бүх идэвхтэй төлөөллийг буцаана.
func (uc *usecase) RegisterEIDOrganization(ctx context.Context, userID, regNo string) ([]eid.Representation, error) {
	regNo = strings.TrimSpace(regNo)
	if regNo == "" {
		return nil, apperror.BadRequest("Байгууллагын регистрийн дугаар шаардлагатай")
	}
	etsi, err := uc.eidPersonEtsi(ctx, userID)
	if err != nil {
		return nil, err
	}
	if etsi == "" {
		return nil, apperror.Forbidden("Байгууллага холбохын тулд eID-ээр нэвтэрсэн байх шаардлагатай")
	}
	if uc.xyp == nil {
		return nil, apperror.Internal("Байгууллагын лавлагаа (XYP) тохируулагдаагүй")
	}
	org, lookupErr := uc.xyp.Lookup(ctx, regNo)
	if lookupErr != nil {
		if errors.Is(lookupErr, xyp.ErrNotFound) {
			return nil, apperror.NotFound("Энэ регистрийн дугаартай байгууллага олдсонгүй")
		}
		if errors.Is(lookupErr, xyp.ErrNotConfigured) {
			return nil, apperror.Internal("Байгууллагын лавлагаа (XYP) тохируулагдаагүй")
		}
		return nil, apperror.InternalCause(fmt.Errorf("xyp lookup: %w", lookupErr))
	}
	in := eid.AddRepresentationInput{
		OrgRegister: org.RegNo,
		OrgName:     org.Name,
		Affiliates:  affiliatesFromXYP(org),
	}
	reps, addErr := uc.eid.AddRepresentation(ctx, etsi, in)
	if addErr != nil {
		if errors.Is(addErr, eid.ErrNotRepresentative) {
			return nil, apperror.Forbidden("Та энэ байгууллагыг төлөөлөх эрхгүй байна (захирал / үүсгэн байгуулагч / хувь эзэмшигч биш)")
		}
		return nil, apperror.InternalCause(fmt.Errorf("eid add representation: %w", addErr))
	}
	return reps, nil
}

// affiliatesFromXYP — XYP-ийн байгууллагаас эрх бүхий этгээдийн (захирал → үүсгэн
// байгуулагч → хувь эзэмшигч дарааллаар) РД жагсаалтыг угсарна. Захирлыг ЭХЭНД
// тавьсан нь eidmongolia эхний таарсан бичлэгээр role-г тодорхойлдогтой холбоотой
// (холбосон этгээд ADMIN эрхтэй болно). Хоосон РД-г алгасна.
func affiliatesFromXYP(org *xyp.Organization) []eid.OrgAffiliate {
	var out []eid.OrgAffiliate
	add := func(regNo, role, kind string) {
		if strings.TrimSpace(regNo) == "" {
			return
		}
		out = append(out, eid.OrgAffiliate{RegNo: strings.TrimSpace(regNo), Role: strings.TrimSpace(role), Kind: kind})
	}
	ceoRole := org.CEOPosition
	if strings.TrimSpace(ceoRole) == "" {
		ceoRole = "Гүйцэтгэх захирал"
	}
	add(org.CEORegNo, ceoRole, "CEO")
	for _, f := range org.Founders {
		add(f.RegNo, "Үүсгэн байгуулагч", "FOUNDER")
	}
	for _, sh := range org.StakeHolders {
		role := sh.Position
		if strings.TrimSpace(role) == "" {
			role = "Хувь эзэмшигч"
		}
		add(sh.RegNo, role, "STAKEHOLDER")
	}
	return out
}

// actingEtsi нь нэвтэрсэн хэрэглэгчийн personEtsi-г буцаана; eID-ээр нэвтрээгүй
// (civil_id байхгүй) бол Forbidden.
func (uc *usecase) actingEtsi(ctx context.Context, userID string) (string, error) {
	etsi, err := uc.eidPersonEtsi(ctx, userID)
	if err != nil {
		return "", err
	}
	if etsi == "" {
		return "", apperror.Forbidden("Энэ үйлдэлд eID-ээр нэвтэрсэн байх шаардлагатай")
	}
	return etsi, nil
}

// UnlinkEIDOrganization нь нэвтэрсэн хэрэглэгч өөрийн байгууллагын төлөөллөө цуцлана.
func (uc *usecase) UnlinkEIDOrganization(ctx context.Context, userID, orgRegister string) ([]eid.Representation, error) {
	if strings.TrimSpace(orgRegister) == "" {
		return nil, apperror.BadRequest("Байгууллагын регистрийн дугаар шаардлагатай")
	}
	etsi, err := uc.actingEtsi(ctx, userID)
	if err != nil {
		return nil, err
	}
	reps, rErr := uc.eid.RemoveRepresentation(ctx, etsi, strings.TrimSpace(orgRegister))
	if rErr != nil {
		if errors.Is(rErr, eid.ErrNotRepresentative) {
			return nil, apperror.Forbidden("Зөвхөн ADMIN эрхтэй хүн байгууллагыг салгаж чадна")
		}
		return nil, apperror.InternalCause(fmt.Errorf("eid unlink: %w", rErr))
	}
	return reps, nil
}

// ListEIDOrgSigners нь байгууллагын гарын үсэг зурагчдыг буцаана (нэвтэрсэн хэрэглэгч
// тухайн байгууллагын төлөөлөгч байх ёстой).
func (uc *usecase) ListEIDOrgSigners(ctx context.Context, userID, orgRegister string) ([]eid.Signer, error) {
	etsi, err := uc.actingEtsi(ctx, userID)
	if err != nil {
		return nil, err
	}
	signers, sErr := uc.eid.OrgSigners(ctx, strings.TrimSpace(orgRegister), etsi)
	if sErr != nil {
		return nil, mapSignerErr(sErr)
	}
	return signers, nil
}

// AddEIDOrgSigner нь байгууллагад өөр иргэнийг (РД) гарын үсэг зурах эрхтэй (MANAGER)
// болгож нэмнэ. Тэр хүн рүү eID sign-push илгээгдэж, өөрөө PIN-ээрээ баталгаажуулах
// хүртэл PENDING (хүчингүй). Шинэ жагсаалт + хүлээгдэж буй баталгаажуулалтыг буцаана.
func (uc *usecase) AddEIDOrgSigner(ctx context.Context, userID, orgRegister, signerRegNo, role string) (*eid.SignersResult, error) {
	if strings.TrimSpace(signerRegNo) == "" {
		return nil, apperror.BadRequest("Гарын үсэг зурагчийн регистрийн дугаар шаардлагатай")
	}
	etsi, err := uc.actingEtsi(ctx, userID)
	if err != nil {
		return nil, err
	}
	res, sErr := uc.eid.AddSigner(ctx, strings.TrimSpace(orgRegister), etsi, eid.AddSignerInput{
		SignerRegNo: strings.TrimSpace(signerRegNo), Role: strings.TrimSpace(role),
	})
	if sErr != nil {
		return nil, mapSignerErr(sErr)
	}
	return res, nil
}

// ResendEIDOrgSigner нь баталгаажаагүй гарын үсэг зурагч руу sign-push дахин илгээнэ.
func (uc *usecase) ResendEIDOrgSigner(ctx context.Context, userID, orgRegister, signerRegNo string) (*eid.SignersResult, error) {
	if strings.TrimSpace(signerRegNo) == "" {
		return nil, apperror.BadRequest("Гарын үсэг зурагчийн регистрийн дугаар шаардлагатай")
	}
	etsi, err := uc.actingEtsi(ctx, userID)
	if err != nil {
		return nil, err
	}
	res, sErr := uc.eid.ResendSigner(ctx, strings.TrimSpace(orgRegister), etsi, strings.TrimSpace(signerRegNo))
	if sErr != nil {
		return nil, mapSignerErr(sErr)
	}
	return res, nil
}

// RemoveEIDOrgSigner нь байгууллагаас гарын үсэг зурагчийг (РД) хасна.
func (uc *usecase) RemoveEIDOrgSigner(ctx context.Context, userID, orgRegister, signerRegNo string) ([]eid.Signer, error) {
	etsi, err := uc.actingEtsi(ctx, userID)
	if err != nil {
		return nil, err
	}
	signers, sErr := uc.eid.RemoveSigner(ctx, strings.TrimSpace(orgRegister), etsi, strings.TrimSpace(signerRegNo))
	if sErr != nil {
		return nil, mapSignerErr(sErr)
	}
	return signers, nil
}

// mapSignerErr нь eid signer client-ийн алдаануудыг apperror болгож буулгана.
func mapSignerErr(err error) error {
	switch {
	case errors.Is(err, eid.ErrNotRepresentative):
		return apperror.Forbidden("Та энэ байгууллагын гарын үсэг зурагчдыг удирдах эрхгүй байна")
	case errors.Is(err, eid.ErrSignerNotEnrolled):
		return apperror.NotFound("Энэ регистрийн дугаартай иргэн eID-д бүртгэлгүй байна")
	default:
		return apperror.InternalCause(fmt.Errorf("eid signer: %w", err))
	}
}

// EIDSummary нь иргэний PKI самбарын нэгдсэн тоог буцаана.
func (uc *usecase) EIDSummary(ctx context.Context, userID string) (*eid.PersonSummary, error) {
	etsi, err := uc.eidPersonEtsi(ctx, userID)
	if err != nil || etsi == "" {
		return nil, err
	}
	res, pErr := uc.eid.PersonSummary(ctx, etsi)
	if pErr != nil {
		return nil, mapPKIErr(pErr)
	}
	return res, nil
}

// EIDCertificates нь иргэний гэрчилгээний жагсаалт + тоог буцаана.
func (uc *usecase) EIDCertificates(ctx context.Context, userID string) (*eid.PersonCertificates, error) {
	etsi, err := uc.eidPersonEtsi(ctx, userID)
	if err != nil || etsi == "" {
		return nil, err
	}
	res, pErr := uc.eid.PersonCertificates(ctx, etsi)
	if pErr != nil {
		return nil, mapPKIErr(pErr)
	}
	return res, nil
}

// EIDDevices нь иргэний холбоотой төхөөрөмжүүдийг буцаана.
func (uc *usecase) EIDDevices(ctx context.Context, userID string) (*eid.PersonDevices, error) {
	etsi, err := uc.eidPersonEtsi(ctx, userID)
	if err != nil || etsi == "" {
		return nil, err
	}
	res, pErr := uc.eid.PersonDevices(ctx, etsi)
	if pErr != nil {
		return nil, mapPKIErr(pErr)
	}
	return res, nil
}

// EIDActivity нь RP-scoped auth/sign түүх + тоог буцаана.
func (uc *usecase) EIDActivity(ctx context.Context, userID string, limit, offset int) (*eid.PersonActivity, error) {
	etsi, err := uc.eidPersonEtsi(ctx, userID)
	if err != nil || etsi == "" {
		return nil, err
	}
	res, pErr := uc.eid.PersonActivity(ctx, etsi, limit, offset)
	if pErr != nil {
		return nil, mapPKIErr(pErr)
	}
	return res, nil
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
