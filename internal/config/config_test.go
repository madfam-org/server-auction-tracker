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

func TestLoadInvalidPath(t *testing.T) {
	_, err := Load("/nonexistent/scout.yaml")
	assert.Error(t, err)
}
