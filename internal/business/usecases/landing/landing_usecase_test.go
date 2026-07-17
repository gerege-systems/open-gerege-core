// Gerege Template Version 27.0
// Gerege Systems Development Team болон Claude AI хамтран бүтээв, 2026.

package landing

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"template/internal/apperror"
	"template/internal/business/domain"
)

// fakeLandingRepo нь LandingConfigRepository-ийн in-memory fake.
type fakeLandingRepo struct {
	config   json.RawMessage
	getErr   error
	getCalls int
	setCalls int
	lastSet  json.RawMessage
}

func (f *fakeLandingRepo) GetConfig(_ context.Context) (domain.LandingConfig, error) {
	f.getCalls++
	if f.getErr != nil {
		return domain.LandingConfig{}, f.getErr
	}
	return domain.LandingConfig{Config: f.config}, nil
}

func (f *fakeLandingRepo) SetConfig(_ context.Context, config json.RawMessage) error {
	f.setCalls++
	f.lastSet = config
	f.config = config
	return nil
}

func TestGetConfigCaches(t *testing.T) {
	repo := &fakeLandingRepo{config: json.RawMessage(`{"brand":{"logoUrl":"/brand.webp"}}`)}
	uc := NewUsecase(repo)

	got := uc.GetConfig(context.Background())
	assert.JSONEq(t, `{"brand":{"logoUrl":"/brand.webp"}}`, string(got))

	// TTL дотор — DB-ээс нэг л удаа уншина.
	_ = uc.GetConfig(context.Background())
	assert.Equal(t, 1, repo.getCalls)
}

func TestGetConfigFailOpen(t *testing.T) {
	repo := &fakeLandingRepo{getErr: errors.New("db down")}
	uc := NewUsecase(repo)

	got := uc.GetConfig(context.Background())
	// Кэш байхгүй тул хоосон объект руу fail-open (алдаа шидэхгүй).
	assert.JSONEq(t, `{}`, string(got))
}

func TestSetConfigInvalidatesCache(t *testing.T) {
	repo := &fakeLandingRepo{config: json.RawMessage(`{"v":1}`)}
	uc := NewUsecase(repo)

	_ = uc.GetConfig(context.Background()) // кэш дүүргэнэ
	require.Equal(t, 1, repo.getCalls)

	require.NoError(t, uc.SetConfig(context.Background(), json.RawMessage(`{"v":2}`)))

	got := uc.GetConfig(context.Background()) // кэш хүчингүй → дахин уншина
	assert.Equal(t, 2, repo.getCalls)
	assert.JSONEq(t, `{"v":2}`, string(got))
}

func TestSetConfigRejectsNonObject(t *testing.T) {
	uc := NewUsecase(&fakeLandingRepo{})

	for _, bad := range []string{`[1,2,3]`, `"a string"`, `42`, `not json`} {
		err := uc.SetConfig(context.Background(), json.RawMessage(bad))
		require.Error(t, err, "input %q should be rejected", bad)
		var domErr *apperror.DomainError
		require.ErrorAs(t, err, &domErr)
		assert.Equal(t, apperror.ErrTypeBadRequest, domErr.Type)
	}
}

func TestSetConfigRejectsTooLarge(t *testing.T) {
	uc := NewUsecase(&fakeLandingRepo{})
	huge := `{"rawCss":"` + strings.Repeat("a", maxConfigBytes) + `"}`

	err := uc.SetConfig(context.Background(), json.RawMessage(huge))
	require.Error(t, err)
	var domErr *apperror.DomainError
	require.ErrorAs(t, err, &domErr)
	assert.Equal(t, apperror.ErrTypeBadRequest, domErr.Type)
}

func TestSetConfigSanitizesRawCSS(t *testing.T) {
	repo := &fakeLandingRepo{}
	uc := NewUsecase(repo)

	dirty := `{"rawCss":"a{color:red} </STYLE><script>alert(1)</script> @import url(x); b{x:expression(1)} c:javascript:foo"}`
	require.NoError(t, uc.SetConfig(context.Background(), json.RawMessage(dirty)))

	var stored map[string]any
	require.NoError(t, json.Unmarshal(repo.lastSet, &stored))
	css, _ := stored["rawCss"].(string)

	lower := strings.ToLower(css)
	assert.NotContains(t, lower, "</style")
	assert.NotContains(t, lower, "<script")
	assert.NotContains(t, lower, "@import")
	assert.NotContains(t, lower, "expression(")
	assert.NotContains(t, lower, "javascript:")
	// Хууль ёсны CSS хадгалагдсан хэвээр.
	assert.Contains(t, css, "color:red")
}
