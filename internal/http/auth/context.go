// Gerege Template Version 27.0
// Gerege Systems Development Team болон Claude AI хамтран бүтээв, 2026.

// Package auth нь request context-д агуулагдсан JWT claim-г handler-уудад
// хэрэгтэй битүүмжилсэн CurrentUser утга руу зохицуулна.
package auth

import (
	"errors"
	"net/http"

	"template/internal/constants"
	"template/pkg/jwt"
)

// CurrentUser нь баталгаажуулагдсан хүсэлтийн HTTP-давхаргын дүр төрх юм.
type CurrentUser struct {
	ID      string
	Email   string
	IsAdmin bool
	RoleID  int
	JTI     string
}

// ErrNotAuthenticated нь auth middleware хүлээгдэж буй context утгыг
// бөглөөгүй гэсэн үг (route дээр auth middleware суулгаагүй, эсвэл токен
// дээд урсгалд татгалзагдсан).
var ErrNotAuthenticated = errors.New("request is not authenticated")

// CurrentUserFromContext нь хүсэлтийн context-аас баталгаажуулагдсан
// хэрэглэгчийг гаргаж авна. Танигдах claim байхгүй үед
// ErrNotAuthenticated-г буцаана; тийм тохиолдолд handler-ууд 401-ээр
// хариулах ёстой.
func CurrentUserFromContext(r *http.Request) (CurrentUser, error) {
	raw := r.Context().Value(constants.CtxAuthenticatedUserKey)
	if raw == nil {
		return CurrentUser{}, ErrNotAuthenticated
	}
	claims, ok := raw.(jwt.JwtCustomClaim)
	if !ok {
		return CurrentUser{}, ErrNotAuthenticated
	}
	return CurrentUser{
		ID:      claims.UserID,
		Email:   claims.Email,
		IsAdmin: claims.IsAdmin,
		RoleID:  claims.RoleID,
		JTI:     claims.ID,
	}, nil
}
