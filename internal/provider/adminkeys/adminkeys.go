// eID based AI enabled Government Template Platform V3.0
// Gerege Systems Development Team болон Claude AI хамтран бүтээв, 2026.

// Package adminkeys нь /admin гадаргууг баталгаажуулах API key-үүдийн байнгын
// бүртгэл (PostgreSQL). Дэлхийн стандарт "secret key" загвар (Stripe/GitHub/
// Okta): plaintext key-г зөвхөн mint үед НЭГ удаа харуулж, зөвхөн SHA-256 hash-
// ыг хадгална — key store алдагдсан ч дахин ашиглах боломжгүй.
//
// Хоёр давхарга зэрэгцэнэ:
//   - env bootstrap key (SSO_ADMIN_API_KEYS) — оператороор өгсөн; хадгалагдахгүй,
//     санах ойд constant-time hash-аар тааруулна.
//   - managed key — POST /admin/api/v1/keys-ээр minted, admin_api_keys хүснэгтэд
//     hash-аар хадгалагдана, API-аар цуцлагдана.
package adminkeys

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// KeyPrefix нь minted key бүрийн хүн-танигдахуйц угтвар. "gsk" = Gerege Secret Key.
const KeyPrefix = "gsk_"

type Key struct {
	ID   string
	Name string

	// Hash нь plaintext secret-ийн hex SHA-256. Plaintext хадгалагдахгүй — mint
	// нэг удаа буцаадаг, дараа нь сэргээгдэхгүй.
	Hash string

	// Display нь key-ийн эхний нууц биш хэсэг (жишээ "gsk_a1b2c3").
	Display string

	CreatedAt  time.Time
	LastUsedAt time.Time // хэзээ ч ашиглаагүй бол zero
	Disabled   bool

	// Env нь SSO_ADMIN_API_KEYS-ээс гаралтай key-г тэмдэглэнэ.
	Env bool
}

type Store struct {
	pool *pgxpool.Pool
	env  []envKey // оператор bootstrap key hash-ууд, урьдчилан тооцсон
}

type envKey struct {
	name string
	hash string
}

// New нь pool дээрх managed-key store-ыг буцааж, оператор bootstrap key-үүдийг
// үргэлж хүчинтэй, цуцлагдашгүй env key болгон нэгтгэнэ.
func New(pool *pgxpool.Pool, bootstrap []string) *Store {
	s := &Store{pool: pool}
	for i, secret := range bootstrap {
		if secret == "" {
			continue
		}
		s.env = append(s.env, envKey{name: fmt.Sprintf("bootstrap-%d", i+1), hash: hashKey(secret)})
	}
	return s
}

// Verify нь өгөгдсөн secret-ыг баталгаажуулж, амжилттай бол харгалзах key-г
// буцаана. Env bootstrap key эхэнд шалгагдана (constant-time, үргэлж идэвхтэй);
// managed key-г hash-аар хайна.
func (s *Store) Verify(ctx context.Context, presented string) (*Key, bool) {
	if presented == "" {
		return nil, false
	}
	h := hashKey(presented)

	for _, e := range s.env {
		if subtle.ConstantTimeCompare([]byte(e.hash), []byte(h)) == 1 {
			return &Key{ID: "env", Name: e.name, Display: KeyPrefix + "env", Env: true}, true
		}
	}

	var (
		k        Key
		lastUsed *time.Time
	)
	err := s.pool.QueryRow(ctx,
		`SELECT id, name, hash, display, created_at, last_used_at, disabled
		   FROM admin_api_keys WHERE hash=$1`, h,
	).Scan(&k.ID, &k.Name, &k.Hash, &k.Display, &k.CreatedAt, &lastUsed, &k.Disabled)
	if err != nil {
		return nil, false
	}
	if k.Disabled {
		return nil, false
	}
	if lastUsed != nil {
		k.LastUsedAt = *lastUsed
	}
	// last-used бичилтийг key тус бүрд минутад нэгээс илүүгүй хязгаарлана.
	_, _ = s.pool.Exec(ctx,
		`UPDATE admin_api_keys SET last_used_at=now()
		   WHERE id=$1 AND (last_used_at IS NULL OR last_used_at < now() - interval '1 minute')`, k.ID)
	return &k, true
}

// Mint нь шинэ managed key үүсгэж, plaintext secret-ыг НЭГ удаа буцаана
// (дараа нь сэргээгдэхгүй).
func (s *Store) Mint(ctx context.Context, name string) (secret string, key *Key, err error) {
	secret = KeyPrefix + randomToken(40)
	k := &Key{
		ID:        "key_" + randomHex(8),
		Name:      name,
		Hash:      hashKey(secret),
		Display:   secret[:12],
		CreatedAt: time.Now().UTC(),
	}
	_, err = s.pool.Exec(ctx,
		`INSERT INTO admin_api_keys (id, name, hash, display, created_at)
		 VALUES ($1,$2,$3,$4,$5)`,
		k.ID, k.Name, k.Hash, k.Display, k.CreatedAt)
	if err != nil {
		return "", nil, err
	}
	return secret, k, nil
}

// List нь бүх managed key-г шинэ эхэнд буцаана. API давхарга hash-ыг халхлана.
func (s *Store) List(ctx context.Context) []Key {
	rows, err := s.pool.Query(ctx,
		`SELECT id, name, hash, display, created_at, last_used_at, disabled
		   FROM admin_api_keys ORDER BY created_at DESC`)
	if err != nil {
		return nil
	}
	defer rows.Close()
	var out []Key
	for rows.Next() {
		var k Key
		var lastUsed *time.Time
		if err := rows.Scan(&k.ID, &k.Name, &k.Hash, &k.Display, &k.CreatedAt, &lastUsed, &k.Disabled); err != nil {
			return out
		}
		if lastUsed != nil {
			k.LastUsedAt = *lastUsed
		}
		out = append(out, k)
	}
	return out
}

// Revoke нь managed key-г устгана. Env bootstrap key-д мөр байхгүй тул энд
// цуцлагдахгүй — операторорчноос хасна.
func (s *Store) Revoke(ctx context.Context, id string) error {
	ct, err := s.pool.Exec(ctx, `DELETE FROM admin_api_keys WHERE id=$1`, id)
	if err != nil {
		return err
	}
	if ct.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

var ErrNotFound = errors.New("adminkeys: key not found")

func hashKey(secret string) string {
	sum := sha256.Sum256([]byte(secret))
	return hex.EncodeToString(sum[:])
}

func randomHex(n int) string {
	b := make([]byte, n)
	// Энэ нь креденшл (key id) үүсгэдэг. crypto/rand алдаа гарвал zero буфертай
	// (таамаглах боломжтой) үргэлжлэхийн оронд заавал зогсооно.
	if _, err := rand.Read(b); err != nil {
		panic("adminkeys: crypto/rand failed: " + err.Error())
	}
	return hex.EncodeToString(b)
}

func randomToken(n int) string {
	const alphabet = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789"
	// Rejection sampling — `b[i] % 62`-ийн modulo bias-аас зайлсхийнэ. 256 % 62
	// == 8 тул 256-аас доош 62-ийн хамгийн том үржвэр нь 248; >= 248 байт
	// татгалзаж дахин авна.
	const maxUnbiased = 256 - (256 % len(alphabet)) // 248
	out := make([]byte, n)
	var buf []byte
	for i := 0; i < n; {
		if len(buf) == 0 {
			buf = make([]byte, n)
			if _, err := rand.Read(buf); err != nil {
				panic("adminkeys: crypto/rand failed: " + err.Error())
			}
		}
		b := buf[len(buf)-1]
		buf = buf[:len(buf)-1]
		if int(b) >= maxUnbiased {
			continue
		}
		out[i] = alphabet[int(b)%len(alphabet)]
		i++
	}
	return string(out)
}
