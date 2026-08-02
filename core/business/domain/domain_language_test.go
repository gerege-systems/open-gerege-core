// Gerege Systems Development Team болон Claude AI хамтран бүтээв, 2026.

package domain_test

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/gerege-systems/open-gerege-core/core/business/domain"
)

func TestValidateLanguageCode(t *testing.T) {
	valid := []string{"mn", "en", "kok", "en-US", "zh-Hans", "zh-Hans-CN"}
	for _, code := range valid {
		assert.NoError(t, domain.ValidateLanguageCode(code), "код %q хүчинтэй байх ёстой", code)
	}

	// Код нь URL зам болон cookie-д ордог тул зам таслах / тэмдэгт оруулахыг барина.
	invalid := []string{"", "e", "ENGLISH", "en_US", "en US", "../etc/passwd", "en/../mn", "en;drop"}
	for _, code := range invalid {
		assert.Error(t, domain.ValidateLanguageCode(code), "код %q татгалзагдах ёстой", code)
	}
}

func TestValidateLanguage(t *testing.T) {
	require.NoError(t, domain.ValidateLanguage("ja", "日本語", "ja-JP"))

	assert.Error(t, domain.ValidateLanguage("ja", "   ", "ja-JP"), "хоосон нэр")
	assert.Error(t, domain.ValidateLanguage("ja", strings.Repeat("x", 81), "ja-JP"), "хэт урт нэр")
	assert.Error(t, domain.ValidateLanguage("ja", "日本語", "not a locale"), "буруу locale")
}

func TestValidateDictionary(t *testing.T) {
	require.NoError(t, domain.ValidateDictionary(map[string]string{"a": "b"}))

	assert.Error(t, domain.ValidateDictionary(nil), "хоосон dictionary")
	assert.Error(t, domain.ValidateDictionary(map[string]string{"  ": "b"}), "хоосон түлхүүр")
	assert.Error(t, domain.ValidateDictionary(map[string]string{
		strings.Repeat("k", domain.LanguageKeyMaxBytes+1): "b",
	}), "хэт урт түлхүүр")
	assert.Error(t, domain.ValidateDictionary(map[string]string{
		"a": strings.Repeat("v", domain.LanguageValueMaxBytes+1),
	}), "хэт урт утга")

	tooMany := make(map[string]string, domain.LanguageDictionaryMaxKeys+1)
	for i := 0; i <= domain.LanguageDictionaryMaxKeys; i++ {
		tooMany[strings.Repeat("k", 3)+string(rune(i))] = "v"
	}
	assert.Error(t, domain.ValidateDictionary(tooMany), "хэт олон түлхүүр")
}

func TestPlaceholders(t *testing.T) {
	assert.Nil(t, domain.Placeholders("энгийн мөр"))
	assert.Equal(t, []string{"{name}"}, domain.Placeholders("Сайн уу, {name}"))
	// Давхардсаныг нэг удаа, эрэмбэлж буцаана.
	assert.Equal(t, []string{"{0}", "{1}"}, domain.Placeholders("{1} ба {0} ба {1}"))
}

func TestPlaceholdersPreserved(t *testing.T) {
	assert.True(t, domain.PlaceholdersPreserved("энгийн", "simple"), "байрлуулагчгүй бол үргэлж зөв")
	assert.True(t, domain.PlaceholdersPreserved("Сайн уу, {name}", "Hello, {name}"))
	assert.True(t, domain.PlaceholdersPreserved("{0} / {1}", "{1} / {0}"), "дараалал өөрчлөгдөж болно")

	assert.False(t, domain.PlaceholdersPreserved("Сайн уу, {name}", "Hello, 名前"), "орчуулагдсан")
	assert.False(t, domain.PlaceholdersPreserved("Сайн уу, {name}", "Hello"), "гээгдсэн")
	assert.False(t, domain.PlaceholdersPreserved("{a}", "{a} {b}"), "нэмэгдсэн")
}
