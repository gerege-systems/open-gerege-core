// Government Template Platform V3.0
// Gerege Systems Development Team болон Claude AI хамтран бүтээв, 2026.

package registry

import (
	"context"
	"encoding/json"
	"errors"
	"regexp"
	"strings"

	"template/internal/apperror"
	"template/internal/business/domain"
	repointerface "template/internal/datasources/repositories/interface"
)

type usecase struct {
	repo repointerface.RegistryRepository
}

// ── Валидацийн туслахууд ────────────────────────────────────────────────────

// codePattern нь паспорт/нотолгоо/үйл явдлын кодын хэлбэр. Код нь тогтвортой
// таних тэмдэг (интеграци, тайлан, хуулийн иш татлагад хэрэглэгддэг) тул
// зөвхөн том үсэг, тоо, доогуур зураас, зураас зөвшөөрнө.
var codePattern = regexp.MustCompile(`^[A-Z0-9][A-Z0-9_-]{1,63}$`)

// allowedChannels нь CPSV-AP-ийн Channel — Монголын нөхцөлд буулгасан.
var allowedChannels = map[string]bool{
	"office":     true, // танхим / нэг цэгийн үйлчилгээ
	"e-mongolia": true,
	"mobile":     true,
	"phone":      true,
	"post":       true,
}

var allowedProactivity = map[string]bool{
	domain.ProactivityInformation: true,
	domain.ProactivityOnline:      true,
	domain.ProactivityOnceOnly:    true,
	domain.ProactivityProactive:   true,
}

// Дээд хязгаарууд — утгагүй/хог өгөгдлөөс сэргийлнэ.
const (
	maxNameLen     = 300
	maxTextLen     = 4000
	maxDaysLimit   = 3650 // 10 жил
	maxStepsLimit  = 500
	maxVolumeLimit = 100_000_000
)

// isNotFound нь репозиторын NotFound алдааг таних (baseline байхгүй тохиолдол
// нь алдаа биш — эхний нийтлэлт гэсэн үг).
func isNotFound(err error) bool {
	var de *apperror.DomainError
	return errors.As(err, &de) && de.Type == apperror.ErrTypeNotFound
}

// normalizeChannels нь сувгуудыг цэвэрлэж, давхардлыг арилгаж, валидацилна.
func normalizeChannels(in []string) ([]string, error) {
	out := make([]string, 0, len(in))
	seen := map[string]bool{}
	for _, c := range in {
		c = strings.ToLower(strings.TrimSpace(c))
		if c == "" {
			continue
		}
		if !allowedChannels[c] {
			return nil, apperror.BadRequest("unknown channel: " + c)
		}
		if seen[c] {
			continue
		}
		seen[c] = true
		out = append(out, c)
	}
	return out, nil
}

// validateService нь паспортын оролтыг шалгаж, цэвэрлэсэн хувилбарыг буцаана.
// withCode=false үед код шалгагдахгүй (засварын үед код өөрчлөгддөггүй).
func validateService(in ServiceInput, withCode bool) (ServiceInput, error) {
	in.Name = strings.TrimSpace(in.Name)
	in.NameEN = strings.TrimSpace(in.NameEN)
	in.Description = strings.TrimSpace(in.Description)
	in.Authority = strings.TrimSpace(in.Authority)
	in.LegalBasis = strings.TrimSpace(in.LegalBasis)
	in.TargetGroup = strings.TrimSpace(in.TargetGroup)
	in.Output = strings.TrimSpace(in.Output)

	if withCode {
		in.Code = strings.ToUpper(strings.TrimSpace(in.Code))
		if !codePattern.MatchString(in.Code) {
			return in, apperror.BadRequest("code must be 2-64 chars of A-Z, 0-9, _ or -")
		}
	}
	if in.Name == "" {
		return in, apperror.BadRequest("name is required")
	}
	if len(in.Name) > maxNameLen || len(in.NameEN) > maxNameLen {
		return in, apperror.BadRequest("name is too long")
	}
	if len(in.Description) > maxTextLen || len(in.LegalBasis) > maxTextLen {
		return in, apperror.BadRequest("text field is too long")
	}
	// Эрх бүхий байгууллага ба хууль зүйн үндэслэл нь CPSV-AP-ийн цөм талбарууд:
	// эдгээргүй паспорт нь "далд" үйлчилгээ хэвээр үлдэнэ.
	if in.Authority == "" {
		return in, apperror.BadRequest("authority is required")
	}
	if in.Fee < 0 {
		return in, apperror.BadRequest("fee must not be negative")
	}
	if in.MaxDays < 0 || in.MaxDays > maxDaysLimit {
		return in, apperror.BadRequest("max_days is out of range")
	}
	if in.StepsCount < 0 || in.StepsCount > maxStepsLimit {
		return in, apperror.BadRequest("steps_count is out of range")
	}
	if in.AnnualVolume < 0 || in.AnnualVolume > maxVolumeLimit {
		return in, apperror.BadRequest("annual_volume is out of range")
	}

	if in.Proactivity == "" {
		in.Proactivity = domain.ProactivityInformation
	}
	in.Proactivity = strings.ToLower(strings.TrimSpace(in.Proactivity))
	if !allowedProactivity[in.Proactivity] {
		return in, apperror.BadRequest("unknown proactivity level: " + in.Proactivity)
	}

	ch, err := normalizeChannels(in.Channels)
	if err != nil {
		return in, err
	}
	in.Channels = ch
	return in, nil
}

// toDomain нь цэвэрлэгдсэн оролтыг domain entity рүү хөрвүүлнэ.
func toDomain(in ServiceInput) domain.RegistryService {
	return domain.RegistryService{
		Code:           in.Code,
		Name:           in.Name,
		NameEN:         in.NameEN,
		Description:    in.Description,
		Authority:      in.Authority,
		AuthorityOrgID: in.AuthorityOrgID,
		LegalBasis:     in.LegalBasis,
		TargetGroup:    in.TargetGroup,
		Output:         in.Output,
		Channels:       in.Channels,
		Fee:            in.Fee,
		MaxDays:        in.MaxDays,
		StepsCount:     in.StepsCount,
		AnnualVolume:   in.AnnualVolume,
		Proactivity:    in.Proactivity,
		LifeEventID:    in.LifeEventID,
	}
}

// ── Паспорт ─────────────────────────────────────────────────────────────────

func (uc *usecase) ListServices(ctx context.Context, f ListFilter) ([]domain.RegistryService, error) {
	return uc.repo.ListServices(ctx, repointerface.RegistryFilter{
		Status:      f.Status,
		Authority:   f.Authority,
		LifeEventID: f.LifeEventID,
		Proactivity: f.Proactivity,
		Query:       f.Query,
	})
}

// PublicCatalog нь иргэн рүү харсан каталог — статусын шүүлтүүрийг үл хамааран
// ЗӨВХӨН нийтлэгдсэн паспортыг буцаана (ноорог үйлчилгээ гадагш гарахгүй).
func (uc *usecase) PublicCatalog(ctx context.Context, f ListFilter) ([]domain.RegistryService, error) {
	return uc.repo.ListServices(ctx, repointerface.RegistryFilter{
		PublishedOnly: true,
		Authority:     f.Authority,
		LifeEventID:   f.LifeEventID,
		Proactivity:   f.Proactivity,
		Query:         f.Query,
	})
}

func (uc *usecase) GetService(ctx context.Context, id string) (domain.RegistryService, error) {
	return uc.repo.GetService(ctx, id)
}

// PublicService нь нийтийн порталын дэлгэрэнгүй. Нийтлэгдээгүй паспорт байхгүй
// мэт харагдана — "байгаа ч харах эрхгүй" гэдгийг ч задлахгүй.
func (uc *usecase) PublicService(ctx context.Context, id string) (domain.RegistryService, error) {
	svc, err := uc.repo.GetService(ctx, id)
	if err != nil {
		return domain.RegistryService{}, err
	}
	if svc.Status != domain.RegistryStatusPublished {
		return domain.RegistryService{}, apperror.NotFound("service not found")
	}
	return svc, nil
}

func (uc *usecase) CreateService(ctx context.Context, in ServiceInput) (domain.RegistryService, error) {
	clean, err := validateService(in, true)
	if err != nil {
		return domain.RegistryService{}, err
	}
	svc := toDomain(clean)
	// Шинэ паспорт үргэлж ноорогоор эхэлнэ — нийтлэлт нь тусдаа, аудитлагдсан
	// үйлдэл (Publish) бөгөөд тэнд л хувилбар үүснэ.
	svc.Status = domain.RegistryStatusDraft
	return uc.repo.CreateService(ctx, &svc)
}

func (uc *usecase) UpdateService(ctx context.Context, id string, in ServiceInput) (domain.RegistryService, error) {
	clean, err := validateService(in, false)
	if err != nil {
		return domain.RegistryService{}, err
	}
	cur, err := uc.repo.GetService(ctx, id)
	if err != nil {
		return domain.RegistryService{}, err
	}
	if cur.Status == domain.RegistryStatusArchived {
		return domain.RegistryService{}, apperror.Conflict("archived service cannot be edited")
	}
	svc := toDomain(clean)
	svc.ID = id
	return uc.repo.UpdateService(ctx, &svc)
}

func (uc *usecase) DeleteService(ctx context.Context, id string) error {
	cur, err := uc.repo.GetService(ctx, id)
	if err != nil {
		return err
	}
	// Нийтлэгдсэн паспортыг устгахыг хориглоно — түүхэн мөрдөлт (хувилбар,
	// delta, once-only статистик) тасарна. Оронд нь архивлана.
	if cur.Status == domain.RegistryStatusPublished {
		return apperror.Conflict("published service cannot be deleted; archive it instead")
	}
	return uc.repo.DeleteService(ctx, id)
}

func (uc *usecase) ArchiveService(ctx context.Context, id string) error {
	return uc.repo.SetServiceStatus(ctx, id, domain.RegistryStatusArchived)
}

// ── Нотолгооны холбоос ──────────────────────────────────────────────────────

func (uc *usecase) SetEvidences(ctx context.Context, serviceID string, list []EvidenceLink) (domain.RegistryService, error) {
	seen := map[string]bool{}
	out := make([]domain.RegistryServiceEvidence, 0, len(list))
	for _, l := range list {
		id := strings.TrimSpace(l.EvidenceID)
		if id == "" {
			return domain.RegistryService{}, apperror.BadRequest("evidence id is required")
		}
		if seen[id] {
			return domain.RegistryService{}, apperror.BadRequest("duplicate evidence: " + id)
		}
		seen[id] = true
		note := strings.TrimSpace(l.Note)
		if len(note) > maxTextLen {
			return domain.RegistryService{}, apperror.BadRequest("note is too long")
		}
		out = append(out, domain.RegistryServiceEvidence{
			EvidenceID:  id,
			Required:    l.Required,
			FromCitizen: l.FromCitizen,
			Note:        note,
		})
	}
	if err := uc.repo.SetServiceEvidences(ctx, serviceID, out); err != nil {
		return domain.RegistryService{}, err
	}
	return uc.repo.GetService(ctx, serviceID)
}

// ── Once-only ───────────────────────────────────────────────────────────────

// eligibleProactivity нь зөрчилтэй байхад зарлаж болох дээд шатыг буцаана.
// Зөрчилтэй үед once_only/proactive гэж зарлах нь регистрийн үнэн зөвийг
// алдагдуулна (Эстонийн once-only зарчим).
func eligibleProactivity(violations int, claimed string) string {
	if violations == 0 {
		return claimed
	}
	if claimed == domain.ProactivityOnceOnly || claimed == domain.ProactivityProactive {
		return domain.ProactivityOnline
	}
	return claimed
}

func (uc *usecase) CheckOnceOnly(ctx context.Context, serviceID string) (OnceOnlyReport, error) {
	svc, err := uc.repo.GetService(ctx, serviceID)
	if err != nil {
		return OnceOnlyReport{}, err
	}
	rep := OnceOnlyReport{
		ServiceID:   svc.ID,
		ServiceCode: svc.Code,
		ServiceName: svc.Name,
		Violations:  make([]domain.RegistryServiceEvidence, 0, 4),
	}
	for _, e := range svc.Evidences {
		if !e.FromCitizen {
			continue
		}
		rep.CitizenDocuments++
		// Иргэнээс шаардаж байгаа АТАЛ ХУР-д аль хэдийн байгаа = зөрчил.
		if e.InKHUR {
			rep.Violations = append(rep.Violations, e)
		}
	}
	rep.Compliant = len(rep.Violations) == 0
	rep.EligibleProactivity = eligibleProactivity(len(rep.Violations), svc.Proactivity)
	return rep, nil
}

func (uc *usecase) OnceOnlyViolations(ctx context.Context, authority string) ([]domain.RegistryOnceOnlyViolation, error) {
	return uc.repo.OnceOnlyViolations(ctx, strings.TrimSpace(authority))
}

// ── Нийтлэлт (хувилбар + baseline delta) ────────────────────────────────────

// Publish нь паспортын одоогийн төлөвийг шинэ хувилбар болгон бэхэлж, түүнийг
// baseline-тай харьцуулсан delta-тай хамт хадгална. Эхний нийтлэлт нь өөрөө
// baseline болно — дараагийн бүх сайжралт үүнтэй харьцуулагдана.
func (uc *usecase) Publish(ctx context.Context, serviceID string, in PublishInput) (domain.RegistryServiceVersion, error) {
	svc, err := uc.repo.GetService(ctx, serviceID)
	if err != nil {
		return domain.RegistryServiceVersion{}, err
	}
	if svc.Status == domain.RegistryStatusArchived {
		return domain.RegistryServiceVersion{}, apperror.Conflict("archived service cannot be published")
	}

	// Зарласан проактив байдлын шатыг бодит once-only байдалтай тулгана —
	// регистр өөрөө худал мэдээлэл агуулахаас сэргийлнэ.
	rep, err := uc.CheckOnceOnly(ctx, serviceID)
	if err != nil {
		return domain.RegistryServiceVersion{}, err
	}
	if rep.EligibleProactivity != svc.Proactivity {
		return domain.RegistryServiceVersion{}, apperror.Conflict(
			"cannot publish as '" + svc.Proactivity + "': service still requests data already available in KHUR")
	}

	docs, err := uc.repo.CountCitizenDocuments(ctx, serviceID)
	if err != nil {
		return domain.RegistryServiceVersion{}, err
	}

	ver := domain.RegistryServiceVersion{
		ServiceID:      serviceID,
		ChangeNote:     strings.TrimSpace(in.ChangeNote),
		StepsCount:     svc.StepsCount,
		DocumentsCount: docs,
		MaxDays:        svc.MaxDays,
		Fee:            svc.Fee,
		PublishedBy:    in.PublishedBy,
	}

	base, err := uc.repo.BaselineVersion(ctx, serviceID)
	switch {
	case isNotFound(err):
		// Эхний нийтлэлт — энэ мөр нь baseline (delta бүгд 0).
		ver.IsBaseline = true
	case err != nil:
		return domain.RegistryServiceVersion{}, err
	default:
		// Сөрөг delta = сайжралт (алхам/баримт/хугацаа буурсан).
		ver.DeltaSteps = svc.StepsCount - base.StepsCount
		ver.DeltaDocuments = docs - base.DocumentsCount
		ver.DeltaDays = svc.MaxDays - base.MaxDays
		ver.DeltaFee = svc.Fee - base.Fee
	}

	// Snapshot — нийтлэх мөчийн паспортын бүтэн хуулбар (маргаангүй түүх).
	if snap, mErr := json.Marshal(svc); mErr == nil {
		ver.Snapshot = snap
	} else {
		return domain.RegistryServiceVersion{}, apperror.InternalCause(mErr)
	}

	return uc.repo.PublishVersion(ctx, &ver)
}

func (uc *usecase) ListVersions(ctx context.Context, serviceID string) ([]domain.RegistryServiceVersion, error) {
	return uc.repo.ListVersions(ctx, serviceID)
}

// ── Нотолгооны каталог ──────────────────────────────────────────────────────

func validateEvidence(in EvidenceInput, withCode bool) (EvidenceInput, error) {
	in.Name = strings.TrimSpace(in.Name)
	in.Description = strings.TrimSpace(in.Description)
	in.HolderAgency = strings.TrimSpace(in.HolderAgency)
	in.SourceSystem = strings.TrimSpace(in.SourceSystem)
	in.KHURServiceCode = strings.TrimSpace(in.KHURServiceCode)

	if withCode {
		in.Code = strings.ToUpper(strings.TrimSpace(in.Code))
		if !codePattern.MatchString(in.Code) {
			return in, apperror.BadRequest("code must be 2-64 chars of A-Z, 0-9, _ or -")
		}
	}
	if in.Name == "" {
		return in, apperror.BadRequest("name is required")
	}
	if len(in.Name) > maxNameLen {
		return in, apperror.BadRequest("name is too long")
	}
	if len(in.Description) > maxTextLen {
		return in, apperror.BadRequest("description is too long")
	}
	// ХУР-д байгаа гэж тэмдэглэсэн бол аль лавлагаагаар авахыг заана —
	// once-only зөрчлийг ЗАСАХ заавар болно (зөвхөн илрүүлээд орхихгүй).
	if in.InKHUR && in.KHURServiceCode == "" {
		return in, apperror.BadRequest("khur_service_code is required when in_khur is set")
	}
	return in, nil
}

func (uc *usecase) ListEvidences(ctx context.Context) ([]domain.RegistryEvidence, error) {
	return uc.repo.ListEvidences(ctx)
}

func (uc *usecase) CreateEvidence(ctx context.Context, in EvidenceInput) (domain.RegistryEvidence, error) {
	clean, err := validateEvidence(in, true)
	if err != nil {
		return domain.RegistryEvidence{}, err
	}
	ev := domain.RegistryEvidence{
		Code: clean.Code, Name: clean.Name, Description: clean.Description,
		HolderAgency: clean.HolderAgency, SourceSystem: clean.SourceSystem,
		InKHUR: clean.InKHUR, KHURServiceCode: clean.KHURServiceCode,
	}
	return uc.repo.CreateEvidence(ctx, &ev)
}

func (uc *usecase) UpdateEvidence(ctx context.Context, id string, in EvidenceInput) (domain.RegistryEvidence, error) {
	clean, err := validateEvidence(in, false)
	if err != nil {
		return domain.RegistryEvidence{}, err
	}
	ev := domain.RegistryEvidence{
		ID: id, Name: clean.Name, Description: clean.Description,
		HolderAgency: clean.HolderAgency, SourceSystem: clean.SourceSystem,
		InKHUR: clean.InKHUR, KHURServiceCode: clean.KHURServiceCode,
	}
	return uc.repo.UpdateEvidence(ctx, &ev)
}

func (uc *usecase) DeleteEvidence(ctx context.Context, id string) error {
	return uc.repo.DeleteEvidence(ctx, id)
}

// ── Амьдралын үйл явдал ─────────────────────────────────────────────────────

func (uc *usecase) ListLifeEvents(ctx context.Context) ([]domain.RegistryLifeEvent, error) {
	return uc.repo.ListLifeEvents(ctx)
}

func (uc *usecase) CreateLifeEvent(ctx context.Context, in LifeEventInput) (domain.RegistryLifeEvent, error) {
	in.Code = strings.ToUpper(strings.TrimSpace(in.Code))
	in.Name = strings.TrimSpace(in.Name)
	in.Kind = strings.ToLower(strings.TrimSpace(in.Kind))
	if !codePattern.MatchString(in.Code) {
		return domain.RegistryLifeEvent{}, apperror.BadRequest("code must be 2-64 chars of A-Z, 0-9, _ or -")
	}
	if in.Name == "" {
		return domain.RegistryLifeEvent{}, apperror.BadRequest("name is required")
	}
	if in.Kind == "" {
		in.Kind = "life"
	}
	if in.Kind != "life" && in.Kind != "business" {
		return domain.RegistryLifeEvent{}, apperror.BadRequest("kind must be 'life' or 'business'")
	}
	le := domain.RegistryLifeEvent{
		Code: in.Code, Name: in.Name, Kind: in.Kind,
		Description: strings.TrimSpace(in.Description),
		LeadAgency:  strings.TrimSpace(in.LeadAgency),
		SortOrder:   in.SortOrder,
	}
	return uc.repo.CreateLifeEvent(ctx, &le)
}

func (uc *usecase) DeleteLifeEvent(ctx context.Context, id string) error {
	return uc.repo.DeleteLifeEvent(ctx, id)
}

// ── Нэгтгэл ─────────────────────────────────────────────────────────────────

func (uc *usecase) Overview(ctx context.Context) (domain.RegistryOverview, error) {
	return uc.repo.Overview(ctx)
}
