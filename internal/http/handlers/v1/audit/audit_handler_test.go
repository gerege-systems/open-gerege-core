// Government Template Platform V3.0
// Gerege Systems Development Team болон Claude AI хамтран бүтээв, 2026.

// Audit handler-ийн unit тест (mock usecase): хуудаслалтын хязгаар (limit clamp
// max=200, default=50, offset<0→0), action/actor шүүлтүүрийн дамжуулалт,
// VerifyChain-ийн хариу.
package audit_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	audituc "template/internal/business/usecases/audit"
	repointerface "template/internal/datasources/repositories/interface"
	v1 "template/internal/http/handlers/v1"
	audithandler "template/internal/http/handlers/v1/audit"
	"template/internal/test/mocks"
)

func TestAuditListPagination(t *testing.T) {
	cases := []struct {
		name       string
		query      string
		wantLimit  int
		wantOffset int
	}{
		{"defaults", "", 50, 0},
		{"explicit", "?limit=25&offset=10", 25, 10},
		{"limit clamped to max", "?limit=9999", 200, 0},
		{"zero limit → default", "?limit=0", 50, 0},
		{"negative limit → default", "?limit=-3", 50, 0},
		{"negative offset → 0", "?offset=-5", 50, 0},
		{"garbage → defaults", "?limit=abc&offset=xyz", 50, 0},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			uc := mocks.NewAuditUsecase(t)
			uc.On("ListEvents", mock.Anything, mock.Anything, tc.wantLimit, tc.wantOffset).
				Return([]repointerface.AuditLogRow{}, nil).Once()

			h := audithandler.NewHandler(uc)
			rec := httptest.NewRecorder()
			r := httptest.NewRequest(http.MethodGet, "/audit"+tc.query, http.NoBody)
			v1.Wrap(h.List)(rec, r)
			require.Equal(t, http.StatusOK, rec.Code)
		})
	}
}

func TestAuditListForwardsFilter(t *testing.T) {
	uc := mocks.NewAuditUsecase(t)
	uc.On("ListEvents", mock.Anything, mock.MatchedBy(func(f repointerface.AuditListFilter) bool {
		return f.Action == "user.login" && f.ActorUserID == "u1"
	}), 50, 0).Return([]repointerface.AuditLogRow{}, nil).Once()

	h := audithandler.NewHandler(uc)
	rec := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/audit?action=user.login&actor=u1", http.NoBody)
	v1.Wrap(h.List)(rec, r)
}

func TestAuditVerify(t *testing.T) {
	uc := mocks.NewAuditUsecase(t)
	uc.On("VerifyChain", mock.Anything).Return(audituc.VerifyResult{OK: false, BrokenID: 42}, nil).Once()

	h := audithandler.NewHandler(uc)
	rec := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/audit/verify", http.NoBody)
	v1.Wrap(h.Verify)(rec, r)
	require.Equal(t, http.StatusOK, rec.Code)

	var body map[string]any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
	data, _ := body["data"].(map[string]any)
	if data["ok"] != false || data["broken_id"].(float64) != 42 {
		t.Errorf("verify data = %v", data)
	}
}
