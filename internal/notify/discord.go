package notify

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/madfam-org/server-auction-tracker/internal/config"
	"github.com/madfam-org/server-auction-tracker/internal/scorer"
)

// DiscordNotifier posts embeds to a Discord webhook.
type DiscordNotifier struct {
	cfg    config.DiscordConfig
	client *http.Client
}

func NewDiscordNotifier(cfg config.DiscordConfig) *DiscordNotifier {
	return &DiscordNotifier{
		cfg:    cfg,
		client: &http.Client{Timeout: 10 * time.Second},
	}
}

func (n *DiscordNotifier) Notify(ctx context.Context, servers []scorer.ScoredServer) error {
	if len(servers) == 0 {
		return nil
	}

	payload := buildDiscordPayload(servers)
	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshalling discord payload: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, n.cfg.WebhookURL, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("creating discord request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := n.client.Do(req)
	if err != nil {
		return fmt.Errorf("posting to discord: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 300 {
		return fmt.Errorf("discord returned status %d", resp.StatusCode)
	}
	return nil
}

func buildDiscordPayload(servers []scorer.ScoredServer) map[string]interface{} {
	var lines []string
	for _, s := range servers {
		lines = append(lines, fmt.Sprintf("**#%d** %s | %dGB | €%.2f | Score: %.1f | %s",
			s.Server.ID, s.Server.CPU, s.Server.RAMSize, s.Server.Price, s.Score, s.Server.Datacenter))
	}

	return map[string]interface{}{
		"embeds": []map[string]interface{}{
			{
				"title":       fmt.Sprintf("Foundry Scout: %d servers found", len(servers)),
				"description": strings.Join(lines, "\n"),
				"color":       3066993, // green
			},
		},
	}
}
