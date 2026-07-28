// Gerege Systems Development Team болон Claude AI хамтран бүтээв, 2026.

package sign

import (
	"context"
	"encoding/base64"
	"encoding/hex"
	"strings"

	"github.com/gerege-systems/public-gerege-core/core/apperror"
	"github.com/gerege-systems/public-gerege-core/pkg/logger"
)

// digestSize нь SHA-256-ийн байтын урт. Бид ЗӨВХӨН SHA-256 digest-д гарын
// үсэг зуруулна (eID-д hashType=SHA256 гэж явуулдаг) тул өөр урттай утгыг
// шууд татгалзана.
const digestSize = 32

// InitDigest нь дурын SHA-256 digest-д PIN2 гарын үсгийн session эхлүүлнэ.
//
// PDF-д гарын үсэг зурдаг Init-ээс ялгаатай нь баримт огт байхгүй — иргэний
// утсанд харагдах ганц зүйл бол displayText (жишээ нь "50000 MNT → …1234").
// Тиймээс ДУУДАГЧ ТАЛ displayText-ийг хэрэглэгчийн үнэхээр батлах агуулгыг
// үнэн зөв тусгахаар бүрдүүлэх ҮҮРЭГТЭЙ: иргэн зөвхөн үүнийг л хараад PIN2
// оруулна.
//
// Түрийвчний шилжүүлгийн урсгал:
//
//	апп: canonical.Transfer → SHA-256 → digestHex
//	     POST /v1/sign/initiate {document_hash_hex, display_text}
//	иргэн: утсан дээрээ дүн + хүлээн авагчийг хараад PIN2
//	сервер: POST /v1/transfer/iban үед хэшийг ДАХИН тооцоолж тулгана
func (u *usecase) InitDigest(ctx context.Context, regNo, fullName, digestHex, displayText string) (InitResult, error) {
	if strings.TrimSpace(regNo) == "" {
		return InitResult{}, apperror.Unauthorized("регистр тодорхойгүй")
	}
	raw, err := hex.DecodeString(strings.TrimSpace(digestHex))
	if err != nil || len(raw) != digestSize {
		return InitResult{}, apperror.BadRequest("document_hash_hex нь 64 тэмдэгтийн SHA-256 hex байх ёстой")
	}
	digestB64 := base64.StdEncoding.EncodeToString(raw)

	// fileName хоосон — digest урсгалд баримт байхгүй (зөвхөн hash), тул
	// verify хуудсанд харуулах файлын нэр ч байхгүй.
	v3SessionID, vc, err := u.startV3Sign(ctx, toEtsi(regNo), digestB64, fullName, "", displayText, "")
	if err != nil {
		if de, ok := err.(*apperror.DomainError); ok {
			return InitResult{}, de
		}
		return InitResult{}, apperror.InternalCause(err)
	}

	sessionID := randID()
	// PDFBase64 хоосон — энэ session-ээс Download хийх боломжгүй (баримт байхгүй).
	st := signState{
		RegNo:       regNo,
		FullName:    fullName,
		DocHashB64:  digestB64,
		V3SessionID: v3SessionID,
		State:       "running",
	}
	if err := u.saveState(ctx, sessionID, st); err != nil {
		return InitResult{}, apperror.InternalCause(err)
	}
	logger.InfoWithContext(ctx, "sign: digest session эхэллээ", logger.Fields{
		"usecase": "sign", "method": "InitDigest", "session_id": sessionID,
	})
	return InitResult{
		SessionID:        sessionID,
		DocumentHash:     digestB64,
		VerificationCode: vc,
	}, nil
}

// VerifiedDigest нь session дууссан эсэхийг шалгаж, гарын үсэг зурагдсан
// digest-ийг буцаана.
//
// Session "running" төлөвтэй байвал НЭГ УДАА poll хийнэ: иргэн PIN2-оо
// оруулмагц апп шууд шилжүүлгээ илгээдэг тул тэр агшинд серверийн хадгалсан
// төлөв хараахан шинэчлэгдээгүй байх нь энгийн үзэгдэл. Poll-гүй бол хууль
// ёсны шилжүүлэг "гарын үсэг дуусаагүй" гэж татгалзагдана.
func (u *usecase) VerifiedDigest(ctx context.Context, ownerRegNo, sessionID string) (string, error) {
	st, err := u.loadState(ctx, sessionID)
	if err != nil {
		return "", apperror.NotFound("гарын үсгийн session олдсонгүй")
	}
	// Эзэмшлийн шалгалт — өөр иргэний гарын үсгээр шилжүүлэг хийхийг хаана
	// (IDOR). Session олдсон эсэхийг ялгаж мэдэгдэхгүйн тулд ижил алдаа.
	if st.RegNo != ownerRegNo {
		return "", apperror.NotFound("гарын үсгийн session олдсонгүй")
	}

	if st.State == "running" {
		if _, pollErr := u.Poll(ctx, ownerRegNo, sessionID); pollErr != nil {
			return "", pollErr
		}
		if st, err = u.loadState(ctx, sessionID); err != nil {
			return "", apperror.NotFound("гарын үсгийн session олдсонгүй")
		}
	}

	switch st.State {
	case "completed":
		if st.DocHashB64 == "" {
			return "", apperror.InternalCause(errEmptyDigest)
		}
		return st.DocHashB64, nil
	case "rejected":
		return "", apperror.Forbidden("иргэн гарын үсгээс татгалзсан")
	case "running":
		return "", apperror.BadRequest("гарын үсэг хараахан дуусаагүй байна")
	default:
		return "", apperror.BadRequest("гарын үсэг амжилтгүй боллоо")
	}
}

// errEmptyDigest — completed session-д digest байхгүй байх нь логикийн алдаа.
var errEmptyDigest = apperror.Internal("sign session completed without a digest")

// clampDisplayText нь eID-ийн displayText60 талбарт багтаах утгыг бэлдэнэ.
// Хоосон бол ерөнхий текст; 60 ТЭМДЭГТЭЭС (байт биш — кирилл үсэг олон байт
// эзэлдэг) урт бол таслана.
func clampDisplayText(text string) string {
	text = strings.TrimSpace(text)
	if text == "" {
		return "Gerege Wallet — гарын үсэг"
	}
	runes := []rune(text)
	if len(runes) > 60 {
		return string(runes[:60])
	}
	return text
}

// maxFileNameRunes нь eID-д илгээх fileName-ийн дээд урт (тэмдэгтээр). Нийтийн
// verify хуудсанд багтаах, мөн хэт урт нэрээр хүсэлт өвдөхөөс сэргийлнэ.
const maxFileNameRunes = 120

// clampFileName нь upload-ын файлын нэрийг eID-ийн fileName талбарт тохируулна:
// зам (client заримдаа бүтэн зам илгээдэг) болон удирдах тэмдэгтийг хасаж,
// уртыг таслана. Утга үлдэхгүй бол хоосон буцаана — дуудагч тал талбарыг огт
// нэмэхгүй бөгөөд сервер хуучнаараа displayText-ээс таамаглана.
func clampFileName(name string) string {
	if i := strings.LastIndexAny(name, `/\`); i >= 0 {
		name = name[i+1:]
	}
	name = strings.Map(func(r rune) rune {
		if r < 0x20 || r == 0x7f {
			return -1
		}
		return r
	}, name)
	name = strings.TrimSpace(name)
	runes := []rune(name)
	if len(runes) > maxFileNameRunes {
		return strings.TrimSpace(string(runes[:maxFileNameRunes]))
	}
	return name
}
