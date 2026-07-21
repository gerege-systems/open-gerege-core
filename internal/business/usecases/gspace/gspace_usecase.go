// Government Template Platform V3.0
// Gerege Systems Development Team болон Claude AI хамтран бүтээв, 2026.

// Package gspace нь "Gerege Space" (апп-ын өөрийн SFTP хадгалалт)-ын business logic —
// хэрэглэгч-тус-бүрийн файл жагсаах/оруулах/татах/устгах + квот (default 2MB) шалгалт.
package gspace

import (
	"context"

	gsclient "template/pkg/gspace"
)

// Overview — хэрэглэгчийн Gerege Space-ийн товч (файлууд + ашиглалт/квот).
type Overview struct {
	Files []gsclient.FileInfo
	Used  int64
	Limit int64
}

type Usecase interface {
	// Overview нь хэрэглэгчийн файлууд + ашигласан/нийт эзлэхүүнийг буцаана.
	Overview(ctx context.Context, userID string) (Overview, error)
	// Upload нь файл оруулна — квот хэтэрвэл татгалзана.
	Upload(ctx context.Context, userID, name string, data []byte) error
	// Download нь файлын агуулгыг буцаана.
	Download(ctx context.Context, userID, name string) ([]byte, error)
	// Delete нь файлыг устгана.
	Delete(ctx context.Context, userID, name string) error
	// Limit нь нэг хэрэглэгчийн квот (байт).
	Limit() int64
}
