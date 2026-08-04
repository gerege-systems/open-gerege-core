// Gerege Systems Development Team болон Claude AI хамтран бүтээв, 2026.

// gerege — модульчлагдсан платформын оператор CLI (V4.0, Phase 3).
//
//	gerege modules list                 модулиудын төлөв (админ token байвал дэлгэрэнгүй)
//	gerege modules enable <id>          модулийг асаана (restart-гүй)
//	gerege modules disable <id>         модулийг унтраана (restart-гүй)
//	gerege modules new <id>             шинэ модулийн skeleton үүсгэнэ
//
// Тохиргоо (env):
//
//	GEREGE_API          платформын суурь URL (default: http://localhost:8080)
//	GEREGE_ADMIN_TOKEN  админы access token (list-ийн дэлгэрэнгүй + enable/disable-д заавал)
package main

import (
	"fmt"
	"os"

	"github.com/gerege-systems/open-gerege-core/cmd/gerege/cli"
)

func main() {
	if err := cli.Run(os.Args[1:], os.Stdout); err != nil {
		fmt.Fprintln(os.Stderr, "gerege:", err)
		os.Exit(1)
	}
}
