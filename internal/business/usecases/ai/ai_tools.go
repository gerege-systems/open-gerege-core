// Gerege Template Platform V3.0
// Gerege Systems Development Team болон Claude AI хамтран бүтээв, 2026.

package ai

import (
	"context"
	"math"
	"time"

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

// knowledgeTopK нь нэг хайлтад буцаах бичлэгийн тоо. Вектор хайлт нь ойролцоо
// утгатай бичлэгүүдийг ч авчирдаг тул ILIKE-аас арай өгөөмөр (модел хамааралгүй
// хэсгийг өөрөө шүүнэ).
const knowledgeTopK = 6

// minVectorScore нь cosine ойролцооллын доод босго. Үүнээс доош таарц нь
// сэдвийн хувьд хамааралгүй байх магадлалтай тул хаяна — модел «мэдэхгүй»
// гэж хэлэх нь буруу баримт зохиохоос дээр.
const minVectorScore = 0.55

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
			Description: "Платформын мэдлэгийн сангаас (DB) мэдээлэл хайна. Хэрэглэгчийн " +
				"платформтой холбоотой асуултад хариулахын өмнө түлхүүр үгээр хайж, олдсон " +
				"бичлэгүүдэд тулгуурлан хариул.",
			Parameters: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"query": map[string]any{
						"type":        "string",
						"description": "Хайх түлхүүр үг эсвэл богино хэллэг (Монголоор).",
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
			// Босгоос доош таарцыг хаяна.
			kept := make([]domain.AIKnowledge, 0, len(hits))
			for _, it := range hits {
				if it.Score >= minVectorScore {
					kept = append(kept, it)
				}
			}
			if len(kept) > 0 {
				return kept, "vector"
			}
		}
	}

	found, err := repo.SearchKnowledge(ctx, query, 5)
	if err != nil {
		logger.WarnWithContext(ctx, "ai: keyword search failed", logger.Fields{
			constants.LoggerCategory: constants.LoggerCategoryAI,
			"error":                  err.Error(),
		})
		return nil, "keyword"
	}
	return found, "keyword"
}
