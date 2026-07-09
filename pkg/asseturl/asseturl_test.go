// Gerege Template Version 27.0
// Gerege Systems Development Team болон Claude AI хамтран бүтээв, 2026.

package asseturl

import "testing"

func TestValidate(t *testing.T) {
	ok := []string{
		"https://lh3.googleusercontent.com/d/abc123",
		"https://drive.google.com/uc?id=abc",
		"https://drive.usercontent.google.com/download?id=abc",
		"https://LH3.GoogleUserContent.com/d/x", // том/жижиг үсэг
	}
	for _, u := range ok {
		if err := Validate(u); err != nil {
			t.Errorf("expected %q to be allowed, got %v", u, err)
		}
	}

	bad := []string{
		"http://lh3.googleusercontent.com/d/x",       // https биш
		"https://169.254.169.254/latest/meta-data/",  // metadata (SSRF)
		"https://localhost:9000/internal",            // дотоод үйлчилгээ
		"https://evil.com/d/x",                       // цагаан жагсаалтад алга
		"https://evil-googleusercontent.com/x",       // suffix trick
		"https://lh3.googleusercontent.com.evil.com", // домэйн trick
		"javascript:alert(1)",                        // XSS схем
		"data:text/html,<script>alert(1)</script>",   // data схем
		"", // хоосон
	}
	for _, u := range bad {
		if err := Validate(u); err == nil {
			t.Errorf("expected %q to be rejected", u)
		}
	}
}
