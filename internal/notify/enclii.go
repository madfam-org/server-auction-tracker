package notify

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"time"

	"github.com/madfam-org/server-auction-tracker/internal/config"
	"github.com/madfam-org/server-auction-tracker/internal/scorer"
)

type encliiPayload struct {
	EventType string           `json:"event_type"`
	Project   string           `json:"project"`
	Timestamp string           `json:"timestamp"`
	Data      encliiServerData `json:"data"`
}

type encliiServerData struct {
	Count    int                   `json:"count"`
	TopScore float64               `json:"top_score"`
	Servers  []encliiServerSummary `json:"servers"`
}

type encliiServerSummary struct {
	ID         int     `json:"id"`
	CPU        string  `json:"cpu"`
	RAMGB      int     `json:"ram_gb"`
	PriceEUR   float64 `json:"price_eur"`
	Score      float64 `json:"score"`
	Datacenter string  `json:"datacenter"`
}

// EncliiNotifier posts lifecycle events to the enclii Switchyard API.
type EncliiNotifier struct {
	cfg    config.EncliiConfig
	client *http.Client
}

func NewEncliiNotifier(cfg config.EncliiConfig) *EncliiNotifier {
	return &EncliiNotifier{
		cfg:    cfg,
		client: &http.Client{Timeout: 10 * time.Second},
	}
}

func (n *EncliiNotifier) Notify(ctx context.Context, servers []scorer.ScoredServer) error {
	if len(servers) == 0 {
		return nil
	}

	summaries := make([]encliiServerSummary, 0, len(servers))
	for i := range servers {
		summaries = append(summaries, encliiServerSummary{
			ID:         servers[i].Server.ID,
			CPU:        servers[i].Server.CPU,
			RAMGB:      servers[i].Server.RAMSize,
			PriceEUR:   servers[i].Server.Price,
			Score:      servers[i].Score,
			Datacenter: servers[i].Server.Datacenter,
		})
	}

	payload := encliiPayload{
		EventType: "auction.servers_found",
		Project:   n.cfg.ProjectSlug,
		Timestamp: time.Now().UTC().Format(time.RFC3339),
		Data: encliiServerData{
			Count:    len(servers),
			TopScore: servers[0].Score,
			Servers:  summaries,
		},
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshalling enclii payload: %w", err)
	}

	url := n.cfg.APIURL + "/v1/callbacks/lifecycle-event"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("creating enclii request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "foundry-scout/1.0")

	// Use Bearer token auth — env var takes precedence over config
	token := os.Getenv("SCOUT_NOTIFY_ENCLII_CALLBACK_TOKEN")
	if token == "" {
		token = n.cfg.WebhookSecret
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}

	resp, err := n.client.Do(req)
	if err != nil {
		return fmt.Errorf("posting to enclii: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 300 {
		return fmt.Errorf("enclii returned status %d", resp.StatusCode)
	}
	return nil
}
