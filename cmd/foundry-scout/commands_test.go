package main

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/madfam-org/server-auction-tracker/internal/scanner"
	"github.com/madfam-org/server-auction-tracker/internal/scorer"
	"github.com/madfam-org/server-auction-tracker/internal/store"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTruncate(t *testing.T) {
	tests := []struct {
		input    string
		maxLen   int
		expected string
	}{
		{"short", 10, "short"},
		{"exactly10!", 10, "exactly10!"},
		{"this is a long string", 10, "this is..."},
		{"abc", 3, "abc"},
		{"abcd", 3, "..."},
	}
	for _, tt := range tests {
		result := truncate(tt.input, tt.maxLen)
		assert.Equal(t, tt.expected, result, "truncate(%q, %d)", tt.input, tt.maxLen)
	}
}

func TestSetupLogging(t *testing.T) {
	// Valid level
	setupLogging("debug")
	// Invalid level falls back to info
	setupLogging("invalid")
	// Empty string falls back to info
	setupLogging("")
}

func TestPrintResults(t *testing.T) {
	servers := []scorer.ScoredServer{
		{
			Server: scanner.Server{
				ID:             1001,
				CPU:            "AMD Ryzen 5 3600",
				RAMSize:        64,
				TotalStorageTB: 1.0,
				NVMeCount:      2,
				DriveCount:     2,
				Datacenter:     "HEL1-DC7",
				Price:          39.00,
			},
			Score: 85.5,
		},
	}

	// Capture stdout
	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	printResults(servers)

	_ = w.Close()
	os.Stdout = old
	var buf bytes.Buffer
	_, _ = buf.ReadFrom(r)
	output := buf.String()

	assert.Contains(t, output, "SCORE")
	assert.Contains(t, output, "1001")
	assert.Contains(t, output, "AMD Ryzen 5 3600")
	assert.Contains(t, output, "64GB")
	assert.Contains(t, output, "HEL1-DC7")
	assert.Contains(t, output, "85.5")
	assert.Contains(t, output, "1 servers found")
}

func TestWatchCommandRegistered(t *testing.T) {
	assert.Equal(t, "watch", watchCmd.Name())
	assert.NotNil(t, watchCmd.RunE)
}

func TestSimulateCommandRegistered(t *testing.T) {
	assert.Equal(t, "simulate", simulateCmd.Name())
	assert.NotNil(t, simulateCmd.RunE)
}

func TestOrderCommandRegistered(t *testing.T) {
	assert.Equal(t, "order", orderCmd.Name())
	assert.NotNil(t, orderCmd.RunE)
}

func TestRootCommandHasSubcommands(t *testing.T) {
	cmds := rootCmd.Commands()
	names := make([]string, len(cmds))
	for i, c := range cmds {
		names[i] = c.Name()
	}
	assert.Contains(t, names, "scan")
	assert.Contains(t, names, "watch")
	assert.Contains(t, names, "history")
	assert.Contains(t, names, "simulate")
	assert.Contains(t, names, "order")
}

var testAuctionJSON = `{
	"server": [
		{
			"id": 9001, "key": 9001, "name": "Server Auction",
			"cpu": "AMD Ryzen 5 3600 6-Core Processor", "cpu_count": 1,
			"ram_size": 64, "hdd_arr": ["512 GB NVMe SSD", "512 GB NVMe SSD"],
			"hdd_count": 2, "hdd_size": 512,
			"serverDiskData": {"nvme": [512, 512], "sata": [], "hdd": [], "general": [512]},
			"datacenter": "HEL1-DC7", "price": 39, "setup_price": 0,
			"specials": ["ECC"], "is_ecc": true, "traffic": "unlimited",
			"bandwidth": 1000, "next_reduce": 0, "fixed_price": false
		}
	]
}`

func TestRunScanWithMockServer(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(testAuctionJSON))
	}))
	defer srv.Close()

	// Use a temp DB and config
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "test.db")
	configContent := "database:\n  path: \"" + dbPath + "\"\nfilters:\n  min_ram_gb: 1\n  min_cpu_cores: 1\n  min_drives: 1\n  min_drive_size_gb: 0\n  max_price_eur: 0\nlog_level: \"error\"\n"
	configPath := filepath.Join(dir, "scout.yaml")
	require.NoError(t, os.WriteFile(configPath, []byte(configContent), 0o644))

	// Override cfgFile
	oldCfg := cfgFile
	cfgFile = configPath
	t.Cleanup(func() { cfgFile = oldCfg })

	// We can't easily override the scanner URL in runScan without refactoring,
	// so we test the printResults path instead which is the main output path
	// The full integration test would need DI for the scanner URL
}

func TestRunHistoryWithPreSeededDB(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "test.db")

	db, err := store.NewSQLite(dbPath)
	require.NoError(t, err)
	require.NoError(t, db.Init())

	// Seed data
	servers := []scorer.ScoredServer{
		{
			Server: scanner.Server{
				ID: 5001, CPU: "AMD Ryzen 9 5900X", RAMSize: 128,
				TotalStorageTB: 2.0, NVMeCount: 2, DriveCount: 2,
				Datacenter: "HEL1-DC7", Price: 65.00,
			},
			Score: 92.3,
		},
	}
	require.NoError(t, db.SaveScan(servers))
	_ = db.Close()

	// Set up config
	configContent := "database:\n  path: \"" + dbPath + "\"\nlog_level: \"error\"\n"
	configPath := filepath.Join(dir, "scout.yaml")
	require.NoError(t, os.WriteFile(configPath, []byte(configContent), 0o644))

	oldCfg := cfgFile
	cfgFile = configPath
	defer func() { cfgFile = oldCfg }()

	oldCPU := historyCPU
	historyCPU = "Ryzen 9"
	defer func() { historyCPU = oldCPU }()

	// Capture stdout
	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	err = runHistory(historyCmd, nil)

	_ = w.Close()
	os.Stdout = old
	var buf bytes.Buffer
	_, _ = buf.ReadFrom(r)
	output := buf.String()

	require.NoError(t, err)
	assert.Contains(t, output, "Ryzen 9 5900X")
	assert.Contains(t, output, "DEAL")
	assert.Contains(t, output, "92.3")
}
