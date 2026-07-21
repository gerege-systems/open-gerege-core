// Government Template Platform V3.0
// Gerege Systems Development Team болон Claude AI хамтран бүтээв, 2026.

package users

import (
	"fmt"

	"template/internal/apperror"
	"template/internal/datasources/caches"
	repointerface "template/internal/datasources/repositories/interface"

	"golang.org/x/sync/singleflight"
)

// Config нь usecase-ийн domain давхарга руу дамжуулдаг тохируулах боломжтой
// утгуудыг агуулна. Domain өөрөө bcryptCost-ийг параметрээр авдаг тул тохиргооны
// асуудлуудаас ангид хэвээр үлдэж чадна; usecase нь config-ийн талаар мэддэг
// хил юм.
type Config struct {
	BcryptCost int
}

// usecase нь хамаарлууд болон method хоорондын төлөвийг агуулдаг. Нэг зан
// төлөв өөрчлөгдөхөд PR-ийн diff нарийн (surgical) хэвээр үлдэхийн тулд method
// бүр өөрийн файлд байрладаг.
type usecase struct {
	repo           repointerface.UserRepository
	ristrettoCache caches.RistrettoCache
	cfg            Config

	// userByEmailGroup нь ижил email-ийн зэрэгцээ кэш алдалтуудыг (cache miss)
	// нэгтгэдэг тул олон зэрэг хүсэлт (thundering herd) N зэрэгцээ DB дуудлага
	// болон тархахгүй. Group нь нормчилсон email-ээр түлхүүрлэгдэнэ.
	userByEmailGroup singleflight.Group
}

// NewUsecase нь User CRUD use case-ийг үүсгэнэ. Энэ нь auth-тай холбоотой ямар
// нэг хамтрагчаас (JWT, Redis, OTP verifier байхгүй) хамаардаггүй — энэ нь User vs
// Auth хуваагдлын гол утга юм.
func NewUsecase(repo repointerface.UserRepository, ristrettoCache caches.RistrettoCache, cfg Config) Usecase {
	return &usecase{
		repo:           repo,
		ristrettoCache: ristrettoCache,
		cfg:            cfg,
	}
}

// mapRepoError нь repository-ээс буцсан DomainError төрлүүдийг хадгалж, харин
// түүхий алдаануудыг форматтай дотоод алдаагаар боодог. Үүнгүйгээр дээд урсгал
// дахь errors.As(err, *DomainError) амжилтгүй болно.
func mapRepoError(err error, op string) error {
	if err == nil {
		return nil
	}
	if _, ok := err.(*apperror.DomainError); ok {
		return err
	}
	return apperror.InternalCause(fmt.Errorf("%s: %w", op, err))
}
