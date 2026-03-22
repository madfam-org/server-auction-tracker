package config

import (
	"fmt"

	"github.com/spf13/viper"
)

type Config struct {
	Filters  Filters  `mapstructure:"filters"`
	Scoring  Scoring  `mapstructure:"scoring"`
	Database Database `mapstructure:"database"`
	LogLevel string   `mapstructure:"log_level"`
	Watch    Watch    `mapstructure:"watch"`
	Notify   Notify   `mapstructure:"notify"`
	Cluster  Cluster  `mapstructure:"cluster"`
	Order    Order    `mapstructure:"order"`
	Digest   Digest   `mapstructure:"digest"`
	Auth     Auth     `mapstructure:"auth"`
}

type Auth struct {
	JanuaIssuer    string   `mapstructure:"janua_issuer"`
	JanuaJWKSURL   string   `mapstructure:"janua_jwks_url"`
	JanuaAudience  string   `mapstructure:"janua_audience"`
	AllowedDomains []string `mapstructure:"allowed_domains"`
	AllowedRoles   []string `mapstructure:"allowed_roles"`
	JWKSCacheTTL   string   `mapstructure:"jwks_cache_ttl"`
}

type Filters struct {
	MinRAMGB         int     `mapstructure:"min_ram_gb"`
	MinCPUCores      int     `mapstructure:"min_cpu_cores"`
	MinDrives        int     `mapstructure:"min_drives"`
	MinDriveSizeGB   int     `mapstructure:"min_drive_size_gb"`
	MaxPriceEUR      float64 `mapstructure:"max_price_eur"`
	DatacenterPrefix string  `mapstructure:"datacenter_prefix"`
}

type Scoring struct {
	CPUWeight       float64 `mapstructure:"cpu_weight"`
	RAMWeight       float64 `mapstructure:"ram_weight"`
	StorageWeight   float64 `mapstructure:"storage_weight"`
	NVMeWeight      float64 `mapstructure:"nvme_weight"`
	CPUGenWeight    float64 `mapstructure:"cpu_gen_weight"`
	LocalityWeight  float64 `mapstructure:"locality_weight"`
	BenchmarkWeight float64 `mapstructure:"benchmark_weight"`
	ECCWeight       float64 `mapstructure:"ecc_weight"`
}

type Database struct {
	Path          string `mapstructure:"path"`
	RetentionDays int    `mapstructure:"retention_days"`
}

type Watch struct {
	Interval    string `mapstructure:"interval"`
	DedupWindow string `mapstructure:"dedup_window"`
}

type WebhookConfig struct {
	URL     string            `mapstructure:"url"`
	Headers map[string]string `mapstructure:"headers"`
}

type TelegramConfig struct {
	BotToken string `mapstructure:"bot_token"`
	ChatID   string `mapstructure:"chat_id"`
}

type NotifyChannel struct {
	Type string `mapstructure:"type"` // "enclii", "slack", "discord", "webhook", "telegram"
}

type Notify struct {
	Type     string          `mapstructure:"type"`
	Channels []NotifyChannel `mapstructure:"channels"`
	MinScore float64         `mapstructure:"min_score"`
	Enclii   EncliiConfig    `mapstructure:"enclii"`
	Slack    SlackConfig     `mapstructure:"slack"`
	Discord  DiscordConfig   `mapstructure:"discord"`
	Webhook  WebhookConfig   `mapstructure:"webhook"`
	Telegram TelegramConfig  `mapstructure:"telegram"`
}

type EncliiConfig struct {
	APIURL        string `mapstructure:"api_url"`
	ProjectSlug   string `mapstructure:"project_slug"`
	WebhookSecret string `mapstructure:"webhook_secret"`
}

type SlackConfig struct {
	WebhookURL string `mapstructure:"webhook_url"`
}

type DiscordConfig struct {
	WebhookURL string `mapstructure:"webhook_url"`
}

type Cluster struct {
	CPUMillicores  int     `mapstructure:"cpu_millicores"`
	CPURequested   int     `mapstructure:"cpu_requested"`
	RAMGB          int     `mapstructure:"ram_gb"`
	RAMRequestedGB int     `mapstructure:"ram_requested_gb"`
	DiskGB         int     `mapstructure:"disk_gb"`
	DiskUsedGB     int     `mapstructure:"disk_used_gb"`
	Nodes          int     `mapstructure:"nodes"`
}

type Digest struct {
	Enabled  bool    `mapstructure:"enabled"`
	Schedule string  `mapstructure:"schedule"` // "daily" or "weekly"
	TopN     int     `mapstructure:"top_n"`
	MinScore float64 `mapstructure:"min_score"`
}

type Order struct {
	Enabled         bool    `mapstructure:"enabled"`
	RobotURL        string  `mapstructure:"robot_url"`
	RobotUser       string  `mapstructure:"robot_user"`
	RobotPassword   string  `mapstructure:"robot_password"`
	MinScore        float64 `mapstructure:"min_score"`
	MaxPriceEUR     float64 `mapstructure:"max_price_eur"`
	RequireApproval bool    `mapstructure:"require_approval"`
}

func Load(path string) (*Config, error) {
	v := viper.New()

	v.SetDefault("filters.min_ram_gb", 64)
	v.SetDefault("filters.min_cpu_cores", 8)
	v.SetDefault("filters.min_drives", 2)
	v.SetDefault("filters.min_drive_size_gb", 512)
	v.SetDefault("filters.max_price_eur", 90)
	v.SetDefault("filters.datacenter_prefix", "HEL1")

	v.SetDefault("scoring.cpu_weight", 0.30)
	v.SetDefault("scoring.ram_weight", 0.25)
	v.SetDefault("scoring.storage_weight", 0.20)
	v.SetDefault("scoring.nvme_weight", 0.10)
	v.SetDefault("scoring.cpu_gen_weight", 0.10)
	v.SetDefault("scoring.locality_weight", 0.05)
	v.SetDefault("scoring.benchmark_weight", 0.0)
	v.SetDefault("scoring.ecc_weight", 0.0)

	v.SetDefault("database.path", "foundry-scout.db")
	v.SetDefault("database.retention_days", 90)
	v.SetDefault("log_level", "info")

	v.SetDefault("watch.interval", "5m")
	v.SetDefault("watch.dedup_window", "1h")

	v.SetDefault("notify.type", "enclii")
	v.SetDefault("notify.enclii.api_url", "http://switchyard-api.enclii.svc.cluster.local")
	v.SetDefault("notify.enclii.project_slug", "foundry-scout")

	v.SetDefault("cluster.nodes", 2)

	v.SetDefault("order.enabled", false)
	v.SetDefault("order.robot_url", "https://robot-ws.your-server.de")
	v.SetDefault("order.min_score", 90)
	v.SetDefault("order.max_price_eur", 80)
	v.SetDefault("order.require_approval", true)

	v.SetDefault("digest.enabled", false)
	v.SetDefault("digest.schedule", "daily")
	v.SetDefault("digest.top_n", 5)
	v.SetDefault("digest.min_score", 0)

	v.SetDefault("auth.janua_issuer", "https://auth.madfam.io")
	v.SetDefault("auth.janua_jwks_url", "https://auth.madfam.io/.well-known/jwks.json")
	v.SetDefault("auth.janua_audience", "deal-sniper")
	v.SetDefault("auth.allowed_domains", []string{"@madfam.io"})
	v.SetDefault("auth.allowed_roles", []string{"superadmin", "admin", "operator"})
	v.SetDefault("auth.jwks_cache_ttl", "1h")

	if path != "" {
		v.SetConfigFile(path)
		if err := v.ReadInConfig(); err != nil {
			return nil, fmt.Errorf("reading config %s: %w", path, err)
		}
	}

	var cfg Config
	if err := v.Unmarshal(&cfg); err != nil {
		return nil, fmt.Errorf("unmarshalling config: %w", err)
	}

	return &cfg, nil
}
