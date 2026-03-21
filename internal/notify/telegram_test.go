package notify

import (
	"context"
	"testing"

	"github.com/madfam-org/server-auction-tracker/internal/config"
	"github.com/madfam-org/server-auction-tracker/internal/scanner"
	"github.com/madfam-org/server-auction-tracker/internal/scorer"
	"github.com/stretchr/testify/assert"
)

func TestTelegramFormatMessage(t *testing.T) {
	servers := []scorer.ScoredServer{
		{
			Server: scanner.Server{ID: 1, CPU: "AMD Ryzen 5 3600", RAMSize: 64, Price: 39.00, Datacenter: "HEL1"},
			Score:  85.0,
		},
		{
			Server: scanner.Server{ID: 2, CPU: "Intel Core i7-13700", RAMSize: 128, Price: 72.00, Datacenter: "FSN1"},
			Score:  91.0,
		},
	}

	msg := formatTelegramMessage(servers)
	assert.Contains(t, msg, "2 server(s) found")
	assert.Contains(t, msg, "AMD Ryzen 5 3600")
	assert.Contains(t, msg, "39.00")
	assert.Contains(t, msg, "Intel Core i7-13700")
}

func TestTelegramNotifierEmpty(t *testing.T) {
	n := NewTelegramNotifier(config.TelegramConfig{BotToken: "x", ChatID: "y"})
	err := n.Notify(context.Background(), nil)
	assert.NoError(t, err)
}

func TestTelegramMessageTruncation(t *testing.T) {
	servers := make([]scorer.ScoredServer, 8)
	for i := range servers {
		servers[i] = scorer.ScoredServer{
			Server: scanner.Server{ID: i + 1, CPU: "CPU", RAMSize: 64, Price: 40.00, Datacenter: "HEL1"},
			Score:  80.0,
		}
	}
	msg := formatTelegramMessage(servers)
	assert.Contains(t, msg, "and 3 more")
}
