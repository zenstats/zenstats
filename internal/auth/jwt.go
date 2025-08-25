package auth

import (
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/zenstats/zenstats/config"
)

var JWTSecret = []byte(config.Conf.SecretKey) // 替换为你的密钥

type CustomClaims struct {
	UserID int64 `json:"user_id"`
	jwt.RegisteredClaims
}

func GenerateRefreshToken(userID int64) (string, error) {
	claims := CustomClaims{
		UserID: userID,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(24 * time.Hour)),
			Issuer:    "zenstats",
			Subject:   "refresh_token",
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(JWTSecret)
}

func GenerateToken(userID int64) (string, error) {
	claims := CustomClaims{
		UserID: userID,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(1 * time.Hour)),
			Issuer:    "zenstats",
			Subject:   "access_token",
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(JWTSecret)
}

func ParseToken(tokenString string) (*CustomClaims, error) {
	token, err := jwt.ParseWithClaims(tokenString, &CustomClaims{}, func(token *jwt.Token) (any, error) {
		return JWTSecret, nil
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
