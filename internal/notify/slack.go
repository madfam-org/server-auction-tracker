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

// SlackNotifier posts Block Kit messages to a Slack incoming webhook.
type SlackNotifier struct {
	cfg    config.SlackConfig
	client *http.Client
}

func NewSlackNotifier(cfg config.SlackConfig) *SlackNotifier {
	return &SlackNotifier{
		cfg:    cfg,
		client: &http.Client{Timeout: 10 * time.Second},
	}
}

func (n *SlackNotifier) Notify(ctx context.Context, servers []scorer.ScoredServer) error {
	if len(servers) == 0 {
		return nil
	}

	payload := buildSlackPayload(servers)
	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshalling slack payload: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, n.cfg.WebhookURL, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("creating slack request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := n.client.Do(req)
	if err != nil {
		return fmt.Errorf("posting to slack: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 300 {
		return fmt.Errorf("slack returned status %d", resp.StatusCode)
	}
	return nil
}

func buildSlackPayload(servers []scorer.ScoredServer) map[string]interface{} {
	var lines []string
	for _, s := range servers {
		lines = append(lines, fmt.Sprintf("• *#%d* %s | %dGB | €%.2f | Score: %.1f | %s",
			s.Server.ID, s.Server.CPU, s.Server.RAMSize, s.Server.Price, s.Score, s.Server.Datacenter))
	}

	return map[string]interface{}{
		"blocks": []map[string]interface{}{
			{
				"type": "header",
				"text": map[string]string{
					"type": "plain_text",
					"text": fmt.Sprintf("Foundry Scout: %d servers found", len(servers)),
				},
			},
			{
				"type": "section",
				"text": map[string]string{
					"type": "mrkdwn",
					"text": strings.Join(lines, "\n"),
				},
			},
		},
	}
}
