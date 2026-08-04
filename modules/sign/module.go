// Gerege Systems Development Team болон Claude AI хамтран бүтээв, 2026.

// Package sign нь баримт бичгийн гарын үсгийн (PAdES, eID Mongolia /v3)
// business модулийн wiring. sign usecase-ээ ServiceSign нэрээр нийтэлдэг —
// App.Sign() болон бусад модуль (relay) түүгээр хүрнэ.
package sign

import (
	"context"
	"fmt"
	"os"
	"strings"

	assetsuc "github.com/gerege-systems/open-gerege-core/core/business/usecases/assets"
	"github.com/gerege-systems/open-gerege-core/core/business/usecases/sign"
	"github.com/gerege-systems/open-gerege-core/core/business/usecases/users"
	"github.com/gerege-systems/open-gerege-core/core/config"
	"github.com/gerege-systems/open-gerege-core/core/constants"
	"github.com/gerege-systems/open-gerege-core/core/datasources/caches"
	"github.com/gerege-systems/open-gerege-core/core/http/routes"
	"github.com/gerege-systems/open-gerege-core/kernel/module"
)

// Module — sign модулийн kernel гэрээний хэрэгжилт.
type Module struct{}

// New нь модулийг бүтээнэ.
func New() *Module { return &Module{} }

// ID — Builtin() манифестийн ID.
func (*Module) ID() string { return "sign" }

// Register нь sign usecase-ээ угсарч /v1/sign route-уудаа суулгаад,
// usecase-ээ ServiceSign нэрээр нийтэлнэ. Гэрчилгээний материалын алдаа
// boot-ыг унагаана (production-д fail-closed — өмнөх server.go-той ижил).
func (m *Module) Register(_ context.Context, host module.Host) error {
	redisCache, ok := module.ServiceAs[caches.RedisCache](host, module.ServiceRedis)
	if !ok {
		return fmt.Errorf("sign: host-д %q service алга", module.ServiceRedis)
	}
	usersUC, ok := module.ServiceAs[users.Usecase](host, module.ServiceUsers)
	if !ok {
		return fmt.Errorf("sign: host-д %q service алга", module.ServiceUsers)
	}
	assetsUC, ok := module.ServiceAs[assetsuc.Usecase](host, module.ServiceAssets)
	if !ok {
		return fmt.Errorf("sign: host-д %q service алга", module.ServiceAssets)
	}

	// Байнгын Document-Signer гэрчилгээ + түлхүүр (файлаас). Хоёулаа хоосон
	// бол production-д fail-closed, development-д dev self-signed.
	signerCertPEM, signerKeyPEM, err := loadSignerMaterial()
	if err != nil {
		return fmt.Errorf("load document-signer material: %w", err)
	}
	uc, err := sign.NewUsecase(redisCache, sign.Config{
		// EIDBaseURL нь "/v3"-ийг агуулдаг (default https://eidmongolia.mn/v3);
		// sign usecase өөрөө "/v3/signature/..." нэмдэг тул суурийг "/v3"-гүй
		// болгож, /v3/v3 давхардлаас сэргийлнэ.
		V3BaseURL:     signV3Base(config.AppConfig.EIDBaseURL),
		RPUUID:        config.AppConfig.EIDRPUUID,
		RPName:        config.AppConfig.EIDRPName,
		APISecret:     config.AppConfig.EIDRPSecret,
		SignerCertPEM: signerCertPEM,
		SignerKeyPEM:  signerKeyPEM,
		IsProduction:  config.AppConfig.Environment == constants.EnvironmentProduction,
	})
	if err != nil {
		return fmt.Errorf("init sign usecase: %w", err)
	}

	routes.NewSignRoute(host.APIRouter(), uc, usersUC, assetsUC, host.AuthMiddleware()).Routes()

	if sp, ok := host.(module.ServiceProvider); ok {
		sp.Provide(module.ServiceSign, uc)
	}
	return nil
}

// signV3Base нь sign usecase-д зориулж eID суурь URL-ийг бэлдэнэ: trailing
// "/v3"-ийг хасаж /v3/v3 давхардлаас сэргийлнэ.
func signV3Base(eidBaseURL string) string {
	base := strings.TrimRight(strings.TrimSpace(eidBaseURL), "/")
	base = strings.TrimSuffix(base, "/v3")
	if base == "" {
		return "https://eidmongolia.mn"
	}
	return base
}

// loadSignerMaterial нь серверийн байнгын Document-Signer гэрчилгээ +
// түлхүүрийн PEM-ийг config-ийн файл замаас уншина. Хоёулаа хоосон бол nil
// (production-д sign.NewUsecase fail-closed); зөвхөн нэг нь өгөгдвөл алдаа.
func loadSignerMaterial() (certPEM, keyPEM []byte, err error) {
	certFile := strings.TrimSpace(config.AppConfig.SignSignerCertFile)
	keyFile := strings.TrimSpace(config.AppConfig.SignSignerKeyFile)
	if certFile == "" && keyFile == "" {
		return nil, nil, nil
	}
	if certFile == "" || keyFile == "" {
		return nil, nil, fmt.Errorf("SIGN_SIGNER_CERT_FILE ба SIGN_SIGNER_KEY_FILE хоёуланг хамт тохируул")
	}
	// #nosec G304 — зам нь оператор SIGN_SIGNER_CERT_FILE env-ээр өгдөг боот
	// тохиргоо; хүсэлтийн/хэрэглэгчийн оролтоос биш (taint биш).
	certPEM, err = os.ReadFile(certFile)
	if err != nil {
		return nil, nil, fmt.Errorf("read signer cert: %w", err)
	}
	// #nosec G304 — оператор SIGN_SIGNER_KEY_FILE env-ээр өгсөн зам.
	keyPEM, err = os.ReadFile(keyFile)
	if err != nil {
		return nil, nil, fmt.Errorf("read signer key: %w", err)
	}
	return certPEM, keyPEM, nil
}
