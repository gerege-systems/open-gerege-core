// Gerege Template Version 27.0
// Gerege Systems Development Team болон Claude AI хамтран бүтээв, 2026.

package gemini

import (
	"encoding/binary"
	"strconv"
	"strings"
)

// TTS model нь түүхий PCM (ихэвчлэн "audio/L16;codec=pcm;rate=24000")
// буцаадаг — browser-ууд үүнийг шууд тоглуулдаггүй тул WAV толгой нэмж
// өгдөг туслахууд.

const defaultPCMRate = 24000

// PCMRateFromMime нь "audio/L16;codec=pcm;rate=24000" хэлбэрийн mime-аас
// sample rate-ийг гаргана; олдохгүй бол Gemini TTS-ийн өгөгдмөл 24000.
func PCMRateFromMime(mime string) int {
	for _, p := range strings.Split(mime, ";") {
		if k, v, ok := strings.Cut(strings.TrimSpace(p), "="); ok && strings.EqualFold(k, "rate") {
			if rate, err := strconv.Atoi(strings.TrimSpace(v)); err == nil && rate > 0 {
				return rate
			}
		}
	}
	return defaultPCMRate
}

// PCMToWAV нь 16-bit mono PCM байтуудыг WAV контейнерт ороож буцаана.
func PCMToWAV(pcm []byte, sampleRate int) []byte {
	const (
		channels      = 1
		bitsPerSample = 16
	)
	byteRate := sampleRate * channels * bitsPerSample / 8
	blockAlign := channels * bitsPerSample / 8

	buf := make([]byte, 0, 44+len(pcm))
	u32 := func(v uint32) []byte { b := make([]byte, 4); binary.LittleEndian.PutUint32(b, v); return b }
	u16 := func(v uint16) []byte { b := make([]byte, 2); binary.LittleEndian.PutUint16(b, v); return b }

	buf = append(buf, []byte("RIFF")...)
	buf = append(buf, u32(uint32(36+len(pcm)))...)
	buf = append(buf, []byte("WAVE")...)
	buf = append(buf, []byte("fmt ")...)
	buf = append(buf, u32(16)...) // fmt chunk хэмжээ
	buf = append(buf, u16(1)...)  // PCM формат
	buf = append(buf, u16(channels)...)
	buf = append(buf, u32(uint32(sampleRate))...)
	buf = append(buf, u32(uint32(byteRate))...)
	buf = append(buf, u16(uint16(blockAlign))...)
	buf = append(buf, u16(bitsPerSample)...)
	buf = append(buf, []byte("data")...)
	buf = append(buf, u32(uint32(len(pcm)))...)
	buf = append(buf, pcm...)
	return buf
}
