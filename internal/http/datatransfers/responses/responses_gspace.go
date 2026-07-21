// Government Template Platform V3.0
// Gerege Systems Development Team болон Claude AI хамтран бүтээв, 2026.

package responses

import (
	"time"

	gspaceuc "template/internal/business/usecases/gspace"
)

// GSpaceFileResponse нь Gerege Space дахь нэг файл.
type GSpaceFileResponse struct {
	Name    string    `json:"name"`
	Size    int64     `json:"size"`
	ModTime time.Time `json:"mod_time"`
}

// GSpaceOverviewResponse нь хэрэглэгчийн Gerege Space-ийн товч (файл + ашиглалт/квот).
type GSpaceOverviewResponse struct {
	Files []GSpaceFileResponse `json:"files"`
	Used  int64                `json:"used"`
	Limit int64                `json:"limit"`
}

// FromGSpaceOverview нь usecase-ийн Overview-ийг DTO руу буулгана.
func FromGSpaceOverview(o gspaceuc.Overview) GSpaceOverviewResponse {
	files := make([]GSpaceFileResponse, 0, len(o.Files))
	for _, f := range o.Files {
		files = append(files, GSpaceFileResponse{Name: f.Name, Size: f.Size, ModTime: f.ModTime})
	}
	return GSpaceOverviewResponse{Files: files, Used: o.Used, Limit: o.Limit}
}
