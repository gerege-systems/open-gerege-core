// Gerege Template Version 27.0
// Gerege Systems Development Team болон Claude AI хамтран бүтээв, 2026.

package logger

// Зохистой анхдагч logger-ийг автоматаар эхлүүлнэ — ингэснээр main() нь
// InitDefault-г дуудах боломжтой болохоос өмнө package түвшний дуудлагууд
// (logger.Info, logger.Error, ...) ажиллана. Bootstrap тохиргоо нь зориудаар
// хамгийн бага хэмжээний: console гаралт, info түвшин, JSON байхгүй, app нэр
// байхгүй. main() үүнийг config-оос үүдэлтэй тохиргоогоор дарж бичнэ гэж тооцно.
//
// Хэрэв дараа нь InitDefault дуудагдвал глобал нь атомлог байдлаар солигдоно;
// явагдаж буй лог дуудлагууд зүгээр л тухайн агшны идэвхтэй instance-ийг хардаг.
func init() {
	_ = InitDefault(Config{
		Level:         LevelInfo,
		EnableConsole: true,
	}, InstanceZap)
}
