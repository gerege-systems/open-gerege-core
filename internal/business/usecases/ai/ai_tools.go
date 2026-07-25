// Gerege Template Platform V3.0
// Gerege Systems Development Team болон Claude AI хамтран бүтээв, 2026.

package ai

import (
	"context"
	"math"
	"sort"
	"strings"
	"time"
	"unicode"

	"template/internal/business/domain"
	"template/internal/constants"
	repointerface "template/internal/datasources/repositories/interface"
	"template/pkg/gemini"
	"template/pkg/logger"
)

// ToolFunc нь backend дээр ажиллах функц. Model args-ийг шийднэ, backend
// гүйцэтгэж үр дүнг map-аар буцаана (Gemini functionResponse болж явна).
type ToolFunc func(ctx context.Context, args map[string]any) (map[string]any, error)

// ToolDef нь нэг tool = model-д зарлах тодорхойлолт + бодит гүйцэтгэл.
// Проект бүр өөрийн tool-уудаа (DB lookup, тооцоолол г.м.) энд нэмдэг.
type ToolDef struct {
	Declaration gemini.FunctionDeclaration
	Execute     ToolFunc
}

// DefaultTools нь template-д хавсаргасан жишээ tool-ууд. Бодит проект энэ
// жагсаалтыг өөрийн domain tool-уудаар сольж/нэмж өргөтгөнө.
func DefaultTools() []ToolDef {
	return []ToolDef{serverTimeTool()}
}

// knowledgeTopK нь DB-ээс татах нэр дэвшигчийн тоо (шүүлтийн өмнөх).
const knowledgeTopK = 8

// maxKnowledgeResults нь model-д өгөх бичлэгийн дээд тоо. Хэт олон бүлэг
// өгөх нь чимээ шуугиан нэмж, хариултыг ерөнхий болгодог.
const maxKnowledgeResults = 4

// relativeScoreMargin — ХАМААРЛЫГ ХАРЬЦУУЛСАН босго: хамгийн сайн таарцаас
// энэ хэмжээгээр л доогуур бичлэгүүдийг үлдээнэ.
//
// Яагаад абсолют босго биш вэ: энэ корпус дээр хэмжихэд ХАМААРАЛГҮЙ хоёр
// бүлгийн хоорондын cosine ижилсэл хамгийн багадаа 0.64, дунджаар 0.75
// байсан (нэг сэдвийн хүрээний нэг хэв маягтай текстүүд тул). Иймд «0.55-аас
// дээш бол хамааралтай» гэх төрлийн тогтмол босго юуг ч шүүхгүй. Харин
// «шилдгээсээ хэр хол вэ» гэдэг нь тухайн асуултын хувьд утга учиртай.
const relativeScoreMargin = 0.06

// ILIKE уналтын параметрүүд: хамгийн богино утга агуулсан үгийн урт,
// нөхцөл тайрсан үндсийн урт, оролдох үгийн дээд тоо.
const (
	minKeywordLen   = 4
	keywordStemLen  = 6
	maxKeywordTerms = 3
)

// minVectorScore нь зөвхөн хог хаях доод шал — үүнээс доош бол огт өөр
// сэдэв (жишээ нь корпус бүхэлдээ хамааралгүй асуулт).
const minVectorScore = 0.35

// KnowledgeSearchTool нь ai_knowledge хүснэгтээс хайдаг tool — AI хэрэглэгчийн
// асуултад хариулахын өмнө мэдлэгийн сангаас (DB) мэдээлэл татаж тулгуурлана.
// Suurь зааварт (baseInstruction) "платформын асуултад эхлээд эндээс хай"
// гэж заасан тул AI үүнийг өөрөө дууддаг.
//
// Хайлтын дараалал:
//  1. Асуултыг Gemini-ээр embed хийж (RETRIEVAL_QUERY) вектор хайлт хийнэ —
//     хэрэглэгч өөр үг хэллэгээр асуусан ч утга санааны хувьд ойр бичлэг олдоно.
//  2. Embedder байхгүй / алдаа гарсан / вектор хайлтаас юу ч олдоогүй бол
//     түлхүүр үгийн (ILIKE) хайлт руу уналт хийнэ.
//
// embedder nil байж болно (тест, эсвэл GEMINI_API_KEY-гүй орчин) — тэр үед
// шууд ILIKE ажиллана.
func KnowledgeSearchTool(repo repointerface.AIRepository, embedder gemini.Embedder) ToolDef {
	return ToolDef{
		Declaration: gemini.FunctionDeclaration{
			Name: "search_knowledge",
			Description: "Платформын мэдлэгийн сангаас (DB) утга санааны (семантик) хайлт хийнэ. " +
				"Хэрэглэгчийн платформтой холбоотой асуултад хариулахын өмнө ЗААВАЛ дуудаж, " +
				"олдсон бичлэгүүдэд тулгуурлан хариул. Хайлт нь embedding дээр суурилдаг тул " +
				"түлхүүр үг биш, хэрэглэгчийн БҮТЭН асуултыг (эсвэл түүний утгыг бүрэн " +
				"илэрхийлсэн өгүүлбэрийг) дамжуул.",
			Parameters: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"query": map[string]any{
						"type": "string",
						"description": "Хэрэглэгчийн асуултыг бүтэн өгүүлбэрээр. Аль ч хэлээр " +
							"болно (мэдлэгийн сан монголоор ч, семантик хайлт хэл дамнана); " +
							"товч түлхүүр үгээс бүтэн асуулт илүү сайн таардаг.",
					},
				},
				"required": []string{"query"},
			},
		},
		Execute: func(ctx context.Context, args map[string]any) (map[string]any, error) {
			query, _ := args["query"].(string)
			if query == "" {
				return map[string]any{"results": []any{}, "note": "query хоосон байна"}, nil
			}

			items, mode := searchKnowledge(ctx, repo, embedder, query)
			results := make([]map[string]any, 0, len(items))
			for _, it := range items {
				res := map[string]any{
					"title":   it.Title,
					"content": it.Content,
				}
				if it.Source != "" {
					res["source"] = it.Source
				}
				if it.Score > 0 {
					// Оноог хоёр орноор — модел хамааралын зэргийг харгалзана.
					res["score"] = math.Round(it.Score*100) / 100
				}
				results = append(results, res)
			}
			return map[string]any{"results": results, "count": len(results), "mode": mode}, nil
		},
	}
}

// serverTimeTool нь серверийн одоогийн цагийг Улаанбаатарын цагаар буцаадаг
// жишээ tool — function calling pipeline-ийг ямар ч гадаад хамааралгүйгээр
// үзүүлэхэд хангалттай.
func serverTimeTool() ToolDef {
	return ToolDef{
		Declaration: gemini.FunctionDeclaration{
			Name:        "get_server_time",
			Description: "Серверийн одоогийн огноо, цагийг Улаанбаатарын цагийн бүсээр буцаана. Хэрэглэгч цаг, огноо, өдрийн талаар асуувал ашигла.",
			Parameters: map[string]any{
				"type":       "object",
				"properties": map[string]any{},
			},
		},
		Execute: func(_ context.Context, _ map[string]any) (map[string]any, error) {
			loc, err := time.LoadLocation("Asia/Ulaanbaatar")
			if err != nil {
				loc = time.UTC
			}
			now := time.Now().In(loc)
			return map[string]any{
				"datetime": now.Format("2006-01-02 15:04:05"),
				"weekday":  now.Weekday().String(),
				"timezone": loc.String(),
			}, nil
		},
	}
}

// searchKnowledge нь семантик хайлтыг оролдоод, боломжгүй/үр дүнгүй үед
// түлхүүр үгийн хайлт руу унана. Хоёр дахь утга нь ямар горимоор олдсоныг
// заана ("vector" | "keyword") — модел болон логт ойлгомжтой байхад.
func searchKnowledge(
	ctx context.Context,
	repo repointerface.AIRepository,
	embedder gemini.Embedder,
	query string,
) (items []domain.AIKnowledge, mode string) {
	if embedder != nil {
		vectors, err := embedder.Embed(ctx, []string{query}, gemini.TaskQuery)
		switch {
		case err != nil:
			// Gemini тохируулаагүй / түр саатсан — хайлтыг бүтэн унагахгүй.
			logger.WarnWithContext(ctx, "ai: query embedding failed, falling back to keyword search", logger.Fields{
				constants.LoggerCategory: constants.LoggerCategoryAI,
				"error":                  err.Error(),
			})
		case len(vectors) == 1:
			hits, vErr := repo.SearchKnowledgeByVector(ctx, vectors[0], knowledgeTopK)
			if vErr != nil {
				logger.WarnWithContext(ctx, "ai: vector search failed, falling back to keyword search", logger.Fields{
					constants.LoggerCategory: constants.LoggerCategoryAI,
					"error":                  vErr.Error(),
				})
				break
			}
			kept := filterByRelevance(hits)
			if len(kept) > 0 {
				logRetrieval(ctx, "vector", kept)
				return kept, "vector"
			}
		}
	}

	found := keywordSearch(ctx, repo, query)
	logRetrieval(ctx, "keyword", found)
	return found, "keyword"
}

// keywordSearch нь ILIKE уналт. Model-д бүтэн асуулт дамжуул гэж заасан тул
// («семантик хайлт үг таарахыг шаарддаггүй») бүтэн мөрөөр ILIKE хийвэл бараг
// хэзээ ч таарахгүй — иймд асуултыг үг болгон задлаад хамгийн мэдээлэл
// агуулсан (урт) үгсээр ээлжлэн хайж, үр дүнг нэгтгэнэ.
func keywordSearch(ctx context.Context, repo repointerface.AIRepository, query string) []domain.AIKnowledge {
	seen := make(map[int]bool)
	out := make([]domain.AIKnowledge, 0, maxKnowledgeResults)

	for _, term := range keywordTerms(query) {
		found, err := repo.SearchKnowledge(ctx, term, 5)
		if err != nil {
			logger.WarnWithContext(ctx, "ai: keyword search failed", logger.Fields{
				constants.LoggerCategory: constants.LoggerCategoryAI,
				"error":                  err.Error(),
			})
			return out
		}
		for _, it := range found {
			if seen[it.ID] {
				continue
			}
			seen[it.ID] = true
			out = append(out, it)
			if len(out) == maxKnowledgeResults {
				return out
			}
		}
	}
	return out
}

// keywordTerms нь ILIKE-д оролдох хайлтын үгсийг эрэмбэлж буцаана: эхлээд
// бүтэн мөр (богино асуултад ажиллана), дараа нь хамгийн урт үгс. Монгол хэлэнд
// нөхцөл залгадаг тул урт үгийг эхний 6 үсгээр нь (үндэс) хайна —
// «нэвтрэлтийн» → «нэвтрэ».
func keywordTerms(query string) []string {
	trimmed := strings.TrimSpace(query)
	terms := make([]string, 0, maxKeywordTerms+1)
	seen := make(map[string]bool)
	add := func(t string) {
		if t == "" || seen[t] {
			return
		}
		seen[t] = true
		terms = append(terms, t)
	}
	add(trimmed)

	words := strings.FieldsFunc(trimmed, func(r rune) bool {
		return !unicode.IsLetter(r) && !unicode.IsDigit(r)
	})
	// Урт үг = илүү өвөрмөц (богино нь «нь», «юу», «яаж» гэх мэт туслах үгс).
	sort.SliceStable(words, func(i, j int) bool {
		return len([]rune(words[i])) > len([]rune(words[j]))
	})
	for _, w := range words {
		runes := []rune(w)
		if len(runes) < minKeywordLen {
			break // эрэмбэлсэн тул үүнээс цааш бүгд богино.
		}
		if len(runes) > keywordStemLen {
			runes = runes[:keywordStemLen]
		}
		add(string(runes))
		if len(terms) > maxKeywordTerms {
			break
		}
	}
	return terms
}

// filterByRelevance нь шилдэг таарцтай харьцуулж хамааралгүйг хасаад, үлдсэнийг
// maxKnowledgeResults-аар таслана. Оноо буурах эрэмбээр ирсэн гэж үзнэ
// (SQL ORDER BY embedding <=> query).
func filterByRelevance(hits []domain.AIKnowledge) []domain.AIKnowledge {
	if len(hits) == 0 {
		return nil
	}
	best := hits[0].Score
	if best < minVectorScore {
		return nil
	}
	kept := make([]domain.AIKnowledge, 0, maxKnowledgeResults)
	for _, it := range hits {
		if it.Score < minVectorScore || it.Score < best-relativeScoreMargin {
			break
		}
		kept = append(kept, it)
		if len(kept) == maxKnowledgeResults {
			break
		}
	}
	return kept
}

// logRetrieval нь хайлтын метаданныг л бичнэ — хэрэглэгчийн асуултын текстийг
// ХЭЗЭЭ Ч логдохгүй (PII). Ингэснээр босго/корпусыг цаашид баримтаар тааруулна.
func logRetrieval(ctx context.Context, mode string, items []domain.AIKnowledge) {
	fields := logger.Fields{
		constants.LoggerCategory: constants.LoggerCategoryAI,
		"mode":                   mode,
		"results":                len(items),
	}
	if len(items) > 0 {
		fields["top_slug"] = items[0].Slug
		if items[0].Score > 0 {
			fields["top_score"] = math.Round(items[0].Score*1000) / 1000
		}
	}
	logger.InfoWithContext(ctx, "ai: knowledge retrieval", fields)
}
