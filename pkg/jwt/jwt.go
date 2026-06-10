// Gerege Template Version 27.0
// Gerege Systems Development Team болон Claude AI хамтран бүтээв, 2026.

package jwt

import (
	"errors"
	"fmt"
	"time"

	golangJWT "github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"template/pkg/clock"
	"template/pkg/logger"
)

// ErrInvalidToken нь токен задлан унших эсвэл баталгаажуулахад амжилтгүй болоход буцаагдана.
var ErrInvalidToken = errors.New("token is not valid")

// ErrWrongTokenKind нь дуудагч access токеныг refresh токен мэтээр (эсвэл
// эсрэгээр) задлан уншихыг оролдоход буцаагдана.
var ErrWrongTokenKind = errors.New("token kind mismatch")

// Kind-ууд нь access болон refresh токеныг ялгана. Claim дотор гарын үсэг
// зурагдсан тул эвдэрсэн refresh токеныг access токен болгон дахин ашиглах боломжгүй.
const (
	KindAccess  = "access"
	KindRefresh = "refresh"
)

type JWTService interface {
	// GenerateToken нь нэг access токен үүсгэнэ. Дуудагчид зэрэгцээ refresh
	// токен хэрэгтэй бол GenerateTokenPair-г илүүд үзнэ.
	GenerateToken(userId string, isAdmin bool, roleID int, email string) (t string, err error)
	// GenerateTokenPair нь access+refresh хосыг үүсгэнэ, хоёулаа ижил secret-ээр
	// гарын үсэг зурагдсан боловч Kind claim-ээр ялгагдана.
	GenerateTokenPair(userID string, isAdmin bool, roleID int, email string) (TokenPair, error)
	// ParseToken нь access токены гарын үсэг, хүчинтэй хугацаа болон HMAC аргыг
	// шалгана. Refresh токенуудыг ErrWrongTokenKind-ээр татгалзана.
	ParseToken(tokenString string) (claims JwtCustomClaim, err error)
	// ParseRefreshToken нь ParseToken-ийн refresh токены эквивалент юм.
	// Access токенуудыг ErrWrongTokenKind-ээр татгалзана.
	ParseRefreshToken(tokenString string) (claims JwtCustomClaim, err error)
}

// TokenPair нь login / refresh үед хамт олгогддог богино настай access
// токен болон урт настай refresh токеныг багцална.
type TokenPair struct {
	AccessToken      string    `json:"access_token"`
	RefreshToken     string    `json:"refresh_token"`
	AccessExpiresAt  time.Time `json:"access_expires_at"`
	RefreshExpiresAt time.Time `json:"refresh_expires_at"`
	AccessJTI        string    `json:"-"`
	RefreshJTI       string    `json:"-"`
}

type JwtCustomClaim struct {
	UserID  string
	IsAdmin bool
	RoleID  int
	Email   string
	Kind    string
	golangJWT.RegisteredClaims
}

type jwtService struct {
	secretKey      string
	issuer         string
	expired        int
	refreshExpired int // өдөр
	// clock нь IssuedAt болон хүчинтэй хугацааны тооцоололд ашиглагдах "одоо"-гийн
	// эх сурвалж юм. Анхдагч нь RealClock; тестүүд унтахгүйгээр яг таг цагийн
	// тэмдгийг шалгахын тулд clock.Frozen эсвэл clock.Stub-г тарьдаг.
	clock clock.Clock
}

func NewJWTService(secretKey, issuer string, expired int) JWTService {
	return &jwtService{
		issuer:         issuer,
		secretKey:      secretKey,
		expired:        expired,
		refreshExpired: 7,
		clock:          clock.RealClock{},
	}
}

// NewJWTServiceWithRefresh нь тус тусдаа тохируулж болох настай access +
// refresh токены хосыг үүсгэдэг сервис байгуулна.
func NewJWTServiceWithRefresh(secretKey, issuer string, expiredHours, refreshExpiredDays int) JWTService {
	return &jwtService{
		issuer:         issuer,
		secretKey:      secretKey,
		expired:        expiredHours,
		refreshExpired: refreshExpiredDays,
		clock:          clock.RealClock{},
	}
}

// WithClock нь өгөгдсөн clock-оор орлуулсан сервисийн хуулбарыг буцаана.
// Тестүүд токен олголтын үеийн цагийг царцаах (freeze) болон яг таг ExpiresAt
// утгуудыг шалгахын тулд үүнийг ашигладаг.
func WithClock(svc JWTService, c clock.Clock) JWTService {
	if s, ok := svc.(*jwtService); ok {
		clone := *s
		clone.clock = c
		return &clone
	}
	return svc
}

func (j *jwtService) GenerateToken(userID string, isAdmin bool, roleID int, email string) (string, error) {
	tok, _, _, err := j.signAccess(userID, isAdmin, roleID, email)
	return tok, err
}

func (j *jwtService) GenerateTokenPair(userID string, isAdmin bool, roleID int, email string) (TokenPair, error) {
	access, accessExp, accessJTI, err := j.signAccess(userID, isAdmin, roleID, email)
	if err != nil {
		return TokenPair{}, err
	}
	refresh, refreshExp, refreshJTI, err := j.signRefresh(userID, email)
	if err != nil {
		return TokenPair{}, err
	}
	return TokenPair{
		AccessToken:      access,
		RefreshToken:     refresh,
		AccessExpiresAt:  accessExp,
		RefreshExpiresAt: refreshExp,
		AccessJTI:        accessJTI,
		RefreshJTI:       refreshJTI,
	}, nil
}

func (j *jwtService) signAccess(userID string, isAdmin bool, roleID int, email string) (token string, expiresAt time.Time, jti string, err error) {
	now := j.clock.Now()
	expiresAt = now.Add(time.Hour * time.Duration(j.expired))
	jti = uuid.NewString()
	claims := &JwtCustomClaim{
		UserID:  userID,
		IsAdmin: isAdmin,
		RoleID:  roleID,
		Email:   email,
		Kind:    KindAccess,
		RegisteredClaims: golangJWT.RegisteredClaims{
			ID:        jti,
			ExpiresAt: golangJWT.NewNumericDate(expiresAt),
			Issuer:    j.issuer,
			IssuedAt:  golangJWT.NewNumericDate(now),
		},
	}
	token, err = j.sign(claims)
	if err != nil {
		return "", time.Time{}, "", err
	}
	return token, expiresAt, jti, nil
}

func (j *jwtService) signRefresh(userID, email string) (token string, expiresAt time.Time, jti string, err error) {
	now := j.clock.Now()
	expiresAt = now.Add(24 * time.Hour * time.Duration(j.refreshExpired))
	jti = uuid.NewString()
	claims := &JwtCustomClaim{
		UserID: userID,
		Email:  email,
		Kind:   KindRefresh,
		RegisteredClaims: golangJWT.RegisteredClaims{
			ID:        jti,
			ExpiresAt: golangJWT.NewNumericDate(expiresAt),
			Issuer:    j.issuer,
			IssuedAt:  golangJWT.NewNumericDate(now),
		},
	}
	token, err = j.sign(claims)
	if err != nil {
		return "", time.Time{}, "", err
	}
	return token, expiresAt, jti, nil
}

func (j *jwtService) sign(claims *JwtCustomClaim) (string, error) {
	token := golangJWT.NewWithClaims(golangJWT.SigningMethodHS256, claims)
	signed, err := token.SignedString([]byte(j.secretKey))
	if err != nil {
		logger.Error("jwt: sign failed", logger.Fields{
			"package": "jwt",
			"step":    "signed_string",
			"kind":    claims.Kind,
			"error":   err.Error(),
		})
		return "", fmt.Errorf("sign jwt: %w", err)
	}
	return signed, nil
}

func (j *jwtService) ParseToken(tokenString string) (JwtCustomClaim, error) {
	claims, err := j.parse(tokenString)
	if err != nil {
		return JwtCustomClaim{}, err
	}
	// Хоосон Kind-г access токен гэж хүлээн авна; зөвхөн илэрхий access биш
	// утгыг (жишээ нь KindRefresh) энд татгалзана.
	if claims.Kind != "" && claims.Kind != KindAccess {
		return JwtCustomClaim{}, ErrWrongTokenKind
	}
	return claims, nil
}

func (j *jwtService) ParseRefreshToken(tokenString string) (JwtCustomClaim, error) {
	claims, err := j.parse(tokenString)
	if err != nil {
		return JwtCustomClaim{}, err
	}
	if claims.Kind != KindRefresh {
		return JwtCustomClaim{}, ErrWrongTokenKind
	}
	return claims, nil
}

func (j *jwtService) parse(tokenString string) (JwtCustomClaim, error) {
	var claims JwtCustomClaim
	token, err := golangJWT.ParseWithClaims(tokenString, &claims, func(token *golangJWT.Token) (interface{}, error) {
		if _, ok := token.Method.(*golangJWT.SigningMethodHMAC); !ok {
			alg := token.Header["alg"]
			logger.Warn("jwt: unexpected signing method", logger.Fields{
				"package": "jwt",
				"step":    "verify_signing_method",
				"alg":     fmt.Sprintf("%v", alg),
			})
			return nil, fmt.Errorf("unexpected signing method: %v", alg)
		}
		return []byte(j.secretKey), nil
	},
		// keyfunc нь non-HMAC-г татгалздаг (alg-confusion); WithValidMethods
		// нь үүнийг declarative-аар давхар баталгаажуулна. WithIssuer нь
		// ижил secret-тэй өөр сервисийн токеныг cross-accept хийхээс,
		// WithExpirationRequired нь exp-гүй токеныг хүчингүй болгоно.
		golangJWT.WithValidMethods([]string{"HS256"}),
		golangJWT.WithIssuer(j.issuer),
		golangJWT.WithExpirationRequired(),
	)
	if err != nil {
		logger.Warn("jwt: parse failed", logger.Fields{
			"package": "jwt",
			"step":    "parse_with_claims",
			"error":   err.Error(),
		})
		return JwtCustomClaim{}, fmt.Errorf("%w: %v", ErrInvalidToken, err)
	}
	if !token.Valid {
		logger.Warn("jwt: token reported invalid by parser", logger.Fields{
			"package": "jwt",
			"step":    "validity_check",
		})
		return JwtCustomClaim{}, ErrInvalidToken
	}
	return claims, nil
}
