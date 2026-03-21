package store

import (
	"path/filepath"
	"testing"
	"time"

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
	t.Cleanup(func() { _ = s.Close() })
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

// --- Server Tracker Tests ---

func TestUpsertServerTracker(t *testing.T) {
	s := setupTestDB(t)

	entries := []ServerTrackerEntry{
		{ServerID: 1001, CPU: "AMD Ryzen 5 3600", Price: 39.00, Datacenter: "HEL1-DC7"},
		{ServerID: 1002, CPU: "Intel Core i5-13500", Price: 55.00, Datacenter: "FSN1-DC14"},
	}
	require.NoError(t, s.UpsertServerTracker(entries))

	// Verify tracker records
	tracker, err := s.GetServerTracker(1001)
	require.NoError(t, err)
	require.NotNil(t, tracker)
	assert.Equal(t, 1001, tracker.ServerID)
	assert.Equal(t, "AMD Ryzen 5 3600", tracker.CPU)
	assert.Equal(t, 39.00, tracker.Price)
	assert.Equal(t, "active", tracker.Status)

	// Upsert again with new price — should update price and last_seen
	entries2 := []ServerTrackerEntry{
		{ServerID: 1001, CPU: "AMD Ryzen 5 3600", Price: 35.00, Datacenter: "HEL1-DC7"},
	}
	require.NoError(t, s.UpsertServerTracker(entries2))

	tracker, err = s.GetServerTracker(1001)
	require.NoError(t, err)
	assert.Equal(t, 35.00, tracker.Price)
	assert.Equal(t, "active", tracker.Status)
}

func TestMarkSoldServers(t *testing.T) {
	s := setupTestDB(t)

	entries := []ServerTrackerEntry{
		{ServerID: 1001, CPU: "AMD Ryzen 5 3600", Price: 39.00, Datacenter: "HEL1"},
		{ServerID: 1002, CPU: "Intel i5-13500", Price: 55.00, Datacenter: "FSN1"},
	}
	require.NoError(t, s.UpsertServerTracker(entries))

	// First scan: only 1001 is active
	require.NoError(t, s.MarkSoldServers([]int{1001}))

	// After 1 miss, 1002 still active (threshold is 2)
	tracker, err := s.GetServerTracker(1002)
	require.NoError(t, err)
	assert.Equal(t, "active", tracker.Status)

	// Second scan: still only 1001 active
	require.NoError(t, s.UpsertServerTracker([]ServerTrackerEntry{
		{ServerID: 1001, CPU: "AMD Ryzen 5 3600", Price: 39.00, Datacenter: "HEL1"},
	}))
	require.NoError(t, s.MarkSoldServers([]int{1001}))

	// After 2 misses, 1002 should be sold
	tracker, err = s.GetServerTracker(1002)
	require.NoError(t, err)
	assert.Equal(t, "sold", tracker.Status)

	// 1001 should still be active
	tracker, err = s.GetServerTracker(1001)
	require.NoError(t, err)
	assert.Equal(t, "active", tracker.Status)
}

func TestGetAvgTimeOnMarket(t *testing.T) {
	s := setupTestDB(t)

	// No sold servers → 0
	avg, err := s.GetAvgTimeOnMarket("")
	require.NoError(t, err)
	assert.Equal(t, 0.0, avg)

	// Insert a sold server with known time range
	_, err = s.db.Exec(`
		INSERT INTO server_tracker (server_id, cpu, price, datacenter, first_seen, last_seen, status, consecutive_misses)
		VALUES (1001, 'AMD Ryzen 5 3600', 39.00, 'HEL1', datetime('now', '-24 hours'), datetime('now'), 'sold', 2)
	`)
	require.NoError(t, err)

	avg, err = s.GetAvgTimeOnMarket("Ryzen")
	require.NoError(t, err)
	assert.InDelta(t, 24.0, avg, 1.0) // ~24 hours with some tolerance
}

func TestServerReappears(t *testing.T) {
	s := setupTestDB(t)

	// Server appears
	entries := []ServerTrackerEntry{
		{ServerID: 2001, CPU: "AMD EPYC 7443P", Price: 75.00, Datacenter: "HEL1"},
	}
	require.NoError(t, s.UpsertServerTracker(entries))

	// Server disappears for 2 scans → sold
	require.NoError(t, s.MarkSoldServers(nil))
	require.NoError(t, s.MarkSoldServers(nil))

	tracker, err := s.GetServerTracker(2001)
	require.NoError(t, err)
	assert.Equal(t, "sold", tracker.Status)

	// Server reappears
	require.NoError(t, s.UpsertServerTracker(entries))

	tracker, err = s.GetServerTracker(2001)
	require.NoError(t, err)
	assert.Equal(t, "active", tracker.Status, "reappearing server should be active again")
}

func TestUpsertServerTrackerEmpty(t *testing.T) {
	s := setupTestDB(t)
	require.NoError(t, s.UpsertServerTracker(nil))
}

func TestMigrationAddsColumns(t *testing.T) {
	s := setupTestDB(t)
	// Init again should be idempotent even with new columns
	require.NoError(t, s.Init())

	// Verify we can save and retrieve records with new fields
	servers := []scorer.ScoredServer{{
		Server: scanner.Server{
			ID: 5001, CPU: "AMD Ryzen 9 5950X", RAMSize: 128,
			TotalStorageTB: 3.84, NVMeCount: 2, DriveCount: 2,
			Datacenter: "HEL1-DC7", Price: 72.50,
			IsECC: true, SetupPrice: 50.0, NextReduce: 48,
			FixedPrice: false, Bandwidth: 1000,
		},
		Score: 90.0,
	}}
	require.NoError(t, s.SaveScan(servers))

	rec, err := s.GetByServerID(5001)
	require.NoError(t, err)
	require.NotNil(t, rec)
	assert.True(t, rec.IsECC)
	assert.Equal(t, 50.0, rec.SetupPrice)
	assert.Equal(t, 48, rec.NextReduce)
	assert.False(t, rec.FixedPrice)
	assert.Equal(t, 1000, rec.Bandwidth)
}

func TestGetMarketAnalytics(t *testing.T) {
	s := setupTestDB(t)

	// Seed data
	servers := []scorer.ScoredServer{
		{
			Server: scanner.Server{
				ID: 1001, CPU: "AMD Ryzen 5 3600", RAMSize: 64,
				TotalStorageTB: 1.0, NVMeCount: 2, DriveCount: 2,
				Datacenter: "HEL1-DC7", Price: 39.00,
			},
			Score: 85.0,
		},
		{
			Server: scanner.Server{
				ID: 1002, CPU: "Intel Core i7-13700", RAMSize: 128,
				TotalStorageTB: 2.0, NVMeCount: 2, DriveCount: 2,
				Datacenter: "FSN1-DC14", Price: 72.00,
			},
			Score: 78.0,
		},
	}
	require.NoError(t, s.SaveScan(servers))

	analytics, err := s.GetMarketAnalytics()
	require.NoError(t, err)
	require.NotNil(t, analytics)

	// Brand trends should have AMD and Intel entries
	assert.NotEmpty(t, analytics.BrandTrends, "should have brand trends")

	// DC volume should have 2 datacenters
	assert.Len(t, analytics.DCVolume, 2, "should have 2 datacenters")

	// Top CPUs should have entries
	assert.NotEmpty(t, analytics.TopValueCPUs, "should have top CPUs")

	// Price buckets should have entries
	assert.NotEmpty(t, analytics.PriceBuckets, "should have price buckets")
}

func TestGetMarketAnalyticsEmpty(t *testing.T) {
	s := setupTestDB(t)

	analytics, err := s.GetMarketAnalytics()
	require.NoError(t, err)
	require.NotNil(t, analytics)
	assert.Empty(t, analytics.BrandTrends)
	assert.Empty(t, analytics.DCVolume)
}

func TestGetTopDeals(t *testing.T) {
	s := setupTestDB(t)

	servers := []scorer.ScoredServer{
		{
			Server: scanner.Server{
				ID: 1001, CPU: "AMD Ryzen 5 3600", RAMSize: 64,
				TotalStorageTB: 1.0, NVMeCount: 2, DriveCount: 2,
				Datacenter: "HEL1", Price: 39.00,
			},
			Score: 90.0,
		},
		{
			Server: scanner.Server{
				ID: 1002, CPU: "Intel Core i5-13500", RAMSize: 64,
				TotalStorageTB: 1.0, NVMeCount: 2, DriveCount: 2,
				Datacenter: "FSN1", Price: 55.00,
			},
			Score: 70.0,
		},
		{
			Server: scanner.Server{
				ID: 1003, CPU: "AMD Ryzen 9 5950X", RAMSize: 128,
				TotalStorageTB: 4.0, NVMeCount: 4, DriveCount: 4,
				Datacenter: "HEL1", Price: 75.00,
			},
			Score: 95.0,
		},
	}
	require.NoError(t, s.SaveScan(servers))

	// Get top 2 deals since 1 hour ago
	since := time.Now().Add(-1 * time.Hour)
	deals, err := s.GetTopDeals(since, 2, 0)
	require.NoError(t, err)
	assert.Len(t, deals, 2)
	// Should be ordered by score desc
	assert.Equal(t, 1003, deals[0].ServerID)
	assert.Equal(t, 1001, deals[1].ServerID)

	// With min score filter
	deals, err = s.GetTopDeals(since, 10, 85.0)
	require.NoError(t, err)
	assert.Len(t, deals, 2) // only 90.0 and 95.0
	for _, d := range deals {
		assert.GreaterOrEqual(t, d.Score, 85.0)
	}

	// Far in the future — nothing
	deals, err = s.GetTopDeals(time.Now().Add(24*time.Hour), 10, 0)
	require.NoError(t, err)
	assert.Empty(t, deals)
}

func TestSaveScanWithNewFields(t *testing.T) {
	s := setupTestDB(t)

	servers := []scorer.ScoredServer{
		{
			Server: scanner.Server{
				ID: 6001, CPU: "AMD EPYC 7443P", RAMSize: 256,
				TotalStorageTB: 7.68, NVMeCount: 4, DriveCount: 4,
				Datacenter: "FSN1-DC14", Price: 120.00,
				IsECC: true, SetupPrice: 0, NextReduce: 0,
				FixedPrice: true, Bandwidth: 20000,
			},
			Score: 95.0,
		},
		{
			Server: scanner.Server{
				ID: 6002, CPU: "Intel Core i5-13500", RAMSize: 64,
				TotalStorageTB: 1.0, NVMeCount: 2, DriveCount: 2,
				Datacenter: "HEL1-DC7", Price: 45.00,
				IsECC: false, SetupPrice: 25.0, NextReduce: 24,
				FixedPrice: false, Bandwidth: 1000,
			},
			Score: 70.0,
		},
	}
	require.NoError(t, s.SaveScan(servers))

	// Verify via GetHistory
	records, err := s.GetHistory("", 10)
	require.NoError(t, err)
	assert.Len(t, records, 2)

	// Find the ECC server
	var eccRec *ScanRecord
	for i := range records {
		if records[i].ServerID == 6001 {
			eccRec = &records[i]
			break
		}
	}
	require.NotNil(t, eccRec)
	assert.True(t, eccRec.IsECC)
	assert.True(t, eccRec.FixedPrice)
	assert.Equal(t, 20000, eccRec.Bandwidth)
}
