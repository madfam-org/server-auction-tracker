package notify

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/madfam-org/server-auction-tracker/internal/config"
	"github.com/madfam-org/server-auction-tracker/internal/scorer"
)

// WebhookNotifier sends JSON payloads to a generic webhook URL.
type WebhookNotifier struct {
	url     string
	headers map[string]string
	client  *http.Client
}

// NewWebhookNotifier creates a notifier that POSTs JSON to any URL.
func NewWebhookNotifier(cfg config.WebhookConfig) *WebhookNotifier {
	return &WebhookNotifier{
		url:     cfg.URL,
		headers: cfg.Headers,
		client:  &http.Client{Timeout: 10 * time.Second},
	}
}

type webhookPayload struct {
	Event     string               `json:"event"`
	Count     int                  `json:"count"`
	TopScore  float64              `json:"top_score"`
	Servers   []webhookServerEntry `json:"servers"`
	Timestamp string               `json:"timestamp"`
}

type webhookServerEntry struct {
	ID         int     `json:"id"`
	CPU        string  `json:"cpu"`
	RAMSize    int     `json:"ram_size"`
	Price      float64 `json:"price"`
	Score      float64 `json:"score"`
	Datacenter string  `json:"datacenter"`
}

func (w *WebhookNotifier) Notify(ctx context.Context, servers []scorer.ScoredServer) error {
	if len(servers) == 0 {
		return nil
	}

	var topScore float64
	entries := make([]webhookServerEntry, len(servers))
	for i, s := range servers {
		entries[i] = webhookServerEntry{
			ID: s.Server.ID, CPU: s.Server.CPU,
			RAMSize: s.Server.RAMSize, Price: s.Server.Price,
			Score: s.Score, Datacenter: s.Server.Datacenter,
		}
		if s.Score > topScore {
			topScore = s.Score
		}
	}

	payload := webhookPayload{
		Event:     "servers_found",
		Count:     len(servers),
		TopScore:  topScore,
		Servers:   entries,
		Timestamp: time.Now().UTC().Format(time.RFC3339),
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshalling webhook payload: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, w.url, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("creating webhook request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	for k, v := range w.headers {
		req.Header.Set(k, v)
	}

	resp, err := w.client.Do(req)
	if err != nil {
		return fmt.Errorf("webhook POST failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		return fmt.Errorf("webhook returned status %d", resp.StatusCode)
	}
	return nil
}
