// Government Template Platform V3.0
// Gerege Systems Development Team болон Claude AI хамтран бүтээв, 2026.

// Package crypto нь storage-д мэдрэмтгий утга (жишээ TOTP secret)-ыг AES-256-GCM-
// ээр шифрлэх энгийн туслах. Түлхүүрийг тохиргооны мөрөөс SHA-256-аар 32 байт
// болгон гаргадаг тул дурын урттай нууц утга ажиллана (production-д хүчтэй
// түлхүүр тохируулна). integrations-ийн tokenCipher-тэй ижил зарчим.
package crypto

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"io"
)

// Cipher нь AES-256-GCM шифрлэгч.
type Cipher struct {
	gcm cipher.AEAD
}

// New нь тохиргооны нууц мөрөөс Cipher үүсгэнэ (SHA-256 → 32 байт түлхүүр).
func New(secret string) (*Cipher, error) {
	key := sha256.Sum256([]byte(secret))
	block, err := aes.NewCipher(key[:])
	if err != nil {
		return nil, fmt.Errorf("new cipher: %w", err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("new gcm: %w", err)
	}
	return &Cipher{gcm: gcm}, nil
}

// Encrypt нь plaintext-ийг base64(nonce||ciphertext) болгоно. Хоосныг хоосноор.
func (c *Cipher) Encrypt(plain string) (string, error) {
	if plain == "" {
		return "", nil
	}
	nonce := make([]byte, c.gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", fmt.Errorf("nonce: %w", err)
	}
	sealed := c.gcm.Seal(nonce, nonce, []byte(plain), nil)
	return base64.StdEncoding.EncodeToString(sealed), nil
}

// Decrypt нь Encrypt-ийн урвуу.
func (c *Cipher) Decrypt(enc string) (string, error) {
	if enc == "" {
		return "", nil
	}
	raw, err := base64.StdEncoding.DecodeString(enc)
	if err != nil {
		return "", fmt.Errorf("decode: %w", err)
	}
	ns := c.gcm.NonceSize()
	if len(raw) < ns {
		return "", fmt.Errorf("ciphertext too short")
	}
	nonce, ciphertext := raw[:ns], raw[ns:]
	plain, err := c.gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return "", fmt.Errorf("open: %w", err)
	}
	return string(plain), nil
}
