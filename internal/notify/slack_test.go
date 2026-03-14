package notify

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/madfam-org/server-auction-tracker/internal/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSlackNotifier(t *testing.T) {
	var received map[string]interface{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		json.Unmarshal(body, &received)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	n := NewSlackNotifier(config.SlackConfig{WebhookURL: srv.URL})
	err := n.Notify(context.Background(), testServers())
	require.NoError(t, err)

	blocks, ok := received["blocks"].([]interface{})
	require.True(t, ok)
	assert.Len(t, blocks, 2)
}

func TestSlackNotifierEmpty(t *testing.T) {
	n := NewSlackNotifier(config.SlackConfig{WebhookURL: "http://localhost"})
	err := n.Notify(context.Background(), nil)
	require.NoError(t, err)
}
