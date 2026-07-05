// Gerege Template Version 27.0
// Gerege Systems Development Team болон Claude AI хамтран бүтээв, 2026.

// Package domain нь enterprise entity-үүдийг агуулдаг — Clean Architecture-ийн
// хамгийн дотоод хүрээ. Гадна давхаргууд (HTTP, DB, framework) хувьсан өөрчлөгдөх
// үед энэ давхаргыг тогтвортой байлгахын тулд domain нь зөвхөн дараахаас хамаардаг:
//
//   - стандарт сан (standard library)
//   - golang.org/x/crypto/bcrypt (тогтвортой шифрлэлтийн primitive бөгөөд
//     стандарт сангийн өргөтгөл мэт хандана)
//
// Domain нь internal/ эсвэл pkg/ багцуудыг import ХИЙХ ЁСГҮЙ — энэ нь хамаарлын
// дүрмийг урвуулна (дотоод нь гадна талаасаа хамаарах болно).
//
// Timestamp-уудыг UTC-ээр тэмдэглэдэг. Харуулах цагийн бүс (жишээ нь WIB / GMT+7)
// нь domain-ийн биш, харин гадна давхаргуудын хариуцдаг танилцуулгын асуудал юм.
package domain

import (
	"errors"
	"net/mail"
	"strings"
	"time"

	"golang.org/x/crypto/bcrypt"
)

// Role-ийн танигчид. IsAdmin() зэрэг эрх олголтын шийдвэрүүд нь domain
// логик тул role ID-ууд нь transport- эсвэл persistence-тэй зэргэлдээ
// constants багцад биш, харин domain дотор байрладаг.
const (
	// Role ID-ууд зэрэглэлийн дарааллаар (1 = хамгийн дээд эрх). RoleSuperAdmin
	// нь admin-аас дээгүүр зэрэглэлийн эрх — зөвхөн super admin админ
	// хэрэглэгчдийг үүсгэх/эрх олгох/хасах боломжтой. Super admin нь admin-ийн
	// бүх эрхийг мөн эдэлдэг (IsAdmin() true) тул RLS/JWT-ийн admin зам түүнд
	// адилхан үйлчилнэ; ялгаа нь RequireSuperAdmin gate-ээр л /superadmin
	// гадаргууг хамгаалдагт бий. Энэ зэрэглэлийг API-аар үүсгэж болохгүй — зөвхөн
	// bootstrap (SUPERADMIN_EMAIL) эсвэл DB-ээр л томилогдоно.
	//
	// АНХААР: role_id 0 нь ямар ч role БИШ — claim-гүй хуучин токенуудын sentinel
	// (RBAC middleware үүнийг хамгийн бага эрх RoleUser рүү буулгадаг). Тиймээс
	// нэг ч role-д 0 оноож болохгүй.
	RoleSuperAdmin = 1
	RoleAdmin      = 2
	RoleManager    = 3
	RoleUser       = 4
)

// Domain алдаануудыг энгийн sentinel хэлбэрээр тодорхойлсон тул дуудагч нь
// аливаа алдааны бүрхүүлд холбогдолгүйгээр errors.Is-ээр харьцуулж чадна.
// Transport давхарга эдгээрийг HTTP хэлбэрийн хариу болгож боодог;
// persistence нь DB хэлбэрийн хариу болгож боодог.
var (
	ErrEmptyUsername = errors.New("username cannot be empty")
	ErrEmptyEmail    = errors.New("email cannot be empty")
	ErrInvalidEmail  = errors.New("email format is invalid")
	ErrEmptyPassword = errors.New("password cannot be empty")
)

// User нь бүртгэгдсэн бүртгэлийн domain entity юм. Password нь
// үүсгэлтийн дараа үргэлж bcrypt hash-ийг агуулна — энгийн текст (plaintext)
// нь зөвхөн NewUser дотор түр зуур оршино.
type User struct {
	ID          string
	Username    string
	FirstName   string // нэр (монгол)
	LastName    string // овог (монгол)
	FirstNameEn string // нэр (англи)
	LastNameEn  string // овог (англи)
	Email       string
	Password    string
	Active      bool
	RoleID      int
	// eID identity-ийн талбарууд. Зөвхөн eID-ээр нэвтэрсэн хэрэглэгчид
	// бөглөгдөнө; нууц үгээр бүртгүүлсэн хэрэглэгчдэд хоосон.
	NationalID string // регистрийн дугаар (улсын танигч)
	CivilID    string // иргэний бүртгэлийн дугаар
	KYCLevel   string // IdP-ийн баталгаажуулалтын түвшин (сертификатын түвшин)
	// eID сертификатын дэлгэрэнгүй — login COMPLETE-ийн cert.value (DER)-ээс
	// задлагдана. Зөвхөн eID хэрэглэгчид бөглөгдөнө.
	DocumentNumber string     // төхөөрөмжийн UUID (eID)
	CertSerial     string     // сертификатын серийн дугаар
	CertNotBefore  *time.Time // хүчинтэй эхлэх
	CertNotAfter   *time.Time // дуусах
	CertIssuer     string     // олгогч CA
	CertKeyType    string     // нийтийн түлхүүрийн алгоритм
	GoogleSub      string     // холбогдсон Google account (sub); хоосон бол холбоогүй
	// Google профайл — холбогдсон account-аас хадгалсан мэдээлэл (Dashboard-д харуулна).
	GoogleEmail         string     // Google и-мэйл
	GoogleEmailVerified bool       // Google и-мэйл баталгаажсан эсэх
	GoogleName          string     // Google дэлгэцийн нэр
	GooglePicture       string     // Google профайл зургийн URL
	GoogleLinkedAt      *time.Time // анх холбосон огноо
	CreatedAt           time.Time
	UpdatedAt           *time.Time
	DeletedAt           *time.Time
	PasswordChangedAt   *time.Time
}

// GoogleAccount нь Google OAuth-аас ирсэн профайл — eID хэрэглэгчид холбоход
// (эсвэл дараагийн нэвтрэлтэд шинэчлэхэд) хадгалах талбарууд.
type GoogleAccount struct {
	Sub           string
	Email         string
	EmailVerified bool
	Name          string
	Picture       string
}

// FullName нь монгол хэлбэрээр "Овог Нэр"-г буцаана; хоёулаа хоосон бол хоосон
// тэмдэгт мөр (дуудагч username руу fallback хийнэ).
func (u User) FullName() string {
	return strings.TrimSpace(strings.TrimSpace(u.LastName) + " " + strings.TrimSpace(u.FirstName))
}

// FullNameEn нь англи (Латин) "Lastname Firstname"-г буцаана.
func (u User) FullNameEn() string {
	return strings.TrimSpace(strings.TrimSpace(u.LastNameEn) + " " + strings.TrimSpace(u.FirstNameEn))
}

// NewUser нь бүртгэлийн оролтоос шинэ User үүсгэнэ. Email нь
// нормчлогддог, нууц үгийг өгөгдсөн bcrypt cost-оор hash хийдэг бөгөөд
// CreatedAt-ийг каноник цагийн бүсээр тэмдэглэдэг.
//
// bcryptCost нь параметр (config-оос уншдаггүй) тул domain нь тохиргооны
// асуудлуудаас ангид хэвээр үлддэг; дуудагч үүнийг inject хийдэг. Хязгаараас
// гадуурх утгууд нь bcrypt.DefaultCost руу шилждэг тул буруу тохируулсан
// гадна давхарга үүнийг panic болгож чадахгүй.
func NewUser(username, email, plainPassword string, roleID, bcryptCost int) (*User, error) {
	username = strings.TrimSpace(username)
	if username == "" {
		return nil, ErrEmptyUsername
	}
	if plainPassword == "" {
		return nil, ErrEmptyPassword
	}
	email = NormalizeEmail(email)
	if email == "" {
		return nil, ErrEmptyEmail
	}
	if _, err := mail.ParseAddress(email); err != nil {
		return nil, ErrInvalidEmail
	}

	if bcryptCost < bcrypt.MinCost || bcryptCost > bcrypt.MaxCost {
		bcryptCost = bcrypt.DefaultCost
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(plainPassword), bcryptCost)
	if err != nil {
		return nil, err
	}

	return &User{
		Username:  username,
		Email:     email,
		Password:  string(hash),
		RoleID:    roleID,
		CreatedAt: time.Now().UTC(),
	}, nil
}

// ErrEmptyCivilID нь eID identity-д иргэний бүртгэлийн дугаар (civil_id) дутуу
// үед буцна — public RP-д IdP нь national_id-г илчилдэггүй тул civil_id нь eID
// хэрэглэгчийн давтагдашгүй түлхүүр болдог. Иймээс заавал шаардлагатай.
var ErrEmptyCivilID = errors.New("civil_id cannot be empty")

// NewEIDUser нь eID-ээр баталгаажсан identity-аас идэвхтэй (Active=true),
// нууц үггүй (Password="") хэрэглэгч үүсгэнэ. eID хэрэглэгчид email байхгүй
// (Email="") тул enumeration-аас ангид; давтагдашгүй байдлыг civil_id-ээр
// хангана. Username нь "eid_"+civil_id (жижиг үсэг) хэлбэрийн нийлэг утга.
//
// АНХААР: IdP нь зөвхөн эрх бүхий auth.gerege.mn RP-д national_id (reg_no)-г
// илчилдэг; public RP (энэ template) зөвхөн civil_id хүлээн авдаг. Тиймээс
// түлхүүр нь civil_id. national_id хоосон бол DB-д NULL болж хадгалагдана
// (хоосон string биш) — эс бөгөөс lower(national_id) WHERE national_id IS NOT
// NULL partial unique index олон eID хэрэглэгчийн хооронд мөргөлдөнө.
//
// IdP нь identity-г аль хэдийн баталгаажуулсан тул энд нууц үг hash хийдэггүй
// — VerifyPassword нь хоосон Password дээр үргэлж false буцаана (bcrypt нь
// хоосон hash-тай таарахгүй), иймээс passwordless хэрэглэгч нууц үгээр
// хэзээ ч нэвтэрч чадахгүй.
func NewEIDUser(civilID, givenName, surname, givenNameEn, surnameEn, nationalID, kycLevel string) (*User, error) {
	civilID = strings.ToLower(strings.TrimSpace(civilID))
	if civilID == "" {
		return nil, ErrEmptyCivilID
	}
	return &User{
		Username:    "eid_" + civilID,
		FirstName:   strings.TrimSpace(givenName),
		LastName:    strings.TrimSpace(surname),
		FirstNameEn: strings.TrimSpace(givenNameEn),
		LastNameEn:  strings.TrimSpace(surnameEn),
		Email:       "",
		Password:    "",
		Active:      true,
		RoleID:      RoleUser,
		// national_id хоосон бол хоосон string үлдээнэ — records.ptrOrNil нь
		// үүнийг SQL NULL болгон хадгалах тул partial unique index мөргөлдөхгүй.
		NationalID: strings.ToLower(strings.TrimSpace(nationalID)),
		CivilID:    civilID,
		KYCLevel:   strings.TrimSpace(kycLevel),
		CreatedAt:  time.Now().UTC(),
	}, nil
}

// NormalizeEmail нь хоосон зайг тайрч, хаягийг жижиг үсэг болгодог тул
// "User@Example.com " болон "user@example.com" нь ижил хайлтын key рүү hash
// хийгдэж, ижил DB мөрийг query хийж, ижил давтагдашгүй байдлын зөрчлийг
// өдөөдөг. RFC 5321-д local хэсэг нь техникийн хувьд том/жижиг үсгийг ялгадаг
// гэж заасан ч, хэрэглээний түвшний бүх mail provider үүнийг ялгадаггүй.
func NormalizeEmail(s string) string { return strings.ToLower(strings.TrimSpace(s)) }

// Activate нь хэрэглэгчийг идэвхтэй болгож, UpdatedAt-ийг тэмдэглэнэ. Энэ нь
// төлөвийг өөрчилдөг тул pointer receiver-тай.
func (u *User) Activate() {
	u.Active = true
	now := time.Now().UTC()
	u.UpdatedAt = &now
}

// VerifyPassword нь plain нь bcrypt-ээр u.Password руу hash хийгдэх тохиолдолд
// л true буцаана. Value receiver — цэвэр унших үйлдэл.
func (u User) VerifyPassword(plain string) bool {
	return bcrypt.CompareHashAndPassword([]byte(u.Password), []byte(plain)) == nil
}

// IsAdmin нь хэрэглэгчийн role нь admin эрх олгож байгаа эсэхийг мэдээлнэ.
// Дүрэм нэг газар байрлахын тулд (дуудах газруудад нүцгэн харьцуулалт хийхгүй)
// method болгосон — RoleAdmin-ийг нэг удаа өөрчилбөл дуудагч бүр дагана.
// Super admin нь admin-аас дээгүүр зэрэглэл тул admin-ийн бүх эрхийг (RLS admin
// GUC, JWT isAdmin, RequirePermission bypass) мөн эдэлнэ.
func (u User) IsAdmin() bool { return u.RoleID == RoleAdmin || u.RoleID == RoleSuperAdmin }

// IsSuperAdmin нь хэрэглэгч super admin (админуудыг удирдах дээд эрх) эсэхийг
// мэдээлнэ. RequireSuperAdmin middleware /superadmin гадаргууг үүгээр хаадаг.
func (u User) IsSuperAdmin() bool { return u.RoleID == RoleSuperAdmin }

// ChangePassword нь plain-ийг өгөгдсөн bcrypt cost-оор hash хийж, хадгалсан
// hash-ийг сольж, PasswordChangedAt + UpdatedAt-ийг тэмдэглэнэ. Энэ timestamp
// нь хүчингүй болгох (revocation) тасалбар цэг юм: түүнээс өмнө олгогдсон
// токенуудыг /refresh дээр татгалзана.
func (u *User) ChangePassword(plain string, bcryptCost int) error {
	if plain == "" {
		return ErrEmptyPassword
	}
	if bcryptCost < bcrypt.MinCost || bcryptCost > bcrypt.MaxCost {
		bcryptCost = bcrypt.DefaultCost
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(plain), bcryptCost)
	if err != nil {
		return err
	}
	now := time.Now().UTC()
	u.Password = string(hash)
	u.PasswordChangedAt = &now
	u.UpdatedAt = &now
	return nil
}

// TokensRevokedBefore нь access/refresh токенуудын тасалбар timestamp-ийг
// буцаана. IssuedAt нь энэ timestamp-аас өмнө байгаа токенуудыг татгалзах
// ёстой. Тэг утга нь "хүчингүй болгох тасалбар байхгүй" гэсэн утгатай
// (бүртгэлээс хойш бүртгэл credential-ээ хэзээ ч сольж эргүүлээгүй).
func (u User) TokensRevokedBefore() time.Time {
	if u.PasswordChangedAt == nil {
		return time.Time{}
	}
	return *u.PasswordChangedAt
}
