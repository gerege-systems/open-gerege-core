// Government Template Platform V3.0
// Gerege Systems Development Team болон Claude AI хамтран бүтээв, 2026.

// Нөөц (recovery) кодын үүсгэлт, нормчлол болон hash-ийн unit тест.
package recovery_test

import (
	"regexp"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"template/pkg/recovery"
)

// codeShape нь "XXXX-XXXX" (base32 том үсэг + 2-7) хэлбэрийг шалгана.
var codeShape = regexp.MustCompile(`^[A-Z2-7]{4}-[A-Z2-7]{4}$`)

func TestGenerate(t *testing.T) {
	t.Run("n ширхэг, давхардалгүй, зөв хэлбэртэй код үүсгэнэ", func(t *testing.T) {
		codes, err := recovery.Generate(10)
		require.NoError(t, err)
		require.Len(t, codes, 10)

		seen := map[string]bool{}
		for _, c := range codes {
			assert.Regexp(t, codeShape, c)
			assert.False(t, seen[c], "код давхардлаа: %s", c)
			seen[c] = true
		}
	})

	t.Run("n <= 0 бол DefaultCount", func(t *testing.T) {
		codes, err := recovery.Generate(0)
		require.NoError(t, err)
		assert.Len(t, codes, recovery.DefaultCount)
	})
}

func TestNormalize(t *testing.T) {
	// Тусгаарлагч/жижиг үсэг ялгаагүй — бүгд нэг каноник хэлбэрт буудаг.
	for _, in := range []string{"abcd-efgh", "ABCD EFGH", "ABCDEFGH", " abcd-EFGH "} {
		assert.Equal(t, "ABCDEFGH", recovery.Normalize(in), "оролт: %q", in)
	}
}

func TestHash(t *testing.T) {
	t.Run("нормчлолын дараа ижил код → ижил hash", func(t *testing.T) {
		assert.Equal(t, recovery.Hash("ABCD-EFGH"), recovery.Hash("abcd efgh"))
	})

	t.Run("өөр код → өөр hash, энгийн текст hash-д агуулагдахгүй", func(t *testing.T) {
		h := recovery.Hash("ABCD-EFGH")
		assert.NotEqual(t, h, recovery.Hash("ABCD-EFGI"))
		assert.Len(t, h, 64) // sha256 hex
		assert.NotContains(t, h, "ABCD")
	})

	t.Run("HashAll нь дарааллыг хадгална", func(t *testing.T) {
		codes := []string{"AAAA-BBBB", "CCCC-DDDD"}
		hashes := recovery.HashAll(codes)
		require.Len(t, hashes, 2)
		assert.Equal(t, recovery.Hash(codes[0]), hashes[0])
		assert.Equal(t, recovery.Hash(codes[1]), hashes[1])
	})
}
