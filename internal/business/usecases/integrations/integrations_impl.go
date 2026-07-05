// Gerege Template Version 27.0
// Gerege Systems Development Team болон Claude AI хамтран бүтээв, 2026.

package integrations

import (
	"context"
	"fmt"
	"strings"

	"template/internal/apperror"
	"template/internal/business/domain"
	repointerface "template/internal/datasources/repositories/interface"
)

// validProviders нь хүлээн зөвшөөрөх провайдерын id-ууд. Frontend-ийн
// lib/integrations.ts-тэй тааруулсан.
var validProviders = map[string]bool{
	"google-drive": true,
	"dropbox":      true,
	"google-meet":  true,
}

type usecase struct {
	repo   repointerface.UserIntegrationRepository
	cipher *tokenCipher
}

// NewUsecase нь интеграцийн usecase-г үүсгэнэ. encKey нь токеныг шифрлэх нууц
// (INTEGRATION_ENC_KEY) — хоосон бол сул default түлхүүр ашиглагдана.
func NewUsecase(repo repointerface.UserIntegrationRepository, encKey string) (Usecase, error) {
	c, err := newTokenCipher(encKey)
	if err != nil {
		return nil, fmt.Errorf("integrations: init cipher: %w", err)
	}
	return &usecase{repo: repo, cipher: c}, nil
}

func (uc *usecase) Connect(ctx context.Context, req ConnectRequest) (domain.UserIntegration, error) {
	provider := strings.TrimSpace(req.Provider)
	if !validProviders[provider] {
		return domain.UserIntegration{}, apperror.BadRequest("unknown integration provider")
	}
	if strings.TrimSpace(req.AccessToken) == "" {
		return domain.UserIntegration{}, apperror.BadRequest("access_token is required")
	}

	encAccess, err := uc.cipher.encrypt(req.AccessToken)
	if err != nil {
		return domain.UserIntegration{}, apperror.InternalCause(fmt.Errorf("encrypt access token: %w", err))
	}
	encRefresh, err := uc.cipher.encrypt(req.RefreshToken)
	if err != nil {
		return domain.UserIntegration{}, apperror.InternalCause(fmt.Errorf("encrypt refresh token: %w", err))
	}

	stored, err := uc.repo.Upsert(ctx, &domain.UserIntegration{
		UserID:       req.UserID,
		Provider:     provider,
		AccessToken:  encAccess,
		RefreshToken: encRefresh,
		ExpiresAt:    req.ExpiresAt,
	})
	if err != nil {
		return domain.UserIntegration{}, err
	}
	// Буцаахдаа токеныг задлахгүй — дуудагч (handler) токен хэрэглэхгүй.
	stored.AccessToken = ""
	stored.RefreshToken = ""
	return stored, nil
}

func (uc *usecase) List(ctx context.Context, userID string) ([]ConnectedProvider, error) {
	rows, err := uc.repo.ListByUser(ctx, userID)
	if err != nil {
		return nil, err
	}
	out := make([]ConnectedProvider, 0, len(rows))
	for _, r := range rows {
		out = append(out, ConnectedProvider{
			Provider:    r.Provider,
			ExpiresAt:   r.ExpiresAt,
			ConnectedAt: r.CreatedAt,
		})
	}
	return out, nil
}

func (uc *usecase) Disconnect(ctx context.Context, userID, provider string) error {
	if !validProviders[strings.TrimSpace(provider)] {
		return apperror.BadRequest("unknown integration provider")
	}
	return uc.repo.DeleteByUserAndProvider(ctx, userID, provider)
}

// Token нь тухайн хэрэглэгчийн нэг провайдерын токеныг decrypt хийж буцаана.
// Холбоогүй бол NotFound. Зөвхөн server-тал (BFF) дуудах ёстой.
func (uc *usecase) Token(ctx context.Context, userID, provider string) (TokenData, error) {
	provider = strings.TrimSpace(provider)
	if !validProviders[provider] {
		return TokenData{}, apperror.BadRequest("unknown integration provider")
	}
	rows, err := uc.repo.ListByUser(ctx, userID)
	if err != nil {
		return TokenData{}, err
	}
	for _, r := range rows {
		if r.Provider != provider {
			continue
		}
		access, err := uc.cipher.decrypt(r.AccessToken)
		if err != nil {
			return TokenData{}, apperror.InternalCause(fmt.Errorf("decrypt access token: %w", err))
		}
		refresh := ""
		if r.RefreshToken != "" {
			if refresh, err = uc.cipher.decrypt(r.RefreshToken); err != nil {
				return TokenData{}, apperror.InternalCause(fmt.Errorf("decrypt refresh token: %w", err))
			}
		}
		return TokenData{AccessToken: access, RefreshToken: refresh, ExpiresAt: r.ExpiresAt}, nil
	}
	return TokenData{}, apperror.NotFound("integration not connected")
}
