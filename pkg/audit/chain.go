// Government Template Platform V3.0
// Gerege Systems Development Team болон Claude AI хамтран бүтээв, 2026.

package audit

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"time"
)

// ChainEntry нь hash-chained audit_log хүснэгтэд бичигдэх нэг үйл явдлын
// агуулга (chain hash тооцоолохоос ӨМНӨХ хэлбэр). Энэ нь pkg/audit-ийн
// io.Writer-д суурилсан Event-ээс ялгаатай: ChainEntry нь Postgres-д
// тогтвортой хадгалагдаж, prev_hash → chain_hash гинжээр холбогддог тул
// дараа нь засвар хийгдсэн эсэхийг (tamper) илрүүлэх боломжтой.
//
// Талбаруудын ДАРААЛАЛ нь канон JSON-д тогтмол — энд талбар нэмэх нь гинж
// эвдэх (chain-breaking) өөрчлөлт тул анхааралтай хандана.
type ChainEntry struct {
	OccurredAt  time.Time      `json:"-"`
	ActorUserID string         `json:"actor_user_id"`
	Action      string         `json:"action"`
	Category    string         `json:"category"`
	Target      string         `json:"target"`
	RequestID   string         `json:"request_id"`
	Metadata    map[string]any `json:"metadata"`
}

// canonicalJSON нь ChainEntry-г hash-д зориулсан детерминист байт болгон
// хувиргана. occurred_at-г unix nano болгож тогтворжуулна (TZ/формат хамаарахгүй);
// metadata нь string түлхүүртэй map тул json.Marshal нь түлхүүрийг эрэмбэлдэг тул
// тогтвортой. Талбарын дараалал нь struct тэгийн дарааллаар тогтоогдоно.
//
// ЧУХАЛ: цагийг hash хийхээс ӨМНӨ МИКРОСЕКУНД болгож таслана. Postgres-ийн
// timestamptz нь микросекунд нарийвчлалтай тул наносекундын үлдэгдэл
// хадгалагдахдаа алдагддаг. Таслахгүй бол Append нь ns-ээр hash хийж, DB нь
// µs-ээр хадгалж, VerifyChain нь буцааж уншаад ӨӨР hash тооцоолох тул
// гэмтээгүй гинж "эвдэрсэн" гэж гарна.
//
// Энэ нь платформыг ажиллуулдаг Linux дээр л илэрдэг: тэнд time.Now() ns
// нарийвчлалтай байдаг бол macOS дээр ихэвчлэн µs-т тэгширсэн байдаг тул
// локал тест өнгөрч, зөвхөн production дээр audit баталгаа ажиллахгүй байв.
func canonicalJSON(e ChainEntry) ([]byte, error) {
	type canon struct {
		OccurredAtNS int64          `json:"occurred_at_ns"`
		ActorUserID  string         `json:"actor_user_id"`
		Action       string         `json:"action"`
		Category     string         `json:"category"`
		Target       string         `json:"target"`
		RequestID    string         `json:"request_id"`
		Metadata     map[string]any `json:"metadata"`
	}
	return json.Marshal(canon{
		OccurredAtNS: e.OccurredAt.UTC().Truncate(time.Microsecond).UnixNano(),
		ActorUserID:  e.ActorUserID,
		Action:       e.Action,
		Category:     e.Category,
		Target:       e.Target,
		RequestID:    e.RequestID,
		Metadata:     e.Metadata,
	})
}

// ComputeChainHash нь шинэ мөрийн chain_hash-г тооцоолно:
//
//	chain_hash = SHA-256( prevHash (hex текст) || canonical-json(entry) )
//
// prevHash нь өмнөх мөрийн chain_hash (hex тэмдэгт мөр); genesis (анхны мөр)-д
// хоосон тэмдэгт мөр "" дамжуулна. Буцах утга нь hex-encoded SHA-256.
//
// АНХААР: prevHash-г hex тэмдэгт мөр хэлбэрээр (DB-д хадгалагддагтай ижил)
// шууд hash-д оруулдаг тул VerifyChain нь DB-ээс уншсан текстийг шууд дахин
// hash хийж чадна — байт хооронд хувиргах алхам шаардлагагүй.
func ComputeChainHash(prevHash string, e ChainEntry) (string, error) {
	canon, err := canonicalJSON(e)
	if err != nil {
		return "", err
	}
	h := sha256.New()
	h.Write([]byte(prevHash))
	h.Write(canon)
	return hex.EncodeToString(h.Sum(nil)), nil
}
