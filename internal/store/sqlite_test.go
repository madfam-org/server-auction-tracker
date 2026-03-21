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

func TestGetByServerID(t *testing.T) {
	s := setupTestDB(t)

	servers := []scorer.ScoredServer{{
		Server: scanner.Server{
			ID: 2001, CPU: "AMD EPYC 7443P", RAMSize: 128,
			TotalStorageTB: 4.0, NVMeCount: 2, DriveCount: 4,
			Datacenter: "HEL1-DC7", Price: 75.00,
		},
		Score: 92.1,
	}}
	require.NoError(t, s.SaveScan(servers))

	rec, err := s.GetByServerID(2001)
	require.NoError(t, err)
	require.NotNil(t, rec)
	assert.Equal(t, 2001, rec.ServerID)
	assert.Equal(t, "AMD EPYC 7443P", rec.CPU)
	assert.Equal(t, 75.00, rec.Price)

	// Not found
	rec, err = s.GetByServerID(9999)
	require.NoError(t, err)
	assert.Nil(t, rec)
}

func TestGetOrderAttempts(t *testing.T) {
	s := setupTestDB(t)

	require.NoError(t, s.SaveOrderAttempt(1001, 85.0, 39.00, true, "order placed"))
	require.NoError(t, s.SaveOrderAttempt(1002, 72.0, 55.00, false, "price too high"))

	orders, err := s.GetOrderAttempts(10)
	require.NoError(t, err)
	assert.Len(t, orders, 2)

	// Verify both records exist (order may vary due to same-second timestamps)
	serverIDs := []int{orders[0].ServerID, orders[1].ServerID}
	assert.Contains(t, serverIDs, 1001)
	assert.Contains(t, serverIDs, 1002)

	// Test limit
	orders, err = s.GetOrderAttempts(1)
	require.NoError(t, err)
	assert.Len(t, orders, 1)
}

func TestPruneOldScans(t *testing.T) {
	s := setupTestDB(t)

	// Insert records via SaveScan (they get current timestamp)
	servers := []scorer.ScoredServer{{
		Server: scanner.Server{
			ID: 3001, CPU: "Intel Xeon", RAMSize: 64,
			TotalStorageTB: 1.0, NVMeCount: 1, DriveCount: 2,
			Datacenter: "FSN1", Price: 40.00,
		},
		Score: 70,
	}}
	require.NoError(t, s.SaveScan(servers))

	// Pruning with a long retention should not delete anything
	pruned, err := s.PruneOldScans(90)
	require.NoError(t, err)
	assert.Equal(t, int64(0), pruned)

	// Verify records still exist
	records, err := s.GetHistory("", 10)
	require.NoError(t, err)
	assert.Len(t, records, 1)

	// Zero retention should be a no-op
	pruned, err = s.PruneOldScans(0)
	require.NoError(t, err)
	assert.Equal(t, int64(0), pruned)
}
