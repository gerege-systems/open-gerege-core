// Government Template Platform V3.0
// Gerege Systems Development Team болон Claude AI хамтран бүтээв, 2026.

// Package clock нь time.Now()-г хийсвэрлэдэг бөгөөд ингэснээр одоогийн
// цагаас хамаардаг код (токены хүчинтэй хугацаа, OTP TTL, аудитын цагийн тэмдэг)
// тестэд тодорхой (deterministic) байдлаар ажиллуулагдах боломжтой болно.
//
// Production код нь time.Now()-г шууд дуудахын оронд Clock interface-ээс
// хамаарах ёстой. Анхдагч хэрэгжүүлэлт болох RealClock нь time.Now-г дуудах
// бөгөөд cmd/api дахь холболтоор тарьдаг утга юм. Тестүүд унтахгүйгээр (sleep)
// цагийг удирдахын тулд Frozen эсвэл Stub-аар орлуулна.
package clock

import "time"

// Clock нь одоогийн бодит цагийг (wall-clock) буцаана. Mock хийх боломжтой.
type Clock interface {
	Now() time.Time
}

// RealClock нь time.Now руу шилжүүлдэг. Үүнийг production холболтод хэрэглэнэ.
type RealClock struct{}

// Now нь Clock-ийг хангана.
func (RealClock) Now() time.Time { return time.Now() }

// Frozen нь үргэлж ижил агшинг мэдээлдэг Clock-ийг буцаана.
// Дуудлагуудын хооронд цаг урагшилснаас үүсэх flake-гүйгээр яг таг
// хүчинтэй хугацааны цагийн тэмдгийг шалгадаг тестэд хэрэгтэй.
func Frozen(t time.Time) Clock { return frozen{t} }

type frozen struct{ t time.Time }

func (f frozen) Now() time.Time { return f.t }

// Stub нь гараар удирддаг Clock бөгөөд тестүүдэд цагийг алхам алхмаар
// урагшлуулах боломж олгодог (жишээ нь, бодитоор унтахгүйгээр токены
// хүчинтэй хугацааг ажиллуулах). Тэг утга нь time.Time{}-г мэдээлнэ;
// хэрэглэхээс өмнө түүнийг тарихын тулд Set-г дуудна.
type Stub struct{ t time.Time }

// Now нь Clock-ийг хангана.
func (s *Stub) Now() time.Time { return s.t }

// Set нь мэдээлж буй агшинг солино.
func (s *Stub) Set(t time.Time) { s.t = t }

// Advance нь мэдээлж буй агшинг d-ээр урагш зөөнө.
func (s *Stub) Advance(d time.Duration) { s.t = s.t.Add(d) }
