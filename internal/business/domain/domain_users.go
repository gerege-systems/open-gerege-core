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
	RoleAdmin   = 1
	RoleUser    = 2
	RoleManager = 3
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
	ID                string
	Username          string
	FirstName         string // нэр
	LastName          string // овог
	Email             string
	Password          string
	Active            bool
	RoleID            int
	CreatedAt         time.Time
	UpdatedAt         *time.Time
	DeletedAt         *time.Time
	PasswordChangedAt *time.Time
}

// FullName нь монгол хэлбэрээр "Овог Нэр"-г буцаана; хоёулаа хоосон бол хоосон
// тэмдэгт мөр (дуудагч username руу fallback хийнэ).
func (u User) FullName() string {
	return strings.TrimSpace(strings.TrimSpace(u.LastName) + " " + strings.TrimSpace(u.FirstName))
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
func (u User) IsAdmin() bool { return u.RoleID == RoleAdmin }

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
