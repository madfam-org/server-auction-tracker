package auth

import (
	"crypto/rand"
	"crypto/rsa"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func makeTestValidator(t *testing.T, key *rsa.PrivateKey, kid string) *Validator {
	t.Helper()
	srv := serveJWKS(t, kid, &key.PublicKey)
	t.Cleanup(srv.Close)

	fetcher := NewJWKSFetcher(srv.URL, time.Minute)
	return NewValidator(fetcher, "https://auth.madfam.io", []string{"@madfam.io"}, []string{"superadmin", "admin", "operator"})
}

func signToken(t *testing.T, key *rsa.PrivateKey, kid string, claims jwt.Claims) string {
	t.Helper()
	token := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)
	token.Header["kid"] = kid
	signed, err := token.SignedString(key)
	require.NoError(t, err)
	return signed
}

func TestValidateToken_Valid(t *testing.T) {
	key, _ := rsa.GenerateKey(rand.Reader, 2048)
	v := makeTestValidator(t, key, "kid-valid")

	tokenStr := signToken(t, key, "kid-valid", &Claims{
		Email: "aldo@madfam.io",
		Name:  "Aldo",
		Roles: []string{"admin"},
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    "https://auth.madfam.io",
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour)),
		},
	})

	claims, err := v.ValidateToken(tokenStr)
	require.NoError(t, err)
	assert.Equal(t, "aldo@madfam.io", claims.Email)
	assert.Equal(t, "Aldo", claims.Name)
	assert.Contains(t, claims.Roles, "admin")
}

func TestValidateToken_Expired(t *testing.T) {
	key, _ := rsa.GenerateKey(rand.Reader, 2048)
	v := makeTestValidator(t, key, "kid-expired")

	tokenStr := signToken(t, key, "kid-expired", &Claims{
		Email: "aldo@madfam.io",
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    "https://auth.madfam.io",
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(-time.Hour)),
		},
	})

	_, err := v.ValidateToken(tokenStr)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "token")
}

func TestValidateToken_WrongIssuer(t *testing.T) {
	key, _ := rsa.GenerateKey(rand.Reader, 2048)
	v := makeTestValidator(t, key, "kid-issuer")

	tokenStr := signToken(t, key, "kid-issuer", &Claims{
		Email: "aldo@madfam.io",
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    "https://evil.example.com",
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour)),
		},
	})

	_, err := v.ValidateToken(tokenStr)
	assert.Error(t, err)
}

func TestValidateToken_WrongKey(t *testing.T) {
	key1, _ := rsa.GenerateKey(rand.Reader, 2048)
	key2, _ := rsa.GenerateKey(rand.Reader, 2048)
	v := makeTestValidator(t, key1, "kid-wrongkey")

	// Sign with key2, but validator only has key1
	tokenStr := signToken(t, key2, "kid-wrongkey", &Claims{
		Email: "aldo@madfam.io",
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    "https://auth.madfam.io",
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour)),
		},
	})

	_, err := v.ValidateToken(tokenStr)
	assert.Error(t, err)
}

func TestIsAuthorized_ValidDomainAndRole(t *testing.T) {
	v := &Validator{
		allowedDomains: []string{"@madfam.io"},
		allowedRoles:   []string{"superadmin", "admin", "operator"},
	}

	assert.True(t, v.IsAuthorized(&Claims{Email: "aldo@madfam.io", Roles: []string{"admin"}}))
	assert.True(t, v.IsAuthorized(&Claims{Email: "ALDO@MADFAM.IO", Roles: []string{"ADMIN"}}))
	assert.True(t, v.IsAuthorized(&Claims{Email: "user@madfam.io", Roles: []string{"viewer", "operator"}}))
}

func TestIsAuthorized_WrongDomain(t *testing.T) {
	v := &Validator{
		allowedDomains: []string{"@madfam.io"},
		allowedRoles:   []string{"admin"},
	}

	assert.False(t, v.IsAuthorized(&Claims{Email: "user@evil.com", Roles: []string{"admin"}}))
}

func TestIsAuthorized_WrongRole(t *testing.T) {
	v := &Validator{
		allowedDomains: []string{"@madfam.io"},
		allowedRoles:   []string{"admin"},
	}

	assert.False(t, v.IsAuthorized(&Claims{Email: "user@madfam.io", Roles: []string{"viewer"}}))
}

func TestIsAuthorized_NoEmail(t *testing.T) {
	v := &Validator{
		allowedDomains: []string{"@madfam.io"},
		allowedRoles:   []string{"admin"},
	}

	assert.False(t, v.IsAuthorized(&Claims{Email: "", Roles: []string{"admin"}}))
	assert.False(t, v.IsAuthorized(nil))
}

func TestIsAuthorized_NoRolesConfigured(t *testing.T) {
	v := &Validator{
		allowedDomains: []string{"@madfam.io"},
		allowedRoles:   []string{},
	}

	// If no roles configured, domain match alone is sufficient
	assert.True(t, v.IsAuthorized(&Claims{Email: "user@madfam.io"}))
}
