// Gerege Template Version 27.0
// Gerege Systems Development Team болон Claude AI хамтран бүтээв, 2026.

package mailer

import (
	"bytes"
	"context"
	"embed"
	"fmt"
	"html/template"
	"time"

	gomail "gopkg.in/mail.v2"
)

// Binary нь өөртөө бүрэн агуулагдсан байхын тулд шигтгэсэн (embedded) — distroless
// runtime нь deploy үед template ачаалах shell эсвэл файлын системгүй.
//
//go:embed templates/*.html
var templatesFS embed.FS

// otpTpl нь package init үед нэг удаа задлан уншигдана. html/template (текст биш)
// нь OTP кодыг автоматаар escape хийдэг бөгөөд халдагч ямар нэгэн байдлаар OTP
// зам руу markup тарихаас хамгаалдаг.
var otpTpl = template.Must(template.ParseFS(templatesFS, "templates/otp.html"))

// otpTemplateData нь template-д өгөгдөл дамжуулна. AppName / Region нь өнөөдөр
// тогтмолууд; тэдгээрийг гаргаж авах нь white-labeling болон i18n-г кодын
// засвар биш, тохиргооны өөрчлөлт болгоно.
type otpTemplateData struct {
	AppName      string
	Region       string
	Code         string
	Year         int
	ValidMinutes int
}

const (
	defaultAppName      = "Go Rest boilerplate"
	defaultRegion       = "East Java, Indonesia"
	defaultValidMinutes = 5
)

type OTPMailer interface {
	// SendOTP нь тохируулсан SMTP relay-аар дамжуулан OTP кодыг хүлээн авагчийн
	// inbox руу хүргэнэ. Асинхрон боодол (AsyncOTPMailer) нь ctx-ээс span context-ийн
	// агшны зургийг авдаг тул worker-ийн илгээх span нь үүсэл болсон хүсэлт рүү
	// буцаж холбогддог; синхрон хэрэгжүүлэлт нь SMTP дуудлагын хувьд ctx-г үл
	// тоомсорлоно (gomail нь өөрийн dialer timeout-той).
	SendOTP(ctx context.Context, otpCode, receiver string) (err error)
	// SendPasswordReset нь хүлээн авагчид тунхаг бус (opaque) сэргээх токеныг хүргэнэ.
	// Хүлээн авагч нь имэйлд агуулагдсан /auth/password/reset руу буцах линкийг
	// дагана гэж тооцно; энэ давхарга зүгээр л токеныг дамжуулна.
	SendPasswordReset(ctx context.Context, token, receiver string) error
}

type otpMailer struct {
	email    string
	password string
	host     string
	port     int
}

// NewOTPMailer нь SMTP relay-ийн host/port-г параметрээр авна. Хоосон host
// эсвэл 0 port өгвөл Gmail-ийн анхдагч (smtp.gmail.com:587)-руу унана —
// ингэснээр өөр provider руу солихдоо зөвхөн config (OTP_SMTP_HOST/PORT)
// солино, код засахгүй.
func NewOTPMailer(email, password, host string, port int) OTPMailer {
	if host == "" {
		host = "smtp.gmail.com"
	}
	if port == 0 {
		port = 587
	}
	return &otpMailer{
		email:    email,
		password: password,
		host:     host,
		port:     port,
	}
}

func (mailer *otpMailer) SendOTP(_ context.Context, otpCode, receiver string) (err error) {
	body, err := renderOTPBody(otpCode)
	if err != nil {
		return fmt.Errorf("render otp template: %w", err)
	}

	msg := gomail.NewMessage()
	msg.SetHeader("From", mailer.email)
	msg.SetHeader("To", receiver)
	msg.SetHeader("Subject", "Verification Email")
	msg.SetBody("text/html", body)

	dialer := gomail.NewDialer(mailer.host, mailer.port, mailer.email, mailer.password)
	dialer.Timeout = 10 * time.Second

	return dialer.DialAndSend(msg)
}

func (mailer *otpMailer) SendPasswordReset(_ context.Context, token, receiver string) error {
	body := fmt.Sprintf(
		`<p>Use the following token to reset your password. The token expires in %d minutes.</p><p><b>%s</b></p>`,
		defaultValidMinutes, template.HTMLEscapeString(token),
	)
	msg := gomail.NewMessage()
	msg.SetHeader("From", mailer.email)
	msg.SetHeader("To", receiver)
	msg.SetHeader("Subject", "Password Reset")
	msg.SetBody("text/html", body)

	dialer := gomail.NewDialer(mailer.host, mailer.port, mailer.email, mailer.password)
	dialer.Timeout = 10 * time.Second
	return dialer.DialAndSend(msg)
}

// renderOTPBody нь тестүүдэд зориулсан туслах функц бөгөөд тэдгээр нь SMTP
// dialer асаалгүйгээр render хийгдсэн HTML дээр шалгалт хийх боломжтой болгоно.
func renderOTPBody(code string) (string, error) {
	var buf bytes.Buffer
	data := otpTemplateData{
		AppName:      defaultAppName,
		Region:       defaultRegion,
		Code:         code,
		Year:         time.Now().Year(),
		ValidMinutes: defaultValidMinutes,
	}
	if err := otpTpl.Execute(&buf, data); err != nil {
		return "", err
	}
	return buf.String(), nil
}
