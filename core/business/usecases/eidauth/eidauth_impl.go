// Gerege Systems Development Team болон Claude AI хамтран бүтээв, 2026.

package eidauth

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"

	"github.com/gerege-systems/open-gerege-core/core/apperror"
	"github.com/gerege-systems/open-gerege-core/pkg/eid"
)

// defaultPollTimeoutMs — IdP-ийн long-poll хүлээх дээд хугацаа (мс). eID
// client-ийн HTTP timeout (30с)-оос богино байх ёстой.
const defaultPollTimeoutMs = 25000

// Config — proxy-ийн тохиргоо.
type Config struct {
	// DisplayText нь иргэний утсанд харагдах "хэн нэвтрэхийг хүсэв" текст.
	DisplayText string
	// PollTimeoutMs — 0 бол defaultPollTimeoutMs.
	PollTimeoutMs int
}

type usecase struct {
	eid eid.AuthClient
	cfg Config
}

// NewUsecase нь eID нэвтрэлтийн proxy usecase үүсгэнэ.
func NewUsecase(client eid.AuthClient, cfg Config) Usecase {
	if cfg.PollTimeoutMs <= 0 {
		cfg.PollTimeoutMs = defaultPollTimeoutMs
	}
	return &usecase{eid: client, cfg: cfg}
}

func (uc *usecase) Start(ctx context.Context, req StartRequest) (StartResponse, error) {
	nonce, err := randomNonce()
	if err != nil {
		return StartResponse{}, apperror.InternalCause(fmt.Errorf("generate nonce: %w", err))
	}
	start, initErr := uc.eid.QRInitiate(ctx, uc.cfg.DisplayText, req.CallbackURL, nonce)
	if initErr != nil {
		return StartResponse{}, mapInitiateErr(initErr, "eID session эхлүүлэх боломжгүй байна")
	}
	return fromStart(start), nil
}

func (uc *usecase) StartByNationalID(ctx context.Context, req StartByNationalIDRequest) (StartResponse, error) {
	nationalID := strings.TrimSpace(req.NationalID)
	if nationalID == "" {
		return StartResponse{}, apperror.BadRequest("national_id is required")
	}
	start, initErr := uc.eid.Initiate(ctx, nationalID, uc.cfg.DisplayText, req.CallbackURL)
	if initErr != nil {
		return StartResponse{}, mapInitiateErr(initErr, "Регистрийн дугаар олдсонгүй эсвэл буруу байна")
	}
	return fromStart(start), nil
}

func (uc *usecase) Poll(ctx context.Context, req PollRequest) (PollResponse, error) {
	sessionID := strings.TrimSpace(req.SessionID)
	if sessionID == "" {
		return PollResponse{}, apperror.BadRequest("session_id is required")
	}
	res, err := uc.eid.Session(ctx, sessionID, uc.cfg.PollTimeoutMs)
	if err != nil {
		return PollResponse{}, apperror.InternalCause(fmt.Errorf("eid session: %w", err))
	}
	out := PollResponse{State: res.State}
	// Identity нь зөвхөн COMPLETE үед ирнэ. Түүнийг лог-д БИЧИХГҮЙ — хувийн
	// мэдээлэл; RP хариугаар л хүлээж авна.
	if res.State == eid.StateComplete && res.Identity != nil {
		id := res.Identity
		out.Identity = &Identity{
			CivilID:        strings.TrimSpace(id.CivilID),
			NationalID:     strings.TrimSpace(id.NationalID),
			GivenName:      strings.TrimSpace(id.GivenName),
			Surname:        strings.TrimSpace(id.Surname),
			GivenNameEn:    strings.TrimSpace(id.GivenNameEn),
			SurnameEn:      strings.TrimSpace(id.SurnameEn),
			FullName:       strings.TrimSpace(id.FullName),
			KYCLevel:       strings.TrimSpace(id.KYCLevel),
			DocumentNumber: strings.TrimSpace(id.DocumentNumber),
		}
	}
	return out, nil
}

func fromStart(s *eid.StartResult) StartResponse {
	if s == nil {
		return StartResponse{}
	}
	return StartResponse{
		SessionID:        s.SessionID,
		DeviceLinkURL:    s.DeviceLinkURL,
		VerificationCode: s.VerificationCode,
		ExpiresAt:        s.ExpiresAt,
	}
}

// randomNonce нь IdP-ийн replay-аас хамгаалах 32 hex тэмдэгтийн nonce үүсгэнэ.
func randomNonce() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

// mapInitiateErr нь IdP-ийн 4xx-ийг цэвэр BadRequest, бусдыг дотоод алдаа болгоно.
func mapInitiateErr(initErr error, clientMsg string) error {
	if errors.Is(initErr, eid.ErrInitiateRejected) {
		return apperror.BadRequest(clientMsg)
	}
	return apperror.InternalCause(fmt.Errorf("eid initiate: %w", initErr))
}
