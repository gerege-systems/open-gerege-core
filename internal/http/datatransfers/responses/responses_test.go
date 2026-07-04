// Gerege Template Version 27.0
// Gerege Systems Development Team болон Claude AI хамтран бүтээв, 2026.

// Response mapper-уудын unit тест. Онцгой анхаарал: FromEIDPollResponse нь
// COMPLETE БИШ төлөвт токен/хэрэглэгчийн мэдээллийг бөглөх ЁСГҮЙ (RUNNING үед
// токен алдагдвал аюулгүй байдлын цоорхой). Мөн domain→DTO талбар зохицуулалт.
package responses_test

import (
	"testing"

	"template/internal/business/domain"
	"template/internal/business/usecases/auth"
	"template/internal/http/datatransfers/responses"
)

func sampleUser() domain.User {
	return domain.User{
		ID: "u1", Username: "bat", FirstName: "Бат", LastName: "Дорж",
		Email: "bat@example.com", RoleID: 2,
	}
}

func TestFromV1Domain(t *testing.T) {
	r := responses.FromV1Domain(sampleUser())
	if r.Id != "u1" || r.Username != "bat" || r.Email != "bat@example.com" || r.RoleId != 2 {
		t.Errorf("mapped = %+v", r)
	}
	// Нууц үгийн hash DTO-д ил гарах ёсгүй (UserResponse-д ийм талбар байхгүй).
}

func TestFromLoginResponseCarriesTokens(t *testing.T) {
	r := responses.FromLoginResponse(auth.LoginResponse{
		User: sampleUser(), AccessToken: "acc", RefreshToken: "ref",
	})
	if r.Token != "acc" || r.RefreshToken != "ref" {
		t.Errorf("login токенууд буруу: %+v", r)
	}
	if r.Id != "u1" {
		t.Errorf("хэрэглэгчийн талбарууд алга: %+v", r)
	}
}

func TestFromEIDStartResponse(t *testing.T) {
	r := responses.FromEIDStartResponse(auth.EIDStartResponse{
		SessionID: "s1", DeviceLinkURL: "s1", VerificationCode: "1234", ExpiresAt: "",
	})
	if r.SessionID != "s1" || r.DeviceLinkURL != "s1" || r.VerificationCode != "1234" {
		t.Errorf("mapped = %+v", r)
	}
}

func TestFromEIDPollResponse_TokensOnlyWhenComplete(t *testing.T) {
	t.Run("RUNNING carries no user or tokens", func(t *testing.T) {
		r := responses.FromEIDPollResponse(auth.EIDPollResponse{
			State: "RUNNING", User: sampleUser(), AccessToken: "acc", RefreshToken: "ref",
		})
		if r.State != "RUNNING" {
			t.Errorf("state = %s", r.State)
		}
		if r.Token != "" || r.RefreshToken != "" {
			t.Errorf("RUNNING үед токен алдагдсан: token=%q refresh=%q", r.Token, r.RefreshToken)
		}
		if r.Id != "" {
			t.Errorf("RUNNING үед хэрэглэгчийн мэдээлэл алдагдсан: %+v", r.UserResponse)
		}
	})

	for _, state := range []string{"EXPIRED", "REFUSED"} {
		t.Run(state+" carries no tokens", func(t *testing.T) {
			r := responses.FromEIDPollResponse(auth.EIDPollResponse{
				State: state, User: sampleUser(), AccessToken: "acc", RefreshToken: "ref",
			})
			if r.Token != "" || r.Id != "" {
				t.Errorf("%s үед токен/хэрэглэгч алдагдсан: %+v", state, r)
			}
		})
	}

	t.Run("COMPLETE carries user and tokens", func(t *testing.T) {
		r := responses.FromEIDPollResponse(auth.EIDPollResponse{
			State: "COMPLETE", User: sampleUser(), AccessToken: "acc", RefreshToken: "ref",
		})
		if r.State != "COMPLETE" || r.Token != "acc" || r.RefreshToken != "ref" || r.Id != "u1" {
			t.Errorf("COMPLETE mapping буруу: %+v", r)
		}
	})
}
