// Government Template Platform V3.0
// Gerege Systems Development Team болон Claude AI хамтран бүтээв, 2026.

// Package gspace нь "Gerege Space" (апп-ын өөрийн SFTP хадгалалт)-ын handler —
// хэрэглэгч файлаа жагсаах/оруулах/татах/устгах.
package gspace

import (
	"encoding/base64"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	gspaceuc "template/internal/business/usecases/gspace"
	httpauth "template/internal/http/auth"
	"template/internal/http/datatransfers/requests"
	"template/internal/http/datatransfers/responses"
	v1 "template/internal/http/handlers/v1"
	"template/pkg/validators"
)

// gspaceUploadBodyMax нь /v1/gspace/upload-ийн JSON body-ийн дээд хэмжээ.
// Файл base64-ээр дамждаг тул 2 MB quota-д base64(≈4/3)+JSON overhead нэмээд
// 4 MiB тавина — эс бөгөөс default 1 MiB body cap файлыг ~750 KB дээр таслана.
const gspaceUploadBodyMax int64 = 4 << 20

type Handler struct {
	usecase gspaceuc.Usecase
}

func NewHandler(usecase gspaceuc.Usecase) Handler {
	return Handler{usecase: usecase}
}

func (h Handler) user(w http.ResponseWriter, r *http.Request) (string, bool) {
	u, err := httpauth.CurrentUserFromContext(r)
	if err != nil {
		_ = v1.NewAbortResponse(w, r, "invalid token")
		return "", false
	}
	return u.ID, true
}

// Overview godoc
// @Summary Gerege Space товч (файлууд + квот)
// @Tags gspace
// @Produce json
// @Security BearerAuth
// @Success 200 {object} v1.BaseResponse
// @Router /v1/gspace [get]
func (h Handler) Overview(w http.ResponseWriter, r *http.Request) error {
	uid, ok := h.user(w, r)
	if !ok {
		return nil
	}
	ov, err := h.usecase.Overview(r.Context(), uid)
	if err != nil {
		return v1.RespondWithError(w, r, err)
	}
	return v1.NewSuccessResponse(w, r, http.StatusOK, "gspace overview", responses.FromGSpaceOverview(ov))
}

// Upload godoc
// @Summary Gerege Space-д файл оруулах
// @Tags gspace
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param payload body requests.GSpaceUploadRequest true "Файл (base64)"
// @Success 200 {object} v1.BaseResponse
// @Router /v1/gspace/upload [post]
func (h Handler) Upload(w http.ResponseWriter, r *http.Request) error {
	uid, ok := h.user(w, r)
	if !ok {
		return nil
	}
	var req requests.GSpaceUploadRequest
	// Файл base64-ээр JSON body дотор ирдэг тул default 1 MiB нь ~750 KB-аас том
	// файлыг таслаж, 2 MB quota-г хүрэхгүй болгодог. base64(2 MB)+JSON overhead-д
	// хүрэлцэхүйц 4 MiB cap тавина (бодит хэмжээ/quota-г usecase шалгана).
	if err := v1.DecodeBodyLimit(r, &req, gspaceUploadBodyMax); err != nil {
		return v1.NewErrorResponse(w, r, http.StatusBadRequest, "invalid request body")
	}
	if err := validators.ValidatePayloads(req); err != nil {
		return v1.RespondWithError(w, r, err)
	}
	data, err := base64.StdEncoding.DecodeString(req.Data)
	if err != nil {
		return v1.NewErrorResponse(w, r, http.StatusBadRequest, "invalid base64 data")
	}
	if err := h.usecase.Upload(r.Context(), uid, req.Name, data); err != nil {
		return v1.RespondWithError(w, r, err)
	}
	return v1.NewSuccessResponse(w, r, http.StatusOK, "file uploaded", nil)
}

// Download godoc
// @Summary Gerege Space-с файл татах
// @Tags gspace
// @Produce octet-stream
// @Security BearerAuth
// @Param name query string true "Файлын нэр"
// @Success 200 {file} binary
// @Router /v1/gspace/download [get]
func (h Handler) Download(w http.ResponseWriter, r *http.Request) error {
	uid, ok := h.user(w, r)
	if !ok {
		return nil
	}
	name := r.URL.Query().Get("name")
	if strings.TrimSpace(name) == "" {
		return v1.NewErrorResponse(w, r, http.StatusBadRequest, "name required")
	}
	data, err := h.usecase.Download(r.Context(), uid, name)
	if err != nil {
		return v1.RespondWithError(w, r, err)
	}
	w.Header().Set("Content-Type", "application/octet-stream")
	w.Header().Set("Content-Disposition", contentDisposition(name))
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(data) //nolint:gosec // octet-stream + attachment + nosniff: bytes are downloaded, never rendered as HTML
	return nil
}

// Delete godoc
// @Summary Gerege Space-с файл устгах
// @Tags gspace
// @Produce json
// @Security BearerAuth
// @Param name query string true "Файлын нэр"
// @Success 200 {object} v1.BaseResponse
// @Router /v1/gspace [delete]
func (h Handler) Delete(w http.ResponseWriter, r *http.Request) error {
	uid, ok := h.user(w, r)
	if !ok {
		return nil
	}
	name := r.URL.Query().Get("name")
	if strings.TrimSpace(name) == "" {
		return v1.NewErrorResponse(w, r, http.StatusBadRequest, "name required")
	}
	if err := h.usecase.Delete(r.Context(), uid, name); err != nil {
		return v1.RespondWithError(w, r, err)
	}
	return v1.NewSuccessResponse(w, r, http.StatusOK, "file deleted", nil)
}

// contentDisposition нь UTF-8 файлын нэрийг RFC 5987 дагуу зөв дамжуулна.
func contentDisposition(name string) string {
	name = strings.TrimSpace(name)
	if name == "" {
		name = "file"
	}
	ascii := make([]byte, 0, len(name))
	for i := 0; i < len(name); i++ {
		c := name[i]
		if c < 0x20 || c >= 0x7f || c == '"' || c == '\\' {
			ascii = append(ascii, '_')
		} else {
			ascii = append(ascii, c)
		}
	}
	return fmt.Sprintf("attachment; filename=%q; filename*=UTF-8''%s", string(ascii), url.PathEscape(name))
}
