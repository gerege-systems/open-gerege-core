// Gerege Template Version 27.0
// Gerege Systems Development Team болон Claude AI хамтран бүтээв, 2026.

package core

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"template/internal/apperror"
)

const maxRespBytes = 4 << 20 // 4 MiB

type usecase struct {
	base   string
	token  string
	client *http.Client
}

// NewUsecase нь Gerege Core клиентийг үүсгэнэ. token нь урт настай service
// bearer (CORE_API_TOKEN) — server-тал л ашиглана.
func NewUsecase(base, token string) Usecase {
	return &usecase{
		base:   strings.TrimRight(base, "/"),
		token:  token,
		client: &http.Client{Timeout: 15 * time.Second},
	}
}

func (u *usecase) FindUsers(ctx context.Context, searchText string) (json.RawMessage, error) {
	body, _ := json.Marshal(map[string]string{"search_text": searchText})
	return u.call(ctx, http.MethodPost, "/api/user/find", "", bytes.NewReader(body))
}

func (u *usecase) FindOrganizations(ctx context.Context, searchText string) (json.RawMessage, error) {
	q := url.Values{}
	q.Set("search_text", searchText)
	return u.call(ctx, http.MethodGet, "/api/organization/find", q.Encode(), nil)
}

func (u *usecase) call(ctx context.Context, method, path, query string, body io.Reader) (json.RawMessage, error) {
	if u.token == "" {
		// CORE_API_TOKEN тохируулаагүй бол Core инерт — 500 биш, UI-д ойлгомжтой
		// мессежээр (илэрцгүй шалтгаан) буцаана. CoreSearchView нь энэ data.message-
		// ийг харуулдаг тул оператор Core-г идэвхжүүлэхэд юу дутууг шууд ойлгоно.
		return json.RawMessage(`{"message":"Core үйлчилгээ (core.gerege.mn) тохируулаагүй байна. CORE_API_TOKEN-ыг backend.env-д тохируулна уу."}`), nil
	}
	endpoint := u.base + path
	if query != "" {
		endpoint += "?" + query
	}
	req, err := http.NewRequestWithContext(ctx, method, endpoint, body)
	if err != nil {
		return nil, apperror.InternalCause(err)
	}
	req.Header.Set("Authorization", "Bearer "+u.token)
	req.Header.Set("Accept", "application/json")
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	res, err := u.client.Do(req)
	if err != nil {
		return nil, apperror.InternalCause(fmt.Errorf("core request: %w", err))
	}
	defer func() { _ = res.Body.Close() }()

	data, err := io.ReadAll(io.LimitReader(res.Body, maxRespBytes))
	if err != nil {
		return nil, apperror.InternalCause(fmt.Errorf("core read: %w", err))
	}
	if res.StatusCode < 200 || res.StatusCode >= 300 {
		return nil, apperror.InternalCause(fmt.Errorf("core api returned %d", res.StatusCode))
	}
	if !json.Valid(data) {
		return json.RawMessage("null"), nil
	}
	return json.RawMessage(data), nil
}
