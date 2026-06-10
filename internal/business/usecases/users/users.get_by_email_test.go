// Gerege Template Version 27.0
// Gerege Systems Development Team болон Claude AI хамтран бүтээв, 2026.

package users_test

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"template/internal/apperror"
	"template/internal/business/domain"
	"template/internal/business/usecases/users"
)

func TestGetByEmail(t *testing.T) {
	cached := sampleUser()

	tests := []struct {
		name       string
		inputEmail string
		setup      func(f *fixture)
		wantUser   domain.User // тэг утга = эерэг identity шалгалт байхгүй
		// wantErr / wantErrType: хослуулсан флаг + утга, учир нь
		// apperror.ErrTypeInternal нь iota-гийн тэг тул "0 нь алдаагүй гэсэн
		// үг" гэсэн ганц sentinel нь тэр төрөлтэй мөргөлдөх байсан.
		wantErr     bool
		wantErrType apperror.ErrorType
	}{
		{
			name:       "cache hit returns immediately without touching repo",
			inputEmail: "patrick@example.com",
			setup: func(f *fixture) {
				f.rc.On("Get", "user/patrick@example.com").Return(cached).Once()
			},
			wantUser: cached,
		},
		{
			name:       "cache miss reads repo and populates cache",
			inputEmail: "patrick@example.com",
			setup: func(f *fixture) {
				f.rc.On("Get", "user/patrick@example.com").Return(nil).Once()
				f.repo.On("GetByEmail", mock.Anything, mock.AnythingOfType("*domain.User")).
					Return(cached, nil).Once()
				f.rc.On("Set", "user/patrick@example.com", cached).Once()
			},
			wantUser: cached,
		},
		{
			name:       "repo NotFound surfaces as DomainError NotFound",
			inputEmail: "ghost@example.com",
			setup: func(f *fixture) {
				f.rc.On("Get", "user/ghost@example.com").Return(nil).Once()
				f.repo.On("GetByEmail", mock.Anything, mock.AnythingOfType("*domain.User")).
					Return(domain.User{}, apperror.NotFound("user not found")).Once()
			},
			wantErr:     true,
			wantErrType: apperror.ErrTypeNotFound,
		},
		{
			// Regression: түүхий repo алдаануудыг NotFound болгож дахин бичих ёсгүй.
			name:       "raw repo error surfaces as Internal, not 404",
			inputEmail: "patrick@example.com",
			setup: func(f *fixture) {
				f.rc.On("Get", "user/patrick@example.com").Return(nil).Once()
				f.repo.On("GetByEmail", mock.Anything, mock.AnythingOfType("*domain.User")).
					Return(domain.User{}, errors.New("connection refused")).Once()
			},
			wantErr:     true,
			wantErrType: apperror.ErrTypeInternal,
		},
		{
			name:       "input email is normalized (trim + lowercase) before cache + repo lookups",
			inputEmail: "  Patrick@Example.COM ",
			setup: func(f *fixture) {
				// Том/жижиг үсэг холилдсон оролт нь каноник хэлбэртэй ижил
				// жижиг үсгийн кэш key рүү hash хийгдэх ёстой — эс бөгөөс
				// том/жижиг үсгийн ялгаатай ижил email-тэй хоёр хэрэглэгч
				// кэшэд салах болно.
				f.rc.On("Get", "user/patrick@example.com").Return(cached).Once()
			},
			wantUser: cached,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := newFixture(t)
			tt.setup(f)

			out, err := f.usecase.GetByEmail(context.Background(), users.GetByEmailRequest{Email: tt.inputEmail})

			if !tt.wantErr {
				require.NoError(t, err)
				if tt.wantUser.ID != "" {
					assert.Equal(t, tt.wantUser, out.User)
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
