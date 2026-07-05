// Gerege Template Version 27.0
// Gerege Systems Development Team болон Claude AI хамтран бүтээв, 2026.

package users

import (
	"context"

	"template/internal/business/domain"
)

// GetByGoogleSub нь холбогдсон Google account (sub)-аар хэрэглэгчийг олно
// (repository руу дамжуулна; Google callback дахь pre-auth хайлт).
func (uc *usecase) GetByGoogleSub(ctx context.Context, sub string) (domain.User, error) {
	return uc.repo.GetByGoogleSub(ctx, sub)
}

// LinkGoogleAccount нь eID-ээр баталгаажсан хэрэглэгчид Google account +
// профайлыг холбоно/шинэчилнэ.
func (uc *usecase) LinkGoogleAccount(ctx context.Context, userID string, acct domain.GoogleAccount) error {
	return uc.repo.LinkGoogleAccount(ctx, userID, acct)
}

// UnlinkGoogle нь хэрэглэгчийн Google холболтыг арилгана.
func (uc *usecase) UnlinkGoogle(ctx context.Context, userID string) error {
	return uc.repo.UnlinkGoogle(ctx, userID)
}
