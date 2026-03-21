package notify

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/madfam-org/server-auction-tracker/internal/config"
	"github.com/madfam-org/server-auction-tracker/internal/scanner"
	"github.com/madfam-org/server-auction-tracker/internal/scorer"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestWebhookNotifier(t *testing.T) {
	var received webhookPayload
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "application/json", r.Header.Get("Content-Type"))
		assert.Equal(t, "test-value", r.Header.Get("X-Custom"))
		require.NoError(t, json.NewDecoder(r.Body).Decode(&received))
		w.WriteHeader(http.StatusOK)
	}))
	defer ts.Close()

	n := NewWebhookNotifier(config.WebhookConfig{
		URL:     ts.URL,
		Headers: map[string]string{"X-Custom": "test-value"},
	})

	servers := []scorer.ScoredServer{{
		Server: scanner.Server{ID: 1, CPU: "AMD Ryzen 5 3600", RAMSize: 64, Price: 39.00, Datacenter: "HEL1"},
		Score:  85.0,
	}}

	err := n.Notify(context.Background(), servers)
	require.NoError(t, err)
	assert.Equal(t, "servers_found", received.Event)
	assert.Equal(t, 1, received.Count)
	assert.Equal(t, 85.0, received.TopScore)
	assert.Len(t, received.Servers, 1)
}

func TestWebhookNotifierEmpty(t *testing.T) {
	n := NewWebhookNotifier(config.WebhookConfig{URL: "http://localhost:9999"})
	err := n.Notify(context.Background(), nil)
	assert.NoError(t, err)
}

func TestWebhookNotifierHTTPError(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer ts.Close()

	n := NewWebhookNotifier(config.WebhookConfig{URL: ts.URL})
	servers := []scorer.ScoredServer{{
		Server: scanner.Server{ID: 1, CPU: "test", RAMSize: 64, Price: 39.00},
		Score:  80.0,
	}}
	err := n.Notify(context.Background(), servers)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "500")
}
