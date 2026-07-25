// Gerege Template Platform V3.0
// Gerege Systems Development Team болон Claude AI хамтран бүтээв, 2026.

package ai

import (
	"context"
	"errors"
	"fmt"

	"template/internal/apperror"
	"template/internal/constants"
	"template/pkg/gemini"
	"template/pkg/logger"
)

const (
	// embedBatchSize нь нэг Gemini дуудалтад явуулах бичлэгийн тоо. Хэт том
	// багц нь хүсэлтийн биеийг сунгаж, түр зуурын алдааны үнийг өсгөнө.
	embedBatchSize = 20
	// maxEmbedBatches нь нэг ажиллагааны дээд хязгаар (embedBatchSize × энэ =
	// нэг удаад embed хийх дээд бичлэг). Гацсан нөхцөлд хязгааргүй давтахаас
	// хамгаална — үлдсэнийг дараагийн ачаалалт эсвэл reindex гүйцээнэ.
	maxEmbedBatches = 25
)

// EmbedKnowledge нь embedding дутуу, эсвэл агуулга нь өөрчлөгдсөн (hash зөрсөн)
// бичлэгүүдийг олж вектор болгон хадгална. Буцаах утга нь энэ дуудалтад
// шинэчлэгдсэн бичлэгийн тоо.
//
// Embedder тохируулаагүй (GEMINI_API_KEY хоосон) бол ажил хийхгүйгээр 0
// буцаана — мэдлэгийн хайлт ILIKE fallback-аар ажиллана.
func (uc *usecase) EmbedKnowledge(ctx context.Context) (int, error) {
	if uc.repo == nil {
		return 0, apperror.Internal("ai knowledge storage not configured")
	}
	if uc.embedder == nil {
		return 0, nil
	}

	total := 0
	for batch := 0; batch < maxEmbedBatches; batch++ {
		chunks, err := uc.repo.ListKnowledgeForEmbedding(ctx, embedBatchSize)
		if err != nil {
			return total, apperror.InternalCause(err)
		}
		if len(chunks) == 0 {
			return total, nil
		}

		texts := make([]string, 0, len(chunks))
		for _, c := range chunks {
			texts = append(texts, c.Content)
		}

		vectors, err := uc.embedder.Embed(ctx, texts, gemini.TaskDocument)
		if err != nil {
			// Түлхүүр байхгүй нь алдаа биш — зүгээр л боломж идэвхгүй.
			if errors.Is(err, gemini.ErrNotConfigured) {
				return total, nil
			}
			return total, apperror.InternalCause(fmt.Errorf("embed knowledge batch: %w", err))
		}
		if len(vectors) != len(chunks) {
			return total, apperror.InternalCause(fmt.Errorf(
				"embed knowledge batch: got %d vectors for %d chunks", len(vectors), len(chunks)))
		}

		for i, c := range chunks {
			if err := uc.repo.SaveKnowledgeEmbedding(ctx, c.ID, vectors[i], c.Hash); err != nil {
				return total, apperror.InternalCause(err)
			}
			total++
		}
	}

	// Хязгаарт хүрсэн ч үлдсэн байж болзошгүй — дараагийн дуудалт гүйцээнэ.
	logger.WarnWithContext(ctx, "ai: knowledge embedding hit batch limit; rerun to finish", logger.Fields{
		constants.LoggerCategory: constants.LoggerCategoryAI,
		"embedded":               total,
	})
	return total, nil
}

// WarmKnowledgeEmbeddings нь ачаалалтын дараа арын дэвсгэрт embedding-ийг
// гүйцээнэ. Boot-ыг блоклохгүй, алдааг зөвхөн логдоно — мэдлэгийн сан
// хоцрогдсон ч апп хэвийн ажиллах ёстой (хайлт ILIKE рүү унана).
func (uc *usecase) WarmKnowledgeEmbeddings(ctx context.Context) {
	if uc.embedder == nil {
		return
	}
	n, err := uc.EmbedKnowledge(ctx)
	if err != nil {
		logger.ErrorWithContext(ctx, "ai: knowledge embedding backfill failed", logger.Fields{
			constants.LoggerCategory: constants.LoggerCategoryAI,
			"error":                  err.Error(),
			"embedded":               n,
		})
		return
	}
	if n > 0 {
		logger.InfoWithContext(ctx, "ai: knowledge embeddings updated", logger.Fields{
			constants.LoggerCategory: constants.LoggerCategoryAI,
			"embedded":               n,
		})
	}
}
