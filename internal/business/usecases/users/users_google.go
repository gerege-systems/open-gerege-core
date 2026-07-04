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

// LinkGoogleSub нь eID-ээр баталгаажсан хэрэглэгчид Google account-ийг холбоно.
func (uc *usecase) LinkGoogleSub(ctx context.Context, userID, sub string) error {
	return uc.repo.LinkGoogleSub(ctx, userID, sub)
}
