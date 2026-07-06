// Gerege Template Version 27.0
// Gerege Systems Development Team болон Claude AI хамтран бүтээв, 2026.

package auth

import (
	"context"
	"fmt"
	"time"

	"template/internal/business/usecases/users"
	"template/internal/datasources/caches"
	"template/pkg/eid"
	"template/pkg/google"
	"template/pkg/jwt"
	"template/pkg/logger"
	"template/pkg/verify"
	"template/pkg/xyp"
)

// usecase нь хамаарлууд болон method хоорондын төлөвийг агуулдаг. Нэг зан
// төлөв өөрчлөгдөхөд PR-ийн diff нарийн (surgical) хэвээр үлдэхийн тулд method
// бүр өөрийн файлд байрладаг.
// GoogleClient нь Google OAuth-ийн auth usecase-д хэрэгтэй хэсэг — *google.Client
// үүнийг хангадаг; тестэд хуурамчаар тавихад хялбар.
type GoogleClient interface {
	Configured() bool
	Exchange(ctx context.Context, code, redirectURI string) (*google.User, error)
}

type usecase struct {
	users      users.Usecase
	jwtService jwt.JWTService
	verifier   verify.Sender
	eid        eid.Client
	xyp        xyp.Lookuper
	google     GoogleClient
	redisCache caches.RedisCache
	cfg        Config
}

// NewUsecase нь auth урсгалуудыг холбодог. Identity унших/бичихэд
// users.Usecase-ээс, бүх email/SMS OTP (бүртгэл баталгаажуулах болон нууц үг
// сэргээх)-д verify.Sender (GeregeCloud Verify API)-ээс, бусад auth-хэсгүүдэд
// jwt/redis-ээс хамаардаг.
func NewUsecase(usersUC users.Usecase, jwtService jwt.JWTService, verifier verify.Sender, eidClient eid.Client, xypClient xyp.Lookuper, googleClient GoogleClient, redisCache caches.RedisCache, cfg Config) Usecase {
	return &usecase{
		users:      usersUC,
		jwtService: jwtService,
		verifier:   verifier,
		eid:        eidClient,
		xyp:        xypClient,
		google:     googleClient,
		redisCache: redisCache,
		cfg:        cfg,
	}
}

// dummyBcryptHash нь Login доторх "хэрэглэгч олдсонгүй" болон "буруу нууц үг"
// гэсэн салаануудын хоорондох цаг хугацааны зөрүүг далдлахад ашигладаг урьдчилан
// тооцоолсон bcrypt hash юм. Үүний эсрэг дурын нууц үг харьцуулах нь бодит
// bcrypt харьцуулалттай ижил ~100мс зарцуулдаг тул хариуны хоцролтоор дамжсан
// хэрэглэгчийн тооллогоос (enumeration) сэргийлдэг.
const dummyBcryptHash = "$2a$10$N9qo8uLOickgx2ZMRZoMyeIjZAgcfl7p92ldGxad68LJZdL17lhWy"

// tokenCutoffTTL нь тасалбар (cutoff) дохио хэр удаан амьдрах ёстойг
// хязгаарладаг. Access токенууд хамгийн ихдээ uc.cfg.JWTExpired цагийн дараа
// дуусдаг тул 24ц нь дамжиж буй аливаа access токеноос тав тухтай удаан
// амьдардаг. Refresh токены хүчингүй болголт нь DB-д
// (User.TokensRevokedBefore) байрладаг бөгөөд энэ TTL-д хамаарахгүй.
const tokenCutoffTTL = 24 * time.Hour

// recordTokenCutoff нь "энэ агшингаас өмнө олгогдсон токенууд хүчингүй болсон"
// гэсэн тэмдгийг нийтэлдэг бөгөөд үүнийг AuthMiddleware хүсэлт бүр дээр
// шалгадаг тул алдагдсан access токен нь жам ёсоор дуусахыг хүлээхгүйгээр,
// хэрэглэгч нууц үгээ эргүүлмэгц л ажиллахаа болино.
func (uc *usecase) recordTokenCutoff(ctx context.Context, userID string, when time.Time) {
	key := TokenCutoffKey(userID)
	if err := uc.redisCache.Set(ctx, key, fmt.Sprintf("%d", when.Unix())); err != nil {
		logger.ErrorWithContext(ctx, "auth: failed to write token cutoff (non-fatal)", logger.Fields{
			"step":    "redis_set_token_cutoff",
			"error":   err.Error(),
			"user_id": userID,
		})
		return
	}
	_ = uc.redisCache.Expire(ctx, key, tokenCutoffTTL)
}

// incrWithExpiry нь brute-force/lockout тоологчдыг атомаар нэмэгдүүлж, тэдгээр
// нь үргэлж дуусах хугацаатай (TTL-тэй) байхыг хангадаг. Анхны Expire алдаа
// гарвал (жишээ нь Redis-ийн түр зуурын саатал) key мөнхөд TTL-гүй үлдэж,
// тоологч хэзээ ч reset болохгүй тул хэрэглэгч бүрмөсөн түгжигдэх эрсдэлтэй.
// Үүнээс сэргийлэхийн тулд:
//   - attempts == 1 (key шинээр үүссэн) үед TTL тогтооно;
//   - дараагийн нэмэгдүүлэлт бүрт PTTL-ээр TTL байхгүй (< 0) бол дахин
//     тогтооно — урьд нь алдаатай эсвэл алдагдсан TTL-г нөхнө.
//
// Expire алдааг хэзээ ч чимээгүй залгидаггүй — бүгдийг лог болгож, дараагийн
// нэмэгдүүлэлт дээр дахин оролдоно. Тоологчид буцах нь зүгээр (зөөлөн
// бүтэлгүйтэл) тул incr алдаа гарвал зүгээр л буцаана.
func (uc *usecase) incrWithExpiry(ctx context.Context, key string, ttl time.Duration, step string) (int64, error) {
	attempts, incrErr := uc.redisCache.Incr(ctx, key)
	if incrErr != nil {
		return 0, incrErr
	}

	needExpire := attempts == 1
	if !needExpire {
		// TTL байхгүй (мөнхийн) эсвэл key байхгүй бол дахин тогтооно. PTTL
		// нь TTL-гүй үед -1, key байхгүй үед -2 (хоёулаа < 0) буцаадаг.
		if pttl, pttlErr := uc.redisCache.PTTL(ctx, key); pttlErr != nil {
			logger.ErrorWithContext(ctx, "auth: failed to read counter TTL (non-fatal)", logger.Fields{
				"step":  step + "_pttl",
				"error": pttlErr.Error(),
				"key":   key,
			})
		} else if pttl < 0 {
			needExpire = true
		}
	}

	if needExpire {
		if expErr := uc.redisCache.Expire(ctx, key, ttl); expErr != nil {
			logger.ErrorWithContext(ctx, "auth: failed to set counter TTL (non-fatal)", logger.Fields{
				"step":  step + "_expire",
				"error": expErr.Error(),
				"key":   key,
			})
		}
	}

	return attempts, nil
}

// rememberRefresh нь refresh jti-г refresh токены exp-тэй тохирох TTL-тэйгээр
// Redis-д хадгалдаг. /refresh болон /logout нь эндхийн байхгүй байдлыг
// "хүчингүй болсон" гэж үздэг бөгөөд энэ нь access токены хар жагсаалтгүйгээр
// logout хэрхэн ажилладгийн учир юм.
func (uc *usecase) rememberRefresh(ctx context.Context, pair jwt.TokenPair) error {
	ttl := time.Until(pair.RefreshExpiresAt)
	if ttl <= 0 {
		return fmt.Errorf("refresh token already expired")
	}
	if err := uc.redisCache.Set(ctx, RefreshKey(pair.RefreshJTI), pair.RefreshJTI); err != nil {
		return err
	}
	// Set() нь кэшийн хэмжээний дуусах хугацааг минутаар хэрэглэдэг; refresh
	// токен бүр өөрийн TTL-тэй байхын тулд тодорхой override хийнэ.
	return uc.redisCache.Expire(ctx, RefreshKey(pair.RefreshJTI), ttl)
}
