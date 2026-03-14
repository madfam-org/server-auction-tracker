package store

import (
	"path/filepath"
	"testing"

	"github.com/madfam-org/server-auction-tracker/internal/scanner"
	"github.com/madfam-org/server-auction-tracker/internal/scorer"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupTestDB(t *testing.T) *SQLiteStore {
	t.Helper()
	dbPath := filepath.Join(t.TempDir(), "test.db")
	s, err := NewSQLite(dbPath)
	require.NoError(t, err)
	require.NoError(t, s.Init())
	t.Cleanup(func() { s.Close() })
	return s
}

func TestInitSchema(t *testing.T) {
	s := setupTestDB(t)
	// Init again should be idempotent
	require.NoError(t, s.Init())
	_ = s
}

func TestSaveScanAndGetHistory(t *testing.T) {
	s := setupTestDB(t)

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
				Price:         39.00,
			},
			Score: 85.5,
		},
		{
			Server: scanner.Server{
				ID:             1002,
				CPU:            "Intel Core i5-13500",
				RAMSize:        64,
				TotalStorageTB: 2.0,
				NVMeCount:      2,
				DriveCount:     2,
				Datacenter:     "FSN1-DC14",
				Price:         55.00,
			},
			Score: 72.3,
		},
	}

	require.NoError(t, s.SaveScan(servers))

	// Get history for Ryzen
	records, err := s.GetHistory("Ryzen", 10)
	require.NoError(t, err)
	assert.Len(t, records, 1)
	assert.Equal(t, 1001, records[0].ServerID)
	assert.Equal(t, "AMD Ryzen 5 3600", records[0].CPU)
	assert.Equal(t, 39.00, records[0].Price)
	assert.Equal(t, 85.5, records[0].Score)

	// Get all history
	records, err = s.GetHistory("", 10)
	require.NoError(t, err)
	assert.Len(t, records, 2)
}

func TestGetStats(t *testing.T) {
	s := setupTestDB(t)

	// Save two scans for the same CPU at different prices
	scan1 := []scorer.ScoredServer{{
		Server: scanner.Server{
			ID: 1, CPU: "AMD Ryzen 5 3600", RAMSize: 64,
			TotalStorageTB: 1.0, NVMeCount: 2, DriveCount: 2,
			Datacenter: "HEL1", Price: 35.00,
		},
		Score: 80,
	}}
	scan2 := []scorer.ScoredServer{{
		Server: scanner.Server{
			ID: 2, CPU: "AMD Ryzen 5 3600", RAMSize: 64,
			TotalStorageTB: 1.0, NVMeCount: 2, DriveCount: 2,
			Datacenter: "HEL1", Price: 45.00,
		},
		Score: 70,
	}}

	require.NoError(t, s.SaveScan(scan1))
	require.NoError(t, s.SaveScan(scan2))

	stats, err := s.GetStats("Ryzen 5 3600")
	require.NoError(t, err)
	require.NotNil(t, stats)

	assert.Equal(t, 2, stats.Count)
	assert.Equal(t, 35.00, stats.MinPrice)
	assert.Equal(t, 45.00, stats.MaxPrice)
	assert.InDelta(t, 40.00, stats.AvgPrice, 0.01)
}

func TestGetStatsNoResults(t *testing.T) {
	s := setupTestDB(t)

	stats, err := s.GetStats("Nonexistent CPU")
	require.NoError(t, err)
	assert.Nil(t, stats)
}
