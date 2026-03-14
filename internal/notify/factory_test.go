package notify

import (
	"testing"

	"github.com/madfam-org/server-auction-tracker/internal/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewNotifierEnclii(t *testing.T) {
	cfg := config.Notify{
		Type:   "enclii",
		Enclii: config.EncliiConfig{APIURL: "http://switchyard.local"},
	}
	n, err := NewNotifier(cfg)
	require.NoError(t, err)
	assert.IsType(t, &EncliiNotifier{}, n)
}

func TestNewNotifierSlack(t *testing.T) {
	cfg := config.Notify{
		Type:  "slack",
		Slack: config.SlackConfig{WebhookURL: "https://hooks.slack.com/test"},
	}
	n, err := NewNotifier(cfg)
	require.NoError(t, err)
	assert.IsType(t, &SlackNotifier{}, n)
}

func TestNewNotifierDiscord(t *testing.T) {
	cfg := config.Notify{
		Type:    "discord",
		Discord: config.DiscordConfig{WebhookURL: "https://discord.com/api/webhooks/test"},
	}
	n, err := NewNotifier(cfg)
	require.NoError(t, err)
	assert.IsType(t, &DiscordNotifier{}, n)
}

func TestNewNotifierSlackMissingURL(t *testing.T) {
	cfg := config.Notify{Type: "slack"}
	_, err := NewNotifier(cfg)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "webhook_url is required")
}

func TestNewNotifierUnknownType(t *testing.T) {
	cfg := config.Notify{Type: "telegram"}
	_, err := NewNotifier(cfg)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unknown notifier type")
}
