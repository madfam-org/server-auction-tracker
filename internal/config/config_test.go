package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoadDefaults(t *testing.T) {
	cfg, err := Load("")
	require.NoError(t, err)

	assert.Equal(t, 64, cfg.Filters.MinRAMGB)
	assert.Equal(t, 8, cfg.Filters.MinCPUCores)
	assert.Equal(t, 2, cfg.Filters.MinDrives)
	assert.Equal(t, 512, cfg.Filters.MinDriveSizeGB)
	assert.Equal(t, 90.0, cfg.Filters.MaxPriceEUR)
	assert.Equal(t, "HEL1", cfg.Filters.DatacenterPrefix)

	assert.InDelta(t, 0.30, cfg.Scoring.CPUWeight, 0.001)
	assert.InDelta(t, 0.25, cfg.Scoring.RAMWeight, 0.001)
	assert.InDelta(t, 0.20, cfg.Scoring.StorageWeight, 0.001)

	assert.Equal(t, "foundry-scout.db", cfg.Database.Path)
	assert.Equal(t, "info", cfg.LogLevel)

	// Watch defaults
	assert.Equal(t, "5m", cfg.Watch.Interval)
	assert.Equal(t, "1h", cfg.Watch.DedupWindow)

	// Notify defaults
	assert.Equal(t, "enclii", cfg.Notify.Type)
	assert.Equal(t, "http://switchyard-api.enclii.svc.cluster.local", cfg.Notify.Enclii.APIURL)
	assert.Equal(t, "foundry-scout", cfg.Notify.Enclii.ProjectSlug)

	// Cluster defaults
	assert.Equal(t, 2, cfg.Cluster.Nodes)

	// Order defaults
	assert.False(t, cfg.Order.Enabled)
	assert.Equal(t, 90.0, cfg.Order.MinScore)
	assert.Equal(t, 80.0, cfg.Order.MaxPriceEUR)
	assert.True(t, cfg.Order.RequireApproval)
}

func TestLoadFromFile(t *testing.T) {
	content := `
filters:
  min_ram_gb: 128
  min_cpu_cores: 16
  max_price_eur: 120
scoring:
  cpu_weight: 0.40
database:
  path: "test.db"
log_level: "debug"
`
	dir := t.TempDir()
	path := filepath.Join(dir, "scout.yaml")
	require.NoError(t, os.WriteFile(path, []byte(content), 0o644))

	cfg, err := Load(path)
	require.NoError(t, err)

	assert.Equal(t, 128, cfg.Filters.MinRAMGB)
	assert.Equal(t, 16, cfg.Filters.MinCPUCores)
	assert.Equal(t, 120.0, cfg.Filters.MaxPriceEUR)
	assert.InDelta(t, 0.40, cfg.Scoring.CPUWeight, 0.001)
	assert.Equal(t, "test.db", cfg.Database.Path)
	assert.Equal(t, "debug", cfg.LogLevel)
}

func TestLoadWithNewSections(t *testing.T) {
	content := `
watch:
  interval: "10m"
  dedup_window: "2h"
notify:
  type: "slack"
  slack:
    webhook_url: "https://hooks.slack.com/test"
  enclii:
    api_url: "http://custom-switchyard:8080"
    project_slug: "test-project"
    webhook_secret: "secret123"
cluster:
  cpu_millicores: 12000
  cpu_requested: 10460
  ram_gb: 64
  ram_requested_gb: 25
  disk_gb: 98
  disk_used_gb: 77
  nodes: 2
order:
  enabled: true
  robot_url: "https://robot-ws.your-server.de"
  robot_user: "testuser"
  robot_password: "testpass"
  min_score: 85
  max_price_eur: 70
  require_approval: false
`
	dir := t.TempDir()
	path := filepath.Join(dir, "scout.yaml")
	require.NoError(t, os.WriteFile(path, []byte(content), 0o644))

	cfg, err := Load(path)
	require.NoError(t, err)

	assert.Equal(t, "10m", cfg.Watch.Interval)
	assert.Equal(t, "2h", cfg.Watch.DedupWindow)

	assert.Equal(t, "slack", cfg.Notify.Type)
	assert.Equal(t, "https://hooks.slack.com/test", cfg.Notify.Slack.WebhookURL)
	assert.Equal(t, "http://custom-switchyard:8080", cfg.Notify.Enclii.APIURL)
	assert.Equal(t, "secret123", cfg.Notify.Enclii.WebhookSecret)

	assert.Equal(t, 12000, cfg.Cluster.CPUMillicores)
	assert.Equal(t, 10460, cfg.Cluster.CPURequested)
	assert.Equal(t, 64, cfg.Cluster.RAMGB)
	assert.Equal(t, 25, cfg.Cluster.RAMRequestedGB)
	assert.Equal(t, 98, cfg.Cluster.DiskGB)
	assert.Equal(t, 77, cfg.Cluster.DiskUsedGB)
	assert.Equal(t, 2, cfg.Cluster.Nodes)

	assert.True(t, cfg.Order.Enabled)
	assert.Equal(t, "testuser", cfg.Order.RobotUser)
	assert.Equal(t, 85.0, cfg.Order.MinScore)
	assert.Equal(t, 70.0, cfg.Order.MaxPriceEUR)
	assert.False(t, cfg.Order.RequireApproval)
}

func TestLoadInvalidPath(t *testing.T) {
	_, err := Load("/nonexistent/scout.yaml")
	assert.Error(t, err)
}
