package notify

import (
	"fmt"

	"github.com/madfam-org/server-auction-tracker/internal/config"
)

// NewNotifier creates a Notifier based on the configured type.
// If Channels is populated, builds a MultiNotifier. Otherwise falls back to Type.
func NewNotifier(cfg config.Notify) (Notifier, error) {
	if len(cfg.Channels) > 0 {
		return buildMultiNotifier(cfg)
	}
	return buildSingleNotifier(cfg.Type, cfg)
}

func buildMultiNotifier(cfg config.Notify) (Notifier, error) {
	var notifiers []Notifier
	for _, ch := range cfg.Channels {
		n, err := buildSingleNotifier(ch.Type, cfg)
		if err != nil {
			return nil, fmt.Errorf("channel %q: %w", ch.Type, err)
		}
		notifiers = append(notifiers, n)
	}
	if len(notifiers) == 0 {
		return nil, fmt.Errorf("no valid channels configured")
	}
	if len(notifiers) == 1 {
		return notifiers[0], nil
	}
	return NewMultiNotifier(notifiers...), nil
}

func buildSingleNotifier(typ string, cfg config.Notify) (Notifier, error) {
	switch typ {
	case "enclii":
		if cfg.Enclii.APIURL == "" {
			return nil, fmt.Errorf("enclii api_url is required")
		}
		return NewEncliiNotifier(cfg.Enclii), nil
	case "slack":
		if cfg.Slack.WebhookURL == "" {
			return nil, fmt.Errorf("slack webhook_url is required")
		}
		return NewSlackNotifier(cfg.Slack), nil
	case "discord":
		if cfg.Discord.WebhookURL == "" {
			return nil, fmt.Errorf("discord webhook_url is required")
		}
		return NewDiscordNotifier(cfg.Discord), nil
	case "webhook":
		if cfg.Webhook.URL == "" {
			return nil, fmt.Errorf("webhook url is required")
		}
		return NewWebhookNotifier(cfg.Webhook), nil
	case "telegram":
		if cfg.Telegram.BotToken == "" || cfg.Telegram.ChatID == "" {
			return nil, fmt.Errorf("telegram bot_token and chat_id are required")
		}
		return NewTelegramNotifier(cfg.Telegram), nil
	default:
		return nil, fmt.Errorf("unknown notifier type: %q", typ)
	}
}
