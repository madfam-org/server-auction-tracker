package order

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/madfam-org/server-auction-tracker/internal/config"
	"github.com/madfam-org/server-auction-tracker/internal/scanner"
)

// RobotClient implements Orderer using the Hetzner Robot API.
type RobotClient struct {
	cfg    config.Order
	client *http.Client
}

// NewRobotClient creates a new Hetzner Robot API client.
func NewRobotClient(cfg *config.Order) *RobotClient {
	return &RobotClient{
		cfg:    *cfg,
		client: &http.Client{Timeout: 30 * time.Second},
	}
}

func (r *RobotClient) CheckEligibility(server *scanner.Server, score float64, cfg *config.Order) *Check {
	check := &Check{Eligible: true}

	if !cfg.Enabled {
		check.Eligible = false
		check.Reasons = append(check.Reasons, "ordering is disabled in config (order.enabled = false)")
	}

	if score < cfg.MinScore {
		check.Eligible = false
		check.Reasons = append(check.Reasons, fmt.Sprintf("score %.1f below minimum %.1f", score, cfg.MinScore))
	}

	if cfg.MaxPriceEUR > 0 && server.Price > cfg.MaxPriceEUR {
		check.Eligible = false
		check.Reasons = append(check.Reasons, fmt.Sprintf("price €%.2f exceeds max €%.2f", server.Price, cfg.MaxPriceEUR))
	}

	return check
}

type robotOrderRequest struct {
	ProductID string `json:"product_id"`
}

type robotOrderResponse struct {
	Transaction struct {
		ID     string `json:"id"`
		Status string `json:"status"`
	} `json:"transaction"`
	Error *struct {
		Status  int    `json:"status"`
		Code    string `json:"code"`
		Message string `json:"message"`
	} `json:"error,omitempty"`
}

func (r *RobotClient) Order(ctx context.Context, serverID int) (*Result, error) {
	url := fmt.Sprintf("%s/order/server_market/transaction", r.cfg.RobotURL)

	reqBody := robotOrderRequest{
		ProductID: fmt.Sprintf("%d", serverID),
	}
	body, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("marshalling order request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("creating order request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.SetBasicAuth(r.cfg.RobotUser, r.cfg.RobotPassword)

	resp, err := r.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("posting order: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("reading order response: %w", err)
	}

	if resp.StatusCode >= 400 {
		return &Result{
			ServerID: serverID,
			Success:  false,
			Message:  fmt.Sprintf("Robot API error: HTTP %d — %s", resp.StatusCode, string(respBody)),
		}, nil
	}

	var orderResp robotOrderResponse
	if err := json.Unmarshal(respBody, &orderResp); err != nil {
		return nil, fmt.Errorf("parsing order response: %w", err)
	}

	if orderResp.Error != nil {
		return &Result{
			ServerID: serverID,
			Success:  false,
			Message:  fmt.Sprintf("Robot API error: %s — %s", orderResp.Error.Code, orderResp.Error.Message),
		}, nil
	}

	return &Result{
		ServerID: serverID,
		TransID:  orderResp.Transaction.ID,
		Success:  true,
		Message:  fmt.Sprintf("Order placed: transaction %s", orderResp.Transaction.ID),
	}, nil
}
