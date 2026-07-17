// Gerege Template Version 27.0
// Gerege Systems Development Team болон Claude AI хамтран бүтээв, 2026.

package domain

import (
	"encoding/json"
	"time"
)

// LandingConfig нь нүүр хуудасны (landing + auth бүрхүүл) ажиллаж байх үед
// тохируулдаг харагдацын ганц JSON баримт. Схемийг frontend эзэмшдэг
// (lib/landing.ts) тул домэйн түвшинд агуулгыг задлахгүй — opaque JSON-оор
// дамжуулна; backend зөвхөн хүчинтэй объект + хэмжээ + rawCss ариутгал
// шалгана.
type LandingConfig struct {
	Config    json.RawMessage
	UpdatedAt *time.Time
}
