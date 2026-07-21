// Government Template Platform V3.0
// Gerege Systems Development Team болон Claude AI хамтран бүтээв, 2026.

package integrations

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"io"
)

// tokenCipher нь OAuth токеныг storage-д хадгалахын өмнө AES-256-GCM-ээр
// шифрлэнэ. Түлхүүрийг тохиргооны мөрөөс SHA-256-аар 32 байт болгон гаргадаг тул
// дурын урттай нууц утга ажиллана (хоосон бол сул default — production-д
// INTEGRATION_ENC_KEY заавал тохируулна).
type tokenCipher struct {
	gcm cipher.AEAD
}

func newTokenCipher(secret string) (*tokenCipher, error) {
	key := sha256.Sum256([]byte(secret))
	block, err := aes.NewCipher(key[:])
	if err != nil {
		return nil, fmt.Errorf("new cipher: %w", err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("new gcm: %w", err)
	}
	return &tokenCipher{gcm: gcm}, nil
}

// encrypt нь plaintext-ийг шифрлэж base64(nonce||ciphertext) болгон буцаана.
// Хоосон оролтыг хоосноор үлдээнэ (refresh_token заримдаа байхгүй).
func (c *tokenCipher) encrypt(plain string) (string, error) {
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

// decrypt нь encrypt-ийн урвуу үйлдэл.
func (c *tokenCipher) decrypt(enc string) (string, error) {
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
