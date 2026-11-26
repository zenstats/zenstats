package auth

import (
	"crypto/sha256"
	"encoding/hex"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/zenstats/zenstats/config"
)

// Token subjects. Middlewares accept only SubjectAccessToken; /auth/refresh accepts only SubjectRefreshToken.
const (
	SubjectAccessToken  = "access_token"
	SubjectRefreshToken = "refresh_token"
)

func jwtSecret() []byte {
	return []byte(config.Conf.SecretKey)
}

type CustomClaims struct {
	UserID       int64    `json:"user_id"`
	UserType     string   `json:"user_type"`      // "user" or "sub_account"
	SubAccountID int64    `json:"sub_account_id"` // only set when UserType == "sub_account"
	Role         string   `json:"role"`           // display label: viewer / editor / admin / custom
	Permissions  []string `json:"permissions"`    // fine-grained permission list
	jwt.RegisteredClaims
}

func GenerateRefreshToken(userID int64) (string, error) {
	claims := CustomClaims{
		UserID:   userID,
		UserType: "user",
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(24 * time.Hour)),
			Issuer:    "zenstats",
			Subject:   SubjectRefreshToken,
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(jwtSecret())
}

func GenerateToken(userID int64) (string, error) {
	claims := CustomClaims{
		UserID:   userID,
		UserType: "user",
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(1 * time.Hour)),
			Issuer:    "zenstats",
			Subject:   SubjectAccessToken,
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(jwtSecret())
}

func GenerateSubAccountToken(subAccountID, parentUserID int64, role string, permissions []string) (string, error) {
	if permissions == nil {
		permissions = []string{}
	}
	claims := CustomClaims{
		UserID:       parentUserID,
		UserType:     "sub_account",
		SubAccountID: subAccountID,
		Role:         role,
		Permissions:  permissions,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(1 * time.Hour)),
			Issuer:    "zenstats",
			Subject:   SubjectAccessToken,
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(jwtSecret())
}

func GenerateSubAccountRefreshToken(subAccountID, parentUserID int64, role string, permissions []string) (string, error) {
	if permissions == nil {
		permissions = []string{}
	}
	claims := CustomClaims{
		UserID:       parentUserID,
		UserType:     "sub_account",
		SubAccountID: subAccountID,
		Role:         role,
		Permissions:  permissions,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(24 * time.Hour)),
			Issuer:    "zenstats",
			Subject:   SubjectRefreshToken,
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(jwtSecret())
}

func ParseToken(tokenString string) (*CustomClaims, error) {
	token, err := jwt.ParseWithClaims(tokenString, &CustomClaims{}, func(token *jwt.Token) (any, error) {
		return jwtSecret(), nil
	}, jwt.WithValidMethods([]string{"HS256"})) // 固定签名算法，防止 alg 混淆攻击
	if err != nil {
		return nil, err
	}
	// Verify token validity
	if !token.Valid {
		return nil, jwt.ErrInvalidKey
	}

	if claims, ok := token.Claims.(*CustomClaims); ok && token.Valid {
		return claims, nil
	}
	return nil, jwt.ErrInvalidKey
}

const SubjectSharedLinkToken = "shared_link_token"

type SharedLinkClaims struct {
	Slug        string `json:"slug"`
	SiteID      int64  `json:"site_id"`
	PasswordSig string `json:"password_sig"`
	jwt.RegisteredClaims
}

// SharedLinkPasswordSignature binds a token to the current password hash.
func SharedLinkPasswordSignature(passwordHash string) string {
	digest := sha256.Sum256([]byte(passwordHash))
	return hex.EncodeToString(digest[:16])
}

func GenerateSharedLinkToken(slug string, siteID int64, passwordHash string, ttl time.Duration) (string, error) {
	if ttl <= 0 {
		ttl = time.Hour
	}
	claims := SharedLinkClaims{
		Slug: slug, SiteID: siteID, PasswordSig: SharedLinkPasswordSignature(passwordHash),
		RegisteredClaims: jwt.RegisteredClaims{
			Subject: SubjectSharedLinkToken, Issuer: "zenstats",
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(ttl)),
		},
	}
	return jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString(jwtSecret())
}

func ParseSharedLinkToken(tokenString string) (*SharedLinkClaims, error) {
	claims := new(SharedLinkClaims)
	token, err := jwt.ParseWithClaims(tokenString, claims, func(*jwt.Token) (any, error) {
		return jwtSecret(), nil
	}, jwt.WithValidMethods([]string{"HS256"}), jwt.WithExpirationRequired(), jwt.WithSubject(SubjectSharedLinkToken))
	if err != nil {
		return nil, err
	}
	if !token.Valid {
		return nil, jwt.ErrTokenInvalidClaims
	}
	return claims, nil
}

// Matches checks the current link, including immediate password-change revocation.
func (c *SharedLinkClaims) Matches(slug string, siteID int64, passwordHash string) bool {
	return c.Slug == slug && c.SiteID == siteID && c.PasswordSig == SharedLinkPasswordSignature(passwordHash)
}
