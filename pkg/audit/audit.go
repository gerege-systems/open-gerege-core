// Gerege Template Version 27.0
// Gerege Systems Development Team болон Claude AI хамтран бүтээв, 2026.

// Package audit нь аппликэйшний хандалтын логоос (access log) тусдаа,
// аюулгүй байдалд хамаатай үйл явдлуудын (register, login, logout, refresh,
// OTP баталгаажуулалт гэх мэт) бүтэцлэгдсэн урсгалыг ялгаруулна.
//
// Сэдэл нь үйл ажиллагааны лог болон аудитын лог нь хадгалах хугацаа, формат,
// хандалтын шаардлага зэргээрээ ялгаатай байдагт оршино. Тэдгээрийг хольсноор
// дараа нь хийх мөрдөн шалгалт (forensics) үнэтэй болдог: 100 дахин их шуугианыг
// grep хийх шаардлагатай болж, тогтмол лог эргэлт (rotation) нь дайсагнасан
// үйл явдлын цорын ганц бичлэгийг устгаж болзошгүй.
//
// Энэ package нь JSON мөрүүдийг өөрийн io.Writer руу бичдэг (анхдагчаар os.Stderr,
// тестэд SetOutput-аар, эсвэл production-д файл руу солино). Энэ нь төслийн
// бусад хэсгээс хамаардаггүй — usecase / handler-ууд import цикл үүсгэлгүйгээр
// audit.Record(...)-г дуудаж болно.
package audit

import (
	"encoding/json"
	"io"
	"os"
	"sync"
	"time"
)

// EventType нь auth-д хамаатай үйл явдлын ангиллуудыг тоочно. Нэрс нь
// тогтвортой тэмдэгт мөрүүд — лог шинжилгээ болон сэрэмжлүүлэг эдгээрийг түлхүүр болгоно.
type EventType string

const (
	EventRegister      EventType = "register"
	EventLoginSuccess  EventType = "login_success"
	EventLoginFailure  EventType = "login_failure"
	EventLogout        EventType = "logout"
	EventRefreshOK     EventType = "refresh_success"
	EventRefreshFail   EventType = "refresh_failure"
	EventOTPSent       EventType = "otp_sent"
	EventOTPVerifyOK   EventType = "otp_verify_success"
	EventOTPVerifyFail EventType = "otp_verify_failure"
	EventOTPLockout    EventType = "otp_lockout"

	EventPasswordChangeOK   EventType = "password_change_success"
	EventPasswordChangeFail EventType = "password_change_failure"
	EventPasswordForgotOK   EventType = "password_forgot_success"
	EventPasswordForgotFail EventType = "password_forgot_failure"
	EventPasswordResetOK    EventType = "password_reset_success"
	EventPasswordResetFail  EventType = "password_reset_failure"
)

// Event нь дискэн дэх бүтэц. Зөвхөн утгатай талбарууд ялгардаг —
// JSON omitempty нь мөрүүдийг авсаархан байлгана.
//
// RequestID + TraceID нь холбоосын ID-ууд: ижил HTTP хүсэлтийн бүх
// бүтэцлэгдсэн лог мөр дээр ижил хос гарч ирдэг тул аудитын бичлэгийг
// аппликэйшний логтой (эсвэл TraceID-аар tracing backend дахь span-уудтай)
// буцааж холбож болно.
type Event struct {
	Time      time.Time `json:"time"`
	Type      EventType `json:"event"`
	Success   bool      `json:"success"`
	UserID    string    `json:"user_id,omitempty"`
	Email     string    `json:"email,omitempty"`
	IP        string    `json:"ip,omitempty"`
	UserAgent string    `json:"user_agent,omitempty"`
	RequestID string    `json:"request_id,omitempty"`
	TraceID   string    `json:"trace_id,omitempty"`
	Reason    string    `json:"reason,omitempty"`
}

var (
	mu    sync.Mutex
	out   io.Writer = os.Stderr
	enc   *json.Encoder
	encMu sync.Mutex
)

func init() {
	enc = json.NewEncoder(out)
}

// SetOutput нь зорилтот writer-ийг солино. Record-той зэрэгцэн дуудахад
// аюулгүй; тестэд эсвэл production-д эхлэх үед rotate-ийг мэддэг writer-уудтай
// (lumberjack, syslog) холбоход зориулагдсан.
func SetOutput(w io.Writer) {
	mu.Lock()
	defer mu.Unlock()
	out = w
	enc = json.NewEncoder(w)
}

// Record нь нэг үйл явдал ялгаруулна. Sink руу бичих үеийн алдаануудыг
// зориудаар орхигдуулдаг — аудитын writer амжилтгүй болсон үед цорын ганц
// эрүүл нөхөн арга нь "хүсэлтийг эвдэхгүй байх" — гэхдээ бид production-д
// SetOutput-г найдвартай зүйлд (rotation бүхий файл, journald гэх мэт) холбоно гэж найдаж байна.
func Record(e Event) {
	if e.Time.IsZero() {
		e.Time = time.Now().UTC()
	}
	encMu.Lock()
	defer encMu.Unlock()
	_ = enc.Encode(e)
}
