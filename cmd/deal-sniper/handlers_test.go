package main

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/madfam-org/server-auction-tracker/internal/config"
	"github.com/madfam-org/server-auction-tracker/internal/order"
	"github.com/madfam-org/server-auction-tracker/internal/scanner"
	"github.com/madfam-org/server-auction-tracker/internal/scorer"
	"github.com/madfam-org/server-auction-tracker/internal/store"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockOrderer implements order.Orderer for testing.
type mockOrderer struct {
	checkResult *order.Check
	orderResult *order.Result
	orderErr    error
}

func (m *mockOrderer) CheckEligibility(server scanner.Server, score float64, cfg config.Order) *order.Check {
	if m.checkResult != nil {
		return m.checkResult
	}
	return order.NewRobotClient(cfg).CheckEligibility(server, score, cfg)
}

func (m *mockOrderer) Order(_ context.Context, serverID int) (*order.Result, error) {
	if m.orderErr != nil {
		return nil, m.orderErr
	}
	if m.orderResult != nil {
		return m.orderResult, nil
	}
	return &order.Result{ServerID: serverID, Success: true, Message: "test order placed", TransID: "TX-123"}, nil
}

func setupTestServer(t *testing.T) (*server, func()) {
	t.Helper()
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "test.db")

	db, err := store.NewSQLite(dbPath)
	require.NoError(t, err)
	require.NoError(t, db.Init())

	cfg := &config.Config{
		Filters: config.Filters{
			MinRAMGB:    128,
			MinCPUCores: 8,
			MaxPriceEUR: 85,
		},
		Scoring: config.Scoring{
			CPUWeight:     0.30,
			RAMWeight:     0.25,
			StorageWeight: 0.20,
			NVMeWeight:    0.10,
			CPUGenWeight:  0.10,
		},
		Cluster: config.Cluster{
			CPUMillicores:  12000,
			CPURequested:   10460,
			RAMGB:          64,
			RAMRequestedGB: 25,
			DiskGB:         98,
			DiskUsedGB:     80,
			Nodes:          2,
		},
		Database: config.Database{Path: dbPath},
		Order: config.Order{
			Enabled:     true,
			MinScore:    70.0,
			MaxPriceEUR: 80.0,
		},
	}

	s := &server{
		store:   db,
		config:  cfg,
		orderer: &mockOrderer{},
	}
	cleanup := func() { _ = db.Close(); _ = os.RemoveAll(dir) }
	return s, cleanup
}

func seedTestData(t *testing.T, db store.Store) {
	t.Helper()
	servers := []scorer.ScoredServer{
		{
			Server: scanner.Server{
				ID: 12345, CPU: "AMD Ryzen 9 5950X", RAMSize: 128,
				TotalStorageTB: 3.84, NVMeCount: 2, DriveCount: 2,
				Datacenter: "HEL1-DC14", Price: 72.50,
				ParsedCores: 16, ParsedThreads: 32,
			},
			Score: 87.5,
			Breakdown: scorer.Breakdown{
				CPUPerDollar: 0.9, RAMPerDollar: 0.85,
				StoragePerDollar: 0.7, NVMeBonus: 1.0,
			},
		},
		{
			Server: scanner.Server{
				ID: 67890, CPU: "Intel Core i7-13700", RAMSize: 64,
				TotalStorageTB: 1.92, NVMeCount: 2, DriveCount: 2,
				Datacenter: "HEL1-DC11", Price: 55.00,
				ParsedCores: 16, ParsedThreads: 24,
			},
			Score: 72.3,
			Breakdown: scorer.Breakdown{
				CPUPerDollar: 0.75, RAMPerDollar: 0.6,
				StoragePerDollar: 0.5, NVMeBonus: 1.0,
			},
		},
	}
	require.NoError(t, db.SaveScan(servers))
}

func TestHealthEndpoint(t *testing.T) {
	s, cleanup := setupTestServer(t)
	defer cleanup()

	req := httptest.NewRequest("GET", "/api/health", nil)
	w := httptest.NewRecorder()
	s.handleHealth(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var resp map[string]string
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Equal(t, "ok", resp["status"])
}

func TestLatestEndpoint(t *testing.T) {
	s, cleanup := setupTestServer(t)
	defer cleanup()

	seedTestData(t, s.store)

	req := httptest.NewRequest("GET", "/api/latest", nil)
	w := httptest.NewRecorder()
	s.handleLatest(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var records []enrichedRecord
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &records))
	assert.Len(t, records, 2)
}

func TestLatestEndpointEmpty(t *testing.T) {
	s, cleanup := setupTestServer(t)
	defer cleanup()

	req := httptest.NewRequest("GET", "/api/latest", nil)
	w := httptest.NewRecorder()
	s.handleLatest(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestHistoryEndpoint(t *testing.T) {
	s, cleanup := setupTestServer(t)
	defer cleanup()

	seedTestData(t, s.store)

	req := httptest.NewRequest("GET", "/api/history?cpu=Ryzen&limit=10", nil)
	w := httptest.NewRecorder()
	s.handleHistory(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var records []store.ScanRecord
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &records))
	assert.Len(t, records, 1)
	assert.Contains(t, records[0].CPU, "Ryzen")
}

func TestAllStatsEndpoint(t *testing.T) {
	s, cleanup := setupTestServer(t)
	defer cleanup()

	seedTestData(t, s.store)

	req := httptest.NewRequest("GET", "/api/stats", nil)
	w := httptest.NewRecorder()
	s.handleAllStats(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var stats map[string]*store.PriceStats
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &stats))
	assert.Contains(t, stats, "AMD Ryzen 9 5950X")
	assert.Contains(t, stats, "Intel Core i7-13700")
}

func TestCPUStatsEndpoint(t *testing.T) {
	s, cleanup := setupTestServer(t)
	defer cleanup()

	seedTestData(t, s.store)

	req := httptest.NewRequest("GET", "/api/stats/{cpu}", nil)
	req.SetPathValue("cpu", "Ryzen")
	w := httptest.NewRecorder()
	s.handleCPUStats(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestCPUStatsNotFound(t *testing.T) {
	s, cleanup := setupTestServer(t)
	defer cleanup()

	req := httptest.NewRequest("GET", "/api/stats/{cpu}", nil)
	req.SetPathValue("cpu", "NonexistentCPU12345")
	w := httptest.NewRecorder()
	s.handleCPUStats(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestSimulateEndpoint(t *testing.T) {
	s, cleanup := setupTestServer(t)
	defer cleanup()

	seedTestData(t, s.store)

	req := httptest.NewRequest("GET", "/api/simulate/{server_id}", nil)
	req.SetPathValue("server_id", "12345")
	w := httptest.NewRecorder()
	s.handleSimulate(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var resp simulateResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.NotNil(t, resp.Result)
	assert.Equal(t, 2, resp.Result.NodesBefore)
	assert.Equal(t, 3, resp.Result.NodesAfter)
	assert.NotEmpty(t, resp.HealthBefore)
	assert.NotEmpty(t, resp.HealthAfter)
}

func TestSimulateInvalidID(t *testing.T) {
	s, cleanup := setupTestServer(t)
	defer cleanup()

	req := httptest.NewRequest("GET", "/api/simulate/{server_id}", nil)
	req.SetPathValue("server_id", "notanumber")
	w := httptest.NewRecorder()
	s.handleSimulate(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestSimulateNotFound(t *testing.T) {
	s, cleanup := setupTestServer(t)
	defer cleanup()

	req := httptest.NewRequest("GET", "/api/simulate/{server_id}", nil)
	req.SetPathValue("server_id", "99999")
	w := httptest.NewRecorder()
	s.handleSimulate(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestConfigEndpoint(t *testing.T) {
	s, cleanup := setupTestServer(t)
	defer cleanup()

	req := httptest.NewRequest("GET", "/api/config", nil)
	w := httptest.NewRecorder()
	s.handleConfig(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var resp configResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Equal(t, 128, resp.Filters.MinRAMGB)
	assert.Equal(t, 85.0, resp.Filters.MaxPriceEUR)
	assert.Equal(t, 2, resp.Cluster.Nodes)
}

func TestOrdersEndpoint(t *testing.T) {
	s, cleanup := setupTestServer(t)
	defer cleanup()

	// Save an order attempt
	require.NoError(t, s.store.SaveOrderAttempt(12345, 87.5, 72.50, true, "Order placed"))

	req := httptest.NewRequest("GET", "/api/orders", nil)
	w := httptest.NewRecorder()
	s.handleOrders(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var orders []store.OrderAttempt
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &orders))
	assert.Len(t, orders, 1)
	assert.Equal(t, 12345, orders[0].ServerID)
	assert.True(t, orders[0].Success)
}

func TestScanRecordToServer(t *testing.T) {
	r := store.ScanRecord{
		ServerID:       12345,
		CPU:            "AMD Ryzen 9 5950X",
		RAMSize:        128,
		TotalStorageTB: 3.84,
		NVMeCount:      2,
		DriveCount:     2,
		Datacenter:     "HEL1-DC14",
		Price:          72.50,
	}
	srv := scanRecordToServer(r)
	assert.Equal(t, 12345, srv.ID)
	assert.Equal(t, "AMD Ryzen 9 5950X", srv.CPU)
	assert.Equal(t, 128, srv.RAMSize)
	assert.Equal(t, 16, srv.ParsedCores) // 128GB → 16 cores estimate
}

func TestEstimateCoresFromRAM(t *testing.T) {
	assert.Equal(t, 32, estimateCoresFromRAM(256))
	assert.Equal(t, 16, estimateCoresFromRAM(128))
	assert.Equal(t, 8, estimateCoresFromRAM(64))
	assert.Equal(t, 4, estimateCoresFromRAM(32))
}

func TestQueryInt(t *testing.T) {
	tests := []struct {
		url      string
		key      string
		def      int
		expected int
	}{
		{"/test?limit=25", "limit", 50, 25},
		{"/test", "limit", 50, 50},
		{"/test?limit=abc", "limit", 50, 50},
		{"/test?limit=-1", "limit", 50, 50},
	}
	for _, tt := range tests {
		req := httptest.NewRequest("GET", tt.url, nil)
		assert.Equal(t, tt.expected, queryInt(req, tt.key, tt.def))
	}
}

// --- Auth middleware tests ---

func TestAuthMiddleware_NoToken(t *testing.T) {
	s, cleanup := setupTestServer(t)
	defer cleanup()

	// No DEAL_SNIPER_AUTH_TOKEN set → 503
	t.Setenv("DEAL_SNIPER_AUTH_TOKEN", "")
	handler := s.authMiddleware(func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, map[string]string{"ok": "true"})
	})

	req := httptest.NewRequest("POST", "/api/order/check", nil)
	w := httptest.NewRecorder()
	handler(w, req)

	assert.Equal(t, http.StatusServiceUnavailable, w.Code)
}

func TestAuthMiddleware_WrongToken(t *testing.T) {
	s, cleanup := setupTestServer(t)
	defer cleanup()

	t.Setenv("DEAL_SNIPER_AUTH_TOKEN", "correct-token")
	handler := s.authMiddleware(func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, map[string]string{"ok": "true"})
	})

	req := httptest.NewRequest("POST", "/api/order/check", nil)
	req.Header.Set("Authorization", "Bearer wrong-token")
	w := httptest.NewRecorder()
	handler(w, req)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestAuthMiddleware_MissingBearer(t *testing.T) {
	s, cleanup := setupTestServer(t)
	defer cleanup()

	t.Setenv("DEAL_SNIPER_AUTH_TOKEN", "correct-token")
	handler := s.authMiddleware(func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, map[string]string{"ok": "true"})
	})

	req := httptest.NewRequest("POST", "/api/order/check", nil)
	// No Authorization header at all
	w := httptest.NewRecorder()
	handler(w, req)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestAuthMiddleware_CorrectToken(t *testing.T) {
	s, cleanup := setupTestServer(t)
	defer cleanup()

	t.Setenv("DEAL_SNIPER_AUTH_TOKEN", "correct-token")
	handler := s.authMiddleware(func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, map[string]string{"ok": "true"})
	})

	req := httptest.NewRequest("POST", "/api/order/check", nil)
	req.Header.Set("Authorization", "Bearer correct-token")
	w := httptest.NewRecorder()
	handler(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

// --- Order check endpoint tests ---

func TestOrderCheckEndpoint(t *testing.T) {
	s, cleanup := setupTestServer(t)
	defer cleanup()

	seedTestData(t, s.store)
	t.Setenv("DEAL_SNIPER_AUTH_TOKEN", "test-token")

	body, _ := json.Marshal(orderCheckRequest{ServerID: 12345})
	req := httptest.NewRequest("POST", "/api/order/check", bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer test-token")
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	// Wrap with auth middleware like the real server
	s.authMiddleware(s.handleOrderCheck)(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp orderCheckResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Equal(t, 12345, resp.ServerID)
	assert.Equal(t, 87.5, resp.Score)
	assert.Equal(t, 72.5, resp.Price)
	assert.True(t, resp.Eligible)
}

func TestOrderCheckEndpoint_NoAuth(t *testing.T) {
	s, cleanup := setupTestServer(t)
	defer cleanup()

	t.Setenv("DEAL_SNIPER_AUTH_TOKEN", "test-token")

	body, _ := json.Marshal(orderCheckRequest{ServerID: 12345})
	req := httptest.NewRequest("POST", "/api/order/check", bytes.NewReader(body))
	// No Authorization header
	w := httptest.NewRecorder()

	s.authMiddleware(s.handleOrderCheck)(w, req)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestOrderCheckEndpoint_ServerNotFound(t *testing.T) {
	s, cleanup := setupTestServer(t)
	defer cleanup()

	t.Setenv("DEAL_SNIPER_AUTH_TOKEN", "test-token")

	body, _ := json.Marshal(orderCheckRequest{ServerID: 99999})
	req := httptest.NewRequest("POST", "/api/order/check", bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer test-token")
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	s.authMiddleware(s.handleOrderCheck)(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestOrderCheckEndpoint_Ineligible(t *testing.T) {
	s, cleanup := setupTestServer(t)
	defer cleanup()

	seedTestData(t, s.store)
	t.Setenv("DEAL_SNIPER_AUTH_TOKEN", "test-token")

	// Set order disabled so server is ineligible
	s.config.Order.Enabled = false

	body, _ := json.Marshal(orderCheckRequest{ServerID: 12345})
	req := httptest.NewRequest("POST", "/api/order/check", bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer test-token")
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	s.authMiddleware(s.handleOrderCheck)(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var resp orderCheckResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.False(t, resp.Eligible)
	assert.NotEmpty(t, resp.Reasons)
	assert.Contains(t, resp.Reasons[0], "disabled")
}

// --- Order confirm endpoint tests ---

func TestOrderConfirmEndpoint(t *testing.T) {
	s, cleanup := setupTestServer(t)
	defer cleanup()

	seedTestData(t, s.store)
	t.Setenv("DEAL_SNIPER_AUTH_TOKEN", "test-token")

	body, _ := json.Marshal(orderCheckRequest{ServerID: 12345})
	req := httptest.NewRequest("POST", "/api/order/confirm", bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer test-token")
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	s.authMiddleware(s.handleOrderConfirm)(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var resp orderConfirmResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.True(t, resp.Success)
	assert.Equal(t, "TX-123", resp.TransactionID)
	assert.Contains(t, resp.Message, "test order placed")
}

func TestOrderConfirmEndpoint_Ineligible(t *testing.T) {
	s, cleanup := setupTestServer(t)
	defer cleanup()

	seedTestData(t, s.store)
	t.Setenv("DEAL_SNIPER_AUTH_TOKEN", "test-token")

	// Make server ineligible by disabling orders
	s.config.Order.Enabled = false

	body, _ := json.Marshal(orderCheckRequest{ServerID: 12345})
	req := httptest.NewRequest("POST", "/api/order/confirm", bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer test-token")
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	s.authMiddleware(s.handleOrderConfirm)(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var resp orderConfirmResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.False(t, resp.Success)
	assert.Contains(t, resp.Message, "ineligible")
}

// --- Breakdown in scan records ---

func TestBreakdownInLatest(t *testing.T) {
	s, cleanup := setupTestServer(t)
	defer cleanup()

	seedTestData(t, s.store)

	req := httptest.NewRequest("GET", "/api/latest", nil)
	w := httptest.NewRecorder()
	s.handleLatest(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var records []enrichedRecord
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &records))
	require.Len(t, records, 2)

	// Check that breakdown JSON is populated
	assert.NotEmpty(t, records[0].BreakdownJSON)
	assert.NotEqual(t, "{}", records[0].BreakdownJSON)

	var bd scorer.Breakdown
	require.NoError(t, json.Unmarshal([]byte(records[0].BreakdownJSON), &bd))
	// The first record (by scanned_at desc) could be either server —
	// just verify the breakdown has non-zero values
	assert.True(t, bd.CPUPerDollar > 0 || bd.RAMPerDollar > 0)
}

func TestDealQualityEnrichment(t *testing.T) {
	s, cleanup := setupTestServer(t)
	defer cleanup()

	// Save multiple scans at different prices for the same CPU
	for _, price := range []float64{30.0, 40.0, 50.0, 60.0, 70.0} {
		servers := []scorer.ScoredServer{{
			Server: scanner.Server{
				ID: int(price * 100), CPU: "AMD Ryzen 5 3600", RAMSize: 64,
				TotalStorageTB: 1.0, NVMeCount: 2, DriveCount: 2,
				Datacenter: "HEL1", Price: price,
			},
			Score: 80,
		}}
		require.NoError(t, s.store.SaveScan(servers))
	}

	req := httptest.NewRequest("GET", "/api/latest?limit=10", nil)
	w := httptest.NewRecorder()
	s.handleLatest(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var records []enrichedRecord
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &records))
	require.NotEmpty(t, records)

	// The cheapest server ($30) should have positive deal quality
	// The most expensive ($70) should have negative deal quality
	var cheapest, expensive *enrichedRecord
	for i := range records {
		if records[i].Price == 30.0 {
			cheapest = &records[i]
		}
		if records[i].Price == 70.0 {
			expensive = &records[i]
		}
	}

	require.NotNil(t, cheapest, "should find $30 server")
	require.NotNil(t, expensive, "should find $70 server")
	assert.Greater(t, cheapest.DealQualityPct, 0.0, "cheap server should be below avg")
	assert.Less(t, expensive.DealQualityPct, 0.0, "expensive server should be above avg")
	assert.Greater(t, cheapest.Percentile, expensive.Percentile, "cheap server should have better percentile")
}

func TestAnalyticsEndpoint(t *testing.T) {
	s, cleanup := setupTestServer(t)
	defer cleanup()

	seedTestData(t, s.store)

	req := httptest.NewRequest("GET", "/api/analytics", nil)
	w := httptest.NewRecorder()
	s.handleAnalytics(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var analytics store.MarketAnalytics
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &analytics))
	// Should have data from seeded records
	assert.NotEmpty(t, analytics.BrandTrends)
}

func TestAnalyticsEndpointEmpty(t *testing.T) {
	s, cleanup := setupTestServer(t)
	defer cleanup()

	req := httptest.NewRequest("GET", "/api/analytics", nil)
	w := httptest.NewRecorder()
	s.handleAnalytics(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var analytics store.MarketAnalytics
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &analytics))
	assert.Empty(t, analytics.BrandTrends)
	assert.Empty(t, analytics.DCVolume)
}

func TestComputePercentile(t *testing.T) {
	// Cheapest possible price -> 100th percentile
	assert.Equal(t, 100, computePercentile(30, 30, 70))
	// Most expensive -> 0th percentile
	assert.Equal(t, 0, computePercentile(70, 30, 70))
	// Middle price -> 50th percentile
	assert.Equal(t, 50, computePercentile(50, 30, 70))
	// Equal min/max -> 50 (edge case)
	assert.Equal(t, 50, computePercentile(50, 50, 50))
}
