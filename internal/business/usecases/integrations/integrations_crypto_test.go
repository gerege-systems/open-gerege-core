// Government Template Platform V3.0
// Gerege Systems Development Team болон Claude AI хамтран бүтээв, 2026.

package integrations

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTokenCipher_RoundTrip(t *testing.T) {
	c, err := newTokenCipher("some-secret-key")
	require.NoError(t, err)

	plain := "ya29.a0AfH-very-secret-access-token"
	enc, err := c.encrypt(plain)
	require.NoError(t, err)
	assert.NotEqual(t, plain, enc, "ciphertext must differ from plaintext")

	dec, err := c.decrypt(enc)
	require.NoError(t, err)
	assert.Equal(t, plain, dec)
}

func TestTokenCipher_EmptyStaysEmpty(t *testing.T) {
	c, err := newTokenCipher("k")
	require.NoError(t, err)

	enc, err := c.encrypt("")
	require.NoError(t, err)
	assert.Empty(t, enc)

	dec, err := c.decrypt("")
	require.NoError(t, err)
	assert.Empty(t, dec)
}

func TestTokenCipher_WrongKeyFails(t *testing.T) {
	a, _ := newTokenCipher("key-a")
	b, _ := newTokenCipher("key-b")

	enc, err := a.encrypt("secret")
	require.NoError(t, err)

	_, err = b.decrypt(enc)
	require.Error(t, err, "decrypting with a different key must fail (GCM auth)")
}

func TestTokenCipher_NonceIsRandom(t *testing.T) {
	c, _ := newTokenCipher("k")
	e1, _ := c.encrypt("same")
	e2, _ := c.encrypt("same")
	assert.NotEqual(t, e1, e2, "each encryption must use a fresh nonce")
}
