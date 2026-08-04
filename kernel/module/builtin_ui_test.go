// Gerege Systems Development Team болон Claude AI хамтран бүтээв, 2026.

package module

import (
	"strings"
	"testing"
)

// UI угтвар нь модулиудын хооронд ДАВХЦАХГҮЙ байх ёстой — эс бөгөөс нэг
// цэсийн зүйл хоёр модульд харьяалагдаж, аль нь түүнийг нуухыг frontend
// таамаглах болно. Хамгийн урт угтвар ялах дүрэм нь давхцлыг зөвшөөрдөг
// (жишээ: /me/eid/ vs /me/eid/sign) тул ЯГ ИЖИЛ угтварыг л хориглоно.
func TestUIPrefixesAreUnique(t *testing.T) {
	owner := map[string]string{}
	for _, m := range Builtin().List() {
		for _, p := range m.Manifest.UIPrefixes {
			if prev, dup := owner[p]; dup {
				t.Errorf("UI угтвар %q нь %q болон %q хоёрт давхцав", p, prev, m.Manifest.ID)
			}
			owner[p] = m.Manifest.ID
			if !strings.HasPrefix(p, "/") {
				t.Errorf("%s: UI угтвар %q нь '/'-ээр эхлэх ёстой", m.Manifest.ID, p)
			}
		}
	}
}

// Core модульд UI угтвар зарлах нь утгагүй: core унтардаггүй тул цэс нь
// хэзээ ч нуугдахгүй. Ийм зарлал нь "нуугдана" гэсэн ХУДАЛ амлалт өгнө.
func TestCoreModulesDeclareNoUIPrefixes(t *testing.T) {
	for _, m := range Builtin().List() {
		if m.Manifest.Kind == KindCore && len(m.Manifest.UIPrefixes) > 0 {
			t.Errorf("core модуль %q нь UIPrefixes зарлаж болохгүй (%v) — core унтардаггүй",
				m.Manifest.ID, m.Manifest.UIPrefixes)
		}
	}
}
