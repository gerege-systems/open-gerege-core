// Government Template Platform V3.0
// Gerege Systems Development Team болон Claude AI хамтран бүтээв, 2026.

package oidc

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"template/internal/apperror"
	"template/internal/business/domain"
)

// memKeys нь keyStore-ийн санах-ой хувилбар.
type memKeys struct {
	keys []domain.SigningKey
}

func (m *memKeys) Active(context.Context) (domain.SigningKey, error) {
	for _, k := range m.keys {
		if k.Active {
			return k, nil
		}
	}
	return domain.SigningKey{}, apperror.NotFound("no active signing key")
}

func (m *memKeys) All(context.Context) ([]domain.SigningKey, error) { return m.keys, nil }

func (m *memKeys) Insert(_ context.Context, k domain.SigningKey) error {
	m.keys = append([]domain.SigningKey{k}, m.keys...)
	return nil
}

func (m *memKeys) RetireActive(context.Context) error {
	for i := range m.keys {
		m.keys[i].Active = false
	}
	return nil
}

func newManager(t *testing.T) (*KeyManager, *memKeys) {
	t.Helper()
	store := &memKeys{}
	m, err := NewKeyManager(store, "test-encryption-key-for-signing-keys")
	if err != nil {
		t.Fatalf("NewKeyManager: %v", err)
	}
	return m, store
}

func TestEnsureKeyGeneratesOnceThenReuses(t *testing.T) {
	m, store := newManager(t)
	ctx := context.Background()

	if err := m.EnsureKey(ctx); err != nil {
		t.Fatalf("EnsureKey: %v", err)
	}
	if len(store.keys) != 1 {
		t.Fatalf("expected one key after first EnsureKey, got %d", len(store.keys))
	}

	// Дахин дуудахад ШИНЭ түлхүүр үүсгэх ёсгүй — эс бөгөөс api дахин эхлэх бүрд
	// RP-үүдийн кэшилсэн JWKS хүчингүй болно.
	if err := m.EnsureKey(ctx); err != nil {
		t.Fatalf("EnsureKey (second): %v", err)
	}
	if len(store.keys) != 1 {
		t.Fatalf("EnsureKey must be idempotent, got %d keys", len(store.keys))
	}
}

func TestPrivateKeyIsNeverStoredInTheClear(t *testing.T) {
	m, store := newManager(t)
	if err := m.EnsureKey(context.Background()); err != nil {
		t.Fatalf("EnsureKey: %v", err)
	}
	stored := store.keys[0].PrivateKeyEnc
	if stored == "" {
		t.Fatal("private key was not stored at all")
	}
	if strings.Contains(stored, "PRIVATE KEY") || strings.Contains(stored, "BEGIN") {
		t.Fatal("private key appears to be stored as PEM rather than encrypted")
	}
	// Шифрлэгдсэн утга нь нийтийн модультай ижил байх учиргүй.
	if strings.Contains(stored, string(store.keys[0].PublicJWK)) {
		t.Fatal("stored private key contains the public JWK verbatim")
	}
}

func TestSignerRoundTripsThroughEncryption(t *testing.T) {
	m, store := newManager(t)
	ctx := context.Background()
	if err := m.EnsureKey(ctx); err != nil {
		t.Fatalf("EnsureKey: %v", err)
	}

	kid, priv, err := m.Signer(ctx)
	if err != nil {
		t.Fatalf("Signer: %v", err)
	}
	if kid == "" || priv == nil {
		t.Fatal("Signer returned an empty key")
	}
	if kid != store.keys[0].KID {
		t.Fatalf("Signer kid %q does not match the active key %q", kid, store.keys[0].KID)
	}

	// Кэш хоосон байхад ч (шинэ manager) хадгалсан түлхүүр задарч ижил байх ёстой.
	fresh, err := NewKeyManager(store, "test-encryption-key-for-signing-keys")
	if err != nil {
		t.Fatalf("NewKeyManager: %v", err)
	}
	_, priv2, err := fresh.Signer(ctx)
	if err != nil {
		t.Fatalf("Signer (fresh manager): %v", err)
	}
	if priv.N.Cmp(priv2.N) != 0 {
		t.Fatal("decrypting the stored key produced a different key")
	}
}

func TestSignerFailsWithWrongEncryptionKey(t *testing.T) {
	m, store := newManager(t)
	ctx := context.Background()
	if err := m.EnsureKey(ctx); err != nil {
		t.Fatalf("EnsureKey: %v", err)
	}

	wrong, err := NewKeyManager(store, "a-completely-different-encryption-key")
	if err != nil {
		t.Fatalf("NewKeyManager: %v", err)
	}
	if _, _, err := wrong.Signer(ctx); err == nil {
		t.Fatal("a wrong INTEGRATION_ENC_KEY must not silently yield a usable signing key")
	}
}

func TestJWKSPublishesRetiredKeysToo(t *testing.T) {
	m, store := newManager(t)
	ctx := context.Background()
	if err := m.EnsureKey(ctx); err != nil {
		t.Fatalf("EnsureKey: %v", err)
	}
	oldKID := store.keys[0].KID

	newKID, err := m.Rotate(ctx)
	if err != nil {
		t.Fatalf("Rotate: %v", err)
	}
	if newKID == oldKID {
		t.Fatal("rotation must produce a new kid")
	}

	set, err := m.JWKS(ctx)
	if err != nil {
		t.Fatalf("JWKS: %v", err)
	}
	if len(set.Keys) != 2 {
		t.Fatalf("JWKS should keep the retired key so tokens signed with it still verify, got %d keys", len(set.Keys))
	}

	// Идэвхтэй нь эхэнд; бүх түлхүүр зөв бүтэцтэй.
	var first jwk
	if err := json.Unmarshal(set.Keys[0], &first); err != nil {
		t.Fatalf("unmarshal jwk: %v", err)
	}
	if first.Kid != newKID {
		t.Fatalf("the active key should be published first, got %q want %q", first.Kid, newKID)
	}
	if first.Kty != "RSA" || first.Alg != algRS256 || first.Use != "sig" {
		t.Fatalf("unexpected jwk header: %+v", first)
	}
	if first.N == "" || first.E == "" {
		t.Fatal("jwk is missing the modulus or exponent")
	}
}

func TestKIDIsStableForTheSameKey(t *testing.T) {
	m, store := newManager(t)
	ctx := context.Background()
	if err := m.EnsureKey(ctx); err != nil {
		t.Fatalf("EnsureKey: %v", err)
	}
	_, priv, err := m.Signer(ctx)
	if err != nil {
		t.Fatalf("Signer: %v", err)
	}
	if got := thumbprint(&priv.PublicKey); got != store.keys[0].KID {
		t.Fatalf("kid must be the RFC 7638 thumbprint of its own key: got %q, stored %q", got, store.keys[0].KID)
	}
}
