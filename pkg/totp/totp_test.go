// Government Template Platform V3.0
// Gerege Systems Development Team болон Claude AI хамтран бүтээв, 2026.

// TOTP (RFC 6238) боодлын unit тест: secret үүсгэх → код баталгаажуулах,
// буруу код татгалзах.
package totp_test

import (
	"net/url"
	"testing"
	"time"

	pqtotp "github.com/pquerna/otp/totp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"template/pkg/totp"
)

func TestGenerate(t *testing.T) {
	secret, uri, err := totp.Generate("Government Template Platform V3.0", "bat@dgov.mn")
	require.NoError(t, err)
	assert.NotEmpty(t, secret)

	// otpauth:// URI нь authenticator app-д QR болж уншигдана — issuer/account
	// нь тэнд харагдах тул зөв суулгагдсан эсэхийг шалгана.
	u, parseErr := url.Parse(uri)
	require.NoError(t, parseErr)
	assert.Equal(t, "otpauth", u.Scheme)
	assert.Equal(t, "totp", u.Host)
	assert.Equal(t, "Government Template Platform V3.0", u.Query().Get("issuer"))
	assert.Equal(t, secret, u.Query().Get("secret"))
	assert.Contains(t, u.Path, "bat@dgov.mn")
}

func TestGenerateUniqueSecrets(t *testing.T) {
	// Хэрэглэгч бүр өөрийн secret-тэй байх ёстой.
	s1, _, err := totp.Generate("DAN", "a@dgov.mn")
	require.NoError(t, err)
	s2, _, err := totp.Generate("DAN", "a@dgov.mn")
	require.NoError(t, err)
	assert.NotEqual(t, s1, s2)
}

func TestValidate(t *testing.T) {
	secret, _, err := totp.Generate("Government Template Platform V3.0", "bat@dgov.mn")
	require.NoError(t, err)

	t.Run("зөв код → true", func(t *testing.T) {
		code, genErr := pqtotp.GenerateCode(secret, time.Now())
		require.NoError(t, genErr)
		assert.True(t, totp.Validate(code, secret))
	})

	t.Run("буруу код → false", func(t *testing.T) {
		// Тоо биш код хэзээ ч хүчинтэй байж чадахгүй (санамсаргүй таарахгүй).
		assert.False(t, totp.Validate("abcdef", secret))
		assert.False(t, totp.Validate("", secret))
	})

	t.Run("өөр secret-ийн код → false", func(t *testing.T) {
		other, _, genErr := totp.Generate("DAN", "b@dgov.mn")
		require.NoError(t, genErr)
		code, codeErr := pqtotp.GenerateCode(other, time.Now())
		require.NoError(t, codeErr)
		assert.False(t, totp.Validate(code, secret))
	})

	t.Run("хугацаа хэтэрсэн код → false", func(t *testing.T) {
		// ±1 цонхноос (30с) хол хугацааны код хүчингүй.
		old, genErr := pqtotp.GenerateCode(secret, time.Now().Add(-10*time.Minute))
		require.NoError(t, genErr)
		assert.False(t, totp.Validate(old, secret))
	})
}
