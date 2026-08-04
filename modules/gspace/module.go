// Gerege Systems Development Team болон Claude AI хамтран бүтээв, 2026.

// Package gspace нь Gerege Space (SFTP хадгалалт) business модулийн wiring.
// Модулийн бүх repo → usecase → route угсралт ЭНД — server.go энэ модулийн
// дотоод бүтцийг мэдэхгүй. Домэйн код нь core/business/usecases/gspace,
// pkg/gspace дотроо хэвээр (Phase 1: wiring-ийн нүүлгэлт; файлын нүүлгэлт
// нь дараагийн шатны механик ажил).
package gspace

import (
	"context"
	"fmt"

	"github.com/gerege-systems/open-gerege-core/core/business/usecases/gspace"
	"github.com/gerege-systems/open-gerege-core/core/config"
	"github.com/gerege-systems/open-gerege-core/core/constants"
	"github.com/gerege-systems/open-gerege-core/core/http/middlewares"
	"github.com/gerege-systems/open-gerege-core/core/http/routes"
	"github.com/gerege-systems/open-gerege-core/kernel/module"
	gspaceclient "github.com/gerege-systems/open-gerege-core/pkg/gspace"
)

// Module — gspace модулийн kernel гэрээний хэрэгжилт.
type Module struct{}

// New нь модулийг бүтээнэ (server.go-ийн модулийн жагсаалтад орно).
func New() *Module { return &Module{} }

// ID — Builtin() манифестийн ID.
func (*Module) ID() string { return "gspace" }

// Register нь SFTP client + usecase-ээ угсарч /v1/gspace route-уудыг суулгана.
func (m *Module) Register(_ context.Context, host module.Host) error {
	limiter, ok := module.ServiceAs[*middlewares.RateLimiter](host, module.ServiceWriteRateLimiter)
	if !ok {
		return fmt.Errorf("gspace: host-д %q service алга", module.ServiceWriteRateLimiter)
	}

	isProduction := config.AppConfig.Environment == constants.EnvironmentProduction
	client := gspaceclient.NewClient(gspaceclient.Config{
		Host:     config.AppConfig.GSpaceHost,
		Port:     config.AppConfig.GSpacePort,
		User:     config.AppConfig.GSpaceUser,
		Password: config.AppConfig.GSpacePassword,
		BasePath: config.AppConfig.GSpaceBasePath,
		HostKey:  config.AppConfig.GSpaceHostKey,
		// Production-д host key заавал (MITM-аас хамгаална); development-д
		// тохируулаагүй бол шалгахгүйгээр зөвшөөрнө.
		AllowInsecureHostKey: !isProduction,
	})
	uc := gspace.NewUsecase(client, config.AppConfig.GSpaceQuota)

	routes.NewGSpaceRoute(host.APIRouter(), uc, host.AuthMiddleware(), limiter).Routes()
	return nil
}
