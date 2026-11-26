package auth

import (
	"strings"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/zenstats/zenstats/config"
)

func TestSharedLinkToken(t *testing.T) {
	old := config.Conf.SecretKey
	config.Conf.SecretKey = "shared-test-key-01234567890123456789"
	t.Cleanup(func() { config.Conf.SecretKey = old })
	token, err := GenerateSharedLinkToken("slug", 42, "hash", 0)
	if err != nil {
		t.Fatal(err)
	}
	claims, err := ParseSharedLinkToken(token)
	if err != nil {
		t.Fatal(err)
	}
	if !claims.Matches("slug", 42, "hash") || claims.Subject != SubjectSharedLinkToken {
		t.Fatalf("bad claims: %+v", claims)
	}
	if ttl := time.Until(claims.ExpiresAt.Time); ttl < 59*time.Minute || ttl > time.Hour {
		t.Fatalf("bad ttl %s", ttl)
	}
	for _, changed := range []string{"new hash", ""} {
		if claims.Matches("slug", 42, changed) {
			t.Fatal("password change did not revoke")
		}
	}
	if claims.Matches("other", 42, "hash") || claims.Matches("slug", 43, "hash") {
		t.Fatal("token accepted for another link/site")
	}
	parts := strings.Split(token, ".")
	parts[1] = "eyJzbHVnIjoiaGFja2VkIn0"
	if _, err := ParseSharedLinkToken(strings.Join(parts, ".")); err == nil {
		t.Fatal("tampering accepted")
	}
	for _, tc := range []struct {
		name, subject string
		method        jwt.SigningMethod
		expiry        *jwt.NumericDate
	}{
		{"wrong subject", SubjectAccessToken, jwt.SigningMethodHS256, jwt.NewNumericDate(time.Now().Add(time.Hour))},
		{"wrong algorithm", SubjectSharedLinkToken, jwt.SigningMethodHS384, jwt.NewNumericDate(time.Now().Add(time.Hour))},
		{"expired", SubjectSharedLinkToken, jwt.SigningMethodHS256, jwt.NewNumericDate(time.Now().Add(-time.Minute))},
		{"missing expiry", SubjectSharedLinkToken, jwt.SigningMethodHS256, nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			signed, err := jwt.NewWithClaims(tc.method, SharedLinkClaims{RegisteredClaims: jwt.RegisteredClaims{Subject: tc.subject, ExpiresAt: tc.expiry}}).SignedString(jwtSecret())
			if err != nil {
				t.Fatal(err)
			}
			if _, err := ParseSharedLinkToken(signed); err == nil {
				t.Fatal("invalid token accepted")
			}
		})
	}
}
