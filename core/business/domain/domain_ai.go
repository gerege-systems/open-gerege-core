// Gerege Systems Development Team болон Claude AI хамтран бүтээв, 2026.

package domain

import "time"

// AI prompt давхаргын зөвшөөрөгдсөн түлхүүрүүд. Suurь (base) дүрэм кодод
// хатуу бичигдсэн — DB-ээс зөвхөн эдгээр давхарга тохируулагдана.
const (
	// AIPromptScope нь туслахын ХАМРАХ ХҮРЭЭ — ямар сэдвээр туслахыг
	// тодорхойлно; хүрээнээс гадуурх асуултад туслах татгалздаг.
	AIPromptScope = "scope"
	// AIPromptInstructions нь нэмэлт заавар (өнгө аяс, онцлох дүрэм г.м.).
	AIPromptInstructions = "instructions"
)

// AIPromptKeys нь зөвшөөрөгдсөн давхаргын жагсаалт (validation-д).
var AIPromptKeys = []string{AIPromptScope, AIPromptInstructions}

// AIPrompt нь DB-д хадгалагддаг, ажиллаж байх үед тохируулж болдог нэг
// prompt давхарга.
type AIPrompt struct {
	Key       string
	Content   string
	UpdatedAt *time.Time
}

// AIKnowledge нь AI туслахын хайдаг мэдлэгийн сангийн нэг бичлэг.
type AIKnowledge struct {
	ID      int
	Title   string
	Content string
	Tags    []string
	// Slug нь тогтвортой танигч (seed-ийн upsert түлхүүр); Source нь агуулга
	// хаанаас гаралтай (docs/backend/frontend) — хариултад эх сурвалж дурдах,
	// засварлахад мөрдөх боломж өгнө.
	Slug   string
	Source string
	// Score нь вектор хайлтын cosine ойролцоолол (0..1). Түлхүүр үгийн
	// (ILIKE) fallback хайлтад 0 үлдэнэ.
	Score float64
}

// AIKnowledgeChunk нь embedding хийгдэх ёстой нэг бичлэг (backfill).
type AIKnowledgeChunk struct {
	ID    int
	Title string
	// Content нь embed хийх бүтэн текст (гарчиг + агуулга).
	Content string
	// Hash нь агуулгын одоогийн hash — embedding-ийг хамт хадгална.
	Hash string
}
