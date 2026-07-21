// Government Template Platform V3.0
// Gerege Systems Development Team болон Claude AI хамтран бүтээв, 2026.

package superadmin

import (
	"context"
	"net/mail"

	"template/internal/apperror"
	"template/internal/business/domain"
)

// ListInvites нь бүх урилгыг (хүлээгдэж буй + ашигласан) буцаана.
func (uc *usecase) ListInvites(ctx context.Context) (ListInvitesResponse, error) {
	if uc.invites == nil {
		return ListInvitesResponse{}, apperror.Internal("superadmin invites are not configured")
	}
	list, err := uc.invites.List(ctx)
	if err != nil {
		return ListInvitesResponse{}, err
	}
	return ListInvitesResponse{Invites: list}, nil
}

// CreateInvite нь и-мэйлийг super admin болох allow-list-д нэмнэ.
//
// АНХААР: урилга нь super admin эрхийг ШУУД олгодоггүй — зөвхөн бүртгэлийн
// шидтэнг (Google + eID + и-мэйл OTP + TOTP) эхлүүлэх эрхийг нээнэ. Иймээс
// энэ нь superadmin давхаргын "API-аар super admin үүсгэхгүй" дүрмийг
// зөрчихгүй: эрх нь зөвхөн бодит хүн бүх баталгаажуулалтыг давсны дараа
// (onboarding finalize) л олгогдоно.
func (uc *usecase) CreateInvite(ctx context.Context, req CreateInviteRequest) (CreateInviteResponse, error) {
	if uc.invites == nil {
		return CreateInviteResponse{}, apperror.Internal("superadmin invites are not configured")
	}
	email := domain.NormalizeInviteEmail(req.Email)
	if email == "" {
		return CreateInviteResponse{}, apperror.BadRequest("email is required")
	}
	if _, err := mail.ParseAddress(email); err != nil {
		return CreateInviteResponse{}, apperror.BadRequest("email format is invalid")
	}

	invite, err := uc.invites.Create(ctx, email, domain.NormalizeInviteEmail(req.ActorEmail))
	if err != nil {
		return CreateInviteResponse{}, err
	}
	uc.record(ctx, actionCreateInvite, email, map[string]any{
		"email":      email,
		"invited_by": invite.InvitedBy,
	})
	return CreateInviteResponse{Invite: invite}, nil
}

// DeleteInvite нь урилгыг цуцална.
func (uc *usecase) DeleteInvite(ctx context.Context, req DeleteInviteRequest) error {
	if uc.invites == nil {
		return apperror.Internal("superadmin invites are not configured")
	}
	email := domain.NormalizeInviteEmail(req.Email)
	if email == "" {
		return apperror.BadRequest("email is required")
	}
	if err := uc.invites.Delete(ctx, email); err != nil {
		return err
	}
	uc.record(ctx, actionDeleteInvite, email, map[string]any{"email": email})
	return nil
}
