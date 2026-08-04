// Gerege Systems Development Team болон Claude AI хамтран бүтээв, 2026.

// Package module нь платформын модульчлагдсан архитектурын суурь гэрээ:
// модулийн манифест, төрөл (core | business), бүртгэл (registry) болон
// route хаалт (gate). Энэ бол V4.0 "Modular Platform" refactor-ийн 1-р
// шатны цөм — модулиудын хил ЭНД зарлагдаж, дараагийн шатуудад DI wiring,
// migration, permission, frontend nav нь энэ л манифестээс урган гарна.
//
// Зарчим: kernel нь модулиудын БҮРТГЭЛИЙГ мэднэ, харин модулийн кодоос
// хамаардаггүй (import-ын чиглэл: modules → kernel, хэзээ ч эсрэгээр биш).
package module

import (
	"fmt"
	"sort"
	"strings"
)

// Kind нь модулийн ангилал.
type Kind string

const (
	// KindCore — платформын цөм модуль: үргэлж суусан, unінstall/disable
	// хийгдэхгүй (auth, users, rbac г.м.).
	KindCore Kind = "core"
	// KindBusiness — business модуль: суулгаж/устгаж, асааж/унтрааж болно.
	KindBusiness Kind = "business"
)

// Manifest нь нэг модулийн зарлал. Дараагийн шатуудад талбарууд нэмэгдэнэ
// (миграцийн fs, permission каталог, frontend nav г.м.) — одоо route-ийн
// эзэмшил + хамаарлын граф гэсэн хамгийн наад захын гэрээ.
type Manifest struct {
	// ID — модулийн тогтмол танигч (жишээ: "gov"). Жижиг үсэг, өөрчлөгдөхгүй.
	ID string
	// Name — хүнд харагдах нэр (UI, лог).
	Name string
	// Kind — core эсвэл business.
	Kind Kind
	// DependsOn — энэ модуль ажиллахад ЗААВАЛ идэвхтэй байх ёстой модулиуд.
	DependsOn []string
	// RoutePrefixes — энэ модулийн эзэмшдэг HTTP замын угтварууд
	// (жишээ: "/api/v1/gov/"). Угтвар "/"-ээр төгссөн бол sub-tree,
	// төгсөөгүй бол яг тэр зам + sub-tree гэж үзнэ. Модулиудын хооронд
	// давхцахгүй байх ёстой (Registry үүсэхэд шалгана); илүү урт угтвар
	// нь илүү тодорхой тул түрүүлж таарна.
	RoutePrefixes []string

	// UIPrefixes — модулийн эзэмшдэг ХЭРЭГЛЭГЧИЙН ИНТЕРФЕЙСИЙН зам
	// (жишээ: "/me/eid/"). RoutePrefixes нь API гадаргууг дүрсэлдэг бол
	// энэ нь бүтээгдэхүүний гадаргууг дүрсэлнэ — хоёр нь нэг-нэгээрээ
	// таардаггүй. Жишээ: /me/eid/certificates нь API талдаа
	// /api/v1/users/me/eid/... руу очдог бөгөөд тэр нь users (core)
	// модулийнх; гэвч цэсэн дэх тэр бүлэг нь eidproxy унтрахад алга болох
	// ёстой. Тиймээс UI эзэмшлийг ТУСАД НЬ зарлана.
	//
	// Frontend нь энэ жагсаалтыг /v1/platform/modules-ээс уншиж, цэсээ
	// хамгийн урт угтвараар шүүнэ — gate-тэй ижил дүрэм. Ингэснээр цэсний
	// gating нь модуль нэмэх бүрд ГАРААР тэмдэглэх шаардлагагүй болно.
	UIPrefixes []string
}

// validate нь манифестийн дотоод бүрэн бүтэн байдлыг шалгана.
func (m Manifest) validate() error {
	if m.ID == "" {
		return fmt.Errorf("module: манифестийн ID хоосон байна")
	}
	if strings.ToLower(m.ID) != m.ID || strings.ContainsAny(m.ID, " /\\") {
		return fmt.Errorf("module: %q — ID нь жижиг үсэгтэй, зайгүй байх ёстой", m.ID)
	}
	if m.Kind != KindCore && m.Kind != KindBusiness {
		return fmt.Errorf("module: %q — Kind нь core эсвэл business байх ёстой", m.ID)
	}
	for _, p := range m.RoutePrefixes {
		if !strings.HasPrefix(p, "/") {
			return fmt.Errorf("module: %q — route угтвар %q нь /-ээр эхлэх ёстой", m.ID, p)
		}
	}
	return nil
}

// sortedIDs нь map түлхүүрүүдийг тогтвортой дарааллаар буцаана.
func sortedIDs[T any](m map[string]T) []string {
	out := make([]string, 0, len(m))
	for id := range m {
		out = append(out, id)
	}
	sort.Strings(out)
	return out
}
