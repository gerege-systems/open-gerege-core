// Gerege Template Version 27.0
// Gerege Systems Development Team болон Claude AI хамтран бүтээв, 2026.

package middlewares

import (
	"context"
	"net/http"
	"strconv"
	"strings"

	"template/internal/business/usecases/auth"
	"template/internal/constants"
	"template/internal/datasources/caches"
	"template/internal/datasources/rls"
	V1Handler "template/internal/http/handlers/v1"
	"template/pkg/jwt"
	"template/pkg/logger"
)

type AuthMiddleware struct {
	jwtService jwt.JWTService
	redisCache caches.RedisCache
	isAdmin    bool
}

// NewAuthMiddleware нь Bearer токеныг баталгаажуулж, нууц үг солих
// (rotation) хязгаарыг хүндэтгэж, задлан шинжилсэн claim-уудыг хүсэлтийн
// context-д хадгалдаг chi middleware буцаана. Хариуг буцааж 401-ээр богино
// холбодог (гинжийг таслах арга барил — алдаа дээр next-г дуудахгүй).
func NewAuthMiddleware(jwtService jwt.JWTService, redisCache caches.RedisCache, isAdmin bool) func(http.Handler) http.Handler {
	m := &AuthMiddleware{
		jwtService: jwtService,
		redisCache: redisCache,
		isAdmin:    isAdmin,
	}
	return m.Handle
}

func (m *AuthMiddleware) Handle(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		const (
			middlewareName = "AuthMiddleware"
			fileName       = "middleware_auth.go"
		)
		logCtx := r.Context()
		path := r.URL.Path

		authHeader := r.Header.Get("Authorization")
		if authHeader == "" {
			logger.WarnWithContext(logCtx, "Auth: missing Authorization header", logger.Fields{
				"middleware": middlewareName,
				"file":       fileName,
				"step":       "read_header",
				"path":       path,
			})
			_ = V1Handler.NewAbortResponse(w, r, "missing authorization header")
			return
		}

		headerParts := strings.Split(authHeader, " ")
		if len(headerParts) != 2 {
			logger.WarnWithContext(logCtx, "Auth: invalid Authorization header format", logger.Fields{
				"middleware": middlewareName,
				"file":       fileName,
				"step":       "parse_header",
				"path":       path,
			})
			_ = V1Handler.NewAbortResponse(w, r, "invalid header format")
			return
		}

		if headerParts[0] != "Bearer" {
			logger.WarnWithContext(logCtx, "Auth: non-Bearer scheme", logger.Fields{
				"middleware": middlewareName,
				"file":       fileName,
				"step":       "scheme_check",
				"path":       path,
				"scheme":     headerParts[0],
			})
			_ = V1Handler.NewAbortResponse(w, r, "token must content bearer")
			return
		}

		user, err := m.jwtService.ParseToken(headerParts[1])
		if err != nil {
			logger.WarnWithContext(logCtx, "Auth: token parse failed", logger.Fields{
				"middleware": middlewareName,
				"file":       fileName,
				"step":       "parse_token",
				"path":       path,
				"error":      err.Error(),
			})
			_ = V1Handler.NewAbortResponse(w, r, "invalid token")
			return
		}

		// Logout хийсэн access токеныг (deny-list) татгалз. Logout нь jti-г
		// токены үлдсэн амьдрах хугацаагаар Redis-д бичдэг; miss (redis.Nil)
		// нь logout хийгдээгүй гэсэн үг. FAIL-CLOSED: Redis-ийн жинхэнэ алдаа
		// (miss биш) үед revocation-ийг шалгаж чадахгүй тул болзошгүй
		// татгалзсан токеныг нэвтрүүлэлгүй 503 буцаана — refresh урсгал аль
		// хэдийн fail-closed тул нийцтэй. (Redis доголдвол богино хугацаанд
		// auth зогсоно; аюулгүй байдлыг availability-ээс дээгүүр тавьсан.)
		if m.redisCache != nil && user.ID != "" {
			denied, getErr := m.redisCache.Get(logCtx, auth.AccessDenyKey(user.ID))
			switch {
			case getErr == nil && denied != "":
				logger.WarnWithContext(logCtx, "Auth: access token denied by logout", logger.Fields{
					"middleware": middlewareName,
					"file":       fileName,
					"step":       "check_access_deny",
					"path":       path,
					"user_id":    user.UserID,
				})
				_ = V1Handler.NewAbortResponse(w, r, "token has been revoked")
				return
			case getErr != nil && !caches.IsCacheMiss(getErr):
				logger.ErrorWithContext(logCtx, "Auth: revocation check unavailable (fail-closed)", logger.Fields{
					"middleware": middlewareName,
					"file":       fileName,
					"step":       "check_access_deny",
					"path":       path,
					"error":      getErr.Error(),
				})
				_ = V1Handler.NewErrorResponse(w, r, http.StatusServiceUnavailable, "session verification temporarily unavailable")
				return
			}
		}

		// Хэрэглэгчийн хамгийн сүүлийн нууц үг солихоос (rotation) өмнө
		// олгогдсон access токенуудыг татгалз. Хязгаарыг ChangePassword
		// Redis руу нийтэлдэг; miss (redis.Nil) нь сүүлийн үед солилт
		// хийгдээгүй гэсэн үг тул токен нэвтэрнэ. Дээрхтэй ижил FAIL-CLOSED —
		// Redis-ийн жинхэнэ алдаа үед cutoff-ийг шалгаж чадахгүй тул 503.
		if m.redisCache != nil && user.IssuedAt != nil {
			cutoffStr, getErr := m.redisCache.Get(logCtx, auth.TokenCutoffKey(user.UserID))
			switch {
			case getErr == nil && cutoffStr != "":
				// JWT IssuedAt нь секунд хүртэл бутархайгүй болгогддог тул нууц
				// үг солихтой яг нэг секундэд олгогдсон токеныг бас татгалзахын
				// тулд <= ашиглана (хил дээрх секундын цоорхойг хаана).
				if cutoff, parseErr := strconv.ParseInt(cutoffStr, 10, 64); parseErr == nil && user.IssuedAt.Unix() <= cutoff {
					logger.WarnWithContext(logCtx, "Auth: token revoked by password rotation", logger.Fields{
						"middleware": middlewareName,
						"file":       fileName,
						"step":       "check_pwd_cutoff",
						"path":       path,
						"user_id":    user.UserID,
						"issued_at":  user.IssuedAt.Unix(),
						"cutoff":     cutoff,
					})
					_ = V1Handler.NewAbortResponse(w, r, "token has been revoked")
					return
				}
			case getErr != nil && !caches.IsCacheMiss(getErr):
				logger.ErrorWithContext(logCtx, "Auth: rotation-cutoff check unavailable (fail-closed)", logger.Fields{
					"middleware": middlewareName,
					"file":       fileName,
					"step":       "check_pwd_cutoff",
					"path":       path,
					"error":      getErr.Error(),
				})
				_ = V1Handler.NewErrorResponse(w, r, http.StatusServiceUnavailable, "session verification temporarily unavailable")
				return
			}
		}

		if user.IsAdmin != m.isAdmin && !user.IsAdmin {
			logger.WarnWithContext(logCtx, "Auth: insufficient privilege", logger.Fields{
				"middleware":     middlewareName,
				"file":           fileName,
				"step":           "privilege_check",
				"path":           path,
				"user_id":        user.UserID,
				"required_admin": m.isAdmin,
				"user_is_admin":  user.IsAdmin,
			})
			_ = V1Handler.NewAbortResponse(w, r, "you don't have access for this action")
			return
		}

		ctx := context.WithValue(r.Context(), constants.CtxAuthenticatedUserKey, user)
		// RLS: баталгаажсан хэрэглэгчийн identity-г DB давхаргад дамжуулна.
		// Admin бол бүх мөр; энгийн хэрэглэгч зөвхөн өөрийн мөр
		// (Postgres Row-Level Security бодлогоор хэрэгждэг).
		if user.IsAdmin {
			ctx = rls.WithAdmin(ctx, user.UserID)
		} else {
			ctx = rls.WithUser(ctx, user.UserID)
		}
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}
