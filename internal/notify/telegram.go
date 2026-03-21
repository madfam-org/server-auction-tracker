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

// TelegramNotifier sends messages via the Telegram Bot API.
type TelegramNotifier struct {
	botToken string
	chatID   string
	client   *http.Client
}

// NewTelegramNotifier creates a notifier for Telegram.
func NewTelegramNotifier(cfg config.TelegramConfig) *TelegramNotifier {
	return &TelegramNotifier{
		botToken: cfg.BotToken,
		chatID:   cfg.ChatID,
		client:   &http.Client{Timeout: 10 * time.Second},
	}
}

func (t *TelegramNotifier) Notify(ctx context.Context, servers []scorer.ScoredServer) error {
	if len(servers) == 0 {
		return nil
	}

	text := formatTelegramMessage(servers)

	payload := map[string]interface{}{
		"chat_id":    t.chatID,
		"text":       text,
		"parse_mode": "HTML",
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshalling telegram payload: %w", err)
	}

	url := fmt.Sprintf("https://api.telegram.org/bot%s/sendMessage", t.botToken)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("creating telegram request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := t.client.Do(req)
	if err != nil {
		return fmt.Errorf("telegram API call failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		return fmt.Errorf("telegram API returned status %d", resp.StatusCode)
	}
	return nil
}

func formatTelegramMessage(servers []scorer.ScoredServer) string {
	var b strings.Builder
	b.WriteString(fmt.Sprintf("<b>Deal Sniper: %d server(s) found</b>\n\n", len(servers)))

	for i := range servers {
		if i >= 5 {
			b.WriteString(fmt.Sprintf("\n... and %d more", len(servers)-5))
			break
		}
		b.WriteString(fmt.Sprintf(
			"<b>#%d</b> — %s\n  EUR %.2f | Score %.1f | %dGB RAM | %s\n\n",
			servers[i].Server.ID, servers[i].Server.CPU,
			servers[i].Server.Price, servers[i].Score,
			servers[i].Server.RAMSize, servers[i].Server.Datacenter,
		))
	}
	return b.String()
}
