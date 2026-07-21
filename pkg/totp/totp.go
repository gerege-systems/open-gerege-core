// Government Template Platform V3.0
// Gerege Systems Development Team болон Claude AI хамтран бүтээв, 2026.

// Package totp нь TOTP (RFC 6238) 2FA-ийн нимгэн боодол — secret үүсгэх,
// authenticator app-д уншуулах otpauth:// URI гаргах, код баталгаажуулах.
// pquerna/otp дээр суурилна; QR-г frontend (otpauth URI)-д зурна.
package totp

import "github.com/pquerna/otp/totp"

// Generate нь шинэ TOTP secret (base32) + otpauth:// provisioning URI буцаана.
// issuer нь app-ийн нэр (жишээ "Government Template Platform V3.0"), account нь хэрэглэгчийн
// таних (email г.м.) — authenticator app-д эдгээр харагдана.
func Generate(issuer, account string) (secret, url string, err error) {
	key, err := totp.Generate(totp.GenerateOpts{
		Issuer:      issuer,
		AccountName: account,
	})
	if err != nil {
		return "", "", err
	}
	return key.Secret(), key.URL(), nil
}

// Validate нь 6 оронтой TOTP кодыг secret-тэй тулгаж шалгана (±1 цонх).
func Validate(code, secret string) bool {
	return totp.Validate(code, secret)
}
