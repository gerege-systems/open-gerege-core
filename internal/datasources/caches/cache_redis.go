// Gerege Template Version 27.0
// Gerege Systems Development Team болон Claude AI хамтран бүтээв, 2026.

package caches

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/redis/go-redis/extra/redisotel/v9"
	"github.com/redis/go-redis/v9"
	"template/internal/constants"
	"template/pkg/logger"
)

// IsCacheMiss нь алдаа нь "key байхгүй" (redis.Nil) эсэхийг заана — Redis-ийн
// жинхэнэ алдаанаас (холболт таслагдсан / timeout) ялгах зорилготой. Revocation
// шалгалтыг fail-closed хийхэд шаардлагатай: miss = бичлэг алга (татгалзаагүй,
// үргэлжлүүл), бусад алдаа = Redis-г шалгаж чадсангүй (болзошгүй татгалзсаныг
// нэвтрүүлэхгүй).
func IsCacheMiss(err error) bool {
	return errors.Is(err, redis.Nil)
}

// defaultOpTimeout нь Redis-ийн алхам бүрийг хязгаарлаж, удаан/хүрэх
// боломжгүй Redis нь дуудагч goroutine-уудыг хязгааргүй хугацаагаар
// блоклохоос сэргийлнэ.
const defaultOpTimeout = 3 * time.Second

type RedisCache interface {
	// Set нь value-г JSON болгон marshal хийж, кэшийн нийтлэг өгөгдмөл
	// TTL-тэйгээр key дор бичнэ. Дуудалт бүр defaultOpTimeout-оор
	// хязгаарлагдсан тул удаан Redis дуудагчийг блоклож чадахгүй.
	Set(ctx context.Context, key string, value interface{}) error
	// Get нь key дахь JSON-оор decode хийсэн мөрийг буцаана, эсвэл key
	// байхгүй үед redis.Nil-г буцаана.
	Get(ctx context.Context, key string) (string, error)
	// GetDel нь key дахь JSON-оор decode хийсэн мөрийг буцаахын зэрэгцээ
	// тэр key-г атомаар устгана (Redis GETDEL). Нэг удаагийн токеныг (жишээ
	// нь refresh jti) уншиж-устгахдаа атомаар хийснээр TOCTOU-гийн улмаас
	// зэрэгцээ хоёр хүсэлт нэгэн зэрэг амжилттай болохоос сэргийлнэ. Key
	// байхгүй үед redis.Nil-г буцаана.
	GetDel(ctx context.Context, key string) (string, error)
	// Del нь key-г устгана. Байхгүй key нь алдаа биш.
	Del(ctx context.Context, key string) error
	// Incr нь key дахь бүхэл тоог атомаар нэгээр нэмэгдүүлж (байхгүй бол
	// 1 болгон үүсгэнэ), шинэ утгыг буцаана.
	Incr(ctx context.Context, key string) (int64, error)
	// Expire нь key дээрх TTL-г (дахин) тогтооно. otp_attempts зэрэг
	// тоологчдыг тогтсон цонхонд хязгаарлахад ашиглагдана.
	Expire(ctx context.Context, key string, ttl time.Duration) error
	// PTTL нь key-ийн үлдсэн амьдрах хугацааг миллисекундын нарийвчлалтай
	// буцаана. Key байхгүй бол -2, TTL-гүй (мөнхийн) бол -1-ийг буцаана.
	// Тоологчийн TTL алдагдсан эсэхийг шалгаж, дахин Expire тогтооход
	// ашиглагдана.
	PTTL(ctx context.Context, key string) (time.Duration, error)
	// Close нь үндсэн холболтын pool-г зогсооно.
	Close() error
	// Client нь энэ interface-ээр гаргаагүй командууд (эрүүл мэндийн
	// шалгалт, pipeline) шаардлагатай дуудагчдад зориулж түүхий
	// *redis.Client-г илчилнэ. Дээрх төрөлжсөн method-уудыг илүүд үз.
	Client() *redis.Client
}

type redisCache struct {
	host     string
	db       int
	password string
	expires  time.Duration
	client   *redis.Client
}

func NewRedisCache(host string, db int, password string, expires time.Duration) RedisCache {
	client := redis.NewClient(&redis.Options{
		Addr:     host,
		Password: password,
		DB:       db,
	})
	// redisotel нь команд бүрийг (GET/SET/INCR/EXPIRE/...) идэвхтэй
	// trace-д semconv DB атрибутаар тэмдэглэсэн child span болгон холбоно.
	// Суулгалт амжилтгүй болсон нь tracing SDK түвшинд буруу
	// тохируулагдсан гэсэн үг — апп-ыг зогсоохын оронд бүртгээд үргэлжлүүл.
	if err := redisotel.InstrumentTracing(client); err != nil {
		logger.Info("redis tracing instrumentation skipped: "+err.Error(),
			logger.Fields{constants.LoggerCategory: constants.LoggerCategoryCache})
	}
	return &redisCache{
		host:     host,
		db:       db,
		password: password,
		expires:  expires,
		client:   client,
	}
}

// withTimeout нь дуудагчийн ctx-ээс хязгаарлагдсан context гаргаж авч,
// Redis-ийн алхмууд defaultOpTimeout-оос удаан блоклохгүй байхыг хангана.
func withTimeout(parent context.Context) (context.Context, context.CancelFunc) {
	if parent == nil {
		parent = context.Background()
	}
	return context.WithTimeout(parent, defaultOpTimeout)
}

func (cache *redisCache) Set(ctx context.Context, key string, value interface{}) error {
	payload, err := json.Marshal(value)
	if err != nil {
		return err
	}

	ctx, cancel := withTimeout(ctx)
	defer cancel()
	return cache.client.Set(ctx, key, payload, cache.expires*time.Minute).Err()
}

func (cache *redisCache) Get(ctx context.Context, key string) (string, error) {
	ctx, cancel := withTimeout(ctx)
	defer cancel()
	val, err := cache.client.Get(ctx, key).Result()
	if err != nil {
		return "", err
	}

	var decoded string
	if err := json.Unmarshal([]byte(val), &decoded); err != nil {
		return "", err
	}
	return decoded, nil
}

func (cache *redisCache) GetDel(ctx context.Context, key string) (string, error) {
	ctx, cancel := withTimeout(ctx)
	defer cancel()
	val, err := cache.client.GetDel(ctx, key).Result()
	if err != nil {
		return "", err
	}

	var decoded string
	if err := json.Unmarshal([]byte(val), &decoded); err != nil {
		return "", err
	}
	return decoded, nil
}

func (cache *redisCache) Del(ctx context.Context, key string) error {
	ctx, cancel := withTimeout(ctx)
	defer cancel()
	return cache.client.Del(ctx, key).Err()
}

func (cache *redisCache) Incr(ctx context.Context, key string) (int64, error) {
	ctx, cancel := withTimeout(ctx)
	defer cancel()
	return cache.client.Incr(ctx, key).Result()
}

func (cache *redisCache) Expire(ctx context.Context, key string, ttl time.Duration) error {
	ctx, cancel := withTimeout(ctx)
	defer cancel()
	return cache.client.Expire(ctx, key, ttl).Err()
}

func (cache *redisCache) PTTL(ctx context.Context, key string) (time.Duration, error) {
	ctx, cancel := withTimeout(ctx)
	defer cancel()
	return cache.client.PTTL(ctx, key).Result()
}

func (cache *redisCache) Close() error {
	return cache.client.Close()
}

func (cache *redisCache) Client() *redis.Client {
	return cache.client
}
