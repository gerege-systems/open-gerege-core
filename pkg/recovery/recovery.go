// Government Template Platform V3.0
// Gerege Systems Development Team болон Claude AI хамтран бүтээв, 2026.

// Package recovery нь 2FA-ийн нөөц (recovery) кодуудыг үүсгэх, нормчлох болон
// hash хийх туслах. Кодыг ЗӨВХӨН нэг удаа (үүсгэх үед) хэрэглэгчид харуулж,
// хадгалахдаа SHA-256 hash-ийг л хадгална — DB алдагдсан ч кодоор нэвтрэх
// боломжгүй. Хэрэглэгч код оруулахад дахин hash хийж тулгана.
package recovery

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base32"
	"encoding/hex"
	"fmt"
	"strings"
)

// DefaultCount нь superadmin-д нэг удаа үүсгэж өгөх нөөц кодын тоо.
const DefaultCount = 10

// groupSize нь кодын нэг бүлгийн урт — код нь "XXXX-XXXX" (base32) хэлбэртэй
// тул гараар бичихэд/уншихад хялбар.
const groupSize = 4

// enc нь padding-гүй base32 (том үсэг + 2-7 тоо) — уншихад ойлгомжтой цагаан
// толгой (0/O, 1/I ялгаа багатай) бөгөөд стандарт сангаар кодлогдоно.
var enc = base32.StdEncoding.WithPadding(base32.NoPadding)

// Generate нь n ширхэг crypto/rand дээр суурилсан нөөц код үүсгэнэ. n <= 0 бол
// DefaultCount хэрэглэнэ. Буцаах кодууд нь ЭНГИЙН ТЕКСТ — дуудагч тэдгээрийг
// хэрэглэгчид нэг удаа харуулж, хадгалахдаа Hash-ийг ашиглана.
func Generate(n int) ([]string, error) {
	if n <= 0 {
		n = DefaultCount
	}
	out := make([]string, 0, n)
	for i := 0; i < n; i++ {
		code, err := one()
		if err != nil {
			return nil, err
		}
		out = append(out, code)
	}
	return out, nil
}

// one нь нэг "XXXX-XXXX" код үүсгэнэ (5 байт = 40 бит = 8 base32 тэмдэгт).
func one() (string, error) {
	b := make([]byte, 5)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("random: %w", err)
	}
	s := enc.EncodeToString(b)
	return s[:groupSize] + "-" + s[groupSize:], nil
}

// Normalize нь хэрэглэгчийн оруулсан кодыг харьцуулах каноник хэлбэрт буулгана:
// зай/зураас зэрэг тусгаарлагчийг хасаж, том үсэг болгоно. Ингэснээр
// "abcd-efgh", "ABCD EFGH", "ABCDEFGH" бүгд ижил hash руу буудаг.
func Normalize(code string) string {
	var b strings.Builder
	for _, r := range strings.ToUpper(strings.TrimSpace(code)) {
		if (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') {
			b.WriteRune(r)
		}
	}
	return b.String()
}

// Hash нь кодыг нормчлоод SHA-256 (hex) болгоно — DB-д ЗӨВХӨН энэ утга
// хадгалагдана. Нөөц код нь өндөр энтропитой (40 бит), санамсаргүй үүсгэгдсэн
// тул нууц үгийн адил удаан KDF (bcrypt) шаардлагагүй.
func Hash(code string) string {
	sum := sha256.Sum256([]byte(Normalize(code)))
	return hex.EncodeToString(sum[:])
}

// HashAll нь кодын жагсаалтыг hash-ийн жагсаалт руу буулгана (хадгалахад).
func HashAll(codes []string) []string {
	out := make([]string, 0, len(codes))
	for _, c := range codes {
		out = append(out, Hash(c))
	}
	return out
}
