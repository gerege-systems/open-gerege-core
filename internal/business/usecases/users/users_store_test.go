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

func TestStore(t *testing.T) {
	tests := []struct {
		name string
		in   *domain.User
		// setup нь шинэ fixture дээр тохиолдол тус бүрийн mock хүлээлтийг
		// холбоно. Тохиолдол бүр цэвэр fixture авдаг; тохиолдлуудын хооронд
		// төлөв алдагддаггүй.
		setup func(f *fixture)
		// wantErr нь баталгаажуулалтын чиглэлийг сэлгэдэг. false үед тест
		// err == nil-ийг хүлээдэг; true үед тест wantErrType-ийн
		// *apperror.DomainError-ийг хүлээдэг. wantErrType == 0-г "алдаагүй"
		// гэж үзэхийн оронд бид тусдаа флаг ашигладаг, учир нь
		// apperror.ErrTypeInternal нь iota-гийн тэг — хоёр sentinel
		// мөргөлдөх байсан.
		wantErr     bool
		wantErrType apperror.ErrorType
		// extraAsserts нь алдааны шалгалтууд тэнцсэний дараа ажиллана; зөвхөн
		// happy path дээр дуудагдана.
		extraAsserts func(t *testing.T, out domain.User)
	}{
		{
			name: "hashes password and normalizes email before storing",
			in: &domain.User{
				Username: "newuser",
				Email:    "  NewUser@Example.COM ",
				Password: "Plaintext_123!",
				RoleID:   domain.RoleUser,
			},
			setup: func(f *fixture) {
				// MatchedBy нь repo-гийн харах ёстой нормчлолын дараах
				// хэлбэрийг шаарддаг — өөр бусад нь regression юм.
				f.repo.On("Store", mock.Anything, mock.MatchedBy(func(u *domain.User) bool {
					return u.Email == "newuser@example.com" &&
						u.Password != "Plaintext_123!" && u.Password != "" &&
						!u.CreatedAt.IsZero()
				})).Return(sampleUser(), nil).Once()
			},
			extraAsserts: func(t *testing.T, out domain.User) {
				assert.NotEmpty(t, out.ID)
			},
		},
		{
			name: "propagates DomainError type from repo (conflict)",
			in: &domain.User{
				Username: "dup", Email: "dup@example.com", Password: "Pwd_123!", RoleID: domain.RoleUser,
			},
			setup: func(f *fixture) {
				f.repo.On("Store", mock.Anything, mock.AnythingOfType("*domain.User")).
					Return(domain.User{}, apperror.Conflict("username or email already exists")).Once()
			},
			wantErr:     true,
			wantErrType: apperror.ErrTypeConflict,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := newFixture(t)
			tt.setup(f)

			out, err := f.usecase.Store(context.Background(), users.StoreRequest{User: tt.in})

			if !tt.wantErr {
				require.NoError(t, err)
				if tt.extraAsserts != nil {
					tt.extraAsserts(t, out.User)
				}
				return
			}
			require.Error(t, err)
			var domErr *apperror.DomainError
			require.True(t, errors.As(err, &domErr))
			assert.Equal(t, tt.wantErrType, domErr.Type)
		})
	}
}
