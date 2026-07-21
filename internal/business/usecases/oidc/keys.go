// Government Template Platform V3.0
// Gerege Systems Development Team болон Claude AI хамтран бүтээв, 2026.

// Package oidc нь өөрийн OAuth2/OIDC provider-ийн протоколын логик. Энэ файл нь
// id_token-ийн гарын үсгийн түлхүүр болон JWKS-ийг хариуцна.
//
// Түлхүүр нь RS256 (RSA-2048) — RP-ийн бүх сан дэмждэг хамгийн өргөн хүлээн
// зөвшөөрөгдсөн алгоритм; Hydra ч мөн адилыг ашиглаж байсан тул RP-үүдийн
// шалгах тал өөрчлөгдөхгүй.
package oidc

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"math/big"
	"sync"

	"template/internal/apperror"
	"template/internal/business/domain"
	"template/pkg/crypto"
)

const (
	rsaKeyBits = 2048
	algRS256   = "RS256"
)

// keyStore нь гарын үсгийн түлхүүрийн хадгалалт.
type keyStore interface {
	Active(ctx context.Context) (domain.SigningKey, error)
	All(ctx context.Context) ([]domain.SigningKey, error)
	Insert(ctx context.Context, k domain.SigningKey) error
	RetireActive(ctx context.Context) error
}

// KeyManager нь идэвхтэй хувийн түлхүүрийг задалж кэшлэн, JWKS-ийг угсарна.
type KeyManager struct {
	store  keyStore
	cipher *crypto.Cipher

	mu     sync.RWMutex
	cached *cachedKey
}

type cachedKey struct {
	kid  string
	priv *rsa.PrivateKey
}

// NewKeyManager нь encKey (INTEGRATION_ENC_KEY)-ээр хувийн түлхүүрийг
// шифрлэх/задлах KeyManager үүсгэнэ.
func NewKeyManager(store keyStore, encKey string) (*KeyManager, error) {
	c, err := crypto.New(encKey)
	if err != nil {
		return nil, fmt.Errorf("oidc: signing key cipher: %w", err)
	}
	return &KeyManager{store: store, cipher: c}, nil
}

// EnsureKey нь идэвхтэй түлхүүр байгаа эсэхийг шалгаж, байхгүй бол үүсгэнэ.
// Boot үед дуудагдана — эхний ажиллагаанд түлхүүр бэлэн болно.
func (m *KeyManager) EnsureKey(ctx context.Context) error {
	if _, err := m.store.Active(ctx); err == nil {
		return nil
	} else if !apperror.IsNotFound(err) {
		return err
	}
	_, err := m.generate(ctx)
	return err
}

// Rotate нь одоогийн түлхүүрийг тэтгэвэрт гаргаж шинийг үүсгэнэ. Хуучин нь
// JWKS-д үлдэх тул түүгээр зурсан id_token-ууд дуусах хүртлээ хүчинтэй.
func (m *KeyManager) Rotate(ctx context.Context) (string, error) {
	if err := m.store.RetireActive(ctx); err != nil {
		return "", err
	}
	return m.generate(ctx)
}

// generate нь шинэ RSA түлхүүр үүсгэж, шифрлэн хадгална.
func (m *KeyManager) generate(ctx context.Context) (string, error) {
	priv, err := rsa.GenerateKey(rand.Reader, rsaKeyBits)
	if err != nil {
		return "", fmt.Errorf("oidc: generate rsa key: %w", err)
	}
	der, err := x509.MarshalPKCS8PrivateKey(priv)
	if err != nil {
		return "", fmt.Errorf("oidc: marshal private key: %w", err)
	}
	enc, err := m.cipher.Encrypt(base64.StdEncoding.EncodeToString(der))
	if err != nil {
		return "", fmt.Errorf("oidc: encrypt private key: %w", err)
	}

	kid := thumbprint(&priv.PublicKey)
	jwk, err := json.Marshal(publicJWK(kid, &priv.PublicKey))
	if err != nil {
		return "", fmt.Errorf("oidc: marshal jwk: %w", err)
	}

	if err := m.store.Insert(ctx, domain.SigningKey{
		KID: kid, Alg: algRS256, PrivateKeyEnc: enc, PublicJWK: jwk, Active: true,
	}); err != nil {
		return "", err
	}

	m.mu.Lock()
	m.cached = &cachedKey{kid: kid, priv: priv}
	m.mu.Unlock()
	return kid, nil
}

// Signer нь гарын үсэг зурах идэвхтэй түлхүүрийг (kid + private key) буцаана.
// Задалсан түлхүүрийг санах ойд кэшилнэ — хүсэлт бүрд AES задлалт хийхгүй.
func (m *KeyManager) Signer(ctx context.Context) (string, *rsa.PrivateKey, error) {
	m.mu.RLock()
	c := m.cached
	m.mu.RUnlock()

	rec, err := m.store.Active(ctx)
	if err != nil {
		return "", nil, err
	}
	if c != nil && c.kid == rec.KID {
		return c.kid, c.priv, nil
	}

	priv, err := m.decrypt(rec.PrivateKeyEnc)
	if err != nil {
		return "", nil, err
	}
	m.mu.Lock()
	m.cached = &cachedKey{kid: rec.KID, priv: priv}
	m.mu.Unlock()
	return rec.KID, priv, nil
}

func (m *KeyManager) decrypt(enc string) (*rsa.PrivateKey, error) {
	b64, err := m.cipher.Decrypt(enc)
	if err != nil {
		return nil, fmt.Errorf("oidc: decrypt private key: %w", err)
	}
	der, err := base64.StdEncoding.DecodeString(b64)
	if err != nil {
		return nil, fmt.Errorf("oidc: decode private key: %w", err)
	}
	parsed, err := x509.ParsePKCS8PrivateKey(der)
	if err != nil {
		return nil, fmt.Errorf("oidc: parse private key: %w", err)
	}
	priv, ok := parsed.(*rsa.PrivateKey)
	if !ok {
		return nil, fmt.Errorf("oidc: stored signing key is %T, want *rsa.PrivateKey", parsed)
	}
	return priv, nil
}

// JWKSet нь RFC 7517-ийн JWK Set.
type JWKSet struct {
	Keys []json.RawMessage `json:"keys"`
}

// JWKS нь нийтлэх бүх нийтийн түлхүүрийг буцаана (идэвхтэй нь эхэнд).
func (m *KeyManager) JWKS(ctx context.Context) (JWKSet, error) {
	keys, err := m.store.All(ctx)
	if err != nil {
		return JWKSet{}, err
	}
	out := JWKSet{Keys: make([]json.RawMessage, 0, len(keys))}
	for _, k := range keys {
		out.Keys = append(out.Keys, json.RawMessage(k.PublicJWK))
	}
	return out, nil
}

// jwk нь RSA нийтийн түлхүүрийн JWK дүрслэл.
type jwk struct {
	Kty string `json:"kty"`
	Use string `json:"use"`
	Alg string `json:"alg"`
	Kid string `json:"kid"`
	N   string `json:"n"`
	E   string `json:"e"`
}

func publicJWK(kid string, pub *rsa.PublicKey) jwk {
	return jwk{
		Kty: "RSA", Use: "sig", Alg: algRS256, Kid: kid,
		N: base64.RawURLEncoding.EncodeToString(pub.N.Bytes()),
		E: base64.RawURLEncoding.EncodeToString(big.NewInt(int64(pub.E)).Bytes()),
	}
}

// thumbprint нь RFC 7638-ийн JWK thumbprint-ыг kid болгон ашиглана — түлхүүрээс
// детерминистик гардаг тул давхардахгүй бөгөөд гадны талд утга учиргүй.
func thumbprint(pub *rsa.PublicKey) string {
	// RFC 7638: зөвхөн шаардлагатай талбарууд, ЦАГААН ТОЛГОЙН дарааллаар, зайгүй.
	// %q — base64url цагаан толгойд escape хийх тэмдэгт байхгүй тул гаралт нь
	// RFC 7638-ийн шаарддаг яг тэр канон хэлбэртэй байна.
	canonical := fmt.Sprintf(`{"e":%q,"kty":"RSA","n":%q}`,
		base64.RawURLEncoding.EncodeToString(big.NewInt(int64(pub.E)).Bytes()),
		base64.RawURLEncoding.EncodeToString(pub.N.Bytes()),
	)
	sum := sha256.Sum256([]byte(canonical))
	return base64.RawURLEncoding.EncodeToString(sum[:])
}

// PublicKey нь kid-ээр нийтийн түлхүүрийг буцаана (тэтгэвэрт гарснаас нь ч).
// id_token_hint зэрэг ӨӨРСДИЙН гаргасан token-ыг шалгахад ашиглана.
func (m *KeyManager) PublicKey(ctx context.Context, kid string) (*rsa.PublicKey, error) {
	keys, err := m.store.All(ctx)
	if err != nil {
		return nil, err
	}
	for _, k := range keys {
		if k.KID != kid {
			continue
		}
		var j jwk
		if err := json.Unmarshal(k.PublicJWK, &j); err != nil {
			return nil, fmt.Errorf("oidc: parse stored jwk: %w", err)
		}
		n, err := base64.RawURLEncoding.DecodeString(j.N)
		if err != nil {
			return nil, fmt.Errorf("oidc: decode jwk modulus: %w", err)
		}
		e, err := base64.RawURLEncoding.DecodeString(j.E)
		if err != nil {
			return nil, fmt.Errorf("oidc: decode jwk exponent: %w", err)
		}
		return &rsa.PublicKey{
			N: new(big.Int).SetBytes(n),
			E: int(new(big.Int).SetBytes(e).Int64()),
		}, nil
	}
	return nil, apperror.NotFound("signing key not found")
}
