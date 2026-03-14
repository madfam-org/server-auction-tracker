package notify

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
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
	for _, s := range servers {
		summaries = append(summaries, encliiServerSummary{
			ID:         s.Server.ID,
			CPU:        s.Server.CPU,
			RAMGB:      s.Server.RAMSize,
			PriceEUR:   s.Server.Price,
			Score:      s.Score,
			Datacenter: s.Server.Datacenter,
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

	if n.cfg.WebhookSecret != "" {
		sig := computeHMAC(body, []byte(n.cfg.WebhookSecret))
		req.Header.Set("X-Webhook-Signature", sig)
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

func computeHMAC(message, key []byte) string {
	mac := hmac.New(sha256.New, key)
	mac.Write(message)
	return "sha256=" + hex.EncodeToString(mac.Sum(nil))
}
