package notify

import (
	"fmt"

	"github.com/madfam-org/server-auction-tracker/internal/config"
)

// NewNotifier creates a Notifier based on the configured type.
func NewNotifier(cfg config.Notify) (Notifier, error) {
	switch cfg.Type {
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
	default:
		return nil, fmt.Errorf("unknown notifier type: %q", cfg.Type)
	}
}
