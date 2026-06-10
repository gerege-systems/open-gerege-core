// Gerege Template Version 27.0
// Gerege Systems Development Team болон Claude AI хамтран бүтээв, 2026.

package v1

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"

	"template/internal/constants"
	"template/pkg/logger"
)

type HealthHandler struct {
	pool        *pgxpool.Pool
	redisClient *redis.Client
}

func NewHealthHandler(pool *pgxpool.Pool, redisClient *redis.Client) HealthHandler {
	return HealthHandler{pool: pool, redisClient: redisClient}
}

func writeRawJSON(w http.ResponseWriter, statusCode int, body any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(statusCode)
	_ = json.NewEncoder(w).Encode(body)
}

func (h HealthHandler) Health(w http.ResponseWriter, r *http.Request) {
	writeRawJSON(w, http.StatusOK, map[string]any{
		"status":  true,
		"message": "service is healthy",
	})
}

func (h HealthHandler) Ready(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 3*time.Second)
	defer cancel()

	checks := map[string]string{}
	healthy := true

	// өгөгдлийн санг шалга — pgx pool-ийн ping. Бодит алдааг зөвхөн логд
	// бичнэ; хариунд driver/host detail гаргахгүй (мэдээлэл задлахаас сэргийлж).
	if err := h.pool.Ping(ctx); err != nil {
		logger.ErrorWithContext(ctx, "readiness: database unreachable", logger.Fields{
			constants.LoggerCategory: constants.LoggerCategoryDatabase,
			"error":                  err.Error(),
		})
		checks["database"] = "unreachable"
		healthy = false
	} else {
		checks["database"] = "ok"
	}

	// redis-г шалга
	if h.redisClient != nil {
		if err := h.redisClient.Ping(ctx).Err(); err != nil {
			logger.ErrorWithContext(ctx, "readiness: redis unreachable", logger.Fields{
				constants.LoggerCategory: constants.LoggerCategoryCache,
				"error":                  err.Error(),
			})
			checks["redis"] = "unreachable"
			healthy = false
		} else {
			checks["redis"] = "ok"
		}
	}

	status := http.StatusOK
	if !healthy {
		status = http.StatusServiceUnavailable
	}
	writeRawJSON(w, status, map[string]any{
		"status": healthy,
		"checks": checks,
	})
}
