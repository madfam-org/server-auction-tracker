package main

import (
	"context"
	"embed"
	"encoding/json"
	"flag"
	"fmt"
	"io/fs"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"encoding/hex"

	"github.com/madfam-org/server-auction-tracker/internal/auth"
	"github.com/madfam-org/server-auction-tracker/internal/config"
	"github.com/madfam-org/server-auction-tracker/internal/order"
	"github.com/madfam-org/server-auction-tracker/internal/simulate"
	"github.com/madfam-org/server-auction-tracker/internal/store"
	log "github.com/sirupsen/logrus"
)

//go:embed web
var webFS embed.FS

func main() {
	cfgPath := flag.String("config", "", "path to scout.yaml config file")
	addr := flag.String("addr", ":4205", "listen address")
	flag.Parse()

	cfg, err := config.Load(*cfgPath)
	if err != nil {
		log.Fatalf("loading config: %v", err)
	}

	db, err := store.NewSQLite(cfg.Database.Path)
	if err != nil {
		log.Fatalf("opening database: %v", err)
	}

	if err := db.Init(); err != nil {
		_ = db.Close()
		log.Fatalf("initializing database: %v", err)
	}
	defer db.Close() //nolint:errcheck,gocritic

	s := &server{
		store:   db,
		config:  cfg,
		orderer: order.NewRobotClient(&cfg.Order),
	}

	// Initialize Janua SSO auth (non-fatal if unreachable)
	s.initAuth()

	mux := http.NewServeMux()

	// API routes
	mux.HandleFunc("GET /api/health", s.handleHealth)
	mux.HandleFunc("GET /api/latest", s.handleLatest)
	mux.HandleFunc("GET /api/history", s.handleHistory)
	mux.HandleFunc("GET /api/stats", s.handleAllStats)
	mux.HandleFunc("GET /api/stats/{cpu}", s.handleCPUStats)
	mux.HandleFunc("GET /api/simulate/{server_id}", s.handleSimulate)
	mux.HandleFunc("GET /api/orders", s.handleOrders)
	mux.HandleFunc("GET /api/config", s.handleConfig)
	mux.HandleFunc("GET /api/export", s.handleExport)
	mux.HandleFunc("GET /api/analytics", s.handleAnalytics)
	mux.HandleFunc("GET /metrics", s.handleMetrics)
	mux.HandleFunc("POST /api/order/check", s.authMiddleware(s.handleOrderCheck))
	mux.HandleFunc("POST /api/order/confirm", s.authMiddleware(s.handleOrderConfirm))

	// Auth routes (SSO)
	if s.oauth != nil {
		mux.HandleFunc("GET /auth/login", s.oauth.LoginHandler)
		mux.HandleFunc("GET /auth/callback", s.oauth.CallbackHandler)
		mux.HandleFunc("POST /auth/logout", s.oauth.LogoutHandler)
		mux.HandleFunc("GET /auth/me", s.oauth.MeHandler)
	} else {
		// Stub endpoints when SSO not configured
		mux.HandleFunc("GET /auth/me", func(w http.ResponseWriter, r *http.Request) {
			writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "SSO not configured"})
		})
	}

	// Static frontend
	webSub, err := fs.Sub(webFS, "web")
	if err != nil {
		log.Fatalf("embedded web fs: %v", err) //nolint:gocritic // defer db.Close above is for graceful shutdown; this path is unreachable
	}
	mux.Handle("GET /", http.FileServer(http.FS(webSub)))

	srv := &http.Server{
		Addr:         *addr,
		Handler:      mux,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	// Graceful shutdown
	shutdownCh := make(chan os.Signal, 1)
	signal.Notify(shutdownCh, os.Interrupt, syscall.SIGTERM)

	go func() {
		log.WithField("addr", *addr).Info("Deal Sniper web server starting")
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.WithError(err).Fatal("Server failed")
		}
	}()

	<-shutdownCh
	log.Info("Shutting down gracefully...")

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		fmt.Fprintf(os.Stderr, "shutdown error: %v\n", err)
		os.Exit(1)
	}
	log.Info("Server stopped")
}

type server struct {
	store     store.Store
	config    *config.Config
	orderer   order.Orderer
	validator *auth.Validator
	oauth     *auth.OAuthFlow
}

func (s *server) initAuth() {
	clientID := os.Getenv("JANUA_CLIENT_ID")
	clientSecret := os.Getenv("JANUA_CLIENT_SECRET")
	sessionSecretHex := os.Getenv("DS_SESSION_SECRET")

	if clientID == "" || clientSecret == "" || sessionSecretHex == "" {
		log.Info("SSO not configured (missing JANUA_CLIENT_ID, JANUA_CLIENT_SECRET, or DS_SESSION_SECRET) — order auth falls back to Bearer token")
		return
	}

	sessionSecret, err := hex.DecodeString(sessionSecretHex)
	if err != nil {
		log.WithError(err).Warn("Invalid DS_SESSION_SECRET (must be hex) — SSO disabled")
		return
	}

	cacheTTL := time.Hour
	if s.config.Auth.JWKSCacheTTL != "" {
		if d, err := time.ParseDuration(s.config.Auth.JWKSCacheTTL); err == nil {
			cacheTTL = d
		}
	}

	issuer := s.config.Auth.JanuaIssuer
	if issuer == "" {
		issuer = "https://auth.madfam.io"
	}
	jwksURL := s.config.Auth.JanuaJWKSURL
	if jwksURL == "" {
		jwksURL = "https://auth.madfam.io/.well-known/jwks.json"
	}
	allowedDomains := s.config.Auth.AllowedDomains
	if len(allowedDomains) == 0 {
		allowedDomains = []string{"@madfam.io"}
	}
	allowedRoles := s.config.Auth.AllowedRoles
	if len(allowedRoles) == 0 {
		allowedRoles = []string{"superadmin", "admin", "operator"}
	}

	fetcher := auth.NewJWKSFetcher(jwksURL, cacheTTL)
	s.validator = auth.NewValidator(fetcher, issuer, allowedDomains, allowedRoles)

	authURL := issuer + "/oauth/authorize"
	tokenURL := issuer + "/oauth/token"
	redirectURL := "https://sniper.madfam.io/auth/callback"

	s.oauth = auth.NewOAuthFlow(clientID, clientSecret, authURL, tokenURL, redirectURL, s.validator, sessionSecret)

	log.WithFields(log.Fields{
		"issuer":   issuer,
		"jwks_url": jwksURL,
	}).Info("Janua SSO authentication initialized")
}

func (s *server) handleHealth(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (s *server) handleLatest(w http.ResponseWriter, r *http.Request) {
	limit := queryInt(r, "limit", 50)
	records, err := s.store.GetHistory("", limit)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	stats, _ := s.store.GetAllCPUStats()

	enriched := make([]enrichedRecord, len(records))
	for i := range records {
		enriched[i].ScanRecord = records[i]
		if stats != nil {
			if cpuStat, ok := stats[records[i].CPU]; ok && cpuStat.AvgPrice > 0 {
				enriched[i].DealQualityPct = ((cpuStat.AvgPrice - records[i].Price) / cpuStat.AvgPrice) * 100
				enriched[i].Percentile = computePercentile(records[i].Price, cpuStat.MinPrice, cpuStat.MaxPrice)
			}
		}
	}

	writeJSON(w, http.StatusOK, enriched)
}

func computePercentile(price, minPrice, maxPrice float64) int {
	if maxPrice <= minPrice {
		return 50
	}
	// Lower price = better percentile (100 = cheapest, 0 = most expensive)
	pct := (1 - (price-minPrice)/(maxPrice-minPrice)) * 100
	if pct < 0 {
		return 0
	}
	if pct > 100 {
		return 100
	}
	return int(pct)
}

func (s *server) handleHistory(w http.ResponseWriter, r *http.Request) {
	cpu := r.URL.Query().Get("cpu")
	limit := queryInt(r, "limit", 100)
	records, err := s.store.GetHistory(cpu, limit)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, records)
}

func (s *server) handleAllStats(w http.ResponseWriter, r *http.Request) {
	stats, err := s.store.GetAllCPUStats()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, stats)
}

func (s *server) handleCPUStats(w http.ResponseWriter, r *http.Request) {
	cpu := r.PathValue("cpu")
	stats, err := s.store.GetStats(cpu)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	if stats == nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "no data for CPU model"})
		return
	}
	writeJSON(w, http.StatusOK, stats)
}

func (s *server) handleSimulate(w http.ResponseWriter, r *http.Request) {
	serverIDStr := r.PathValue("server_id")
	serverID, err := strconv.Atoi(serverIDStr)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid server_id"})
		return
	}

	found, err := s.store.GetByServerID(serverID)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	if found == nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "server not found in recent scans"})
		return
	}

	// Build a minimal scanner.Server from the scan record for simulation
	srv := scanRecordToServer(found)
	currentCost := float64(s.config.Cluster.Nodes) * 50.0 // estimate from existing nodes
	result := simulate.Simulate(&s.config.Cluster, &srv, currentCost)

	writeJSON(w, http.StatusOK, simulateResponse{
		Result:     result,
		HealthBefore: healthLabels(result.CPUBefore, result.RAMBefore, result.DiskBefore),
		HealthAfter:  healthLabels(result.CPUAfter, result.RAMAfter, result.DiskAfter),
	})
}

func (s *server) handleOrders(w http.ResponseWriter, r *http.Request) {
	orders, err := s.store.GetOrderAttempts(100)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, orders)
}

func (s *server) handleConfig(w http.ResponseWriter, r *http.Request) {
	// Return config with secrets redacted
	redacted := configResponse{
		Filters: s.config.Filters,
		Scoring: s.config.Scoring,
		Cluster: s.config.Cluster,
		Watch:   s.config.Watch,
		Order: orderRedacted{
			Enabled:         s.config.Order.Enabled,
			MinScore:        s.config.Order.MinScore,
			MaxPriceEUR:     s.config.Order.MaxPriceEUR,
			RequireApproval: s.config.Order.RequireApproval,
		},
	}
	writeJSON(w, http.StatusOK, redacted)
}

func (s *server) handleExport(w http.ResponseWriter, r *http.Request) {
	format := r.URL.Query().Get("format")
	if format == "" {
		format = "json"
	}

	records, err := s.store.GetHistory("", 500)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	switch format {
	case "csv":
		w.Header().Set("Content-Type", "text/csv")
		w.Header().Set("Content-Disposition", "attachment; filename=deal-sniper-export.csv")
		w.WriteHeader(http.StatusOK)
		fmt.Fprintln(w, "ServerID,CPU,RAM_GB,Storage_TB,NVMe,Drives,Datacenter,Price_EUR,Score,ScannedAt")
		for i := range records {
			fmt.Fprintf(w, "%d,%q,%d,%.2f,%d,%d,%q,%.2f,%.1f,%s\n",
				records[i].ServerID, records[i].CPU, records[i].RAMSize, records[i].TotalStorageTB,
				records[i].NVMeCount, records[i].DriveCount, records[i].Datacenter,
				records[i].Price, records[i].Score, records[i].ScannedAt.Format(time.RFC3339))
		}
	default:
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Content-Disposition", "attachment; filename=deal-sniper-export.json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(records) //nolint:errcheck
	}
}

func (s *server) handleAnalytics(w http.ResponseWriter, r *http.Request) {
	analytics, err := s.store.GetMarketAnalytics()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, analytics)
}

func (s *server) handleMetrics(w http.ResponseWriter, _ *http.Request) {
	records, err := s.store.GetHistory("", 500)
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	var total int
	var sumPrice, bestScore float64
	var lastScan time.Time

	for i := range records {
		total++
		sumPrice += records[i].Price
		if records[i].Score > bestScore {
			bestScore = records[i].Score
		}
		if records[i].ScannedAt.After(lastScan) {
			lastScan = records[i].ScannedAt
		}
	}

	avgPrice := 0.0
	if total > 0 {
		avgPrice = sumPrice / float64(total)
	}

	w.Header().Set("Content-Type", "text/plain; version=0.0.4; charset=utf-8")
	fmt.Fprintf(w, "# HELP deal_sniper_deals_total Number of tracked deals\n")
	fmt.Fprintf(w, "# TYPE deal_sniper_deals_total gauge\n")
	fmt.Fprintf(w, "deal_sniper_deals_total %d\n", total)
	fmt.Fprintf(w, "# HELP deal_sniper_avg_price_eur Average deal price in EUR\n")
	fmt.Fprintf(w, "# TYPE deal_sniper_avg_price_eur gauge\n")
	fmt.Fprintf(w, "deal_sniper_avg_price_eur %.2f\n", avgPrice)
	fmt.Fprintf(w, "# HELP deal_sniper_best_score Highest deal score\n")
	fmt.Fprintf(w, "# TYPE deal_sniper_best_score gauge\n")
	fmt.Fprintf(w, "deal_sniper_best_score %.1f\n", bestScore)
	fmt.Fprintf(w, "# HELP deal_sniper_last_scan_unix Timestamp of last scan in unix epoch\n")
	fmt.Fprintf(w, "# TYPE deal_sniper_last_scan_unix gauge\n")
	fmt.Fprintf(w, "deal_sniper_last_scan_unix %d\n", lastScan.Unix())
}

// --- helpers ---

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-cache")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v) //nolint:errcheck
}

func queryInt(r *http.Request, key string, defaultVal int) int {
	s := r.URL.Query().Get(key)
	if s == "" {
		return defaultVal
	}
	v, err := strconv.Atoi(s)
	if err != nil || v <= 0 {
		return defaultVal
	}
	return v
}

// --- types ---

type enrichedRecord struct {
	store.ScanRecord
	DealQualityPct float64 `json:"deal_quality_pct"`
	Percentile     int     `json:"percentile"`
}

type simulateResponse struct {
	Result       *simulate.Result       `json:"result"`
	HealthBefore map[string]string      `json:"health_before"`
	HealthAfter  map[string]string      `json:"health_after"`
}

type configResponse struct {
	Filters config.Filters `json:"filters"`
	Scoring config.Scoring `json:"scoring"`
	Cluster config.Cluster `json:"cluster"`
	Watch   config.Watch   `json:"watch"`
	Order   orderRedacted  `json:"order"`
}

type orderRedacted struct {
	Enabled         bool    `json:"enabled"`
	MinScore        float64 `json:"min_score"`
	MaxPriceEUR     float64 `json:"max_price_eur"`
	RequireApproval bool    `json:"require_approval"`
}

func healthLabels(cpu, ram, disk float64) map[string]string {
	return map[string]string{
		"cpu":  simulate.HealthLabel(cpu),
		"ram":  simulate.HealthLabel(ram),
		"disk": simulate.HealthLabel(disk),
	}
}
