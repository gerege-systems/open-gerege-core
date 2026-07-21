// Government Template Platform V3.0
// Gerege Systems Development Team болон Claude AI хамтран бүтээв, 2026.

package requests

// SSONativeRequest нь POST /sso/native-ийн body юм — mobile (PKCE, public
// client) урсгалаас ирсэн authorization code-ийг солиход хэрэглэгдэнэ. State
// байхгүй; орлуулан PKCE-ийн code_verifier interception/replay-аас хамгаална.
type SSONativeRequest struct {
	Code         string `json:"code" validate:"required"`
	CodeVerifier string `json:"code_verifier" validate:"required"`
	// RedirectURI нь native client-д бүртгэгдсэн (жишээ
	// geregetemp://oauth2/callback) байх ёстой — code exchange-д яг тааруулна.
	RedirectURI string `json:"redirect_uri" validate:"required"`
}
