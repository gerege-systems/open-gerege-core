//go:build integration

// Gerege Template Version 27.0
// Gerege Systems Development Team болон Claude AI хамтран бүтээв, 2026.

package testenv

import (
	"context"
	"testing"
	"time"

	"github.com/testcontainers/testcontainers-go"
	tcredis "github.com/testcontainers/testcontainers-go/modules/redis"
	"template/internal/datasources/caches"
)

// StartRedis нь устгагдах Redis контейнер асааж, түүн рүү чиглэсэн
// caches.RedisCache-г буцаана. StartPostgres-ийн семантикийг тусгана:
// контейнер нь t.Cleanup-ээр зогсдог, defaultTTL богино тул хугацаа
// дуусахад тулгуурласан тестүүд минут хүлээх шаардлагагүй.
func StartRedis(t *testing.T) caches.RedisCache {
	t.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	c, err := tcredis.Run(ctx, "redis:7-alpine")
	if err != nil {
		t.Fatalf("start redis container: %v", err)
	}
	t.Cleanup(func() {
		stopCtx, stopCancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer stopCancel()
		if err := testcontainers.TerminateContainer(c, testcontainers.StopContext(stopCtx)); err != nil {
			t.Logf("terminate redis container: %v", err)
		}
	})

	host, err := c.Host(ctx)
	if err != nil {
		t.Fatalf("redis host: %v", err)
	}
	port, err := c.MappedPort(ctx, "6379/tcp")
	if err != nil {
		t.Fatalf("redis port: %v", err)
	}

	addr := host + ":" + port.Port()
	// 1 минутын өгөгдмөл TTL — ямар ч тест санамсаргүйгээр түүнд хүрэхгүй
	// байх хангалттай урт, тодорхой Expire-д тулгуурласан тестүүд
	// байгалийн хугацаа дуусахыг хүлээх шаардлагагүй хангалттай богино.
	rc := caches.NewRedisCache(addr, 0, "", time.Minute)
	t.Cleanup(func() { _ = rc.Close() })
	return rc
}
