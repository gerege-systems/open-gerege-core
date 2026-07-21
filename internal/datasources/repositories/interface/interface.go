// Government Template Platform V3.0
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
	"encoding/json"
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
	// CreatePreRegistered нь админ иргэнийг РЕГИСТРИЙН ДУГААР (national_id)-аар
	// урьдчилан бүртгэнэ (private платформ): national_id + нэр + role-той идэвхтэй
	// мөр (password/email/civil_id/sso_sub-гүй). Давхардсан national_id →
	// apperror.Conflict.
	CreatePreRegistered(ctx context.Context, in *domain.User) (domain.User, error)
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
	// UpsertSuperAdmin нь superadmin onboarding-ийн ТӨГСГӨЛД (Google + eID +
	// email OTP + TOTP бүгд баталгаажсаны дараа) super admin хэрэглэгчийг НЭГ
	// ТРАНЗАКЦИД үүсгэх/ахиулна: users мөр (google_sub-аар түлхүүрлэсэн, role_id=1,
	// civil_id/MFA НЭ) + superadmin_accounts satellite мөр (civil_id/national_id,
	// email_verified, mfa_enabled, шифрлэгдсэн totp_secret, invited_by, onboarded_at).
	// civil_id-г users-д ТАВИХГҮЙ тул нэг хүн eID-ээр admin, Google-оор super admin
	// байж чадна (civil_id partial unique index зөрчихгүй). totp_secret нь usecase
	// давхаргад AES-GCM-ээр шифрлэгдсэн ирнэ. Давхардсан email/google_sub нь
	// apperror.Conflict болно. Буцаах user нь account-ийн MFA утгуудаар hydrate хийгдсэн.
	UpsertSuperAdmin(ctx context.Context, in *domain.User, account *domain.SuperadminAccount) (domain.User, error)
}

// SuperadminAccountRepository нь super admin-ы satellite бүртгэлийн (superadmin_accounts)
// READ gateway юм. Хүснэгт нь эмзэг тул RLS-тэй (service/admin). Бичилтийг
// UserRepository.UpsertSuperAdmin нь users мөртэй нэг транзакцид хийдэг.
type SuperadminAccountRepository interface {
	// Get нь user_id-аар super admin бүртгэлийг буцаана (MFA challenge-д TOTP
	// secret-ыг авах). Байхгүй бол apperror.NotFound.
	Get(ctx context.Context, userID string) (domain.SuperadminAccount, error)
}

// RecoveryCodeRepository нь 2FA нөөц кодуудын (user_recovery_codes) gateway юм.
// Кодууд нь per-user тул хүснэгт RLS-тэй (repo нь withRLS транзакцид
// app.user_id/app.user_role GUC тавьдаг — migration 35). DB-д зөвхөн SHA-256
// hash хадгалагдана; энгийн текст код энэ давхаргад хэзээ ч хүрэхгүй.
type RecoveryCodeRepository interface {
	// Replace нь тухайн хэрэглэгчийн ӨМНӨХ бүх кодыг устгаад, шинэ hash-уудыг
	// нэг транзакцид оруулна (нөөц кодыг дахин үүсгэх нь хуучныг хүчингүй
	// болгоно).
	Replace(ctx context.Context, userID string, hashes []string) error
	// ListActive нь хэрэглэгчийн хэрэглэгдээгүй (used_at IS NULL) кодуудыг
	// буцаана — үлдсэн кодын тоог харуулахад.
	ListActive(ctx context.Context, userID string) ([]domain.RecoveryCode, error)
	// Consume нь өгсөн hash-тай, хэрэглэгдээгүй НЭГ кодыг атомаар "хэрэглэсэн"
	// болгож тэмдэглэнэ (used_at = now()). Тохирох идэвхтэй код байхгүй
	// (буруу код эсвэл аль хэдийн хэрэглэсэн) бол apperror.NotFound — иймээс
	// код нэг л удаа ажиллана.
	Consume(ctx context.Context, userID, hash string) error
}

// SuperadminInviteRepository нь superadmin урилгын allow-list
// (superadmin_invites) gateway юм. Хэрэглэгч-тус-бүрийн биш, админаар
// удирдагддаг нийтийн config хүснэгт тул RLS-гүй (plain pool query).
type SuperadminInviteRepository interface {
	// Create нь урилга үүсгэнэ (email нь нормчлогдсон ирнэ). Аль хэдийн
	// урьсан и-мэйл дээр apperror.Conflict.
	Create(ctx context.Context, email, invitedBy string) (domain.SuperadminInvite, error)
	// List нь бүх урилгыг (шинэ нь эхэндээ) буцаана.
	List(ctx context.Context) ([]domain.SuperadminInvite, error)
	// GetByEmail нь и-мэйлээр урилгыг олно; байхгүй бол apperror.NotFound
	// (onboarding-ийн Google алхам үүгээр гатлана).
	GetByEmail(ctx context.Context, email string) (domain.SuperadminInvite, error)
	// Delete нь урилгыг цуцална. Байхгүй бол apperror.NotFound.
	Delete(ctx context.Context, email string) error
	// MarkAccepted нь урилгыг ашигласан гэж тэмдэглэнэ (accepted_at = now())
	// — onboarding төгсөхөд дуудагдана. Дахин ашиглах боломжгүй болно.
	MarkAccepted(ctx context.Context, email string) error
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

	// Telemetry — сүүлийн log-ууд + dashboard-ийн нэгтгэл + бодит хүсэлт бичих.
	ListRequestLogs(ctx context.Context, limit int) ([]domain.GatewayRequestLog, error)
	CreateRequestLog(ctx context.Context, l *domain.GatewayRequestLog) error
	Overview(ctx context.Context) (domain.GatewayOverview, error)
}

// ApplicationRepository нь нэгдсэн Applications (Gateway consumer + SSO RP)
// overlay-г хадгална: applications мөр + зөвшөөрсөн gateway service-үүд
// (application_services). OAuth2 client өөрөө oauth_clients-д амьдардаг тул энд зөвхөн
// client_id болон overlay талбарууд. RLS-гүй нийтийн config.
type ApplicationRepository interface {
	List(ctx context.Context) ([]domain.Application, error)
	Get(ctx context.Context, id string) (domain.Application, error)
	Create(ctx context.Context, a *domain.Application) (domain.Application, error)
	Update(ctx context.Context, a *domain.Application) (domain.Application, error)
	Delete(ctx context.Context, id string) error
	// SetServices нь апп-ын зөвшөөрсөн service-ийн жагсаалтыг бүхэлд нь орлуулна.
	SetServices(ctx context.Context, appID string, serviceIDs []string) error
	// ServiceScopes нь өгсөн service id-уудын OAuth scope нэрсийг буцаана.
	ServiceScopes(ctx context.Context, serviceIDs []string) ([]string, error)
	// ServiceIDsForScopes нь OAuth scope нэрсэд харгалзах gateway service id-
	// уудыг буцаана (ServiceScopes-ийн урвуу — client scope → service id).
	ServiceIDsForScopes(ctx context.Context, scopes []string) ([]string, error)
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

// GovDecisionInput нь менежерийн approve/reject шийдвэрийн параметрүүд.
// OutputRef нь зөвшөөрөгдсөн тохиолдолд олгогдох лавлагаа (байхгүй байж болно —
// жишээ нь биет үнэмлэх захиалахад лавлагаа үүсэхгүй).
type GovDecisionInput struct {
	ApplicationID string
	OfficerID     string
	Approve       bool
	// Target нь шилжих ЭЦСИЙН төлөв. Зөвшөөрсөн үед гаралт тэр дороо
	// олгогдож байвал 'completed', биет зүйл хүргэгдэх шаардлагатай бол
	// 'approved' (домэйн давхарга шийднэ).
	Target    string
	Note      string
	Result    string
	OutputRef *domain.GovReference
	Notify    *domain.GovNotification
}

// GovRepository нь иргэний "Төрийн үйлчилгээ" порталын өгөгдлийг хариуцна.
// Каталог (ListServices) нь нийтийн; бусад нь хэрэглэгч-тус-бүрийн тул query
// бүр userID-гаар scope хийгдэхээс гадна per-user хүснэгтүүд RLS-тэй (repo нь
// withRLS транзакцид app.user_id/app.user_role GUC тавьдаг — migration 20).
type GovRepository interface {
	// Каталог
	ListServices(ctx context.Context) ([]domain.GovService, error)
	GetService(ctx context.Context, id string) (domain.GovService, error)
	// ListLifeEvents нь CPSV-AP Event каталогийг буцаана.
	ListLifeEvents(ctx context.Context) ([]domain.GovLifeEvent, error)

	// Хүсэлт (иргэн)
	ListApplications(ctx context.Context, userID string) ([]domain.GovApplication, error)
	GetApplication(ctx context.Context, userID, id string) (domain.GovApplication, error)
	CreateApplication(ctx context.Context, in *domain.GovApplication) (domain.GovApplication, error)
	SetApplicationStatus(ctx context.Context, userID, id, status string) error

	// CreateApplicationWithOutput нь AUTO горимын үйлчилгээг НЭГ ТРАНЗАКЦИД
	// биелүүлнэ: хүсэлт (completed) + лавлагаа + мэдэгдэл + timeline. Аль нэг нь
	// бүтэлгүйтвэл бүгд буцна — иргэнд "олгогдсон" гэж харагдаад лавлагаа нь
	// байхгүй байх завсрын төлөв үүсэхээс сэргийлнэ.
	CreateApplicationWithOutput(ctx context.Context, app *domain.GovApplication, ref *domain.GovReference, notify *domain.GovNotification) (domain.GovApplication, domain.GovReference, error)

	// Хүсэлт (менежер — officer RLS үүргээр)
	QueueStats(ctx context.Context, officerID string) (domain.GovQueueStats, error)
	ListQueue(ctx context.Context, f domain.GovQueueFilter) ([]domain.GovApplication, error)
	GetApplicationAny(ctx context.Context, id string) (domain.GovApplication, error)
	// AssignApplication нь хүсэлтийг менежерт оноож in_review болгоно. Зэрэг
	// ирсэн хоёр дахь оролдлого 0 мөр хөндөнө → apperror.Conflict.
	AssignApplication(ctx context.Context, id, officerID string) (domain.GovApplication, error)
	// DecideApplication нь approve/reject шийдвэрийг бичнэ (SQL WHERE guard-аар
	// зөвшөөрөгдсөн эх төлвөөс л шилжинэ).
	DecideApplication(ctx context.Context, in GovDecisionInput) (domain.GovApplication, error)
	// CompleteApplication нь 'approved' (биет гаралт хүлээгдэж буй) хүсэлтийг
	// хүргэгдсэн гэж хааж 'completed' болгоно.
	CompleteApplication(ctx context.Context, id, officerID string, notify *domain.GovNotification) (domain.GovApplication, error)
	// RequestMoreInfo нь info_required руу шилжүүлж SLA цагийг ЗОГСООНО.
	RequestMoreInfo(ctx context.Context, id, officerID, note string) (domain.GovApplication, error)
	// ResumeFromInfo нь иргэн баримт нэмсний дараа цагийг ҮРГЭЛЖЛҮҮЛЖ, due_at-г
	// зогссон хугацаагаар хойшлуулна.
	ResumeFromInfo(ctx context.Context, userID, id string) (domain.GovApplication, error)

	// Timeline
	AppendApplicationEvent(ctx context.Context, in *domain.GovApplicationEvent) error
	ListApplicationEvents(ctx context.Context, applicationID string) ([]domain.GovApplicationEvent, error)

	// SLA sweep (background worker)
	// SLAOverdue нь хугацаа хэтэрсэн ч хараахан тэмдэглэгдээгүй хүсэлтүүдийг
	// буцаана (breach_notified маягийн latch — нэг хүсэлтэд нэг л удаа).
	MarkSLABreached(ctx context.Context) ([]domain.GovApplication, error)
	// TacitApprovals нь чимээгүй зөвшөөрөл идэвхтэй үйлчилгээний хугацаа
	// хэтэрсэн хүсэлтүүдийг зөвшөөрөгдсөн төлөвт шилжүүлнэ.
	TacitApprovals(ctx context.Context) ([]domain.GovApplication, error)

	// Лавлагаа
	ListReferences(ctx context.Context, userID string) ([]domain.GovReference, error)
	CreateReference(ctx context.Context, in *domain.GovReference) (domain.GovReference, error)

	// Мэдэгдэл
	// CreateNotification нь иргэнд мэдэгдэл бичнэ. Менежер өөрийнх нь биш
	// хэрэглэгчид бичих тул officer/service RLS үүрэг шаардана.
	CreateNotification(ctx context.Context, in *domain.GovNotification) error
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

// SiteRepository нь сайтын нийтийн харагдацын default (site_appearance) ганц
// мөрийг унших/шинэчлэхийг хариуцна. Per-user биш нийтийн config тул RLS-гүй;
// app зөвхөн UPDATE хийдэг (мөр migration-д seed хийгддэг).
type SiteRepository interface {
	// GetAppearance нь одоогийн харагдацын default-ыг буцаана.
	GetAppearance(ctx context.Context) (domain.SiteAppearance, error)
	// SetAppearance нь харагдацын default-ыг шинэчилнэ (UPDATE-only).
	SetAppearance(ctx context.Context, a domain.SiteAppearance) error
}

// ThemeRepository нь landing-ийн нэрлэсэн theme-үүдийг (themes хүснэгт) удирдана.
// Нийтийн config тул RLS-гүй; app бүрэн CRUD хийдэг (админ theme үүсгэж/устгана).
type ThemeRepository interface {
	// ListThemes нь бүх theme-ийг (config-той) буцаана.
	ListThemes(ctx context.Context) ([]domain.Theme, error)
	// GetTheme нь id-аар нэг theme буцаана; олдохгүй бол apperror.NotFound.
	GetTheme(ctx context.Context, id string) (domain.Theme, error)
	// GetActiveTheme нь идэвхтэй theme-ийг буцаана; байхгүй бол apperror.NotFound.
	GetActiveTheme(ctx context.Context) (domain.Theme, error)
	// CreateTheme нь шинэ theme үүсгэж, үүсгэсэн мөрийг буцаана.
	CreateTheme(ctx context.Context, name string, config json.RawMessage) (domain.Theme, error)
	// UpdateTheme нь theme-ийн нэр/config-ыг шинэчилнэ.
	UpdateTheme(ctx context.Context, id, name string, config json.RawMessage) error
	// DeleteTheme нь theme-ийг устгана (идэвхтэйг устгаж болохгүй — usecase шалгана).
	DeleteTheme(ctx context.Context, id string) error
	// SetActive нь нэг theme-ийг идэвхтэй болгож бусдыг идэвхгүй болгоно (tx).
	SetActive(ctx context.Context, id string) error
}
