// Government Template Platform V3.0
// Gerege Systems Development Team болон Claude AI хамтран бүтээв, 2026.

package domain

import "testing"

func TestRelayWebhookSignVerify(t *testing.T) {
	secret := "s3cr3t-key"
	body := []byte(`{"event":"forward","service_code":"passport"}`)

	sig := RelaySignWebhook(secret, body)
	if sig == "" || sig[:7] != "sha256=" {
		t.Fatalf("unexpected signature format: %q", sig)
	}
	if !RelayVerifyWebhook(secret, sig, body) {
		t.Error("valid signature should verify")
	}
	// Буруу нууц.
	if RelayVerifyWebhook("wrong", sig, body) {
		t.Error("signature must fail with wrong secret")
	}
	// Өөрчлөгдсөн бие.
	if RelayVerifyWebhook(secret, sig, []byte(`{"event":"tampered"}`)) {
		t.Error("signature must fail when body changes")
	}
	// Хоосон нууц/гарын үсэг.
	if RelayVerifyWebhook("", sig, body) || RelayVerifyWebhook(secret, "", body) {
		t.Error("empty secret or signature must not verify")
	}
}

func TestRelayNewWebhookSecret(t *testing.T) {
	a, b := RelayNewWebhookSecret(), RelayNewWebhookSecret()
	if len(a) != 64 {
		t.Errorf("expected 64 hex chars, got %d", len(a))
	}
	if a == b {
		t.Error("secrets should be random/unique")
	}
}

func TestRelayIsDemoEndpoint(t *testing.T) {
	for _, e := range []string{"", "  ", "demo://loopback"} {
		if !RelayIsDemoEndpoint(e) {
			t.Errorf("%q should be demo", e)
		}
	}
	if RelayIsDemoEndpoint("https://peer.gov.mn/relay/webhook") {
		t.Error("real https endpoint should not be demo")
	}
}
