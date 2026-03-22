package auth

import (
	"crypto/rand"
	"crypto/rsa"
	"encoding/base64"
	"encoding/json"
	"math/big"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func generateTestKey(t *testing.T) *rsa.PrivateKey {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	return key
}

func serveJWKS(t *testing.T, kid string, pubKey *rsa.PublicKey) *httptest.Server {
	t.Helper()
	jwks := map[string]interface{}{
		"keys": []map[string]string{
			{
				"kty": "RSA",
				"kid": kid,
				"use": "sig",
				"n":   base64.RawURLEncoding.EncodeToString(pubKey.N.Bytes()),
				"e":   base64.RawURLEncoding.EncodeToString(big.NewInt(int64(pubKey.E)).Bytes()),
			},
		},
	}
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(jwks) //nolint:errcheck
	}))
}

func TestJWKSFetcher_GetPublicKey(t *testing.T) {
	key := generateTestKey(t)
	srv := serveJWKS(t, "test-kid-1", &key.PublicKey)
	defer srv.Close()

	fetcher := NewJWKSFetcher(srv.URL, time.Minute)

	pub, err := fetcher.GetPublicKey("test-kid-1")
	require.NoError(t, err)
	assert.Equal(t, key.N, pub.N)
	assert.Equal(t, key.E, pub.E)
}

func TestJWKSFetcher_CacheHit(t *testing.T) {
	key := generateTestKey(t)
	callCount := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		jwks := map[string]interface{}{
			"keys": []map[string]string{
				{
					"kty": "RSA",
					"kid": "cached-kid",
					"use": "sig",
					"n":   base64.RawURLEncoding.EncodeToString(key.N.Bytes()),
					"e":   base64.RawURLEncoding.EncodeToString(big.NewInt(int64(key.E)).Bytes()),
				},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(jwks) //nolint:errcheck
	}))
	defer srv.Close()

	fetcher := NewJWKSFetcher(srv.URL, time.Minute)

	_, err := fetcher.GetPublicKey("cached-kid")
	require.NoError(t, err)
	assert.Equal(t, 1, callCount)

	// Second call should use cache
	_, err = fetcher.GetPublicKey("cached-kid")
	require.NoError(t, err)
	assert.Equal(t, 1, callCount)
}

func TestJWKSFetcher_KeyNotFound(t *testing.T) {
	key := generateTestKey(t)
	srv := serveJWKS(t, "existing-kid", &key.PublicKey)
	defer srv.Close()

	fetcher := NewJWKSFetcher(srv.URL, time.Minute)

	_, err := fetcher.GetPublicKey("nonexistent-kid")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "key not found")
}

func TestJWKSFetcher_StaleWhileRevalidate(t *testing.T) {
	key := generateTestKey(t)
	callCount := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		if callCount > 1 {
			// Simulate failure on second fetch
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		jwks := map[string]interface{}{
			"keys": []map[string]string{
				{
					"kty": "RSA",
					"kid": "stale-kid",
					"use": "sig",
					"n":   base64.RawURLEncoding.EncodeToString(key.N.Bytes()),
					"e":   base64.RawURLEncoding.EncodeToString(big.NewInt(int64(key.E)).Bytes()),
				},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(jwks) //nolint:errcheck
	}))
	defer srv.Close()

	fetcher := NewJWKSFetcher(srv.URL, 1*time.Millisecond)

	// First fetch succeeds
	pub, err := fetcher.GetPublicKey("stale-kid")
	require.NoError(t, err)
	assert.NotNil(t, pub)

	// Wait for cache to expire
	time.Sleep(5 * time.Millisecond)

	// Second fetch fails but stale cache should be used
	pub, err = fetcher.GetPublicKey("stale-kid")
	require.NoError(t, err)
	assert.NotNil(t, pub)
}

func TestJWKSFetcher_Unreachable(t *testing.T) {
	fetcher := NewJWKSFetcher("http://127.0.0.1:1", time.Minute)
	_, err := fetcher.GetPublicKey("any-kid")
	assert.Error(t, err)
}

func TestJWKSFetcher_EmptyKid(t *testing.T) {
	key := generateTestKey(t)
	srv := serveJWKS(t, "default-kid", &key.PublicKey)
	defer srv.Close()

	fetcher := NewJWKSFetcher(srv.URL, time.Minute)

	// Empty kid should return the first available key
	pub, err := fetcher.GetPublicKey("")
	require.NoError(t, err)
	assert.Equal(t, key.N, pub.N)
}
