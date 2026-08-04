// Gerege Systems Development Team болон Claude AI хамтран бүтээв, 2026.

// Package modules нь бүх модулийн НЭГДСЭН цэг. Одоогоор зөвхөн
// migration-ийн эх сурвалжийн ЖАГСААЛТ — түүний ДАРААЛАЛ нь зөв байх нь
// шинэ суулгацын схем зөв босох нөхцөл юм.
//
// ЯАГААД ТУСДАА PACKAGE: энэ дарааллыг хоёр өөр газар хэрэглэдэг —
// cmd/migration (deploy) болон core/test/testenv (integration harness).
// Хоёрт хуулбарлавал салангид болж, тест ногоон атлаа deploy унах (эсвэл
// эсрэгээр) байдалд хүрнэ. Kernel энэ жагсаалтыг эзэмшиж БОЛОХГҮЙ —
// kernel нь business модуль import хийдэггүй.
package modules

import (
	aimod "github.com/gerege-systems/open-gerege-core/modules/ai"
	assetsmod "github.com/gerege-systems/open-gerege-core/modules/assets"
	auditmod "github.com/gerege-systems/open-gerege-core/modules/audit"
	authmod "github.com/gerege-systems/open-gerege-core/modules/auth"
	gatewayconsolemod "github.com/gerege-systems/open-gerege-core/modules/gatewayconsole"
	govmod "github.com/gerege-systems/open-gerege-core/modules/gov"
	integrationsmod "github.com/gerege-systems/open-gerege-core/modules/integrations"
	orgmod "github.com/gerege-systems/open-gerege-core/modules/org"
	platformmod "github.com/gerege-systems/open-gerege-core/modules/platform"
	providermod "github.com/gerege-systems/open-gerege-core/modules/provider"
	rbacmod "github.com/gerege-systems/open-gerege-core/modules/rbac"
	registrymod "github.com/gerege-systems/open-gerege-core/modules/registry"
	relaymod "github.com/gerege-systems/open-gerege-core/modules/relay"
	sitemod "github.com/gerege-systems/open-gerege-core/modules/site"
	superadminmod "github.com/gerege-systems/open-gerege-core/modules/superadmin"
	usersmod "github.com/gerege-systems/open-gerege-core/modules/users"

	"github.com/gerege-systems/open-gerege-core/kernel/data/migrate"
)

// MigrationSources нь өөрийн migration-тай модулиудыг ХАМААРЛЫН
// дарааллаар буцаана.
func MigrationSources() []migrate.Source {
	return []migrate.Source{
		// ДАРААЛАЛ НЬ ЗААВАЛ: доорх хамаарлууд файлын агуулгаас гарна.
		//   users  — суурь хүснэгт; rbac/assets/superadmin бүгд ALTER хийдэг
		//   rbac   — 8 нь users-ийг ALTER хийж, permissions-ыг үүсгэнэ
		//   gateway-console — 22/36 нь permissions рүү INSERT хийнэ
		//   provider — 51 нь applications-ыг лавладаг (22 үүсгэдэг)
		//   site   — platform-ийн 43/52 нь site_appearance-ыг лавладаг
		// Дарааллыг өөрчилвөл шинэ суулгац унана; kernel/data/migrate-ийн
		// тэнцүүлэлтийн тест үүнийг барина.
		usersmod.New(),
		rbacmod.New(),
		gatewayconsolemod.New(),
		aimod.New(),
		assetsmod.New(),
		auditmod.New(),
		authmod.New(),
		govmod.New(),
		integrationsmod.New(),
		orgmod.New(),
		providermod.New(),
		registrymod.New(),
		relaymod.New(),
		superadminmod.New(),
		sitemod.New(),
		platformmod.New(),
	}
}
