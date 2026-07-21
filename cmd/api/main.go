// Government Template Platform V3.0
// Gerege Systems Development Team болон Claude AI хамтран бүтээв, 2026.

// Package main нь Government Template Platform V3.0-ийн API эхлэх цэг юм.
//
// Энэ нь нээлттэй эхийн snykk/go-rest-boilerplate (MIT, зохиогч Najib
// Fikri)-ээс үүсэлтэй; HTTP давхаргыг chi (net/http) руу, өгөгдлийн давхаргыг
// jackc/pgx (pgxpool, түүхий SQL) руу хөрвүүлсэн. Бүрэн зохиогчийн мэдээллийг README.md болон docs/ARCHITECTURE.md-ээс үзнэ үү.
//
// @title           Government Template Platform V3.0 API
// @version         1.0
// @description     chi (net/http) + pgx (PostgreSQL) + Redis дээр суурилсан Clean Architecture бүхий Go backend. Нээлттэй эхийн snykk/go-rest-boilerplate (MIT, зохиогч Najib Fikri)-ээс үүсэлтэй; HTTP давхаргыг chi, өгөгдлийн давхаргыг pgx руу хөрвүүлсэн.
// @termsOfService  https://github.com/snykk/go-rest-boilerplate
//
// @contact.name   Government Template Platform V3.0
// @contact.url    https://github.com/snykk/go-rest-boilerplate
//
// @license.name  MIT
// @license.url   https://opensource.org/licenses/MIT
//
// @host      localhost:8080
// @BasePath  /api/v1
// @schemes   http https
//
// @securityDefinitions.apikey  BearerAuth
// @in                          header
// @name                        Authorization
// @description                 /auth/login эсвэл /auth/refresh-аас олгогдсон Bearer хандах токен (access token).
package main

import (
	"runtime"

	"template/cmd/api/server"
	_ "template/docs" // OpenAPI тодорхойлолт, `make swag`-аар үүсгэгддэг
	"template/internal/config"
	"template/internal/constants"
	"template/pkg/logger"
)

func init() {
	if err := config.InitializeAppConfig(); err != nil {
		logger.Fatal(err.Error(), logger.Fields{constants.LoggerCategory: constants.LoggerCategoryConfig})
	}
	// Орчноос (env) гарган авсан тохиргоогоор logger-ийг дахин эхлүүлнэ
	// (production = JSON; dev = console). Package түвшний init() аль хэдийн
	// зохистой анхдагч утга өгсөн тул энэ нь амжилтгүй болсон ч дээрх мөр лог бичиж чадна.
	_ = logger.InitDefault(loggerConfig(), logger.InstanceZap)
	logger.Info("configuration loaded", logger.Fields{constants.LoggerCategory: constants.LoggerCategoryConfig})
}

func loggerConfig() logger.Config {
	cfg := logger.Config{
		Level:         logger.LevelInfo,
		EnableConsole: true,
		AppName:       "gerege-template",
	}
	if config.AppConfig.Environment == constants.EnvironmentProduction {
		cfg.ConsoleJSONFormat = true
	} else if config.AppConfig.Debug {
		cfg.Level = logger.LevelDebug
	}
	return cfg
}

func main() {
	numCPU := runtime.NumCPU()
	logger.WithFields(logger.Fields{constants.LoggerCategory: constants.LoggerCategoryConfig}).
		Infof("The project is running on %d CPU(s)", numCPU)

	if runtime.NumCPU() > 2 {
		runtime.GOMAXPROCS(numCPU / 2)
	}

	app, err := server.NewApp()
	if err != nil {
		logger.Panic(err.Error(), logger.Fields{constants.LoggerCategory: constants.LoggerCategoryServer})
	}
	if err := app.Run(); err != nil {
		logger.Fatal(err.Error(), logger.Fields{constants.LoggerCategory: constants.LoggerCategoryServer})
	}
}
