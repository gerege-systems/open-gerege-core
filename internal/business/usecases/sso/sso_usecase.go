// Gerege Template Version 27.0
// Gerege Systems Development Team болон Claude AI хамтран бүтээв, 2026.

// Package sso нь Gerege SSO (sso.gerege.mn, OIDC) нэвтрэлтийн 2 дахь урсгал —
// eID-ийн зэрэгцээ нэвтрэх арга. Authorization Code flow: Start нь authorize URL
// (state-тэй) буцаана, Complete нь callback-ийн code-ийг солиж, иргэнийг
// sso_sub-ээр upsert хийж, өөрийн JWT хос олгоно (login-тэй ижил session).
package sso

import (
	"context"

	"template/internal/business/domain"
)

// UserStore нь SSO иргэнийг users хүснэгтэд upsert хийх repo (postgres/ssouser).
type UserStore interface {
	UpsertBySSOSub(ctx context.Context, ssoSub string, in *domain.User) (domain.User, error)
}

// StartResponse нь browser-ийг чиглүүлэх SSO authorize URL.
type StartResponse struct {
	AuthURL string
}

// CompleteResponse нь callback дуусахад олгосон токен хос + хэрэглэгч.
type CompleteResponse struct {
	Token        string
	RefreshToken string
	User         domain.User
}

// Usecase нь SSO нэвтрэлтийн урсгал.
type Usecase interface {
	// Configured нь SSO client бүрэн тохируулагдсан (нэвтрэлт идэвхтэй) эсэх.
	Configured() bool
	// Start нь шинэ state үүсгэж (Redis-д хадгалж), authorize URL буцаана.
	Start(ctx context.Context) (StartResponse, error)
	// Complete нь callback-ийн state-ийг шалгаж, code-ийг солиж, /userinfo-оос
	// иргэнийг тодорхойлж upsert хийн, JWT хос олгоно.
	Complete(ctx context.Context, state, code string) (CompleteResponse, error)
}
