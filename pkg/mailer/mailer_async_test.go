// Gerege Template Version 27.0
// Gerege Systems Development Team болон Claude AI хамтран бүтээв, 2026.

package mailer_test

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"template/pkg/mailer"
)

// stubMailer нь SendOTP дуудлага бүрийг тэмдэглэдэг бөгөөд дахин оролдлогын
// замыг турших зорилгоор эхний N оролдлогыг бүтэлгүй болгохоор тохируулж болно.
type stubMailer struct {
	mu          sync.Mutex
	calls       int32
	failUntil   int32
	forceErr    error
	deliveredTo []string
}

func (s *stubMailer) SendOTP(_ context.Context, code, receiver string) error {
	n := atomic.AddInt32(&s.calls, 1)
	if s.forceErr != nil && n <= s.failUntil {
		return s.forceErr
	}
	s.mu.Lock()
	s.deliveredTo = append(s.deliveredTo, receiver)
	s.mu.Unlock()
	return nil
}

func (s *stubMailer) SendPasswordReset(ctx context.Context, token, receiver string) error {
	return s.SendOTP(ctx, token, receiver)
}

func TestAsyncMailer_Delivers(t *testing.T) {
	stub := &stubMailer{}
	async := mailer.NewAsyncOTPMailer(stub, 1, 4, 3, 10*time.Millisecond)

	assert.NoError(t, async.SendOTP(context.Background(), "123456", "alice@example.com"))

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	assert.NoError(t, async.Shutdown(ctx))

	stub.mu.Lock()
	defer stub.mu.Unlock()
	assert.Equal(t, []string{"alice@example.com"}, stub.deliveredTo)
}

func TestAsyncMailer_RetriesTransientFailures(t *testing.T) {
	stub := &stubMailer{forceErr: errors.New("smtp timeout"), failUntil: 2}
	async := mailer.NewAsyncOTPMailer(stub, 1, 4, 3, 10*time.Millisecond)

	assert.NoError(t, async.SendOTP(context.Background(), "123456", "bob@example.com"))

	// Унтраахаас өмнө worker гурван оролдлогоо бүгдийг хийх хүртэл хүлээнэ —
	// Shutdown нь хүлээгдэж буй дахин оролдлогын backoff-уудыг цуцалдаг бөгөөд
	// үгүй бол энэ нь хүргэлттэй зэрэгцэн уралдана (race).
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) && atomic.LoadInt32(&stub.calls) < 3 {
		time.Sleep(5 * time.Millisecond)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	assert.NoError(t, async.Shutdown(ctx))

	assert.Equal(t, int32(3), atomic.LoadInt32(&stub.calls), "should retry twice after initial failure")
	stub.mu.Lock()
	defer stub.mu.Unlock()
	assert.Equal(t, []string{"bob@example.com"}, stub.deliveredTo)
}

func TestAsyncMailer_ErrQueueFull(t *testing.T) {
	// тэг worker + жижигхэн дараалал + үүрд хориглодог mailer → дараалал дүүрнэ.
	blocker := make(chan struct{})
	stub := &blockingMailer{release: blocker}
	async := mailer.NewAsyncOTPMailer(stub, 1, 1, 1, time.Millisecond)

	// Эхний илгээлт worker-ийг эзэлнэ; хоёр дахь нь дарааллыг дүүргэнэ; гурав дахь нь бүтэлгүй болох ёстой.
	ctxBg := context.Background()
	_ = async.SendOTP(ctxBg, "1", "a@a")
	_ = async.SendOTP(ctxBg, "2", "b@b")
	// Эхний ажлыг авч авахад worker-т хэсэг хугацаа өгнө.
	time.Sleep(20 * time.Millisecond)
	_ = async.SendOTP(ctxBg, "3", "c@c")
	err := async.SendOTP(ctxBg, "4", "d@d")
	assert.ErrorIs(t, err, mailer.ErrQueueFull)

	close(blocker)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	_ = async.Shutdown(ctx)
}

type blockingMailer struct {
	release chan struct{}
}

func (b *blockingMailer) SendOTP(_ context.Context, code, receiver string) error {
	<-b.release
	return nil
}

func (b *blockingMailer) SendPasswordReset(ctx context.Context, token, receiver string) error {
	return b.SendOTP(ctx, token, receiver)
}
