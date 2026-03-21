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
	cfg := config.Notify{Type: "carrier_pigeon"}
	_, err := NewNotifier(cfg)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unknown notifier type")
}

func TestNewNotifierWebhook(t *testing.T) {
	cfg := config.Notify{
		Type:    "webhook",
		Webhook: config.WebhookConfig{URL: "https://example.com/hook"},
	}
	n, err := NewNotifier(cfg)
	require.NoError(t, err)
	assert.IsType(t, &WebhookNotifier{}, n)
}

func TestNewNotifierTelegram(t *testing.T) {
	cfg := config.Notify{
		Type:     "telegram",
		Telegram: config.TelegramConfig{BotToken: "123:ABC", ChatID: "-100123"},
	}
	n, err := NewNotifier(cfg)
	require.NoError(t, err)
	assert.IsType(t, &TelegramNotifier{}, n)
}

func TestNewNotifierTelegramMissing(t *testing.T) {
	cfg := config.Notify{Type: "telegram"}
	_, err := NewNotifier(cfg)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "bot_token and chat_id")
}

func TestNewNotifierMultiChannel(t *testing.T) {
	cfg := config.Notify{
		Channels: []config.NotifyChannel{
			{Type: "slack"},
			{Type: "discord"},
		},
		Slack:   config.SlackConfig{WebhookURL: "https://hooks.slack.com/test"},
		Discord: config.DiscordConfig{WebhookURL: "https://discord.com/api/webhooks/test"},
	}
	n, err := NewNotifier(cfg)
	require.NoError(t, err)
	assert.IsType(t, &MultiNotifier{}, n)
}

func TestNewNotifierSingleChannel(t *testing.T) {
	cfg := config.Notify{
		Channels: []config.NotifyChannel{
			{Type: "slack"},
		},
		Slack: config.SlackConfig{WebhookURL: "https://hooks.slack.com/test"},
	}
	n, err := NewNotifier(cfg)
	require.NoError(t, err)
	// Single channel should unwrap to the underlying notifier
	assert.IsType(t, &SlackNotifier{}, n)
}

func TestBackwardCompatSingleType(t *testing.T) {
	// Old-style config with just Type (no Channels) should still work
	cfg := config.Notify{
		Type:   "enclii",
		Enclii: config.EncliiConfig{APIURL: "http://switchyard.local"},
	}
	n, err := NewNotifier(cfg)
	require.NoError(t, err)
	assert.IsType(t, &EncliiNotifier{}, n)
}
