// Gerege Template Version 27.0
// Gerege Systems Development Team болон Claude AI хамтран бүтээв, 2026.

// Package rls нь Postgres-ийн Row-Level Security (RLS)-д зориулсан "хэн энэ
// хүсэлтийг гүйцэтгэж байна" гэдгийг context.Context-оор зөөвөрлөдөг хамгийн
// доод түвшний (leaf) package юм. Энэ нь зөвхөн стандартын "context"-оос
// хамаардаг тул HTTP давхарга (middleware), business давхарга (auth usecase)
// болон datasource давхарга (repository) хоорондоо import cycle үүсгэхгүйгээр
// хуваалцаж чадна.
//
// Identity-г repository давхарга нь транзакц бүрийн эхэнд
// `SET LOCAL app.user_id` / `SET LOCAL app.user_role` GUC болгон Postgres руу
// нийтэлдэг (datasources/repositories/postgres/users.withRLS-г үз);
// migrations/7_enable_rls_users.up.sql дахь бодлогууд (policy) эдгээр GUC-уудыг
// уншиж аль мөрийг харагдах/өөрчлөгдөхийг шийднэ. context-д Identity байхгүй
// бол repository нь хоосон GUC тавьдаг тул бодлогууд бүх мөрийг ХААНА (аюулгүй
// өгөгдмөл, fail-closed).
//
// АНХААР: RLS нь зөвхөн non-superuser DB role-оор холбогдсон үед хүчинтэй.
// Postgres-ийн superuser (болон BYPASSRLS role) бодлогуудыг алгасдаг тул
// production-д апп non-superuser app_user-ээр холбогдох ёстой
// (docs/SECURITY.md §DB role separation-г үз).
package rls

import "context"

// Role нь RLS бодлогуудын уншдаг `app.user_role` GUC-ийн утга юм. Утгууд нь SQL
// бодлогын текст дэх string литералуудтай ЯГ таарах ёстой.
type Role string

const (
	// RoleService нь нэвтрэхээс ӨМНӨХ болон системийн итгэмжлэгдсэн урсгалуудад
	// (login дахь email хайлт, register дахь INSERT, OTP, нууц үг сэргээх)
	// зориулагдсан — эдгээр нь хараахан баталгаажаагүй хэрэглэгчийн мөрд хандах
	// шаардлагатай тул "зөвхөн өөрийн мөр" хязгаарлалтаас чөлөөлөгдөнө.
	RoleService Role = "service"
	// RoleAdmin нь бүх мөрийг харж/өөрчилж чадна.
	RoleAdmin Role = "admin"
	// RoleUser нь зөвхөн өөрийн (app.user_id-тэй таарах) мөрд хандана.
	RoleUser Role = "user"
)

// Identity нь нэг хүсэлтийн RLS контекст юм: ямар хэрэглэгчийн нэрийн өмнөөс,
// ямар үүрэгтэйгээр DB рүү хандаж байгаа.
type Identity struct {
	// UserID нь баталгаажсан хэрэглэгчийн UUID (string). RoleService эсвэл
	// RoleAdmin үед хоосон байж болно.
	UserID string
	// Role нь RLS бодлогын шийдвэрийг тодорхойлно.
	Role Role
}

type ctxKey struct{}

// With нь Identity-г context-д суулгаж шинэ context буцаана.
func With(ctx context.Context, id Identity) context.Context {
	return context.WithValue(ctx, ctxKey{}, id)
}

// WithService нь context-г RoleService үүргээр тэмдэглэнэ — auth-ийн нэвтрэхээс
// өмнөх урсгалуудад (login/register/OTP/нууц үг сэргээх) ашиглах товчлол.
func WithService(ctx context.Context) context.Context {
	return With(ctx, Identity{Role: RoleService})
}

// WithUser нь context-г тодорхой userID-тэй RoleUser үүргээр тэмдэглэнэ
// (least-privilege: зөвхөн тухайн хэрэглэгчийн өөрийнх нь мөр).
func WithUser(ctx context.Context, userID string) context.Context {
	return With(ctx, Identity{UserID: userID, Role: RoleUser})
}

// WithAdmin нь context-г RoleAdmin үүргээр тэмдэглэнэ (бүх мөрд хандана).
func WithAdmin(ctx context.Context, userID string) context.Context {
	return With(ctx, Identity{UserID: userID, Role: RoleAdmin})
}

// FromContext нь суулгасан Identity-г гаргаж авна. Хоёр дахь утга нь Identity
// тавигдсан эсэхийг (ok) илэрхийлнэ; тавигдаагүй бол тэг Identity буцна — энэ нь
// хоосон GUC болж RLS бодлогоор бүх мөрийг хааж, аюулгүй өгөгдмөлд хүргэнэ.
func FromContext(ctx context.Context) (Identity, bool) {
	id, ok := ctx.Value(ctxKey{}).(Identity)
	return id, ok
}
