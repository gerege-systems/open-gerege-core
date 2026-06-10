// Gerege Template Version 27.0
// Gerege Systems Development Team болон Claude AI хамтран бүтээв, 2026.

package v1

import (
	"net/http"
)

func Root(w http.ResponseWriter, r *http.Request) {
	writeRawJSON(w, http.StatusOK, map[string]any{
		"status":  true,
		"message": "v1 online...",
	})
}
