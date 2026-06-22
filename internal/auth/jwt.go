package auth

import (
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/zenstats/zenstats/config"
)

func jwtSecret() []byte {
	return []byte(config.Conf.SecretKey)
}

type CustomClaims struct {
	UserID       int64  `json:"user_id"`
	UserType     string `json:"user_type"`      // "user" or "sub_account"
	SubAccountID int64  `json:"sub_account_id"` // only set when UserType == "sub_account"
	Role         string `json:"role"`           // sub-account role: "viewer", "admin", etc.
	jwt.RegisteredClaims
}

func GenerateRefreshToken(userID int64) (string, error) {
	claims := CustomClaims{
		UserID:   userID,
		UserType: "user",
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(24 * time.Hour)),
			Issuer:    "zenstats",
			Subject:   "refresh_token",
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
			Subject:   "access_token",
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(jwtSecret())
}

func GenerateSubAccountToken(subAccountID, parentUserID int64, role string) (string, error) {
	claims := CustomClaims{
		UserID:       parentUserID,
		UserType:     "sub_account",
		SubAccountID: subAccountID,
		Role:         role,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(1 * time.Hour)),
			Issuer:    "zenstats",
			Subject:   "access_token",
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(jwtSecret())
}

func GenerateSubAccountRefreshToken(subAccountID, parentUserID int64, role string) (string, error) {
	claims := CustomClaims{
		UserID:       parentUserID,
		UserType:     "sub_account",
		SubAccountID: subAccountID,
		Role:         role,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(24 * time.Hour)),
			Issuer:    "zenstats",
			Subject:   "refresh_token",
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(jwtSecret())
}

func ParseToken(tokenString string) (*CustomClaims, error) {
	token, err := jwt.ParseWithClaims(tokenString, &CustomClaims{}, func(token *jwt.Token) (any, error) {
		return jwtSecret(), nil
	})
	if err != nil {
		return nil, err
	}
	// 验证token是否有效
	if !token.Valid {
		return nil, jwt.ErrInvalidKey
	}

	if claims, ok := token.Claims.(*CustomClaims); ok && token.Valid {
		return claims, nil
	}
	return nil, jwt.ErrInvalidKey
}
