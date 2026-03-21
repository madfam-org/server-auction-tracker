package notify

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/madfam-org/server-auction-tracker/internal/config"
	"github.com/madfam-org/server-auction-tracker/internal/scanner"
	"github.com/madfam-org/server-auction-tracker/internal/scorer"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func testServers() []scorer.ScoredServer {
	return []scorer.ScoredServer{
		{
			Server: scanner.Server{
				ID: 1001, CPU: "AMD Ryzen 5 3600", RAMSize: 64,
				Price: 39, Datacenter: "HEL1-DC7",
			},
			Score: 85.5,
		},
	}
}

func TestEncliiNotifier(t *testing.T) {
	var received encliiPayload
	var gotAuth string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		body, _ := io.ReadAll(r.Body)
		json.Unmarshal(body, &received)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	cfg := config.EncliiConfig{
		APIURL:        srv.URL,
		ProjectSlug:   "foundry-scout",
		WebhookSecret: "test-secret",
	}

	n := NewEncliiNotifier(cfg)
	err := n.Notify(context.Background(), testServers())
	require.NoError(t, err)

	assert.Equal(t, "auction.servers_found", received.EventType)
	assert.Equal(t, "foundry-scout", received.Project)
	assert.Equal(t, 1, received.Data.Count)
	assert.Equal(t, 85.5, received.Data.TopScore)
	assert.Len(t, received.Data.Servers, 1)
	assert.Equal(t, 1001, received.Data.Servers[0].ID)
	assert.Equal(t, "Bearer test-secret", gotAuth)
}

func TestEncliiNotifierEnvOverride(t *testing.T) {
	var gotAuth string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	t.Setenv("SCOUT_NOTIFY_ENCLII_CALLBACK_TOKEN", "env-token")

	cfg := config.EncliiConfig{
		APIURL:        srv.URL,
		ProjectSlug:   "foundry-scout",
		WebhookSecret: "config-secret",
	}

	n := NewEncliiNotifier(cfg)
	err := n.Notify(context.Background(), testServers())
	require.NoError(t, err)

	assert.Equal(t, "Bearer env-token", gotAuth, "env var should take precedence over config")
}

func TestEncliiNotifierEmpty(t *testing.T) {
	n := NewEncliiNotifier(config.EncliiConfig{APIURL: "http://localhost"})
	err := n.Notify(context.Background(), nil)
	require.NoError(t, err)
}

func TestEncliiNotifierServerError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	n := NewEncliiNotifier(config.EncliiConfig{APIURL: srv.URL})
	err := n.Notify(context.Background(), testServers())
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "status 500")
}
