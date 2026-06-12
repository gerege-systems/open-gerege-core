// Gerege Template Version 27.0
// Gerege Systems Development Team болон Claude AI хамтран бүтээв, 2026.

package audit_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"template/pkg/audit"
)

// at нь тогтвортой тестийн цаг (микросекунд хүртэл тайрсан — Postgres-ийн
// нарийвчлалтай таарна).
func at(sec int) time.Time {
	return time.Unix(int64(sec), 0).UTC()
}

// TestComputeChainHash_Deterministic нь ижил оролт ижил hash өгөхийг батална.
func TestComputeChainHash_Deterministic(t *testing.T) {
	e := audit.ChainEntry{
		OccurredAt:  at(1000),
		ActorUserID: "u-1",
		Action:      "auth.eid.login",
		Category:    "auth",
		Metadata:    map[string]any{"method": "eid"},
	}
	h1, err := audit.ComputeChainHash("", e)
	require.NoError(t, err)
	h2, err := audit.ComputeChainHash("", e)
	require.NoError(t, err)
	assert.Equal(t, h1, h2)
	assert.NotEmpty(t, h1)
}

// TestChain_VerifyOK нь genesis-ээс хоёр мөрийн гинж байгуулж, дахин тооцоолоход
// бүрэн бүтэн (таарна) гэдгийг шалгана — repository-ийн VerifyChain-ийн цөм
// логикийг DB-гүйгээр тусгаарлан шалгана.
func TestChain_VerifyOK(t *testing.T) {
	e1 := audit.ChainEntry{OccurredAt: at(1000), ActorUserID: "u-1", Action: "org.create", Category: "org", Target: "org-1"}
	e2 := audit.ChainEntry{OccurredAt: at(1001), ActorUserID: "u-2", Action: "rbac.role.permissions.set", Category: "rbac", Target: "3"}

	// Genesis prev = "".
	h1, err := audit.ComputeChainHash("", e1)
	require.NoError(t, err)
	h2, err := audit.ComputeChainHash(h1, e2)
	require.NoError(t, err)

	// Дахин тооцоолж шалгах (verify-walk).
	r1, err := audit.ComputeChainHash("", e1)
	require.NoError(t, err)
	assert.Equal(t, h1, r1, "эхний мөр genesis-ээс зөв тооцоологдох ёстой")
	r2, err := audit.ComputeChainHash(r1, e2)
	require.NoError(t, err)
	assert.Equal(t, h2, r2, "хоёр дахь мөр өмнөх hash дээр зөв холбогдох ёстой")
}

// TestChain_TamperDetected нь мөрийн агуулга өөрчлөгдвөл дахин тооцоолсон hash
// хадгалагдсантай таарахгүй (= илрэх) болохыг батална.
func TestChain_TamperDetected(t *testing.T) {
	original := audit.ChainEntry{OccurredAt: at(1000), ActorUserID: "u-1", Action: "org.create", Category: "org", Target: "org-1"}
	stored, err := audit.ComputeChainHash("", original)
	require.NoError(t, err)

	// Дайсан action-г өөрчилсөн гэж үзье.
	tampered := original
	tampered.Action = "org.delete"
	recomputed, err := audit.ComputeChainHash("", tampered)
	require.NoError(t, err)

	assert.NotEqual(t, stored, recomputed, "өөрчлөгдсөн мөрийн hash хадгалагдсантай таарах ёсгүй")
}

// TestChain_TamperPropagates нь эхний мөрийг өөрчилбөл түүн дээр холбогдсон
// хожуу мөрүүдийн hash бүгд эвдрэхийг (зөвхөн нэг мөр биш) харуулна.
func TestChain_TamperPropagates(t *testing.T) {
	e1 := audit.ChainEntry{OccurredAt: at(1000), ActorUserID: "u-1", Action: "org.create", Category: "org", Target: "org-1"}
	e2 := audit.ChainEntry{OccurredAt: at(1001), ActorUserID: "u-1", Action: "org.member.add", Category: "org", Target: "org-1"}

	h1, _ := audit.ComputeChainHash("", e1)
	h2, _ := audit.ComputeChainHash(h1, e2)

	// e1-г өөрчилснөөр h1 өөрчлөгдөж, улмаар h2-ийн prev (h1) өөрчлөгдөнө.
	e1Tampered := e1
	e1Tampered.Target = "org-evil"
	h1New, _ := audit.ComputeChainHash("", e1Tampered)
	require.NotEqual(t, h1, h1New)
	h2New, _ := audit.ComputeChainHash(h1New, e2)
	assert.NotEqual(t, h2, h2New, "эхний мөрийн засвар хожуу мөрийн hash руу тархах ёстой")
}
