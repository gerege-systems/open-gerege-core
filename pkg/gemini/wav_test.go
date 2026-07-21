// Government Template Platform V3.0
// Gerege Systems Development Team болон Claude AI хамтран бүтээв, 2026.

package gemini

import (
	"encoding/binary"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPCMRateFromMime(t *testing.T) {
	tests := []struct {
		mime string
		want int
	}{
		{"audio/L16;codec=pcm;rate=24000", 24000},
		{"audio/L16; rate=16000", 16000},
		{"audio/L16", defaultPCMRate},
		{"", defaultPCMRate},
		{"audio/L16;rate=abc", defaultPCMRate},
	}
	for _, tt := range tests {
		assert.Equal(t, tt.want, PCMRateFromMime(tt.mime), tt.mime)
	}
}

func TestPCMToWAV(t *testing.T) {
	pcm := []byte{0x10, 0x20, 0x30, 0x40}
	wav := PCMToWAV(pcm, 24000)

	require.Len(t, wav, 44+len(pcm))
	assert.Equal(t, "RIFF", string(wav[0:4]))
	assert.Equal(t, "WAVE", string(wav[8:12]))
	assert.Equal(t, "fmt ", string(wav[12:16]))
	assert.Equal(t, "data", string(wav[36:40]))
	assert.Equal(t, uint32(24000), binary.LittleEndian.Uint32(wav[24:28]))
	assert.Equal(t, uint32(len(pcm)), binary.LittleEndian.Uint32(wav[40:44]))
	assert.Equal(t, pcm, wav[44:])
}
