package auth

import (
	"crypto/rsa"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"math/big"
	"net/http"
	"sync"
	"time"

	log "github.com/sirupsen/logrus"
)

// JWKSFetcher fetches and caches RSA public keys from a JWKS endpoint.
type JWKSFetcher struct {
	url      string
	cacheTTL time.Duration
	client   *http.Client

	mu        sync.RWMutex
	keys      map[string]*rsa.PublicKey
	expiresAt time.Time
	lastFetch time.Time
	lastErr   error
}

// NewJWKSFetcher creates a fetcher for the given JWKS URL.
func NewJWKSFetcher(url string, cacheTTL time.Duration) *JWKSFetcher {
	if cacheTTL <= 0 {
		cacheTTL = time.Hour
	}
	return &JWKSFetcher{
		url:      url,
		cacheTTL: cacheTTL,
		client:   &http.Client{Timeout: 10 * time.Second},
		keys:     make(map[string]*rsa.PublicKey),
	}
}

// GetPublicKey returns the RSA public key for the given kid, refreshing the cache if needed.
func (f *JWKSFetcher) GetPublicKey(kid string) (*rsa.PublicKey, error) {
	f.mu.RLock()
	if time.Now().Before(f.expiresAt) {
		if key, ok := f.keys[kid]; ok {
			f.mu.RUnlock()
			return key, nil
		}
		if kid == "" && len(f.keys) > 0 {
			for _, key := range f.keys {
				f.mu.RUnlock()
				return key, nil
			}
		}
	}
	f.mu.RUnlock()

	if err := f.refresh(); err != nil {
		return nil, err
	}

	f.mu.RLock()
	defer f.mu.RUnlock()

	if key, ok := f.keys[kid]; ok {
		return key, nil
	}
	if kid == "" && len(f.keys) > 0 {
		for _, key := range f.keys {
			return key, nil
		}
	}
	return nil, fmt.Errorf("key not found in JWKS: %s", kid)
}

type jwksResponse struct {
	Keys []jwkKey `json:"keys"`
}

type jwkKey struct {
	Kty string `json:"kty"`
	Kid string `json:"kid"`
	Use string `json:"use"`
	N   string `json:"n"`
	E   string `json:"e"`
}

func (f *JWKSFetcher) refresh() error {
	f.mu.Lock()
	defer f.mu.Unlock()

	// Double-check after acquiring write lock
	if time.Now().Before(f.expiresAt) {
		return nil
	}

	log.WithField("url", f.url).Debug("Fetching JWKS")

	resp, err := f.client.Get(f.url)
	if err != nil {
		return f.handleFetchError(fmt.Errorf("fetching JWKS: %w", err))
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return f.handleFetchError(fmt.Errorf("JWKS endpoint returned status %d", resp.StatusCode))
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return f.handleFetchError(fmt.Errorf("reading JWKS response: %w", err))
	}

	var jwks jwksResponse
	if err := json.Unmarshal(body, &jwks); err != nil {
		return f.handleFetchError(fmt.Errorf("parsing JWKS: %w", err))
	}

	newKeys := make(map[string]*rsa.PublicKey)
	for _, k := range jwks.Keys {
		if k.Kty != "RSA" {
			continue
		}

		nBytes, err := base64.RawURLEncoding.DecodeString(k.N)
		if err != nil {
			log.WithError(err).WithField("kid", k.Kid).Warn("Failed to decode key modulus")
			continue
		}
		eBytes, err := base64.RawURLEncoding.DecodeString(k.E)
		if err != nil {
			log.WithError(err).WithField("kid", k.Kid).Warn("Failed to decode key exponent")
			continue
		}

		n := new(big.Int).SetBytes(nBytes)
		e := 0
		for _, b := range eBytes {
			e = e<<8 + int(b)
		}

		newKeys[k.Kid] = &rsa.PublicKey{N: n, E: e}
	}

	if len(newKeys) == 0 {
		return f.handleFetchError(fmt.Errorf("no valid RSA keys found in JWKS"))
	}

	f.keys = newKeys
	f.expiresAt = time.Now().Add(f.cacheTTL)
	f.lastFetch = time.Now()
	f.lastErr = nil

	log.WithField("key_count", len(newKeys)).Info("JWKS cache refreshed")
	return nil
}

// handleFetchError implements stale-while-revalidate: if cached keys exist, use them.
// Must be called with mutex held.
func (f *JWKSFetcher) handleFetchError(err error) error {
	f.lastErr = err

	if len(f.keys) > 0 {
		cacheAge := time.Since(f.lastFetch)
		log.WithFields(log.Fields{
			"error":     err.Error(),
			"cache_age": cacheAge.String(),
		}).Warn("JWKS fetch failed, using cached keys")
		return nil
	}

	return err
}
