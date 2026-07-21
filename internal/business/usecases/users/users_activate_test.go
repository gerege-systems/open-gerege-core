// Government Template Platform V3.0
// Gerege Systems Development Team болон Claude AI хамтран бүтээв, 2026.

package users_test

import (
	"context"
	"errors"
	"testing"

	"template/internal/apperror"
	"template/internal/business/domain"
	"template/internal/business/usecases/users"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func TestActivate(t *testing.T) {
	tests := []struct {
		name   string
		userID string
		setup  func(f *fixture)
		// wantErr / wantErrType-ийг хослуулсан, учир нь apperror.ErrTypeInternal
		// нь iota-гийн тэг — ганц sentinel нь тэр төрөлтэй мөргөлдөж, чимээгүйхэн
		// тэнцэх байсан.
		wantErr     bool
		wantErrType apperror.ErrorType
	}{
		{
			name:   "flips active flag with the right ID and invalidates cache by email",
			userID: "user-123",
			setup: func(f *fixture) {
				f.repo.On("ChangeActiveUser", mock.Anything, mock.MatchedBy(func(u *domain.User) bool {
					return u.ID == "user-123" && u.Active == true
				})).Return(nil).Once()
				// Activate нь OTP урсгалын бөглөсөн хуучирсан "user/<email>"
				// бичлэгийг хөөн зайлуулж чадахын тулд email-ийг хайдаг.
				f.repo.On("GetByID", mock.Anything, "user-123").
					Return(domain.User{ID: "user-123", Email: "patrick@example.com"}, nil).Once()
				f.rc.On("Del", "user/patrick@example.com").Once()
			},
		},
		{
			name:   "raw repo error becomes DomainError Internal",
			userID: "user-123",
			setup: func(f *fixture) {
				f.repo.On("ChangeActiveUser", mock.Anything, mock.AnythingOfType("*domain.User")).
					Return(errors.New("deadlock")).Once()
			},
			wantErr:     true,
			wantErrType: apperror.ErrTypeInternal,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := newFixture(t)
			tt.setup(f)

			err := f.usecase.Activate(context.Background(), users.ActivateRequest{UserID: tt.userID})

			if !tt.wantErr {
				require.NoError(t, err)
				return
			}
			require.Error(t, err)
			var domErr *apperror.DomainError
			require.True(t, errors.As(err, &domErr))
			assert.Equal(t, tt.wantErrType, domErr.Type)
		})
	}
}
