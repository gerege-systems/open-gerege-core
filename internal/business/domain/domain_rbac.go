// Gerege Template Version 27.0
// Gerege Systems Development Team болон Claude AI хамтран бүтээв, 2026.

package domain

import "time"

// Эрхийн (permission) түлхүүрүүд — migration 8-ийн seed-тэй таарна. Код доторх
// шалгалтууд эдгээр тогтмолыг ашиглана (мөр шууд бичихгүй). 'admin' role нь
// каталогийн БҮХ эрхийг автоматаар авдаг тул энд тусад нь бичигдээгүй.
const (
	PermDashboardView  = "dashboard.view"  // admin/manager хяналтын самбар үзэх
	PermSettingsManage = "settings.manage" // системийн тохиргоо удирдах
	PermUsersManage    = "users.manage"    // хэрэглэгч жагсаах/role солих/идэвхжүүлэх
	PermRolesManage    = "roles.manage"    // RBAC: role/permission удирдах
	PermPersonalView   = "personal.view"   // энгийн хэрэглэгчийн өөрийн хэсэг
	PermManagerView    = "manager.view"    // manager-ийн хэсэг
	PermGatewayManage  = "gateway.manage"  // API Gateway (services/routes/consumers/policies) удирдах
)

// AllPermissions нь эрхийн каталог (seed + ListPermissions-д ашиглана). Label/
// Category нь admin UI-ийн RBAC matrix-д бүлэглэхэд зориулагдсан.
var AllPermissions = []Permission{
	{Key: PermDashboardView, Label: "Хяналтын самбар үзэх", Category: "general"},
	{Key: PermSettingsManage, Label: "Тохиргоо удирдах", Category: "general"},
	{Key: PermUsersManage, Label: "Хэрэглэгч удирдах", Category: "administration"},
	{Key: PermRolesManage, Label: "Эрх (role) удирдах", Category: "administration"},
	{Key: PermManagerView, Label: "Менежерийн хэсэг", Category: "management"},
	{Key: PermPersonalView, Label: "Хувийн хэсэг", Category: "personal"},
	{Key: PermGatewayManage, Label: "API Gateway удирдах", Category: "administration"},
}

// Role нь динамик эрх (RBAC). IsSystem эрхүүдийг (admin/manager/user) устгаж/
// түлхүүрийг нь өөрчилж болохгүй — seed-ээр тогтсон.
type Role struct {
	ID          int
	Key         string
	Name        string
	Description string
	IsSystem    bool
	CreatedAt   time.Time
	UpdatedAt   *time.Time
}

// Permission нь эрхийн каталогийн нэг бичлэг (код дотор тодорхойлогдсон, зөвхөн
// role-д онооно).
type Permission struct {
	Key      string
	Label    string
	Category string
}
