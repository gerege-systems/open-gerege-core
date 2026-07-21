// Government Template Platform V3.0
// Gerege Systems Development Team болон Claude AI хамтран бүтээв, 2026.

package gspace

import (
	"context"
	"fmt"
	"strings"

	"template/internal/apperror"
	gsclient "template/pkg/gspace"
)

type usecase struct {
	client *gsclient.Client
	quota  int64
}

func NewUsecase(client *gsclient.Client, quota int64) Usecase {
	if quota <= 0 {
		quota = 2 << 20
	}
	return &usecase{client: client, quota: quota}
}

func (uc *usecase) Limit() int64 { return uc.quota }

func (uc *usecase) Overview(ctx context.Context, userID string) (Overview, error) {
	if !uc.client.Configured() {
		return Overview{}, apperror.Internal("Gerege Space тохируулаагүй байна")
	}
	files, err := uc.client.List(userID)
	if err != nil {
		return Overview{}, apperror.InternalCause(fmt.Errorf("gspace list: %w", err))
	}
	var used int64
	for _, f := range files {
		used += f.Size
	}
	return Overview{Files: files, Used: used, Limit: uc.quota}, nil
}

func (uc *usecase) Upload(ctx context.Context, userID, name string, data []byte) error {
	if !uc.client.Configured() {
		return apperror.Internal("Gerege Space тохируулаагүй байна")
	}
	if strings.TrimSpace(name) == "" || len(data) == 0 {
		return apperror.BadRequest("Файл дутуу байна")
	}
	if int64(len(data)) > uc.quota {
		return apperror.BadRequest(fmt.Sprintf("Файл хэт том — квот %d MB", uc.quota/(1<<20)))
	}
	// Одоогийн ашиглалт + шинэ файл нийлбэр квотоос хэтрэхгүй байх ёстой. Ижил
	// нэртэй файл байвал орлуулна (тэр хэмжээг хасна).
	used, err := uc.client.Usage(userID)
	if err != nil {
		return apperror.InternalCause(fmt.Errorf("gspace usage: %w", err))
	}
	if existing := uc.existingSize(userID, name); existing > 0 {
		used -= existing
	}
	if used+int64(len(data)) > uc.quota {
		return apperror.BadRequest(fmt.Sprintf("Зай хүрэлцэхгүй — квот %d MB", uc.quota/(1<<20)))
	}
	if err := uc.client.Upload(userID, name, data); err != nil {
		return apperror.InternalCause(fmt.Errorf("gspace upload: %w", err))
	}
	return nil
}

// existingSize нь ижил нэртэй файлын одоогийн хэмжээ (орлуулах үед квотоос хасна).
func (uc *usecase) existingSize(userID, name string) int64 {
	files, err := uc.client.List(userID)
	if err != nil {
		return 0
	}
	base := strings.TrimSpace(name)
	for _, f := range files {
		if f.Name == base {
			return f.Size
		}
	}
	return 0
}

func (uc *usecase) Download(ctx context.Context, userID, name string) ([]byte, error) {
	if !uc.client.Configured() {
		return nil, apperror.Internal("Gerege Space тохируулаагүй байна")
	}
	data, err := uc.client.Download(userID, name)
	if err != nil {
		return nil, apperror.NotFound("Файл олдсонгүй")
	}
	return data, nil
}

func (uc *usecase) Delete(ctx context.Context, userID, name string) error {
	if !uc.client.Configured() {
		return apperror.Internal("Gerege Space тохируулаагүй байна")
	}
	if err := uc.client.Delete(userID, name); err != nil {
		return apperror.InternalCause(fmt.Errorf("gspace delete: %w", err))
	}
	return nil
}
