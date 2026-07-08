// Gerege Template Version 27.0
// Gerege Systems Development Team болон Claude AI хамтран бүтээв, 2026.

package assets

import (
	"context"
	"errors"
	"strings"

	"template/internal/apperror"
	"template/internal/business/usecases/users"
	repointerface "template/internal/datasources/repositories/interface"
	"template/pkg/eid"
)

type usecase struct {
	users    users.Usecase
	userRepo repointerface.UserRepository
	stamps   repointerface.OrgStampRepository
	eid      eid.Client
}

func NewUsecase(usersUC users.Usecase, userRepo repointerface.UserRepository, stampRepo repointerface.OrgStampRepository, eidClient eid.Client) Usecase {
	return &usecase{users: usersUC, userRepo: userRepo, stamps: stampRepo, eid: eidClient}
}

// ── Гарын үсэг (хувь хүн) ──

func (uc *usecase) GetSignature(ctx context.Context, userID string) (string, error) {
	return uc.userRepo.GetSignature(ctx, userID)
}

func (uc *usecase) SetSignature(ctx context.Context, userID, url string) error {
	url = strings.TrimSpace(url)
	if url == "" {
		return apperror.BadRequest("Зургийн URL шаардлагатай")
	}
	return uc.userRepo.SetSignature(ctx, userID, url)
}

func (uc *usecase) DeleteSignature(ctx context.Context, userID string) error {
	return uc.userRepo.SetSignature(ctx, userID, "")
}

// ── Байгууллагын тамга ──

func (uc *usecase) GetStamp(ctx context.Context, userID, orgRegister string) (string, error) {
	return uc.stamps.Get(ctx, strings.TrimSpace(orgRegister))
}

func (uc *usecase) SetStamp(ctx context.Context, userID, orgRegister, url string) error {
	url = strings.TrimSpace(url)
	if url == "" {
		return apperror.BadRequest("Зургийн URL шаардлагатай")
	}
	if err := uc.requireOrgAdmin(ctx, userID, orgRegister); err != nil {
		return err
	}
	return uc.stamps.Upsert(ctx, strings.TrimSpace(orgRegister), url, userID)
}

func (uc *usecase) DeleteStamp(ctx context.Context, userID, orgRegister string) error {
	if err := uc.requireOrgAdmin(ctx, userID, orgRegister); err != nil {
		return err
	}
	return uc.stamps.Delete(ctx, strings.TrimSpace(orgRegister))
}

// requireOrgAdmin нь нэвтэрсэн хэрэглэгч тухайн байгууллагын ADMIN эрхтэй төлөөлөгч
// мөн эсэхийг eID-ээр (eidmongolia OrgSigners) шалгана.
func (uc *usecase) requireOrgAdmin(ctx context.Context, userID, orgRegister string) error {
	got, err := uc.users.GetByID(ctx, users.GetByIDRequest{ID: userID})
	if err != nil {
		return err
	}
	civ := strings.TrimSpace(got.User.CivilID)
	if civ == "" {
		return apperror.Forbidden("eID-ээр нэвтэрсэн байх шаардлагатай")
	}
	etsi := "PNOMN-" + strings.ToUpper(civ)
	signers, err := uc.eid.OrgSigners(ctx, strings.TrimSpace(orgRegister), etsi)
	if err != nil {
		if errors.Is(err, eid.ErrNotRepresentative) {
			return apperror.Forbidden("Та энэ байгууллагыг төлөөлдөггүй байна")
		}
		return apperror.InternalCause(err)
	}
	for _, s := range signers {
		if s.Self && s.RightType == "ADMIN" {
			return nil
		}
	}
	return apperror.Forbidden("Зөвхөн ADMIN эрхтэй хүн тамга тавьж чадна")
}
