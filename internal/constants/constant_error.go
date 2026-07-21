// Government Template Platform V3.0
// Gerege Systems Development Team болон Claude AI хамтран бүтээв, 2026.

package constants

import "fmt"

// Config loader-ийн ашигладаг sentinel алдаанууд. Бүтэцлэгдсэн
// DomainError төрөл нь internal/apperror дотор байрладаг — constants нь
// зөвхөн жинхэнэ тогтмол утгуудад зориулагдсан.
var (
	ErrLoadConfig  = fmt.Errorf("failed to load config file")
	ErrParseConfig = fmt.Errorf("failed to parse env to config struct")
	ErrEmptyVar    = fmt.Errorf("required variable environment is empty")
)
