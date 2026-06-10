// Gerege Template Version 27.0
// Gerege Systems Development Team болон Claude AI хамтран бүтээв, 2026.

package mailer

import (
	"context"
	"errors"
	"sync"
	"time"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
	"template/internal/constants"
	"template/pkg/logger"
	"template/pkg/observability"
)

// AsyncOTPMailer нь OTPMailer-г боож, SendOTP нь ажлыг дараалалд оруулаад
// тэр даруй буцдаг болгоно. Анхны синхрон хэрэгжүүлэлт нь SMTP IO-г хүсэлтийн
// замд хийдэг тул OTP илгээх хоцрол (≈100ms–2s) нь /send-otp болон /register-ийн
// p99-ийн нэг хэсэг болдог. Дараалалд оруулах нь HTTP хариуг зөвхөн кэш/DB-ийн
// хоцролоор буцаах боломж олгоно; worker pool нь хүргэлтийг хийж, түр зуурын
// SMTP бүтэлгүйтлийг дахин оролддог.
//
// ErrQueueFull нь ажлын суваг (channel) дүүрсэн үед буцаагдана — дуудагч нь
// хүсэлтийг бүтэлгүй болгох уу эсвэл синхрон илгээлт рүү шилжих үү гэдгийг
// шийдэх ёстой (бид лог бичээд орхино; OTP-ийн хувьд дахин илгээх нь хямд).
type AsyncOTPMailer struct {
	inner   OTPMailer
	queue   chan otpJob
	workers int
	retries int
	backoff time.Duration

	wg       sync.WaitGroup
	stopOnce sync.Once
	stop     chan struct{}
}

type jobKind int

const (
	jobOTP jobKind = iota
	jobPasswordReset
)

type otpJob struct {
	kind     jobKind
	payload  string
	receiver string
	// spanCtx нь үүсэл болсон хүсэлтийн trace танигчдыг авч явдаг бөгөөд
	// ингэснээр worker-ийн илгээх span нь өнчин trace эхлүүлэхийн оронд дэд
	// (child) болж буцаж холбогдоно. Дараалалд хүлээх хугацаа нь context
	// цуцлалт өдөөж чадахгүйн тулд өөрчлөгдөшгүй SpanContext утга болгон хадгална.
	spanCtx trace.SpanContext
}

// ErrQueueFull нь асинхрон mailer хориглолтгүйгээр (blocking) илүү ажил
// хүлээж авч чадахгүйг илэрхийлнэ; дуудагчид үүнийг зөөлөн бүтэлгүйтэл гэж үзэх ёстой.
var ErrQueueFull = errors.New("otp mailer queue is full")

// NewAsyncOTPMailer нь `workers` тооны goroutine эхлүүлж, OTPMailer interface-ийг
// хэрэгжүүлдэг боодлыг (wrapper) буцаана.
func NewAsyncOTPMailer(inner OTPMailer, workers, queueSize, retries int, backoff time.Duration) *AsyncOTPMailer {
	if workers <= 0 {
		workers = 2
	}
	if queueSize <= 0 {
		queueSize = 64
	}
	if retries <= 0 {
		retries = 3
	}
	if backoff <= 0 {
		backoff = time.Second
	}

	a := &AsyncOTPMailer{
		inner:   inner,
		queue:   make(chan otpJob, queueSize),
		workers: workers,
		retries: retries,
		backoff: backoff,
		stop:    make(chan struct{}),
	}

	for i := 0; i < workers; i++ {
		a.wg.Add(1)
		go a.worker(i)
	}
	return a
}

// SendOTP нь имэйлийн ажлыг дараалалд оруулна. Амжилттай дараалалд оруулбал
// nil, суваг дүүрсэн бол ErrQueueFull-г буцаана.
func (a *AsyncOTPMailer) SendOTP(ctx context.Context, otpCode, receiver string) error {
	return a.enqueue(otpJob{
		kind:     jobOTP,
		payload:  otpCode,
		receiver: receiver,
		spanCtx:  trace.SpanFromContext(ctx).SpanContext(),
	})
}

// SendPasswordReset нь нууц үг сэргээх имэйлийг дараалалд оруулна. SendOTP-тэй
// ижил дахин оролдлого + backoff баталгаатай.
func (a *AsyncOTPMailer) SendPasswordReset(ctx context.Context, token, receiver string) error {
	return a.enqueue(otpJob{
		kind:     jobPasswordReset,
		payload:  token,
		receiver: receiver,
		spanCtx:  trace.SpanFromContext(ctx).SpanContext(),
	})
}

// ErrMailerClosed нь Shutdown дуудагдсаны дараа enqueue хийх оролдлогыг
// илэрхийлнэ. Дараалал руу хаалттай үед бичих нь panic үүсгэх тул дуудагч
// руу алдаа болгон буцаана.
var ErrMailerClosed = errors.New("otp mailer is shutting down")

func (a *AsyncOTPMailer) enqueue(j otpJob) error {
	// Shutdown эхэлсэн эсэхийг эхлээд шалга — дарааллыг хаадаггүй (close нь
	// send-after-close panic үүсгэдэг) тул оронд нь stop channel-аар хамгаална.
	select {
	case <-a.stop:
		return ErrMailerClosed
	default:
	}

	select {
	case <-a.stop:
		return ErrMailerClosed
	case a.queue <- j:
		return nil
	default:
		return ErrQueueFull
	}
}

// Shutdown нь worker-уудад дарааллыг гүйцээж дуусгаад гарахыг дохио өгнө. Бүх
// worker дуусах хүртэл, эсвэл ctx дуусах хүртэл хориглоно (block).
func (a *AsyncOTPMailer) Shutdown(ctx context.Context) error {
	// Дарааллыг ХААХГҮЙ — нэгэнт хаасан channel руу enqueue нь panic үүсгэдэг.
	// Зөвхөн `stop`-г хааж дохио өгнө; worker-ууд дараа нь дарааллын үлдсэн
	// ажлуудыг гүйцээгээд гарна (доорх worker-ийн drain логикийг үз).
	a.stopOnce.Do(func() {
		close(a.stop)
	})

	done := make(chan struct{})
	go func() {
		a.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (a *AsyncOTPMailer) worker(id int) {
	defer a.wg.Done()
	for {
		// Хэвийн ажиллагаанд аль аль суваг дээр хүлээнэ. stop хаагдсан үед
		// ч дарааллыг үргэлжлүүлэн уншиж, явж буй ажлуудыг гүйцээнэ; зөвхөн
		// дараалал хоосорсон үед л гарна — ингэснээр Shutdown нь дарааллын
		// ажлуудыг алдахгүй, goroutine leak гарахгүй.
		select {
		case job := <-a.queue:
			a.deliver(id, job)
		case <-a.stop:
			// Stop дохио ирлээ — дарааллыг шавхаж дуусаад гар.
			for {
				select {
				case job := <-a.queue:
					a.deliver(id, job)
				default:
					return
				}
			}
		}
	}
}

func (a *AsyncOTPMailer) deliver(workerID int, job otpJob) {
	spanName := "mailer.SendOTP"
	if job.kind == jobPasswordReset {
		spanName = "mailer.SendPasswordReset"
	}
	// Дахин оролдлого бүрийн span-д үүсэл болсон хүсэлтийн trace-г эцэг (parent)
	// болгон дахин хавсаргана. context.Background-г суурь болгосон нь хүсэлтийн
	// цуцлалтын хугацааг (хүсэлт нь worker ажиллахаас нэлээд өмнө буцдаг)
	// өвлөхөөс сэргийлнэ.
	parentCtx := context.Background()
	if job.spanCtx.IsValid() {
		parentCtx = trace.ContextWithSpanContext(parentCtx, job.spanCtx)
	}

	var lastErr error
	for attempt := 1; attempt <= a.retries; attempt++ {
		ctx, span := observability.Tracer().Start(parentCtx, spanName)
		span.SetAttributes(
			attribute.String("mail.receiver", job.receiver),
			attribute.Int("mail.attempt", attempt),
			attribute.Int("mail.worker", workerID),
		)

		switch job.kind {
		case jobPasswordReset:
			lastErr = a.inner.SendPasswordReset(ctx, job.payload, job.receiver)
		default:
			lastErr = a.inner.SendOTP(ctx, job.payload, job.receiver)
		}
		if lastErr == nil {
			span.SetStatus(codes.Ok, "")
			span.End()
			observability.ObserveMailerOp("sent")
			logger.Info("mail sent", logger.Fields{
				constants.LoggerCategory: constants.LoggerCategoryCache,
				"receiver":               job.receiver,
				"attempt":                attempt,
				"worker":                 workerID,
				"kind":                   spanName,
			})
			return
		}
		span.RecordError(lastErr)
		span.SetStatus(codes.Error, lastErr.Error())
		span.End()

		if attempt == a.retries {
			break
		}
		// Дээд хязгаартай экспоненциал backoff — ингэснээр түр зуурын 4xx SMTP
		// алдаанууд worker-ийг хэдэн минутаар гацаахгүй.
		wait := a.backoff * time.Duration(1<<uint(attempt-1))
		if wait > 30*time.Second {
			wait = 30 * time.Second
		}
		select {
		case <-time.After(wait):
		case <-a.stop:
			// Shutdown хүсэлт гарсан — Shutdown нь backoff таймер дээр хориглохгүйн
			// тулд үлдсэн дахин оролдлогуудыг алгасна.
			return
		}
	}

	observability.ObserveMailerOp("failed")
	logger.Error("otp email failed after retries", logger.Fields{
		constants.LoggerCategory: constants.LoggerCategoryCache,
		"receiver":               job.receiver,
		"retries":                a.retries,
		"error":                  lastErr.Error(),
	})
}
