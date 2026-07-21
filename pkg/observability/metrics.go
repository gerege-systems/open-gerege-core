// Government Template Platform V3.0
// Gerege Systems Development Team болон Claude AI хамтран бүтээв, 2026.

// Package observability нь бизнес package-ууд болон collector бүртгэдэг HTTP
// middleware-ийн хооронд import цикл үүсгэлгүйгээр аливаа давхаргаас дуудаж
// болох Prometheus metric туслахуудыг ил гаргадаг.
//
// Collector-уудыг энд init() үед нэг удаа бүртгэдэг; дуудагчид үйл явдал
// тэмдэглэхдээ доорх Observe* функцуудыг ашиглана. HTTP давхарга нь өөрийн
// хүсэлтийн хүрээнд (request-scoped) collector-уудыг тусад нь бүртгэдэг.
package observability

import (
	"github.com/prometheus/client_golang/prometheus"
)

var (
	cacheOpsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "cache_operations_total",
			Help: "Cache operations by layer (ristretto|redis), operation, and result (hit|miss|error|ok).",
		},
		[]string{"layer", "op", "result"},
	)

	otpSendTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "otp_send_total",
			Help: "OTP send outcomes via GeregeCloud Verify: sent, failed.",
		},
		[]string{"result"},
	)

	dbPoolOpen = prometheus.NewGaugeFunc(
		prometheus.GaugeOpts{
			Name: "db_pool_open_connections",
			Help: "Open Postgres connections (idle + in use)",
		},
		func() float64 { return float64(currentDBStats().OpenConnections) },
	)
	dbPoolInUse = prometheus.NewGaugeFunc(
		prometheus.GaugeOpts{
			Name: "db_pool_in_use_connections",
			Help: "Postgres connections currently in use",
		},
		func() float64 { return float64(currentDBStats().InUse) },
	)
	dbPoolWait = prometheus.NewGaugeFunc(
		prometheus.GaugeOpts{
			Name: "db_pool_wait_count_total",
			Help: "Cumulative connections that waited for a free slot",
		},
		func() float64 { return float64(currentDBStats().WaitCount) },
	)
)

func init() {
	prometheus.MustRegister(cacheOpsTotal, otpSendTotal, dbPoolOpen, dbPoolInUse, dbPoolWait)
}

// DBPoolStats нь database/sql эсвэл pgxpool-аас үл хамааран pool-ийн
// статистикийн снапшот юм — provider үүнийг бөглөж, gauge callback-ууд
// уншина.
type DBPoolStats struct {
	OpenConnections int
	InUse           int
	WaitCount       int64
}

// dbStatsProvider нь эхлэх үед холбогддог бөгөөд ингэснээр GaugeFunc
// callback-ууд server package-г import хийлгүйгээр амьд pool-статистикийг
// унших боломжтой болно. Энэ нь pgxpool.Pool.Stat()-аас авсан снапшотыг
// буцаана.
var dbStatsProvider func() DBPoolStats

type dbStatsSnapshot struct {
	OpenConnections int
	InUse           int
	WaitCount       int64
}

func currentDBStats() dbStatsSnapshot {
	if dbStatsProvider == nil {
		return dbStatsSnapshot{}
	}
	s := dbStatsProvider()
	return dbStatsSnapshot(s)
}

// RegisterDBStatsProvider-г эхлэх үед pgxpool.Stat()-аас снапшот гаргаж
// өгдөг provider-ийн хамт нэг удаа дуудах ёстой бөгөөд ингэснээр
// pool-статистикийн gauge-ууд scrape бүрт түүнийг унших боломжтой болно.
func RegisterDBStatsProvider(provider func() DBPoolStats) {
	dbStatsProvider = provider
}

// ObserveCacheOp нь нэг кэш үйлдлийн үр дүнг тэмдэглэнэ.
//
//	layer:  "ristretto" | "redis"
//	op:     "get" | "set" | "del"
//	result: "hit" | "miss" | "ok" | "error"
func ObserveCacheOp(layer, op, result string) {
	cacheOpsTotal.WithLabelValues(layer, op, result).Inc()
}

// ObserveOTPSend нь GeregeCloud Verify-ээр OTP илгээх нэг үйлдлийн үр дүнг
// тэмдэглэнэ.
//
//	result: "sent" | "failed"
func ObserveOTPSend(result string) {
	otpSendTotal.WithLabelValues(result).Inc()
}
