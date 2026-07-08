// Gerege Template Version 27.0
// Gerege Systems Development Team болон Claude AI хамтран бүтээв, 2026.

// Package _interface нь repositories давхарга дахь домэйн бүрийн
// gateway хийсвэрлэлийг агуулна. Package-ийн нэр "_interface" байгаа
// шалтгаан нь "interface" нь Go-гийн нөөц түлхүүр үг бөгөөд шууд
// identifier болгон ашиглах боломжгүй; эхэнд тавьсан доогуур зураас
// нь үзэл баримтлалын утгыг өөрчлөхгүйгээр түүнийг хүчинтэй identifier
// болгон үлдээдэг.
//
// Тодорхой adapter-ууд (postgres/, ирээдүйн mongo/, г.м.) эдгээр
// interface-үүдийг хэрэгжүүлдэг бөгөөд энэ package-ийн ах дүүс болж
// оршино. Usecase давхарга нь зөвхөн энэ package-аас хамаардаг —
// хэзээ ч тодорхой adapter-аас хамаардаггүй — тиймээс хадгалалтын
// engine-г солих нь business код руу нэвчдэггүй.
package _interface

import (
	"context"
	"time"

	"template/internal/business/domain"

	"template/pkg/audit"
)

// UserListFilter нь UserRepository.List() үр дүнг нарийсгана. Талбар
// бүр сонголттой; хоосон утга нь "энэ хэмжээст шүүлтгүй" гэсэн үг.
// Домэйн угтвартай (UserListFilter, ирээдүйн ProductListFilter) тул
// олон шүүлтийн төрөл энэ хуваалцсан package-д мөргөлдөөнгүйгээр
// зэрэгцэн оршиж чадна.
type UserListFilter struct {
	RoleID         int  // 0 = аль ч role
	ActiveOnly     bool // true = зөвхөн active=true хэрэглэгчид
	IncludeDeleted bool // false (өгөгдмөл) = WHERE deleted_at IS NULL
}

// UserRepository нь хэрэглэгчдийг ачаалах болон хадгалах gateway юм.
type UserRepository interface {
	// Store нь хэрэглэгчийг оруулж, хадгалагдсан мөрийг нэг round-trip-д
	// буцаадаг тул дуудагчдад дараагийн GetByEmail хэрэггүй (амжилтгүй
	// бол INSERT-г өнчрүүлэх байсан). Давхардсан username/email нь
	// apperror.Conflict болж гарна.
	Store(ctx context.Context, in *domain.User) (domain.User, error)
	// GetByEmail нь soft-delete хийгдсэн мөрүүдийг хасч, email-ээр
	// хэрэглэгчийг хайна. Тохирох мөр байхгүй үед apperror.NotFound-г
	// буцаана.
	GetByEmail(ctx context.Context, in *domain.User) (out domain.User, err error)
	// GetByID нь soft-delete хийгдсэн мөрүүдийг хасч, primary key-ээр
	// хэрэглэгчийг хайна. Тохирох мөр байхгүй үед apperror.NotFound-г
	// буцаана.
	GetByID(ctx context.Context, id string) (domain.User, error)
	// GetByGoogleSub нь холбогдсон Google account (sub)-аар хэрэглэгчийг хайна.
	// Холбоогүй бол apperror.NotFound. (Google callback дахь pre-auth хайлт —
	// service RLS дор ажиллана.)
	GetByGoogleSub(ctx context.Context, sub string) (domain.User, error)
	// LinkGoogleAccount нь userID-тай хэрэглэгчид Google account + профайлыг
	// (email, нэр, зураг г.м.) холбоно/шинэчилнэ (eID-ээр баталгаажсаны дараа
	// эсвэл дараагийн нэвтрэлтэд). Давхардсан sub нь apperror.Conflict. Анх
	// холбосон огноог (google_linked_at) нэг л удаа тэмдэглэнэ.
	LinkGoogleAccount(ctx context.Context, userID string, acct domain.GoogleAccount) error
	// UnlinkGoogle нь хэрэглэгчийн Google холболтыг (sub + профайл) арилгана.
	UnlinkGoogle(ctx context.Context, userID string) error
	// GetByNationalID нь soft-delete хийгдсэн мөрүүдийг хасч, eID-ийн
	// national_id-ээр (жижиг үсгээр) хэрэглэгчийг хайна. Тохирох мөр байхгүй
	// үед apperror.NotFound-г буцаана.
	GetByNationalID(ctx context.Context, nationalID string) (domain.User, error)
	// UpsertFromEID нь eID identity-аар хэрэглэгчийг үүсгэх/шинэчлэх. national_id
	// аль хэдийн байгаа бол нэр/kyc-г шинэчилж, идэвхжүүлж, тухайн мөрийг
	// буцаана; эс бөгөөс шинэ идэвхтэй мөр оруулна. Бүгд нэг round-trip
	// (INSERT … ON CONFLICT … RETURNING).
	UpsertFromEID(ctx context.Context, in *domain.User) (domain.User, error)
	// List нь filter-т тохирох хэрэглэгчдийг offset/limit-ээр хуудаслан
	// буцаана. Limit нь сервер талд хатуу хязгаарлагдсан тул буруу
	// ажиллаж буй дуудагч бүх хүснэгтийг татаж чадахгүй.
	List(ctx context.Context, filter UserListFilter, offset, limit int) ([]domain.User, error)
	// ListAdmins нь админ түвшний бүх бүртгэлийг (super admin + admin) буцаана —
	// super admin-ий "админуудыг удирдах" хуудсанд зориулагдсан. Зэрэглэлээр
	// (role_id өсөхөөр), дараа нь шинээр үүсгэснээр эрэмбэлж, soft-delete
	// хийгдсэнийг хасна.
	ListAdmins(ctx context.Context) ([]domain.User, error)
	// ChangeActiveUser нь active flag-г сольдог (OTP-verify урсгалд
	// ашиглагддаг) ба updated_at-г тэмдэглэнэ. Soft-delete хийгдсэн
	// мөрүүд дээр no-op.
	ChangeActiveUser(ctx context.Context, in *domain.User) (err error)
	// UpdatePassword нь bcrypt hash-г сольж, password_changed_at +
	// updated_at-г тэмдэглэнэ. Хэрэглэгч байхгүй/soft-delete хийгдсэн бол
	// apperror.NotFound-г буцаана.
	UpdatePassword(ctx context.Context, in *domain.User) error
	// SoftDelete нь deleted_at = NOW() гэж тогтоодог тул мөр нь
	// audit/сэргээх зорилгоор хүснэгтэд хэвээр үлддэг боловч өгөгдмөл
	// query-үүдтэй таарахаа болино. Мөр байхгүй эсвэл аль хэдийн устгагдсан
	// бол apperror.NotFound-г буцаана.
	SoftDelete(ctx context.Context, id string) error
	// UpdateRole нь хэрэглэгчийн role_id-г солино (admin удирдлага). Мөр
	// байхгүй/soft-delete хийгдсэн бол apperror.NotFound буцаана.
	UpdateRole(ctx context.Context, id string, roleID int) error
	// GetSignature нь хэрэглэгчийн гарын үсгийн зургийг (data-URL) буцаана (хоосон бол "").
	GetSignature(ctx context.Context, userID string) (string, error)
	// SetSignature нь гарын үсгийн зургийг тавина/шинэчилнэ; хоосон img нь устгана.
	SetSignature(ctx context.Context, userID, img string) error
	// SetLatinName нь хэрэглэгчийн латин нэрийг (first_name_en/last_name_en) гараар засна.
	SetLatinName(ctx context.Context, userID, firstEn, lastEn string) error
}

// RBACRepository нь динамик role-ууд болон тэдгээрийн эрхийг (role↔permission)
// хадгалах/уншихыг хариуцна. Permission каталог нь код дотор тодорхойлогддог тул
// энд зөвхөн уншина (ListPermissions нь seed хийгдсэн каталогийг буцаана).
type RBACRepository interface {
	ListRoles(ctx context.Context) ([]domain.Role, error)
	GetRole(ctx context.Context, id int) (domain.Role, error)
	CreateRole(ctx context.Context, in *domain.Role) (domain.Role, error)
	UpdateRole(ctx context.Context, in *domain.Role) (domain.Role, error)
	DeleteRole(ctx context.Context, id int) error
	// CountUsersWithRole нь тухайн role-д оноогдсон (soft-delete хийгдээгүй)
	// хэрэглэгчдийн тоог буцаана — ашиглагдаж буй role-ийг устгуулахгүйн тулд.
	CountUsersWithRole(ctx context.Context, roleID int) (int, error)
	ListPermissions(ctx context.Context) ([]domain.Permission, error)
	GetRolePermissions(ctx context.Context, roleID int) ([]string, error)
	SetRolePermissions(ctx context.Context, roleID int, keys []string) error
}

// GatewayRepository нь API Gateway-ийн тохиргоо (services/routes/consumers/
// api keys/policies) болон request-log телеметрийг хадгалах/уншихыг хариуцна.
// Эдгээр нь хэрэглэгч-тус-бүрийн биш тул RLS-д хамаарахгүй (plain pool query).
type GatewayRepository interface {
	// Services — upstream backend.
	ListServices(ctx context.Context) ([]domain.GatewayService, error)
	GetService(ctx context.Context, id string) (domain.GatewayService, error)
	CreateService(ctx context.Context, in *domain.GatewayService) (domain.GatewayService, error)
	UpdateService(ctx context.Context, in *domain.GatewayService) (domain.GatewayService, error)
	DeleteService(ctx context.Context, id string) error

	// Routes — (methods, paths) → service.
	ListRoutes(ctx context.Context) ([]domain.GatewayRoute, error)
	GetRoute(ctx context.Context, id string) (domain.GatewayRoute, error)
	CreateRoute(ctx context.Context, in *domain.GatewayRoute) (domain.GatewayRoute, error)
	UpdateRoute(ctx context.Context, in *domain.GatewayRoute) (domain.GatewayRoute, error)
	DeleteRoute(ctx context.Context, id string) error

	// Consumers — API client + (тоологдсон) key тоо.
	ListConsumers(ctx context.Context) ([]domain.GatewayConsumer, error)
	GetConsumer(ctx context.Context, id string) (domain.GatewayConsumer, error)
	CreateConsumer(ctx context.Context, in *domain.GatewayConsumer) (domain.GatewayConsumer, error)
	UpdateConsumer(ctx context.Context, in *domain.GatewayConsumer) (domain.GatewayConsumer, error)
	DeleteConsumer(ctx context.Context, id string) error

	// API keys — hash нь usecase-д урьдчилан тооцоологдоно.
	ListKeys(ctx context.Context, consumerID string) ([]domain.GatewayAPIKey, error)
	CreateKey(ctx context.Context, in *domain.GatewayAPIKey) (domain.GatewayAPIKey, error)
	RevokeKey(ctx context.Context, id string) error
	DeleteKey(ctx context.Context, id string) error

	// Policies — route-д (эсвэл global) хавсаргасан plugin.
	ListPolicies(ctx context.Context) ([]domain.GatewayPolicy, error)
	GetPolicy(ctx context.Context, id string) (domain.GatewayPolicy, error)
	CreatePolicy(ctx context.Context, in *domain.GatewayPolicy) (domain.GatewayPolicy, error)
	UpdatePolicy(ctx context.Context, in *domain.GatewayPolicy) (domain.GatewayPolicy, error)
	DeletePolicy(ctx context.Context, id string) error

	// Telemetry — сүүлийн log-ууд + dashboard-ийн нэгтгэл.
	ListRequestLogs(ctx context.Context, limit int) ([]domain.GatewayRequestLog, error)
	Overview(ctx context.Context) (domain.GatewayOverview, error)
}

// OrgRepository нь байгууллага болон гишүүнчлэлийг (organization_memberships)
// хадгалах/уншихыг хариуцна. Бичих үйлдлүүд (CreateOrg, AddMember, ...) нь
// тухайн дуудагч бизнесийн эрх (owner/admin) шалгалтыг usecase давхаргад аль
// хэдийн давсан гэж үздэг; repository нь RLS-ийн "service" GUC дор бичдэг тул
// шинээр үүсгэгдсэн org/membership мөрүүд (хэрэглэгч хараахан гишүүн болоогүй
// үед ч) бичигдэж чадна. Уншилтууд нь дуудагчийн user/admin identity-аар
// (RLS-ийн гишүүнчлэлд суурилсан харагдах байдал) явна.
type OrgRepository interface {
	// CreateOrg нь байгууллага оруулж, үүсгэгчийг owner гишүүн болгож, нэг
	// транзакцид хадгалаад хадгалагдсан мөрийг буцаана. reg_no давхцвал
	// (case-insensitive) apperror.Conflict болж гарна.
	CreateOrg(ctx context.Context, in *domain.Organization) (domain.Organization, error)
	// GetOrgByID нь soft-delete хийгдсэн мөрүүдийг хасч, primary key-ээр
	// байгууллагыг хайна. Олдоогүй үед apperror.NotFound.
	GetOrgByID(ctx context.Context, id string) (domain.Organization, error)
	// GetOrgByRegNo нь reg_no-оор (case-insensitive) байгууллагыг хайна.
	// Олдоогүй үед apperror.NotFound.
	GetOrgByRegNo(ctx context.Context, regNo string) (domain.Organization, error)
	// ListOrgsForUser нь тухайн хэрэглэгч гишүүн болсон бүх байгууллагыг буцаана.
	ListOrgsForUser(ctx context.Context, userID string) ([]domain.Organization, error)
	// GetMembership нь (orgID, userID) хосын гишүүнчлэлийг буцаана. Олдоогүй
	// үед apperror.NotFound — энэ нь usecase-д эрх шалгахад ашиглагдана.
	GetMembership(ctx context.Context, orgID, userID string) (domain.OrganizationMembership, error)
	// ListMembers нь тухайн байгууллагын бүх гишүүнийг буцаана.
	ListMembers(ctx context.Context, orgID string) ([]domain.OrganizationMembership, error)
	// AddMember нь гишүүн нэмнэ. Аль хэдийн гишүүн бол apperror.Conflict.
	AddMember(ctx context.Context, in *domain.OrganizationMembership) (domain.OrganizationMembership, error)
	// UpdateMemberRole нь гишүүний дүрийг солино. Гишүүн биш бол apperror.NotFound.
	UpdateMemberRole(ctx context.Context, orgID, userID, role string) error
	// RemoveMember нь гишүүнийг хасна. Гишүүн биш бол apperror.NotFound.
	RemoveMember(ctx context.Context, orgID, userID string) error
}

// GovRepository нь иргэний "Төрийн үйлчилгээ" порталын өгөгдлийг хариуцна.
// Каталог (ListServices) нь нийтийн; бусад нь хэрэглэгч-тус-бүрийн тул query
// бүр userID-гаар scope хийгдэхээс гадна per-user хүснэгтүүд RLS-тэй (repo нь
// withRLS транзакцид app.user_id/app.user_role GUC тавьдаг — migration 20).
type GovRepository interface {
	// Каталог
	ListServices(ctx context.Context) ([]domain.GovService, error)
	GetService(ctx context.Context, id string) (domain.GovService, error)

	// Хүсэлт
	ListApplications(ctx context.Context, userID string) ([]domain.GovApplication, error)
	CreateApplication(ctx context.Context, in *domain.GovApplication) (domain.GovApplication, error)
	SetApplicationStatus(ctx context.Context, userID, id, status string) error

	// Лавлагаа
	ListReferences(ctx context.Context, userID string) ([]domain.GovReference, error)
	CreateReference(ctx context.Context, in *domain.GovReference) (domain.GovReference, error)

	// Мэдэгдэл
	ListNotifications(ctx context.Context, userID string) ([]domain.GovNotification, error)
	MarkNotificationRead(ctx context.Context, userID, id string) error
	MarkAllNotificationsRead(ctx context.Context, userID string) error

	// Төлбөр
	ListPayments(ctx context.Context, userID string) ([]domain.GovPayment, error)
	PayPayment(ctx context.Context, userID, id string) error

	// Цаг захиалга
	ListAppointments(ctx context.Context, userID string) ([]domain.GovAppointment, error)
	CreateAppointment(ctx context.Context, in *domain.GovAppointment) (domain.GovAppointment, error)
	CancelAppointment(ctx context.Context, userID, id string) error

	// Нэгтгэл + lazy demo seed
	Overview(ctx context.Context, userID string) (domain.GovOverview, error)
	CountUserRows(ctx context.Context, userID string) (int, error)
	SeedDemoData(ctx context.Context, userID string) error
}

// AIRepository нь AI туслахын тохируулдаг prompt давхаргууд болон мэдлэгийн
// санг (knowledge base) хадгалах/уншихыг хариуцна. Suurь (base) дүрэм кодод
// хатуу бичигдсэн тул эндээс зөвхөн scope/instructions давхарга уншигдана.
type AIRepository interface {
	// ListPrompts нь тохируулдаг бүх prompt давхаргыг буцаана.
	ListPrompts(ctx context.Context) ([]domain.AIPrompt, error)
	// SetPrompt нь нэг давхаргын агуулгыг солино. Танигдаагүй key дээр
	// apperror.NotFound буцаана (зөвшөөрөгдсөн key-үүд migration-д seed
	// хийгддэг — INSERT хийдэггүй).
	SetPrompt(ctx context.Context, key, content string) error
	// SearchKnowledge нь мэдлэгийн сангаас query-д тохирох бичлэгүүдийг
	// буцаана (title/content ILIKE + tag тэнцэл). AI-ийн search_knowledge
	// tool үүгээр ажилладаг.
	SearchKnowledge(ctx context.Context, query string, limit int) ([]domain.AIKnowledge, error)
}

// AuditLogRow нь hash-chained audit_log хүснэгтийн нэг мөрийн уншсан хэлбэр —
// admin жагсаалт болон гинж шалгахад (VerifyChain) ашиглагдана.
type AuditLogRow struct {
	ID          int64
	OccurredAt  time.Time
	ActorUserID string
	Action      string
	Category    string
	Target      string
	RequestID   string
	Metadata    map[string]any
	PrevHash    string
	ChainHash   string
}

// AuditListFilter нь admin жагсаалтыг нарийсгана. Хоосон утга нь "шүүлтгүй".
type AuditListFilter struct {
	Action      string // тухайн action-аар тэнцэл шүүлт
	ActorUserID string // тухайн actor-оор тэнцэл шүүлт
}

// AuditRepository нь hash-chained, append-only audit_log хүснэгтийн gateway юм.
// Append нь шинэ мөрийн chain_hash-г тооцоолж, гинжийг зөв холбохын тулд
// бичилтийг цувралжуулна (serialize). audit_log нь admin-only тул бичилт/уншилт
// нь repository доторх "service"/"admin" GUC дор явна — хүсэлтийн (user) RLS
// identity-аас үл хамаарна.
type AuditRepository interface {
	// Append нь нэг үйл явдлыг гинжийн төгсгөлд нэмж, бичигдсэн мөрийн
	// chain_hash-г буцаана. Хамгийн сүүлийн мөрийг түгжээтэй уншиж prev_hash
	// болгоно (хоосон гинжид genesis = "").
	Append(ctx context.Context, e audit.ChainEntry) (string, error)
	// List нь audit мөрүүдийг id буурахаар (хамгийн сүүлийнх эхэндээ)
	// хуудаслан буцаана. Admin GUC дор ажиллана.
	List(ctx context.Context, filter AuditListFilter, limit, offset int) ([]AuditLogRow, error)
	// VerifyChain нь гинжийг genesis-ээс эхлэн дахин тооцоолж шалгана. Гинж
	// бүрэн бол ok=true, эвдэрсэн бол ok=false + эвдэрсэн ЭХНИЙ мөрийн id-г
	// (brokenID) буцаана.
	VerifyChain(ctx context.Context) (ok bool, brokenID int64, err error)
}

// SecurityEventRecord нь security_events хүснэгтэд бичигдэх (Ingest) болон
// уншигдах (List) нэг мөр юм.
type SecurityEventRecord struct {
	ID         int64
	ReceivedAt time.Time
	UserID     string // хоосон бол NULL (тодорхойгүй / нэвтрээгүй)
	Kind       string
	Severity   string
	Source     string
	UserAgent  string
	IP         string
	Detail     map[string]any
}

// SecurityEventRepository нь RASP-style security_events хүснэгтийн gateway юм.
// Ingest нь нэвтэрсэн хэрэглэгчийн (user) RLS identity дор ажилладаг тул RLS
// бодлого user_id = app.user_id-г баталгаажуулна; List нь admin GUC дор ажиллана.
type SecurityEventRepository interface {
	// Ingest нь нэг security event бичнэ.
	Ingest(ctx context.Context, e SecurityEventRecord) error
	// List нь event-үүдийг received_at буурахаар хуудаслан буцаана (admin).
	List(ctx context.Context, limit, offset int) ([]SecurityEventRecord, error)
}
