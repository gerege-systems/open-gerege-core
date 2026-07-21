// Government Template Platform V3.0
// Gerege Systems Development Team болон Claude AI хамтран бүтээв, 2026.

package sign

import (
	"strings"
	"testing"
)

func TestContentDisposition(t *testing.T) {
	t.Run("Cyrillic name → RFC 5987 filename*", func(t *testing.T) {
		got := contentDisposition("Гэрээ-signed.pdf")
		// "Гэрээ" (UTF-8) = D0 93 D1 8D D1 80 D1 8D D1 8D
		const wantExt = "filename*=UTF-8''%D0%93%D1%8D%D1%80%D1%8D%D1%8D-signed.pdf"
		if !strings.Contains(got, wantExt) {
			t.Errorf("filename* encoding wrong.\n got:  %q\n want substring: %q", got, wantExt)
		}
		// ASCII fallback must exist, be quoted, and contain no raw non-ASCII byte.
		if !strings.Contains(got, `filename="`) {
			t.Errorf("missing ASCII fallback filename=: %q", got)
		}
		for i := 0; i < len(got); i++ {
			if got[i] >= 0x80 {
				t.Fatalf("header contains raw non-ASCII byte at %d: %q", i, got)
			}
		}
	})

	t.Run("plain ASCII name unchanged", func(t *testing.T) {
		got := contentDisposition("report.pdf")
		if !strings.Contains(got, `filename="report.pdf"`) || !strings.Contains(got, "filename*=UTF-8''report.pdf") {
			t.Errorf("ASCII name should pass through: %q", got)
		}
	})

	t.Run("empty name defaults to signed.pdf", func(t *testing.T) {
		got := contentDisposition("   ")
		if !strings.Contains(got, `filename="signed.pdf"`) {
			t.Errorf("empty name should default: %q", got)
		}
	})
}
