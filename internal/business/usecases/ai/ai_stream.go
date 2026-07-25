// Gerege Template Platform V3.0
// Gerege Systems Development Team болон Claude AI хамтран бүтээв, 2026.

package ai

import (
	"context"
	"errors"
	"strings"

	"template/internal/apperror"
	"template/internal/constants"
	"template/pkg/gemini"
	"template/pkg/logger"
)

// StreamEvent нь урсгалаар (SSE) хэрэглэгч рүү явах нэг үйл явдал.
type StreamEvent struct {
	// Transcript нь дуут мессежийн текст хуулбар — нэг л удаа явна.
	Transcript string
	// Delta нь хариултын дараагийн хэсэг (үг/өгүүлбэрийн тасархай).
	Delta string
	// Reset үнэн бол өмнө нь илгээсэн Delta-г хаяж, шинээр эхлэхийг заана
	// (model tool дуудахаасаа өмнө ямар нэг текст бичсэн ховор тохиолдол).
	Reset bool
}

// transcriptMarker нь дуут мессежийн хуулбарыг хариултаас ялгах тэмдэг.
// Хэрэглэгчид ХЭЗЭЭ Ч харагдахгүй — сервер тал энэ мөрийг таслаж авна.
const transcriptMarker = "[ХЭЛСЭН]"

// transcriptInstruction нь audio ирсэн үед system prompt-д нэмэгдэнэ.
// Ингэснээр НЭГ дуудалтаар хуулбар (STT) ба хариулт хоёуланг авна —
// тусдаа STT дуудалт хийхгүй тул хариу мэдэгдэхүйц хурдан ирнэ.
const transcriptInstruction = "\n\n[ДУУТ МЕССЕЖ]\nХэрэглэгч дуут мессеж илгээлээ. " +
	"Хариултынхаа ХАМГИЙН ЭХНИЙ мөрөнд `" + transcriptMarker + " <хэрэглэгчийн хэлсэн үг>` " +
	"гэж яг сонсогдсоноор нь (нэмэлт тайлбаргүй) бич. Дараа нь ШИНЭ МӨРНӨӨС " +
	"жинхэнэ хариултаа үргэлжлүүл. Энэ тэмдгийг хариултын өөр хаана ч бүү хэрэглэ."

// maxTranscriptHold — хуулбарын мөрийг хүлээх дээд урт. Model зааврыг
// дагаагүй (шинэ мөр гаргаагүй) бол хэрэглэгч хоосон дэлгэц ширтэхгүйн тулд
// хуримтлуулснаа шууд гаргана.
const maxTranscriptHold = 400

// RunStream нь Run-тэй ижил pipeline-ыг ажиллуулах ч эцсийн хариултыг
// хэсэг хэсгээр нь emit руу дамжуулна. Дуут мессежийн хувьд НЭГ дуудалтаар
// хуулбар + хариулт хоёуланг авна.
//
// client нь streaming дэмждэггүй бол (тест дэх хуурамч хэрэгжүүлэлт) Run
// руу уналт хийж, бүтэн хариултыг ганц Delta болгож өгнө — зан төлөв
// ижилхэн, зөвхөн хэсэгчлэлт байхгүй.
func (uc *usecase) RunStream(ctx context.Context, req RunRequest, emit func(StreamEvent) error) (RunResult, error) {
	streamer, ok := uc.client.(gemini.Streamer)
	if !ok {
		res, err := uc.Run(ctx, req)
		if err != nil || res.Reply == "" {
			return res, err
		}
		if emitErr := emit(StreamEvent{Delta: res.Reply}); emitErr != nil {
			return res, emitErr
		}
		return res, nil
	}

	geminiReq := uc.buildStreamRequest(ctx, req)
	filter := &transcriptFilter{want: req.Audio != nil, emit: emit}

	var steps []Step
	for step := 0; step < uc.cfg.MaxSteps; step++ {
		roundStart := filter.emitted
		resp, err := streamer.StreamGenerateContent(ctx, geminiReq, filter.push)
		if err != nil {
			if errors.Is(err, gemini.ErrNotConfigured) {
				return RunResult{}, apperror.InternalCause(err)
			}
			// Хэрэглэгч ямар ч текст хараагүй бол fallback мессежээр намжаана;
			// хагас хариулт гарчихсан бол давхардуулахгүйгээр таслана.
			logger.ErrorWithContext(ctx, "ai pipeline: gemini stream failed", logger.Fields{
				constants.LoggerCategory: constants.LoggerCategoryAI,
				"error":                  err.Error(),
				"step":                   step,
			})
			if filter.emitted {
				return RunResult{Reply: filter.Reply(), Steps: steps, Degraded: true}, nil
			}
			return uc.streamFallback(req, steps, emit)
		}

		calls := resp.FunctionCalls()
		if len(calls) == 0 {
			if err := filter.flush(); err != nil {
				return RunResult{}, err
			}
			reply := filter.Reply()
			if reply == "" {
				return uc.streamFallback(req, steps, emit)
			}
			return RunResult{Reply: reply, Steps: steps, Transcript: filter.transcript}, nil
		}

		// Model tool дуудахаасаа өмнө текст бичсэн бол тэр нь хариулт биш —
		// клиентэд «энэ хүртэлхийг хая» гэж хэлээд шинээр эхэлнэ.
		if filter.emitted && !roundStart {
			if err := emit(StreamEvent{Reset: true}); err != nil {
				return RunResult{}, err
			}
			filter.reset()
		}

		geminiReq.Contents = append(geminiReq.Contents, resp.ModelContent())
		responseParts := make([]gemini.Part, 0, len(calls))
		for _, call := range calls {
			result := uc.executeTool(ctx, call)
			steps = append(steps, Step{Tool: call.Name, Args: call.Args, Result: result})
			responseParts = append(responseParts, gemini.Part{
				FunctionResponse: &gemini.FunctionResponse{Name: call.Name, Response: result},
			})
		}
		geminiReq.Contents = append(geminiReq.Contents, gemini.Content{Role: "user", Parts: responseParts})
	}

	logger.WarnWithContext(ctx, "ai pipeline: max steps reached without final answer (stream)", logger.Fields{
		constants.LoggerCategory: constants.LoggerCategoryAI,
		"max_steps":              uc.cfg.MaxSteps,
	})
	return uc.streamFallback(req, steps, emit)
}

// buildStreamRequest нь Run-ийнхтай ижил хүсэлт бэлдэнэ; audio байвал
// хуулбарын зааврыг system prompt-д нэмнэ.
func (uc *usecase) buildStreamRequest(ctx context.Context, req RunRequest) gemini.Request {
	system := uc.systemInstruction(ctx, req.Lang, uc.styleHint(), req.Anonymous)
	if req.Audio != nil {
		system += transcriptInstruction
	}

	temperature, topP := chatTemperature, chatTopP
	out := gemini.Request{
		SystemInstruction: &gemini.Content{Parts: []gemini.Part{{Text: system}}},
		Contents:          buildContents(req),
		GenerationConfig:  &gemini.GenerationConfig{Temperature: &temperature, TopP: &topP},
	}
	if len(uc.decls) > 0 {
		out.Tools = []gemini.Tool{{FunctionDeclarations: uc.decls}}
	}
	return out
}

// streamFallback нь Gemini бүтэлгүйтсэн үед хэрэглэгчийн хэл дээрх
// fallback мессежийг урсгалаар өгнө (чат унахгүй).
func (uc *usecase) streamFallback(req RunRequest, steps []Step, emit func(StreamEvent) error) (RunResult, error) {
	reply := fallbackReply(req.Lang)
	if err := emit(StreamEvent{Delta: reply}); err != nil {
		return RunResult{}, err
	}
	return RunResult{Reply: reply, Steps: steps, Degraded: true}, nil
}

// transcriptFilter нь урсгалаас хуулбарын эхний мөрийг таслаж авна.
// Хуулбар олдтол текстийг түр хуримтлуулна (богино — ихэвчлэн эхний хэдэн
// chunk), дараа нь бүх зүйл шууд дамжина.
type transcriptFilter struct {
	want       bool
	done       bool
	emitted    bool
	transcript string
	hold       strings.Builder
	reply      strings.Builder
	emit       func(StreamEvent) error
}

func (f *transcriptFilter) push(text string) error {
	if text == "" {
		return nil
	}
	if !f.want || f.done {
		return f.send(text)
	}

	f.hold.WriteString(text)
	buf := f.hold.String()
	idx := strings.IndexByte(buf, '\n')
	if idx < 0 {
		if len(buf) < maxTranscriptHold {
			return nil // мөр дуусаагүй — үргэлжлүүлж хүлээнэ
		}
		// Model заавраа дагаагүй — хүлээхээ болиод байгаагаар нь гаргана.
		f.done = true
		f.hold.Reset()
		return f.send(buf)
	}

	first, rest := buf[:idx], buf[idx+1:]
	f.done = true
	f.hold.Reset()

	if trimmed := strings.TrimSpace(first); strings.HasPrefix(trimmed, transcriptMarker) {
		f.transcript = strings.TrimSpace(strings.TrimPrefix(trimmed, transcriptMarker))
		if f.transcript != "" {
			if err := f.emit(StreamEvent{Transcript: f.transcript}); err != nil {
				return err
			}
		}
		return f.send(strings.TrimLeft(rest, "\n"))
	}
	// Тэмдэг байхгүй — бүх текст нь хариулт.
	return f.send(buf)
}

// flush нь хүлээгдэж үлдсэн текстийг (шинэ мөргүй богино хариу) гаргана.
func (f *transcriptFilter) flush() error {
	if f.hold.Len() == 0 {
		return nil
	}
	buf := f.hold.String()
	f.hold.Reset()
	f.done = true

	if trimmed := strings.TrimSpace(buf); strings.HasPrefix(trimmed, transcriptMarker) {
		// Зөвхөн хуулбар ирсэн (хариулт хоосон) — хуулбарыг өгөөд дуусна.
		f.transcript = strings.TrimSpace(strings.TrimPrefix(trimmed, transcriptMarker))
		if f.transcript == "" {
			return nil
		}
		return f.emit(StreamEvent{Transcript: f.transcript})
	}
	return f.send(buf)
}

func (f *transcriptFilter) send(text string) error {
	if text == "" {
		return nil
	}
	f.emitted = true
	f.reply.WriteString(text)
	return f.emit(StreamEvent{Delta: text})
}

// reset нь tool дуудлагын өмнөх текстийг хаяна (клиент талд Reset event).
func (f *transcriptFilter) reset() {
	f.reply.Reset()
	f.hold.Reset()
	f.emitted = false
}

func (f *transcriptFilter) Reply() string { return strings.TrimSpace(f.reply.String()) }
