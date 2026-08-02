// Gerege Systems Development Team болон Claude AI хамтран бүтээв, 2026.

package language

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/gerege-systems/open-gerege-core/pkg/gemini"
)

// geminiTranslator нь Translator-ийн үйлдвэрлэлийн хэрэгжилт — Gemini-ийн
// structured output (responseMimeType=application/json + responseSchema)
// ашиглана. Ингэснээр хариу нь код-хашилт, оршил тайлбаргүй цэвэр JSON болж,
// чөлөөт текст задлан шинжлэх шаардлагагүй.
type geminiTranslator struct {
	client *gemini.Client
}

// NewGeminiTranslator нь Gemini дээр суурилсан багц орчуулагч буцаана.
// client нь тохируулагдаагүй (API key байхгүй) бол TranslateBatch нь
// gemini.ErrNotConfigured буцаах ба usecase түүнийг ойлгомжтой алдаа болгоно.
func NewGeminiTranslator(client *gemini.Client) Translator {
	return &geminiTranslator{client: client}
}

// translationSchema нь гаралтын хэлбэрийг тогтооно: {key, value} объектуудын
// массив. Түлхүүр нь аппаас ирдэг динамик утга тул объектын шинж болгож
// зарлах боломжгүй — массив хэлбэр нь ерөнхий бөгөөд найдвартай.
var translationSchema = map[string]any{
	"type": "ARRAY",
	"items": map[string]any{
		"type": "OBJECT",
		"properties": map[string]any{
			"key":   map[string]any{"type": "STRING"},
			"value": map[string]any{"type": "STRING"},
		},
		"required": []string{"key", "value"},
	},
}

// translationPair нь model-ийн буцаах нэг мөр.
type translationPair struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

const translatorInstruction = `Чи програм хангамжийн интерфейсийн (UI) мэргэжлийн орчуулагч.

Дүрэм:
1. Өгөгдсөн JSON массив дахь мөр бүрийн "value"-г %s (%s) хэл рүү орчуул.
2. "key"-г ЯГ хэвээр нь буцаа — бүү орчуул, бүү өөрчил.
3. Мөр бүрийг заавал буцаа. Оруулсан мөрийн тоо = буцаах мөрийн тоо.
4. Буржгар хаалтан доторх байрлуулагчийг ({0}, {name} гэх мэт) ЯГ хэвээр нь,
   ижил бичиглэлээр үлдээ — орчуулж ч, устгаж ч болохгүй.
5. Энэ бол UI-ийн богино мөр (товч, цэс, гарчиг) — товч, байгалийн, тухайн
   хэлний програм хангамжид хэрэглэгддэг нэр томьёог сонго.
6. Техникийн нэр (API, eID, PDF, OAuth, QR), брэндийн нэрийг бүү орчуул.
7. Эх мөрийн том/жижиг үсгийн хэв маяг, төгсгөлийн цэг байгаа эсэхийг даган мөрд.`

func (t *geminiTranslator) TranslateBatch(ctx context.Context, req TranslateBatchRequest) (map[string]string, error) {
	if len(req.Items) == 0 {
		return map[string]string{}, nil
	}

	pairs := make([]translationPair, 0, len(req.Items))
	for key, value := range req.Items {
		pairs = append(pairs, translationPair{Key: key, Value: value})
	}
	payload, err := json.Marshal(pairs)
	if err != nil {
		return nil, fmt.Errorf("translate batch: marshal items: %w", err)
	}

	label := req.TargetLabel
	if label == "" {
		label = req.TargetLang
	}
	instruction := fmt.Sprintf(translatorInstruction, label, req.TargetLang)

	// Temperature 0 — орчуулга нь бүтээлч бус, тогтвортой байх ёстой.
	temperature := 0.0
	resp, err := t.client.GenerateContent(ctx, gemini.Request{
		SystemInstruction: &gemini.Content{Parts: []gemini.Part{{Text: instruction}}},
		Contents:          []gemini.Content{{Role: "user", Parts: []gemini.Part{{Text: string(payload)}}}},
		GenerationConfig: &gemini.GenerationConfig{
			Temperature:      &temperature,
			ResponseMimeType: "application/json",
			ResponseSchema:   translationSchema,
		},
	})
	if err != nil {
		// gemini.ErrNotConfigured-ийг дуудагч таньдаг тул боож нуухгүй.
		return nil, err
	}

	text := strings.TrimSpace(resp.Text())
	if text == "" {
		return nil, fmt.Errorf("translate batch: empty response")
	}

	var out []translationPair
	if err := json.Unmarshal([]byte(text), &out); err != nil {
		return nil, fmt.Errorf("translate batch: response is not valid JSON: %w", err)
	}

	result := make(map[string]string, len(out))
	for _, pair := range out {
		// Model нь оруулаагүй түлхүүр зохиож болзошгүй — зөвхөн хүссэнээ авна.
		if _, ok := req.Items[pair.Key]; ok {
			result[pair.Key] = pair.Value
		}
	}
	return result, nil
}
