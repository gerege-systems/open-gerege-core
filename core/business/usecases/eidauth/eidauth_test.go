// Gerege Systems Development Team болон Claude AI хамтран бүтээв, 2026.

package eidauth

import (
	"context"
	"errors"
	"testing"

	"github.com/gerege-systems/open-gerege-core/core/apperror"
	"github.com/gerege-systems/open-gerege-core/pkg/eid"
)

// fakeEID нь eid.AuthClient-ийн туршилтын хэрэгжилт.
type fakeEID struct {
	start      *eid.StartResult
	startErr   error
	session    *eid.SessionResult
	sessionErr error

	gotNationalID  string
	gotDisplayText string
	gotTimeoutMs   int
	gotNonce       string
}

func (f *fakeEID) QRInitiate(_ context.Context, displayText, _, nonce string) (*eid.StartResult, error) {
	f.gotDisplayText, f.gotNonce = displayText, nonce
	return f.start, f.startErr
}

func (f *fakeEID) Initiate(_ context.Context, nationalID, displayText, _ string) (*eid.StartResult, error) {
	f.gotNationalID, f.gotDisplayText = nationalID, displayText
	return f.start, f.startErr
}

func (f *fakeEID) Session(_ context.Context, _ string, timeoutMs int) (*eid.SessionResult, error) {
	f.gotTimeoutMs = timeoutMs
	return f.session, f.sessionErr
}

func TestStartPassesDisplayTextAndNonce(t *testing.T) {
	f := &fakeEID{start: &eid.StartResult{SessionID: "s1", DeviceLinkURL: "https://link", VerificationCode: "1234"}}
	uc := NewUsecase(f, Config{DisplayText: "Гэрэгэ"})

	res, err := uc.Start(context.Background(), StartRequest{})
	if err != nil {
		t.Fatalf("Start: %v", err)
	}
	if res.SessionID != "s1" || res.DeviceLinkURL != "https://link" {
		t.Errorf("start буруу: %+v", res)
	}
	if f.gotDisplayText != "Гэрэгэ" {
		t.Errorf("displayText=%q", f.gotDisplayText)
	}
	if len(f.gotNonce) != 32 {
		t.Errorf("nonce 32 hex тэмдэгт байх ёстой: %q", f.gotNonce)
	}
}

func TestStartByNationalIDRequiresNationalID(t *testing.T) {
	uc := NewUsecase(&fakeEID{}, Config{})
	_, err := uc.StartByNationalID(context.Background(), StartByNationalIDRequest{NationalID: "   "})
	if !apperror.Is(err, apperror.ErrTypeBadRequest) {
		t.Fatalf("хоосон РД нь BadRequest байх ёстой: %v", err)
	}
}

func TestStartMapsRejectedToBadRequest(t *testing.T) {
	f := &fakeEID{startErr: eid.ErrInitiateRejected}
	uc := NewUsecase(f, Config{})
	_, err := uc.StartByNationalID(context.Background(), StartByNationalIDRequest{NationalID: "УБ11223344"})
	if !apperror.Is(err, apperror.ErrTypeBadRequest) {
		t.Fatalf("IdP-ийн 4xx нь BadRequest болох ёстой: %v", err)
	}
}

func TestStartMapsTransportErrorToInternal(t *testing.T) {
	f := &fakeEID{startErr: errors.New("dial tcp: timeout")}
	uc := NewUsecase(f, Config{})
	_, err := uc.StartByNationalID(context.Background(), StartByNationalIDRequest{NationalID: "УБ11223344"})
	if !apperror.Is(err, apperror.ErrTypeInternal) {
		t.Fatalf("сүлжээний алдаа нь Internal байх ёстой: %v", err)
	}
}

func TestPollReturnsIdentityOnlyWhenComplete(t *testing.T) {
	// RUNNING — identity ирсэн ч гаргахгүй (төлөв нь terminal биш).
	f := &fakeEID{session: &eid.SessionResult{
		State:    "RUNNING",
		Identity: &eid.Identity{CivilID: "ab1234567"},
	}}
	uc := NewUsecase(f, Config{})
	res, err := uc.Poll(context.Background(), PollRequest{SessionID: "s1"})
	if err != nil {
		t.Fatalf("Poll: %v", err)
	}
	if res.State != "RUNNING" || res.Identity != nil {
		t.Fatalf("terminal бус төлөвт identity гарах ёсгүй: %+v", res)
	}
	if f.gotTimeoutMs != defaultPollTimeoutMs {
		t.Errorf("timeoutMs=%d, хүлээсэн %d", f.gotTimeoutMs, defaultPollTimeoutMs)
	}

	// COMPLETE — identity бүрэн гарна.
	f.session = &eid.SessionResult{State: eid.StateComplete, Identity: &eid.Identity{
		CivilID: " ab1234567 ", GivenName: "Бат", Surname: "Дорж", KYCLevel: "HIGH",
	}}
	res, err = uc.Poll(context.Background(), PollRequest{SessionID: "s1"})
	if err != nil {
		t.Fatalf("Poll: %v", err)
	}
	if res.Identity == nil || res.Identity.CivilID != "ab1234567" || res.Identity.GivenName != "Бат" {
		t.Fatalf("identity буруу: %+v", res.Identity)
	}
}

func TestPollRequiresSessionID(t *testing.T) {
	uc := NewUsecase(&fakeEID{}, Config{})
	_, err := uc.Poll(context.Background(), PollRequest{})
	if !apperror.Is(err, apperror.ErrTypeBadRequest) {
		t.Fatalf("хоосон session_id нь BadRequest байх ёстой: %v", err)
	}
}
