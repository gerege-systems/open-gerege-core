// Government Template Platform V3.0
// Gerege Systems Development Team болон Claude AI хамтран бүтээв, 2026.

// Package secrethash нь OAuth2 client secret-ийг хадгалах/шалгах hash-ийг
// удирдана.
//
// ХОЁР ФОРМАТ дэмжинэ:
//
//   - `$pbkdf2-sha256$i=25000,l=32$<salt>$<hash>` — Ory Hydra-гийн формат.
//     Hydra-аас шилжүүлсэн client-ууд secret-ээ СОЛИЛГҮЙ ажиллаж байхын тулд
//     ЗӨВХӨН шалгах (verify) зорилгоор дэмжинэ.
//   - `$argon2id$v=19$m=...,t=...,p=...$<salt>$<hash>` — ШИНЭ secret-ийг үүнээр
//     хэшлэнэ. Админ гараар богино secret оноож болдог тул (16 тэмдэгтийн доод
//     хязгаар) PBKDF2-25000-аас хамаагүй тэсвэртэй KDF хэрэгтэй.
//
// base64 нь ХОЁР форматад стандарт alphabet (+/), padding-гүй — Ory-гийн
// бодит гаралтаас баталгаажуулсан (secrethash_test.go дахь тест векторууд).
package secrethash

import (
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"golang.org/x/crypto/argon2"
	"golang.org/x/crypto/pbkdf2"
)

// Argon2id-ийн параметрүүд. OWASP-ийн зөвлөмжийн дагуу (64 MiB, 3 дамжлага).
const (
	argonMemory  uint32 = 64 * 1024
	argonTime    uint32 = 3
	argonThreads uint8  = 4
	argonKeyLen  uint32 = 32
	saltLen             = 16
	// Хадгалагдсан hash-аас уншсан түлхүүрийн зөвшөөрөгдөх урт.
	minKeyLen = 16
	maxKeyLen = 64
	// PBKDF2-ийн давталтын дээд хязгаар (Ory нь 25000 ашигладаг).
	maxIterations = 1_000_000
)

// ErrUnknownFormat нь hash мөрийг таних боломжгүй үед буцна. Дуудагч үүнийг
// "secret тохирохгүй"-тэй ижилхэн (fail-closed) хандах ёстой.
var ErrUnknownFormat = errors.New("secrethash: unknown hash format")

// Hash нь шинэ secret-ийг Argon2id-ээр хэшилнэ (PHC мөр буцаана).
func Hash(secret string) (string, error) {
	salt := make([]byte, saltLen)
	if _, err := rand.Read(salt); err != nil {
		return "", fmt.Errorf("secrethash: rand: %w", err)
	}
	sum := argon2.IDKey([]byte(secret), salt, argonTime, argonMemory, argonThreads, argonKeyLen)
	return fmt.Sprintf("$argon2id$v=%d$m=%d,t=%d,p=%d$%s$%s",
		argon2.Version, argonMemory, argonTime, argonThreads,
		b64(salt), b64(sum),
	), nil
}

// Verify нь secret нь хадгалсан hash-тай тохирч байгаа эсэхийг шалгана.
// Харьцуулалт тогтмол хугацаанд (constant-time) хийгдэнэ.
func Verify(encoded, secret string) (bool, error) {
	switch {
	case strings.HasPrefix(encoded, "$argon2id$"):
		return verifyArgon2id(encoded, secret)
	case strings.HasPrefix(encoded, "$pbkdf2-sha256$"):
		return verifyPBKDF2(encoded, secret)
	default:
		return false, ErrUnknownFormat
	}
}

// NeedsRehash нь hash нь хуучин (Hydra-гийн PBKDF2) форматтай эсэхийг хэлнэ —
// дуудагч амжилттай нэвтрэлтийн дараа Argon2id руу чимээгүй шинэчилж болно.
func NeedsRehash(encoded string) bool {
	return !strings.HasPrefix(encoded, "$argon2id$")
}

// verifyPBKDF2 нь Ory-гийн `$pbkdf2-sha256$i=<iter>,l=<len>$<salt>$<hash>`-ыг шалгана.
func verifyPBKDF2(encoded, secret string) (bool, error) {
	parts := strings.Split(encoded, "$")
	// ["", "pbkdf2-sha256", "i=25000,l=32", salt, hash]
	if len(parts) != 5 {
		return false, ErrUnknownFormat
	}
	iter, keyLen, err := parsePBKDF2Params(parts[2])
	if err != nil {
		return false, err
	}
	salt, err := unb64(parts[3])
	if err != nil {
		return false, ErrUnknownFormat
	}
	want, err := unb64(parts[4])
	if err != nil {
		return false, ErrUnknownFormat
	}
	if keyLen != len(want) || keyLen < minKeyLen || keyLen > maxKeyLen {
		return false, ErrUnknownFormat
	}
	got := pbkdf2.Key([]byte(secret), salt, iter, keyLen, sha256.New)
	return subtle.ConstantTimeCompare(got, want) == 1, nil
}

// parsePBKDF2Params нь "i=25000,l=32"-ыг задална.
func parsePBKDF2Params(s string) (iter, keyLen int, err error) {
	for _, kv := range strings.Split(s, ",") {
		k, v, ok := strings.Cut(kv, "=")
		if !ok {
			return 0, 0, ErrUnknownFormat
		}
		n, convErr := strconv.Atoi(v)
		if convErr != nil || n <= 0 {
			return 0, 0, ErrUnknownFormat
		}
		switch k {
		case "i":
			iter = n
		case "l":
			keyLen = n
		}
	}
	// Давталтын тоог дээрээс хязгаарлана — hash мөр нь итерацийг тодорхойлдог
	// тул гэмтсэн/хорлонтой утга (i=10^9) CPU-г шавхах боломжтой.
	if iter == 0 || keyLen == 0 || iter > maxIterations {
		return 0, 0, ErrUnknownFormat
	}
	return iter, keyLen, nil
}

// verifyArgon2id нь `$argon2id$v=19$m=..,t=..,p=..$<salt>$<hash>`-ыг шалгана.
func verifyArgon2id(encoded, secret string) (bool, error) {
	parts := strings.Split(encoded, "$")
	// ["", "argon2id", "v=19", "m=..,t=..,p=..", salt, hash]
	if len(parts) != 6 {
		return false, ErrUnknownFormat
	}
	var version int
	if _, err := fmt.Sscanf(parts[2], "v=%d", &version); err != nil || version != argon2.Version {
		return false, ErrUnknownFormat
	}
	var memory, time uint32
	var threads uint8
	if _, err := fmt.Sscanf(parts[3], "m=%d,t=%d,p=%d", &memory, &time, &threads); err != nil {
		return false, ErrUnknownFormat
	}
	if memory == 0 || time == 0 || threads == 0 {
		return false, ErrUnknownFormat
	}
	salt, err := unb64(parts[4])
	if err != nil {
		return false, ErrUnknownFormat
	}
	want, err := unb64(parts[5])
	if err != nil {
		return false, ErrUnknownFormat
	}
	// Уртыг хязгаарлана — hash мөр гэмтсэн/хорлонтой бол argon2-д асар том
	// keyLen дамжуулж санах ой шавхахаас сэргийлнэ.
	if len(want) < minKeyLen || len(want) > maxKeyLen {
		return false, ErrUnknownFormat
	}
	//nolint:gosec // G115: len(want) дээрх мөрөнд [minKeyLen, maxKeyLen] муж руу хязгаарлагдсан тул хөрвүүлэлт аюулгүй
	got := argon2.IDKey([]byte(secret), salt, time, memory, threads, uint32(len(want)))
	return subtle.ConstantTimeCompare(got, want) == 1, nil
}

func b64(b []byte) string { return base64.RawStdEncoding.EncodeToString(b) }

// unb64 нь padding-тай/padding-гүй, стандарт/URL alphabet-ийн аль ч хувилбарыг
// хүлээж авна — өөр хэрэгслээр үүсгэсэн hash-д тэсвэртэй байхын тулд.
func unb64(s string) ([]byte, error) {
	if strings.ContainsAny(s, "-_") {
		if strings.HasSuffix(s, "=") {
			return base64.URLEncoding.DecodeString(s)
		}
		return base64.RawURLEncoding.DecodeString(s)
	}
	if strings.HasSuffix(s, "=") {
		return base64.StdEncoding.DecodeString(s)
	}
	return base64.RawStdEncoding.DecodeString(s)
}
