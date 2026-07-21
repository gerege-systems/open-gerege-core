//go:build integration

// Government Template Platform V3.0
// Gerege Systems Development Team болон Claude AI хамтран бүтээв, 2026.

// eID нэвтрэлтийн бүрэн end-to-end тест — "апп хүртэл". Fake eID Mongolia v3
// сервер (httptest) нь бодит wire protocol-ийг (device-link/anonymous +
// notification/etsi + session long-poll) хэрэгжүүлж, иргэний eID апп
// баталгаажуулахыг дуурайна (эхний poll RUNNING, дараа нь COMPLETE+OK+person).
// Тест нь БОДИТ eid.Client + БОДИТ auth/users usecase + БОДИТ Postgres/Redis/JWT
// гинжийг ажиллуулж, EIDStart → EIDPoll → хэрэглэгч DB-д үүсэх → JWT хос олгогдох,
// refresh Redis-д хадгалагдахыг шалгана. rpChallenge регрессийг wire түвшинд
// (сервер challenge-ийг хүлээж авсан эсэх) мөн батална.
package auth_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"template/internal/business/usecases/auth"
	"template/internal/business/usecases/users"
	"template/internal/config"
	"template/internal/datasources/caches"
	userspostgres "template/internal/datasources/repositories/postgres/users"
	"template/internal/test/testenv"
	"template/pkg/eid"
	"template/pkg/google"
	"template/pkg/jwt"
	"template/pkg/verify"
)

// fakeEIDServer нь eID Mongolia v3 RP API-гийн бодит wire хэлбэрийг дуурайна.
// Session бүр эхний poll-д RUNNING, хоёр дахьд COMPLETE+OK болно.
type fakeEIDServer struct {
	mu            sync.Mutex
	polls         map[string]int    // sessionID → poll count
	gotChallenge  map[string]string // sessionID → хүлээж авсан rpChallenge (регрессийн шалгалт)
	seq           int
	bearerSeen    string
	civilID       string
	completeAfter int
}

func newFakeEIDServer(civilID string) *fakeEIDServer {
	return &fakeEIDServer{
		polls:         map[string]int{},
		gotChallenge:  map[string]string{},
		civilID:       civilID,
		completeAfter: 1, // 1 RUNNING poll дараа COMPLETE
	}
}

func (s *fakeEIDServer) handler() http.Handler {
	mux := http.NewServeMux()

	// device-link/anonymous (QR) + notification/etsi (push) — хоёулаа adaptation.
	initiate := func(w http.ResponseWriter, r *http.Request, withDeviceLink bool) {
		s.mu.Lock()
		defer s.mu.Unlock()
		s.bearerSeen = r.Header.Get("Authorization")
		var body struct {
			RPChallenge  string `json:"rpChallenge"`
			Hash         string `json:"hash"` // хуучин буруу талбар — байвал регресс
			Interactions []struct {
				Type string `json:"type"`
			} `json:"interactions"`
		}
		_ = json.NewDecoder(r.Body).Decode(&body)
		s.seq++
		sid := "sess-" + strconv.Itoa(s.seq)
		s.gotChallenge[sid] = body.RPChallenge
		w.Header().Set("Content-Type", "application/json")
		if withDeviceLink {
			_ = json.NewEncoder(w).Encode(map[string]any{
				"sessionID": sid, "sessionToken": "tok-" + sid,
				"sessionSecret": "secret", "deviceLinkBase": "https://eidmongolia.mn/dl",
				"vc": "1234",
			})
		} else {
			_ = json.NewEncoder(w).Encode(map[string]any{
				"sessionID": sid,
				"vc":        map[string]string{"type": "alphaNumeric4", "value": "5678"},
			})
		}
	}
	mux.HandleFunc("/authentication/device-link/anonymous", func(w http.ResponseWriter, r *http.Request) {
		initiate(w, r, true)
	})
	mux.HandleFunc("/authentication/notification/etsi/", func(w http.ResponseWriter, r *http.Request) {
		initiate(w, r, false)
	})

	// session long-poll — эхний удаа RUNNING, дараа нь COMPLETE+OK+person.
	mux.HandleFunc("/session/", func(w http.ResponseWriter, r *http.Request) {
		sid := strings.TrimPrefix(r.URL.Path, "/session/")
		s.mu.Lock()
		s.polls[sid]++
		n := s.polls[sid]
		s.mu.Unlock()

		w.Header().Set("Content-Type", "application/json")
		if n <= s.completeAfter {
			_, _ = w.Write([]byte(`{"state":"RUNNING"}`))
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"state":  "COMPLETE",
			"result": map[string]string{"endResult": "OK", "documentNumber": "DOC-1"},
			"cert":   map[string]string{"certificateLevel": "QUALIFIED"},
			"person": map[string]string{
				"givenName": "Бат", "surname": "Дорж",
				"givenNameEn": "Bat", "surnameEn": "Dorj",
				"civilId": s.civilID, "regNo": "1234567",
			},
		})
	})
	return mux
}

// buildEIDStack нь бодит eid.Client-ийг fake сервер рүү холбож, auth/users
// usecase-ийг жинхэнэ Postgres/Redis/JWT дээр угсарна.
func buildEIDStack(t *testing.T, baseURL string) (auth.Usecase, users.Usecase) {
	t.Helper()
	db := testenv.StartPostgres(t)
	redis := testenv.StartRedis(t)

	if config.AppConfig.BcryptCost == 0 {
		config.AppConfig.BcryptCost = 4
	}
	ristretto, err := caches.NewRistrettoCache()
	require.NoError(t, err)
	jwtSvc := jwt.NewJWTServiceWithRefresh("integration-test-secret-thirty-two-chars!", "eid-e2e", 1, 7)

	usersUC := users.NewUsecase(userspostgres.NewUserRepository(db), ristretto, users.Config{BcryptCost: 4})
	eidClient := eid.NewClient(baseURL, "c4f371c3-20bd-462e-8d97-5bc4a20fde08", "template-web", "rp_sk_test", "ADVANCED")
	authUC := auth.NewUsecase(usersUC, jwtSvc, &testenv.FakeVerifier{}, eidClient, nil, google.NewClient("", ""), redis, auth.Config{
		OTPMaxAttempts: 5, OTPTTL: 5 * time.Minute, PasswordResetTTL: 30 * time.Minute,
		BcryptCost: 4, LoginMaxAttempts: 10, LoginLockoutTTL: 15 * time.Minute,
		ForgotMaxAttempts: 3, ForgotLockoutTTL: 15 * time.Minute,
		EIDCallbackURL: "https://template.dgov.mn/login/verify", EIDDisplayText: "template.dgov.mn",
	})
	return authUC, usersUC
}

// pollUntilTerminal нь frontend-ийн EidVerify-г дуурайж COMPLETE хүртэл дахин
// дахин poll хийнэ (fake сервер удаан хүлээхгүй тул хэдхэн давталт хангалттай).
func pollUntilTerminal(t *testing.T, uc auth.Usecase, sessionID string) auth.EIDPollResponse {
	t.Helper()
	for i := 0; i < 5; i++ {
		resp, err := uc.EIDPoll(context.Background(), auth.EIDPollRequest{SessionID: sessionID})
		require.NoError(t, err)
		if resp.State != eid.StateRunning {
			return resp
		}
	}
	t.Fatal("session нэвтрэлт COMPLETE болсонгүй (5 poll)")
	return auth.EIDPollResponse{}
}

func TestE2E_EIDQRLoginCreatesUserAndIssuesTokens(t *testing.T) {
	fake := newFakeEIDServer("УБ99887766")
	srv := httptest.NewServer(fake.handler())
	t.Cleanup(srv.Close)

	authUC, usersUC := buildEIDStack(t, srv.URL)

	// 1. QR нэвтрэлт эхлүүлнэ — QR агуулга нь bare sessionID (demo-той ижил) + vc.
	start, err := authUC.EIDStart(context.Background(), "")
	require.NoError(t, err)
	require.NotEmpty(t, start.SessionID)
	assert.Equal(t, "1234", start.VerificationCode)
	assert.Equal(t, start.SessionID, start.DeviceLinkURL, "QR агуулга нь session UUID байх ёстой")

	// Регресс: сервер rpChallenge-ийг ХООСОН БИШ хүлээж авсан байх (hash биш).
	fake.mu.Lock()
	assert.NotEmpty(t, fake.gotChallenge[start.SessionID], "rpChallenge серверт хоосон ирсэн — hash талбарын регресс")
	assert.True(t, strings.HasPrefix(fake.bearerSeen, "Bearer rp_sk_"), "Bearer secret илгээгдээгүй")
	fake.mu.Unlock()

	// 2. Апп баталгаажуулахыг дуурайж COMPLETE хүртэл poll хийнэ.
	final := pollUntilTerminal(t, authUC, start.SessionID)
	assert.Equal(t, eid.StateComplete, final.State)
	assert.NotEmpty(t, final.AccessToken)
	assert.NotEmpty(t, final.RefreshToken)

	// 3. person блокоос иргэн DB-д бодитоор үүссэн байх (civil_id жижиг үсгээр).
	assert.Equal(t, "уб99887766", final.User.CivilID)
	assert.Equal(t, "Бат", final.User.FirstName)
	assert.Equal(t, "Dorj", final.User.LastNameEn)
	require.NotEmpty(t, final.User.ID)

	// 4. Иргэн Postgres-д бодитоор хадгалагдсаныг репозиторын GetByID-аар батална.
	me, err := usersUC.GetByID(context.Background(), users.GetByIDRequest{ID: final.User.ID})
	require.NoError(t, err)
	assert.Equal(t, final.User.ID, me.User.ID)
	assert.Equal(t, "уб99887766", me.User.CivilID)
}

func TestE2E_EIDPushLoginByNationalID(t *testing.T) {
	fake := newFakeEIDServer("УБ55443322")
	srv := httptest.NewServer(fake.handler())
	t.Cleanup(srv.Close)

	authUC, _ := buildEIDStack(t, srv.URL)

	// РД push — device link байхгүй, зөвхөн session + vc.
	start, err := authUC.EIDStartByNationalID(context.Background(), "УБ55443322", "")
	require.NoError(t, err)
	require.NotEmpty(t, start.SessionID)
	assert.Equal(t, "5678", start.VerificationCode)
	assert.Empty(t, start.DeviceLinkURL)

	final := pollUntilTerminal(t, authUC, start.SessionID)
	assert.Equal(t, eid.StateComplete, final.State)
	assert.Equal(t, "уб55443322", final.User.CivilID)
	assert.NotEmpty(t, final.AccessToken)
}

// ensure verify.Sender-ийн import ашиглагдана (FakeVerifier нь verify.Sender).
var _ verify.Sender = (*testenv.FakeVerifier)(nil)
