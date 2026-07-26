// Package migrations нь суурийн (core) SQL migration-уудыг хоёртын файлд
// шингээж (embed) өгдөг.
//
// Яагаад: апп repo-ууд суурийн migration-уудыг файлаар хуулж авдаг байсан
// нь давхардал үүсгэж, шинэчлэлт бүрийг гараар зөөхөд хүргэдэг байв. Одоо
// апп нь энэ FS-ийг өөрийн `migrations/` хавтастай хамт runner-т өгнө:
//
//	runner := migration.NewFS(pool, coremigrations.FS, os.DirFS("migrations"))
//
// Runner нь эх сурвалжуудыг нэгтгээд файлын эхний дугаараар нийтэд нь
// эрэмбэлдэг тул суурийн 1–999 нь аппын 1000+ -аас өмнө ажиллана
// (дугаарлалтын конвенцыг README.md-ээс үзнэ үү).
package migrations

import "embed"

// FS нь энэ хавтас доторх бүх .sql файлыг агуулна.
//
//go:embed *.sql
var FS embed.FS
