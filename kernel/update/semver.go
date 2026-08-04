// Gerege Systems Development Team болон Claude AI хамтран бүтээв, 2026.

// Package update нь платформын өөрийгөө шинэчлэх (platformd) цөм логик:
// semver харьцуулалт, release манифест унших, шинэчлэх шийдвэр, apply +
// health + rollback төлөвийн машин. Гадаад хамааралгүй, бүрэн тестлэгдэнэ;
// cmd/platformd нь үүнийг env тохиргоотой холбосон нимгэн бүрхүүл.
package update

import (
	"fmt"
	"strconv"
	"strings"
)

// Version — задлагдсан semver ("v" угтвартай эсвэл угтваргүй хоёуланг уншина).
type Version struct {
	Major, Minor, Patch int
	// Pre — prerelease пайз ("beta.1" г.м.); хоосон = эцсийн release.
	Pre string
}

// ParseVersion нь "v1.2.3", "1.2.3-beta.1" хэлбэрийг задална.
func ParseVersion(s string) (Version, error) {
	raw := strings.TrimPrefix(strings.TrimSpace(s), "v")
	if raw == "" {
		return Version{}, fmt.Errorf("update: хоосон version")
	}
	body, pre, _ := strings.Cut(raw, "-")
	parts := strings.Split(body, ".")
	if len(parts) != 3 {
		return Version{}, fmt.Errorf("update: %q нь MAJOR.MINOR.PATCH биш", s)
	}
	nums := [3]int{}
	for i, p := range parts {
		n, err := strconv.Atoi(p)
		if err != nil || n < 0 {
			return Version{}, fmt.Errorf("update: %q — тоон хэсэг буруу", s)
		}
		nums[i] = n
	}
	return Version{Major: nums[0], Minor: nums[1], Patch: nums[2], Pre: pre}, nil
}

// Compare нь -1/0/+1 буцаана. Semver дүрмээр prerelease нь мөн дугаартай
// эцсийн release-ээс БАГА ("1.2.0-beta.1" < "1.2.0").
func Compare(a, b Version) int {
	for _, d := range [3]int{a.Major - b.Major, a.Minor - b.Minor, a.Patch - b.Patch} {
		if d < 0 {
			return -1
		}
		if d > 0 {
			return 1
		}
	}
	switch {
	case a.Pre == b.Pre:
		return 0
	case a.Pre == "":
		return 1
	case b.Pre == "":
		return -1
	case a.Pre < b.Pre:
		return -1
	default:
		return 1
	}
}

// String нь "vMAJOR.MINOR.PATCH[-PRE]" хэлбэрээр буцаана.
func (v Version) String() string {
	s := fmt.Sprintf("v%d.%d.%d", v.Major, v.Minor, v.Patch)
	if v.Pre != "" {
		s += "-" + v.Pre
	}
	return s
}
