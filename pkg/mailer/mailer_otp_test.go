// Gerege Template Version 27.0
// Gerege Systems Development Team болон Claude AI хамтран бүтээв, 2026.

package mailer

import (
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestRenderOTPBody_ContainsCodeAndYear(t *testing.T) {
	body, err := renderOTPBody("987654")
	assert.NoError(t, err)
	assert.Contains(t, body, "987654", "code must appear in the rendered HTML")
	assert.Contains(t, body, strconv.Itoa(time.Now().Year()), "current year must be embedded")
	assert.Contains(t, body, defaultAppName)
}

func TestRenderOTPBody_AutoEscapesUnsafeInput(t *testing.T) {
	// html/template нь <script>-г escape хийх ёстой; хэрэв хэн нэгэн хэрэглэгчийн
	// удирддаг өгөгдлийг OTP кодын талбарт холбовол энэ нь түүнийг хамгаална.
	body, err := renderOTPBody("<script>alert(1)</script>")
	assert.NoError(t, err)
	assert.NotContains(t, body, "<script>alert(1)</script>")
	assert.True(t, strings.Contains(body, "&lt;script&gt;"))
}
