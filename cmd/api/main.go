// Gerege Systems Development Team болон Claude AI хамтран бүтээв, 2026.

// Package main нь Gerege Template Platform V3.0-ийн API эхлэх цэг юм.
//
// Энэ нь нээлттэй эхийн snykk/go-rest-boilerplate (MIT, зохиогч Najib
// Fikri)-ээс үүсэлтэй; HTTP давхаргыг chi (net/http) руу, өгөгдлийн давхаргыг
// jackc/pgx (pgxpool, түүхий SQL) руу хөрвүүлсэн. Бүрэн зохиогчийн мэдээллийг README.md болон docs/ARCHITECTURE.md-ээс үзнэ үү.
//
// @title           Gerege Template Platform V3.0 API
// @version         1.0
// @description     chi (net/http) + pgx (PostgreSQL) + Redis дээр суурилсан Clean Architecture бүхий Go backend. Нээлттэй эхийн snykk/go-rest-boilerplate (MIT, зохиогч Najib Fikri)-ээс үүсэлтэй; HTTP давхаргыг chi, өгөгдлийн давхаргыг pgx руу хөрвүүлсэн.
// @termsOfService  https://github.com/snykk/go-rest-boilerplate
//
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

	"github.com/gerege-systems/platform-core/cmd/api/server"
	"github.com/gerege-systems/platform-core/core/constants"
	_ "github.com/gerege-systems/platform-core/docs" // OpenAPI тодорхойлолт, `make swag`-аар үүсгэгддэг
	"github.com/gerege-systems/platform-core/pkg/logger"
)

func init() {
	if err := server.Bootstrap(); err != nil {
		logger.Fatal(err.Error(), logger.Fields{constants.LoggerCategory: constants.LoggerCategoryConfig})
	}
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
