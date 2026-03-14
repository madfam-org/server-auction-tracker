package main

import (
	"bytes"
	"os"
	"testing"

	"github.com/madfam-org/server-auction-tracker/internal/scanner"
	"github.com/madfam-org/server-auction-tracker/internal/scorer"
	"github.com/stretchr/testify/assert"
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

	w.Close()
	os.Stdout = old
	var buf bytes.Buffer
	buf.ReadFrom(r)
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
}
