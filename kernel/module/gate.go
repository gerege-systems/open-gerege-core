// Gerege Systems Development Team болон Claude AI хамтран бүтээв, 2026.

package module

import (
	"encoding/json"
	"net/http"
	"strings"
)

// ModuleForPath нь HTTP замыг эзэмшигч модулийг олно (хамгийн УРТ таарсан
// угтвар ялна). Аль ч модульд харьяалагдахгүй зам (жишээ: /api/ root,
// /metrics) → ok=false, тэдгээрийг gate хаадаггүй.
//
// Угтварын семантик: "/"-ээр төгссөн угтвар нь sub-tree ("/api/v1/gov/" нь
// /api/v1/gov/services-т таарна, мөн "/api/v1/gov" замд өөрт нь ч таарна);
// "/"-гүй нь яг зам эсвэл sub-tree ("/userinfo" ба "/userinfo/x").
func (r *Registry) ModuleForPath(path string) (string, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	bestID, bestLen := "", -1
	for prefix, id := range r.byPrefix {
		if len(prefix) <= bestLen || !prefixMatches(prefix, path) {
			continue
		}
		bestID, bestLen = id, len(prefix)
	}
	return bestID, bestLen >= 0
}

func prefixMatches(prefix, path string) bool {
	if strings.HasSuffix(prefix, "/") {
		return strings.HasPrefix(path, prefix) || path == strings.TrimSuffix(prefix, "/")
	}
	return path == prefix || strings.HasPrefix(path, prefix+"/")
}

// Gate нь идэвхгүй модулийн бүх route-ыг 404-өөр хаадаг middleware.
// 404 (403 биш) — модуль байхгүйтэй ижил гадаргуу үзүүлж, суусан модулиудын
// жагсаалтыг гаднаас тандах боломж олгохгүй. Хариу нь v1 BaseResponse-ийн
// хэлбэртэй тул frontend-ийн алдааны зам өөрчлөгдөхгүй.
func Gate(reg *Registry) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if id, ok := reg.ModuleForPath(r.URL.Path); ok && !reg.Enabled(id) {
				w.Header().Set("Content-Type", "application/json; charset=utf-8")
				w.WriteHeader(http.StatusNotFound)
				_ = json.NewEncoder(w).Encode(map[string]any{
					"status":  false,
					"message": "Хуудас олдсонгүй",
				})
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}
