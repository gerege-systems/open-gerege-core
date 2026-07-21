//go:build integration

// Government Template Platform V3.0
// Gerege Systems Development Team болон Claude AI хамтран бүтээв, 2026.

// Ring R1 — регистрийн repository-ийн integration тест (жинхэнэ Postgres,
// migration 42 хэрэгжсэн). Гараар бичсэн SQL-ийг бодитоор баталгаажуулна:
// паспортын CRUD ба TEXT[] сувгийн scan, нотолгооны жагсаалт солих
// транзакц, once-only view, хувилбарын дугаар атомаар олгогдох нь.
package registry_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"template/internal/business/domain"
	repointerface "template/internal/datasources/repositories/interface"
	registrypg "template/internal/datasources/repositories/postgres/registry"
	"template/internal/test/testenv"
)

// newService нь тестийн жишиг паспорт үүсгэнэ.
func newService(code string) domain.RegistryService {
	return domain.RegistryService{
		Code: code, Name: "Туршилтын үйлчилгээ", Authority: "УБЕГ",
		Channels: []string{"office", "e-mongolia"},
		Fee:      1000, MaxDays: 5, StepsCount: 4, AnnualVolume: 100,
		Proactivity: domain.ProactivityOnline, Status: domain.RegistryStatusDraft,
	}
}

func TestServiceCRUDAndChannels(t *testing.T) {
	repo := registrypg.NewRegistryRepository(testenv.StartPostgres(t))
	ctx := context.Background()

	in := newService("RS_INT_CRUD")
	created, err := repo.CreateService(ctx, &in)
	require.NoError(t, err)
	require.NotEmpty(t, created.ID)
	// TEXT[] → []string scan зөв ажиллаж байгаа эсэх.
	assert.Equal(t, []string{"office", "e-mongolia"}, created.Channels)
	assert.Equal(t, domain.RegistryStatusDraft, created.Status)

	// Кодын давхардал → Conflict.
	dup := newService("RS_INT_CRUD")
	_, err = repo.CreateService(ctx, &dup)
	require.Error(t, err)

	created.Name = "Шинэчилсэн нэр"
	created.Channels = []string{"mobile"}
	updated, err := repo.UpdateService(ctx, &created)
	require.NoError(t, err)
	assert.Equal(t, "Шинэчилсэн нэр", updated.Name)
	assert.Equal(t, []string{"mobile"}, updated.Channels)

	got, err := repo.GetService(ctx, created.ID)
	require.NoError(t, err)
	assert.Equal(t, updated.Name, got.Name)

	require.NoError(t, repo.DeleteService(ctx, created.ID))
	_, err = repo.GetService(ctx, created.ID)
	require.Error(t, err, "устгагдсаны дараа NotFound байх ёстой")
}

func TestListServicesFilters(t *testing.T) {
	repo := registrypg.NewRegistryRepository(testenv.StartPostgres(t))
	ctx := context.Background()

	draft := newService("RS_INT_DRAFT")
	_, err := repo.CreateService(ctx, &draft)
	require.NoError(t, err)

	// Seed-ийн 3 паспорт нийтлэгдсэн; шинэ нь ноорог.
	published, err := repo.ListServices(ctx, repointerface.RegistryFilter{PublishedOnly: true})
	require.NoError(t, err)
	for _, s := range published {
		assert.Equal(t, domain.RegistryStatusPublished, s.Status)
	}

	drafts, err := repo.ListServices(ctx, repointerface.RegistryFilter{Status: domain.RegistryStatusDraft})
	require.NoError(t, err)
	require.Len(t, drafts, 1)
	assert.Equal(t, "RS_INT_DRAFT", drafts[0].Code)

	// Хайлт нь нэр болон кодын аль алинд ажиллана (нэг аргумент, хоёр багана).
	found, err := repo.ListServices(ctx, repointerface.RegistryFilter{Query: "INT_DRAFT"})
	require.NoError(t, err)
	require.Len(t, found, 1)
}

func TestSetServiceEvidencesAndOnceOnly(t *testing.T) {
	repo := registrypg.NewRegistryRepository(testenv.StartPostgres(t))
	ctx := context.Background()

	svcIn := newService("RS_INT_EV")
	svc, err := repo.CreateService(ctx, &svcIn)
	require.NoError(t, err)

	// Нэг ХУР-тай, нэг ХУР-гүй нотолгоо үүсгэнэ.
	khurIn := domain.RegistryEvidence{
		Code: "EV_INT_KHUR", Name: "ХУР-д байгаа лавлагаа",
		InKHUR: true, KHURServiceCode: "WS_TEST_001",
	}
	khur, err := repo.CreateEvidence(ctx, &khurIn)
	require.NoError(t, err)

	paperIn := domain.RegistryEvidence{Code: "EV_INT_PAPER", Name: "Зөвхөн цаасан баримт"}
	paper, err := repo.CreateEvidence(ctx, &paperIn)
	require.NoError(t, err)

	require.NoError(t, repo.SetServiceEvidences(ctx, svc.ID, []domain.RegistryServiceEvidence{
		{EvidenceID: khur.ID, Required: true, FromCitizen: true},  // ⚠ зөрчил
		{EvidenceID: paper.ID, Required: true, FromCitizen: true}, // зөрчил биш
	}))

	got, err := repo.GetService(ctx, svc.ID)
	require.NoError(t, err)
	require.Len(t, got.Evidences, 2)

	docs, err := repo.CountCitizenDocuments(ctx, svc.ID)
	require.NoError(t, err)
	assert.Equal(t, 2, docs)

	// View нь яг нэг зөрчил (ХУР-д байгааг иргэнээс шаардсан) буцаана.
	all, err := repo.OnceOnlyViolations(ctx, "")
	require.NoError(t, err)
	var mine int
	for _, v := range all {
		if v.ServiceID == svc.ID {
			mine++
			assert.Equal(t, "EV_INT_KHUR", v.EvidenceCode)
			assert.Equal(t, "WS_TEST_001", v.KHURServiceCode)
		}
	}
	assert.Equal(t, 1, mine)

	// Жагсаалтыг СОЛИНО (нэмэхгүй) — зөрчилтэйг нь хасахад 0 болно.
	require.NoError(t, repo.SetServiceEvidences(ctx, svc.ID, []domain.RegistryServiceEvidence{
		{EvidenceID: paper.ID, Required: true, FromCitizen: true},
	}))
	got, err = repo.GetService(ctx, svc.ID)
	require.NoError(t, err)
	require.Len(t, got.Evidences, 1)

	// Байхгүй нотолгоо → BadRequest, мөн хуучин жагсаалт эвдрэхгүй (транзакц).
	err = repo.SetServiceEvidences(ctx, svc.ID, []domain.RegistryServiceEvidence{
		{EvidenceID: "00000000-0000-4000-8000-000000000000", Required: true, FromCitizen: true},
	})
	require.Error(t, err)
	got, err = repo.GetService(ctx, svc.ID)
	require.NoError(t, err)
	assert.Len(t, got.Evidences, 1, "бүтэлгүй солилт хуучин жагсаалтыг эвдсэн байна")
}

func TestPublishVersionAssignsSequentialNumbers(t *testing.T) {
	repo := registrypg.NewRegistryRepository(testenv.StartPostgres(t))
	ctx := context.Background()

	svcIn := newService("RS_INT_PUB")
	svc, err := repo.CreateService(ctx, &svcIn)
	require.NoError(t, err)

	// Эхний хувилбар = baseline.
	base := domain.RegistryServiceVersion{
		ServiceID: svc.ID, IsBaseline: true, StepsCount: 11, DocumentsCount: 4, MaxDays: 10, Fee: 1000,
	}
	v1, err := repo.PublishVersion(ctx, &base)
	require.NoError(t, err)
	assert.Equal(t, 1, v1.Version)

	// Хоёр дахь нь дараагийн дугаарыг DB дотор атомаар авна.
	next := domain.RegistryServiceVersion{
		ServiceID: svc.ID, StepsCount: 6, DocumentsCount: 2, MaxDays: 3, Fee: 1000,
		DeltaSteps: -5, DeltaDocuments: -2, DeltaDays: -7,
	}
	v2, err := repo.PublishVersion(ctx, &next)
	require.NoError(t, err)
	assert.Equal(t, 2, v2.Version)
	assert.Equal(t, -5, v2.DeltaSteps)

	// Нийтлэлт нь паспортын статус/хувилбарыг шинэчилсэн байх ёстой.
	got, err := repo.GetService(ctx, svc.ID)
	require.NoError(t, err)
	assert.Equal(t, domain.RegistryStatusPublished, got.Status)
	assert.Equal(t, 2, got.Version)
	require.NotNil(t, got.PublishedAt)

	gotBase, err := repo.BaselineVersion(ctx, svc.ID)
	require.NoError(t, err)
	assert.Equal(t, 1, gotBase.Version)
	assert.True(t, gotBase.IsBaseline)

	versions, err := repo.ListVersions(ctx, svc.ID)
	require.NoError(t, err)
	require.Len(t, versions, 2)
	assert.Equal(t, 2, versions[0].Version, "хамгийн сүүлийн хувилбар эхэнд байх ёстой")
}

func TestOverviewCountsSeedData(t *testing.T) {
	repo := registrypg.NewRegistryRepository(testenv.StartPostgres(t))
	ctx := context.Background()

	o, err := repo.Overview(ctx)
	require.NoError(t, err)
	// Migration 42-ийн seed: 3 үйлчилгээ, 10 нотолгоо, 6 once-only зөрчил.
	assert.Equal(t, 3, o.TotalServices)
	assert.Equal(t, 3, o.PublishedServices)
	assert.Equal(t, 10, o.Evidences)
	assert.Equal(t, 6, o.EvidencesInKHUR)
	assert.Equal(t, 6, o.OnceOnlyViolations)
	assert.Positive(t, o.OnceOnlyAnnualHits)
	assert.NotEmpty(t, o.ByProactivity)
	assert.Positive(t, o.AvgMaxDays)
}

func TestLifeEventsAndEvidenceUpdate(t *testing.T) {
	repo := registrypg.NewRegistryRepository(testenv.StartPostgres(t))
	ctx := context.Background()

	events, err := repo.ListLifeEvents(ctx)
	require.NoError(t, err)
	require.NotEmpty(t, events, "seed-ийн амьдралын үйл явдлууд байх ёстой")

	leIn := domain.RegistryLifeEvent{Code: "LE_INT", Name: "Туршилтын үйл явдал", Kind: "business", SortOrder: 99}
	le, err := repo.CreateLifeEvent(ctx, &leIn)
	require.NoError(t, err)
	assert.Equal(t, "business", le.Kind)
	require.NoError(t, repo.DeleteLifeEvent(ctx, le.ID))

	evIn := domain.RegistryEvidence{Code: "EV_INT_UPD", Name: "Засварлах баримт"}
	ev, err := repo.CreateEvidence(ctx, &evIn)
	require.NoError(t, err)
	assert.False(t, ev.InKHUR)

	// ХУР-д боломжтой болгож тэмдэглэх — once-only зөрчлийг ЗАСАХ гол үйлдэл.
	ev.InKHUR = true
	ev.KHURServiceCode = "WS_TEST_UPD"
	updated, err := repo.UpdateEvidence(ctx, &ev)
	require.NoError(t, err)
	assert.True(t, updated.InKHUR)
	assert.Equal(t, "WS_TEST_UPD", updated.KHURServiceCode)
	require.NotNil(t, updated.UpdatedAt)
}
