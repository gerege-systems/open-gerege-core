// Gerege Template Version 27.0
// Gerege Systems Development Team болон Claude AI хамтран бүтээв, 2026.

package ai

import (
	"context"
	"time"

	"template/pkg/gemini"
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
