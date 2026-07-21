// Government Template Platform V3.0
// Gerege Systems Development Team болон Claude AI хамтран бүтээв, 2026.

// Package main нь distroless image-д зориулсан хамгийн жижиг health-probe
// binary юм — distroless-д shell/wget/curl байхгүй тул HEALTHCHECK нь
// гүйцэтгэх боломжтой binary шаарддаг. /health-руу GET хийж 200 бол 0,
// эс бол 1-ээр гарна. Зөвхөн stdlib — нэмэлт хамаарал авчрахгүй.
package main

import (
	"net/http"
	"os"
	"time"
)

func main() {
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	client := &http.Client{Timeout: 3 * time.Second}
	resp, err := client.Get("http://127.0.0.1:" + port + "/health") //nolint:gosec // fixed 127.0.0.1 loopback probe; port from this container's own env
	if err != nil {
		os.Exit(1)
	}
	_ = resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		os.Exit(1)
	}
}
