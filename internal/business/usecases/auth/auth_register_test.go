// Government Template Platform V3.0
// Gerege Systems Development Team болон Claude AI хамтран бүтээв, 2026.

package auth_test

import (
	"context"
	"testing"

	"template/internal/business/domain"
	"template/internal/business/usecases/auth"
	"template/internal/business/usecases/users"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func TestRegister(t *testing.T) {
	stored := domain.User{ID: "u-1", Email: "x@y.com"}

	tests := []struct {
		name    string
		in      *domain.User
		setup   func(f *fixture)
		wantOut domain.User
	}{
		{
			name: "delegates to users.Store and returns its result unchanged",
			in: &domain.User{
				Username: "x", Email: "x@y.com", Password: "Pwd_123!", RoleID: domain.RoleUser,
			},
			setup: func(f *fixture) {
				f.users.On("Store", mock.Anything, mock.AnythingOfType("users.StoreRequest")).
					Return(users.StoreResponse{User: stored}, nil).Once()
			},
			wantOut: stored,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := newFixture(t)
			tt.setup(f)

			out, err := f.usecase.Register(context.Background(), auth.RegisterRequest{User: tt.in})
			require.NoError(t, err)
			assert.Equal(t, tt.wantOut, out.User)
		})
	}
}
