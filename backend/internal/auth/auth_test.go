package auth

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/alexedwards/argon2id"
	"github.com/golang-jwt/jwt/v5"
)

const testSecret = "test-signing-key-with-at-least-32-characters"
const testUserID = "11111111-1111-4111-8111-111111111111"

func TestPasswordHashesUseRandomSalts(t *testing.T) {
	first, err := argon2id.CreateHash("correct horse battery", passwordParams)
	if err != nil {
		t.Fatal(err)
	}
	second, err := argon2id.CreateHash("correct horse battery", passwordParams)
	if err != nil {
		t.Fatal(err)
	}
	if first == second || !strings.HasPrefix(first, "$argon2id$") {
		t.Fatal("password hashes should use Argon2id with independent salts")
	}
	for _, test := range []struct {
		password string
		match    bool
	}{{"correct horse battery", true}, {"a different password", false}} {
		match, err := argon2id.ComparePasswordAndHash(test.password, first)
		if err != nil || match != test.match {
			t.Fatalf("password match=%v, want %v: %v", match, test.match, err)
		}
	}
}

func TestTokenValidation(t *testing.T) {
	service, err := NewService(nil, testSecret)
	if err != nil {
		t.Fatal(err)
	}
	_, valid, err := service.session(account{User: User{ID: testUserID}, sessionVersion: 3})
	if err != nil {
		t.Fatal(err)
	}
	claims, err := service.parseToken(valid)
	if err != nil || claims.Subject != testUserID || claims.SessionVersion != 3 {
		t.Fatalf("valid session rejected: %v", err)
	}

	base := sessionClaims{
		SessionVersion: 3,
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer: "mediashare", Audience: jwt.ClaimStrings{"mediashare-web"}, Subject: testUserID,
			IssuedAt:  jwt.NewNumericDate(time.Now().Add(-time.Minute)),
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour)),
		},
	}
	for _, test := range []struct {
		name   string
		change func(*sessionClaims)
		method jwt.SigningMethod
		key    string
	}{
		{"wrong_key", nil, jwt.SigningMethodHS256, "a-different-signing-key"},
		{"wrong_algorithm", nil, jwt.SigningMethodHS512, testSecret},
		{"expired", func(c *sessionClaims) { c.ExpiresAt = jwt.NewNumericDate(time.Now().Add(-time.Minute)) }, jwt.SigningMethodHS256, testSecret},
		{"missing_expiration", func(c *sessionClaims) { c.ExpiresAt = nil }, jwt.SigningMethodHS256, testSecret},
		{"wrong_issuer", func(c *sessionClaims) { c.Issuer = "another-app" }, jwt.SigningMethodHS256, testSecret},
		{"wrong_audience", func(c *sessionClaims) { c.Audience = jwt.ClaimStrings{"another-client"} }, jwt.SigningMethodHS256, testSecret},
		{"invalid_subject", func(c *sessionClaims) { c.Subject = "not-a-user-id" }, jwt.SigningMethodHS256, testSecret},
		{"future_issued_at", func(c *sessionClaims) { c.IssuedAt = jwt.NewNumericDate(time.Now().Add(time.Hour)) }, jwt.SigningMethodHS256, testSecret},
		{"negative_version", func(c *sessionClaims) { c.SessionVersion = -1 }, jwt.SigningMethodHS256, testSecret},
	} {
		t.Run(test.name, func(t *testing.T) {
			c := base
			if test.change != nil {
				test.change(&c)
			}
			raw, err := jwt.NewWithClaims(test.method, c).SignedString([]byte(test.key))
			if err != nil {
				t.Fatal(err)
			}
			if _, err := service.parseToken(raw); !errors.Is(err, ErrInvalidSession) {
				t.Fatalf("invalid token was accepted: %v", err)
			}
		})
	}
	if _, err := service.parseToken("not-a-token"); !errors.Is(err, ErrInvalidSession) {
		t.Fatal("malformed token accepted")
	}
	if _, err := NewService(nil, "short"); err == nil {
		t.Fatal("short signing key accepted")
	}
}
