package store

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	_ "github.com/mattn/go-sqlite3"
	"github.com/madfam-org/server-auction-tracker/internal/scorer"
)

type SQLiteStore struct {
	db *sql.DB
}

func NewSQLite(dbPath string) (*SQLiteStore, error) {
	db, err := sql.Open("sqlite3", dbPath+"?_journal_mode=WAL")
	if err != nil {
		return nil, fmt.Errorf("opening sqlite: %w", err)
	}
	return &SQLiteStore{db: db}, nil
}

func (s *SQLiteStore) Init() error {
	schema := `
	CREATE TABLE IF NOT EXISTS scans (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		server_id INTEGER NOT NULL,
		cpu TEXT NOT NULL,
		ram_size INTEGER NOT NULL,
		total_storage_tb REAL NOT NULL,
		nvme_count INTEGER NOT NULL,
		drive_count INTEGER NOT NULL,
		datacenter TEXT NOT NULL,
		price REAL NOT NULL,
		score REAL NOT NULL,
		scanned_at DATETIME NOT NULL DEFAULT (datetime('now'))
	);
	CREATE INDEX IF NOT EXISTS idx_scans_cpu ON scans(cpu);
	CREATE INDEX IF NOT EXISTS idx_scans_scanned_at ON scans(scanned_at);
	CREATE INDEX IF NOT EXISTS idx_scans_server_id ON scans(server_id);
	CREATE TABLE IF NOT EXISTS order_attempts (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		server_id INTEGER NOT NULL,
		score REAL NOT NULL,
		price REAL NOT NULL,
		success INTEGER NOT NULL DEFAULT 0,
		message TEXT NOT NULL DEFAULT '',
		attempted_at DATETIME NOT NULL DEFAULT (datetime('now'))
	);
	`
	_, err := s.db.Exec(schema)
	if err != nil {
		return fmt.Errorf("creating schema: %w", err)
	}

	// Migration: add breakdown column if it doesn't exist
	var hasBreakdown bool
	rows, err := s.db.Query("PRAGMA table_info(scans)")
	if err != nil {
		return fmt.Errorf("checking scans columns: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var cid int
		var name, colType string
		var notNull, pk int
		var dflt sql.NullString
		if err := rows.Scan(&cid, &name, &colType, &notNull, &dflt, &pk); err != nil {
			return fmt.Errorf("scanning column info: %w", err)
		}
		if name == "breakdown" {
			hasBreakdown = true
		}
	}
	if !hasBreakdown {
		if _, err := s.db.Exec("ALTER TABLE scans ADD COLUMN breakdown TEXT NOT NULL DEFAULT '{}'"); err != nil {
			return fmt.Errorf("adding breakdown column: %w", err)
		}
	}

	// Migration: add Hetzner extra fields to scans
	{
		cols := map[string]string{
			"is_ecc":      "INTEGER DEFAULT 0",
			"setup_price": "REAL DEFAULT 0",
			"next_reduce": "INTEGER DEFAULT 0",
			"fixed_price": "INTEGER DEFAULT 0",
			"bandwidth":   "INTEGER DEFAULT 0",
		}
		existingCols := make(map[string]bool)
		colRows, err := s.db.Query("PRAGMA table_info(scans)")
		if err != nil {
			return fmt.Errorf("checking scans columns for extra fields: %w", err)
		}
		for colRows.Next() {
			var cid int
			var name, colType string
			var notNull, pk int
			var dflt sql.NullString
			if err := colRows.Scan(&cid, &name, &colType, &notNull, &dflt, &pk); err != nil {
				colRows.Close()
				return fmt.Errorf("scanning column info: %w", err)
			}
			existingCols[name] = true
		}
		colRows.Close()

		for col, def := range cols {
			if !existingCols[col] {
				if _, err := s.db.Exec("ALTER TABLE scans ADD COLUMN " + col + " " + def); err != nil {
					return fmt.Errorf("adding %s column: %w", col, err)
				}
			}
		}
	}

	// Migration: create server_tracker table
	var hasTracker bool
	{
		row := s.db.QueryRow("SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name='server_tracker'")
		var count int
		if err := row.Scan(&count); err == nil {
			hasTracker = count > 0
		}
	}
	if !hasTracker {
		trackerSchema := `
		CREATE TABLE server_tracker (
			server_id INTEGER PRIMARY KEY,
			cpu TEXT NOT NULL,
			price REAL NOT NULL,
			datacenter TEXT NOT NULL DEFAULT '',
			first_seen DATETIME NOT NULL DEFAULT (datetime('now')),
			last_seen DATETIME NOT NULL DEFAULT (datetime('now')),
			status TEXT NOT NULL DEFAULT 'active',
			consecutive_misses INTEGER NOT NULL DEFAULT 0
		);
		CREATE INDEX IF NOT EXISTS idx_tracker_status ON server_tracker(status);
		CREATE INDEX IF NOT EXISTS idx_tracker_cpu ON server_tracker(cpu);
		`
		if _, err := s.db.Exec(trackerSchema); err != nil {
			return fmt.Errorf("creating server_tracker table: %w", err)
		}
	}

	return nil
}

func (s *SQLiteStore) SaveScan(servers []scorer.ScoredServer) error {
	tx, err := s.db.Begin()
	if err != nil {
		return fmt.Errorf("beginning transaction: %w", err)
	}
	defer tx.Rollback() //nolint:errcheck

	stmt, err := tx.Prepare(`
		INSERT INTO scans (server_id, cpu, ram_size, total_storage_tb, nvme_count, drive_count, datacenter, price, score, scanned_at, breakdown, is_ecc, setup_price, next_reduce, fixed_price, bandwidth)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`)
	if err != nil {
		return fmt.Errorf("preparing statement: %w", err)
	}
	defer stmt.Close()

	now := time.Now().UTC().Format("2006-01-02 15:04:05")
	for i := range servers {
		bdJSON, _ := json.Marshal(servers[i].Breakdown)
		eccInt := 0
		if servers[i].Server.IsECC {
			eccInt = 1
		}
		fixedInt := 0
		if servers[i].Server.FixedPrice {
			fixedInt = 1
		}
		_, err := stmt.Exec(
			servers[i].Server.ID,
			servers[i].Server.CPU,
			servers[i].Server.RAMSize,
			servers[i].Server.TotalStorageTB,
			servers[i].Server.NVMeCount,
			servers[i].Server.DriveCount,
			servers[i].Server.Datacenter,
			servers[i].Server.Price,
			servers[i].Score,
			now,
			string(bdJSON),
			eccInt,
			servers[i].Server.SetupPrice,
			servers[i].Server.NextReduce,
			fixedInt,
			servers[i].Server.Bandwidth,
		)
		if err != nil {
			return fmt.Errorf("inserting scan for server %d: %w", servers[i].Server.ID, err)
		}
	}

	return tx.Commit()
}

func (s *SQLiteStore) GetHistory(cpuModel string, limit int) ([]ScanRecord, error) {
	if limit <= 0 {
		limit = 50
	}

	rows, err := s.db.Query(`
		SELECT id, server_id, cpu, ram_size, total_storage_tb, nvme_count, drive_count, datacenter, price, score, scanned_at, breakdown, is_ecc, setup_price, next_reduce, fixed_price, bandwidth
		FROM scans
		WHERE cpu LIKE ?
		ORDER BY scanned_at DESC
		LIMIT ?
	`, "%"+cpuModel+"%", limit)
	if err != nil {
		return nil, fmt.Errorf("querying history: %w", err)
	}
	defer rows.Close()

	var records []ScanRecord
	for rows.Next() {
		var r ScanRecord
		var scannedAt string
		var eccInt, fixedInt int
		err := rows.Scan(&r.ID, &r.ServerID, &r.CPU, &r.RAMSize, &r.TotalStorageTB,
			&r.NVMeCount, &r.DriveCount, &r.Datacenter, &r.Price, &r.Score, &scannedAt, &r.BreakdownJSON,
			&eccInt, &r.SetupPrice, &r.NextReduce, &fixedInt, &r.Bandwidth)
		if err != nil {
			return nil, fmt.Errorf("scanning row: %w", err)
		}
		r.IsECC = eccInt == 1
		r.FixedPrice = fixedInt == 1
		r.ScannedAt = parseTimestamp(scannedAt)
		records = append(records, r)
	}
	return records, rows.Err()
}

func (s *SQLiteStore) GetStats(cpuModel string) (*PriceStats, error) {
	row := s.db.QueryRow(`
		SELECT cpu, COUNT(*), MIN(price), MAX(price), AVG(price), MIN(scanned_at), MAX(scanned_at)
		FROM scans
		WHERE cpu LIKE ?
		GROUP BY cpu
		LIMIT 1
	`, "%"+cpuModel+"%")

	var stats PriceStats
	var firstSeen, lastSeen string
	err := row.Scan(&stats.CPU, &stats.Count, &stats.MinPrice, &stats.MaxPrice,
		&stats.AvgPrice, &firstSeen, &lastSeen)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("querying stats: %w", err)
	}

	stats.FirstSeen = parseTimestamp(firstSeen)
	stats.LastSeen = parseTimestamp(lastSeen)

	return &stats, nil
}

func (s *SQLiteStore) GetAllCPUStats() (map[string]*PriceStats, error) {
	rows, err := s.db.Query(`
		SELECT cpu, COUNT(*), MIN(price), MAX(price), AVG(price), MIN(scanned_at), MAX(scanned_at)
		FROM scans
		GROUP BY cpu
	`)
	if err != nil {
		return nil, fmt.Errorf("querying all CPU stats: %w", err)
	}
	defer rows.Close()

	result := make(map[string]*PriceStats)
	for rows.Next() {
		var stats PriceStats
		var firstSeen, lastSeen string
		err := rows.Scan(&stats.CPU, &stats.Count, &stats.MinPrice, &stats.MaxPrice,
			&stats.AvgPrice, &firstSeen, &lastSeen)
		if err != nil {
			return nil, fmt.Errorf("scanning CPU stats row: %w", err)
		}
		stats.FirstSeen = parseTimestamp(firstSeen)
		stats.LastSeen = parseTimestamp(lastSeen)
		result[stats.CPU] = &stats
	}
	return result, rows.Err()
}

func (s *SQLiteStore) GetByServerID(serverID int) (*ScanRecord, error) {
	row := s.db.QueryRow(`
		SELECT id, server_id, cpu, ram_size, total_storage_tb, nvme_count, drive_count, datacenter, price, score, scanned_at, breakdown, is_ecc, setup_price, next_reduce, fixed_price, bandwidth
		FROM scans
		WHERE server_id = ?
		ORDER BY scanned_at DESC
		LIMIT 1
	`, serverID)

	var r ScanRecord
	var scannedAt string
	var eccInt, fixedInt int
	err := row.Scan(&r.ID, &r.ServerID, &r.CPU, &r.RAMSize, &r.TotalStorageTB,
		&r.NVMeCount, &r.DriveCount, &r.Datacenter, &r.Price, &r.Score, &scannedAt, &r.BreakdownJSON,
		&eccInt, &r.SetupPrice, &r.NextReduce, &fixedInt, &r.Bandwidth)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("querying server %d: %w", serverID, err)
	}
	r.IsECC = eccInt == 1
	r.FixedPrice = fixedInt == 1
	r.ScannedAt = parseTimestamp(scannedAt)
	return &r, nil
}

func (s *SQLiteStore) GetOrderAttempts(limit int) ([]OrderAttempt, error) {
	if limit <= 0 {
		limit = 100
	}
	rows, err := s.db.Query(`
		SELECT id, server_id, score, price, success, message, attempted_at
		FROM order_attempts
		ORDER BY attempted_at DESC
		LIMIT ?
	`, limit)
	if err != nil {
		return nil, fmt.Errorf("querying order attempts: %w", err)
	}
	defer rows.Close()

	var orders []OrderAttempt
	for rows.Next() {
		var o OrderAttempt
		var successInt int
		var attemptedAt string
		if err := rows.Scan(&o.ID, &o.ServerID, &o.Score, &o.Price, &successInt, &o.Message, &attemptedAt); err != nil {
			return nil, fmt.Errorf("scanning order row: %w", err)
		}
		o.Success = successInt == 1
		o.AttemptedAt = parseTimestamp(attemptedAt)
		orders = append(orders, o)
	}
	return orders, rows.Err()
}

func (s *SQLiteStore) PruneOldScans(retentionDays int) (int64, error) {
	if retentionDays <= 0 {
		return 0, nil
	}
	result, err := s.db.Exec(`
		DELETE FROM scans WHERE scanned_at < datetime('now', ? || ' days')
	`, -retentionDays)
	if err != nil {
		return 0, fmt.Errorf("pruning old scans: %w", err)
	}
	return result.RowsAffected()
}

func (s *SQLiteStore) SaveOrderAttempt(serverID int, score, price float64, success bool, message string) error {
	successInt := 0
	if success {
		successInt = 1
	}
	_, err := s.db.Exec(`
		INSERT INTO order_attempts (server_id, score, price, success, message, attempted_at)
		VALUES (?, ?, ?, ?, ?, ?)
	`, serverID, score, price, successInt, message, time.Now().UTC().Format("2006-01-02 15:04:05"))
	if err != nil {
		return fmt.Errorf("saving order attempt: %w", err)
	}
	return nil
}

func (s *SQLiteStore) UpsertServerTracker(servers []ServerTrackerEntry) error {
	if len(servers) == 0 {
		return nil
	}
	tx, err := s.db.Begin()
	if err != nil {
		return fmt.Errorf("beginning tracker upsert tx: %w", err)
	}
	defer tx.Rollback() //nolint:errcheck

	stmt, err := tx.Prepare(`
		INSERT INTO server_tracker (server_id, cpu, price, datacenter, first_seen, last_seen, status, consecutive_misses)
		VALUES (?, ?, ?, ?, datetime('now'), datetime('now'), 'active', 0)
		ON CONFLICT(server_id) DO UPDATE SET
			price = excluded.price,
			last_seen = datetime('now'),
			status = 'active',
			consecutive_misses = 0
	`)
	if err != nil {
		return fmt.Errorf("preparing tracker upsert: %w", err)
	}
	defer stmt.Close()

	for _, e := range servers {
		if _, err := stmt.Exec(e.ServerID, e.CPU, e.Price, e.Datacenter); err != nil {
			return fmt.Errorf("upserting tracker for server %d: %w", e.ServerID, err)
		}
	}
	return tx.Commit()
}

func (s *SQLiteStore) MarkSoldServers(activeIDs []int) error {
	if len(activeIDs) == 0 {
		// If no active servers at all, increment all
		_, err := s.db.Exec(`
			UPDATE server_tracker
			SET consecutive_misses = consecutive_misses + 1
			WHERE status = 'active'
		`)
		if err != nil {
			return fmt.Errorf("incrementing all misses: %w", err)
		}
	} else {
		// Build a set of active IDs for the NOT IN clause
		// Use a temp approach: increment all active, then reset the ones we saw
		tx, err := s.db.Begin()
		if err != nil {
			return fmt.Errorf("beginning mark sold tx: %w", err)
		}
		defer tx.Rollback() //nolint:errcheck

		// Increment misses for all active servers
		if _, err := tx.Exec(`UPDATE server_tracker SET consecutive_misses = consecutive_misses + 1 WHERE status = 'active'`); err != nil {
			return fmt.Errorf("incrementing misses: %w", err)
		}

		// Reset misses for servers we just saw (they were already upserted with misses=0,
		// but this handles the case where MarkSoldServers is called separately)
		stmt, err := tx.Prepare(`UPDATE server_tracker SET consecutive_misses = 0 WHERE server_id = ?`)
		if err != nil {
			return fmt.Errorf("preparing reset stmt: %w", err)
		}
		defer stmt.Close()

		for _, id := range activeIDs {
			if _, err := stmt.Exec(id); err != nil {
				return fmt.Errorf("resetting misses for %d: %w", id, err)
			}
		}

		if err := tx.Commit(); err != nil {
			return fmt.Errorf("committing mark sold: %w", err)
		}
	}

	// Mark servers as sold if they've been missing for 2+ consecutive scans
	_, err := s.db.Exec(`
		UPDATE server_tracker
		SET status = 'sold', last_seen = datetime('now')
		WHERE status = 'active' AND consecutive_misses >= 2
	`)
	return err
}

func (s *SQLiteStore) GetServerTracker(serverID int) (*ServerTracker, error) {
	row := s.db.QueryRow(`
		SELECT server_id, cpu, price, datacenter, first_seen, last_seen, status, consecutive_misses
		FROM server_tracker
		WHERE server_id = ?
	`, serverID)

	var t ServerTracker
	var firstSeen, lastSeen string
	err := row.Scan(&t.ServerID, &t.CPU, &t.Price, &t.Datacenter, &firstSeen, &lastSeen, &t.Status, &t.ConsecutiveMisses)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("querying tracker for %d: %w", serverID, err)
	}
	t.FirstSeen = parseTimestamp(firstSeen)
	t.LastSeen = parseTimestamp(lastSeen)
	return &t, nil
}

func (s *SQLiteStore) GetAvgTimeOnMarket(cpu string) (float64, error) {
	query := `
		SELECT AVG((julianday(last_seen) - julianday(first_seen)) * 24)
		FROM server_tracker
		WHERE status = 'sold'
	`
	args := []any{}
	if cpu != "" {
		query += ` AND cpu LIKE ?`
		args = append(args, "%"+cpu+"%")
	}

	var avgHours sql.NullFloat64
	err := s.db.QueryRow(query, args...).Scan(&avgHours)
	if err != nil {
		return 0, fmt.Errorf("querying avg time on market: %w", err)
	}
	if !avgHours.Valid {
		return 0, nil
	}
	return avgHours.Float64, nil
}

func (s *SQLiteStore) GetMarketAnalytics() (*MarketAnalytics, error) {
	analytics := &MarketAnalytics{}

	// Brand trends: AVG price by date, grouped by AMD/Intel (last 14 days)
	brandRows, err := s.db.Query(`
		SELECT date(scanned_at) as d,
			CASE WHEN cpu LIKE '%AMD%' OR cpu LIKE '%Ryzen%' OR cpu LIKE '%EPYC%' THEN 'AMD' ELSE 'Intel' END as brand,
			AVG(price), COUNT(*)
		FROM scans
		WHERE scanned_at >= datetime('now', '-14 days')
		GROUP BY d, brand
		ORDER BY d ASC
	`)
	if err != nil {
		return nil, fmt.Errorf("querying brand trends: %w", err)
	}
	defer brandRows.Close()
	for brandRows.Next() {
		var bt BrandTrend
		if err := brandRows.Scan(&bt.Date, &bt.Brand, &bt.AvgPrice, &bt.Count); err != nil {
			return nil, fmt.Errorf("scanning brand trend: %w", err)
		}
		analytics.BrandTrends = append(analytics.BrandTrends, bt)
	}

	// DC distribution: COUNT DISTINCT server_id by datacenter (last 7 days)
	dcRows, err := s.db.Query(`
		SELECT datacenter, COUNT(DISTINCT server_id)
		FROM scans
		WHERE scanned_at >= datetime('now', '-7 days')
		GROUP BY datacenter
		ORDER BY COUNT(DISTINCT server_id) DESC
	`)
	if err != nil {
		return nil, fmt.Errorf("querying dc volume: %w", err)
	}
	defer dcRows.Close()
	for dcRows.Next() {
		var dv DCVolume
		if err := dcRows.Scan(&dv.Datacenter, &dv.Count); err != nil {
			return nil, fmt.Errorf("scanning dc volume: %w", err)
		}
		analytics.DCVolume = append(analytics.DCVolume, dv)
	}

	// Top value CPUs: AVG score DESC LIMIT 10 (last 7 days)
	cpuRows, err := s.db.Query(`
		SELECT cpu, AVG(score), AVG(price), COUNT(*)
		FROM scans
		WHERE scanned_at >= datetime('now', '-7 days')
		GROUP BY cpu
		ORDER BY AVG(score) DESC
		LIMIT 10
	`)
	if err != nil {
		return nil, fmt.Errorf("querying top value cpus: %w", err)
	}
	defer cpuRows.Close()
	for cpuRows.Next() {
		var cv CPUValue
		if err := cpuRows.Scan(&cv.CPU, &cv.AvgScore, &cv.AvgPrice, &cv.Count); err != nil {
			return nil, fmt.Errorf("scanning cpu value: %w", err)
		}
		analytics.TopValueCPUs = append(analytics.TopValueCPUs, cv)
	}

	// Price histogram: 10-euro buckets (last 24h)
	bucketRows, err := s.db.Query(`
		SELECT CAST(price/10 AS INTEGER)*10 || '-' || (CAST(price/10 AS INTEGER)*10 + 10) as bucket,
			COUNT(*)
		FROM scans
		WHERE scanned_at >= datetime('now', '-24 hours')
		GROUP BY CAST(price/10 AS INTEGER)
		ORDER BY CAST(price/10 AS INTEGER) ASC
	`)
	if err != nil {
		return nil, fmt.Errorf("querying price buckets: %w", err)
	}
	defer bucketRows.Close()
	for bucketRows.Next() {
		var pb PriceBucket
		if err := bucketRows.Scan(&pb.Bucket, &pb.Count); err != nil {
			return nil, fmt.Errorf("scanning price bucket: %w", err)
		}
		analytics.PriceBuckets = append(analytics.PriceBuckets, pb)
	}

	return analytics, nil
}

func (s *SQLiteStore) GetTopDeals(since time.Time, limit int, minScore float64) ([]ScanRecord, error) {
	if limit <= 0 {
		limit = 5
	}
	sinceStr := since.UTC().Format("2006-01-02 15:04:05")

	rows, err := s.db.Query(`
		SELECT id, server_id, cpu, ram_size, total_storage_tb, nvme_count, drive_count, datacenter, price, score, scanned_at, breakdown, is_ecc, setup_price, next_reduce, fixed_price, bandwidth
		FROM scans
		WHERE scanned_at >= ? AND score >= ?
		GROUP BY server_id
		HAVING score = MAX(score)
		ORDER BY score DESC
		LIMIT ?
	`, sinceStr, minScore, limit)
	if err != nil {
		return nil, fmt.Errorf("querying top deals: %w", err)
	}
	defer rows.Close()

	var records []ScanRecord
	for rows.Next() {
		var r ScanRecord
		var scannedAt string
		var eccInt, fixedInt int
		err := rows.Scan(&r.ID, &r.ServerID, &r.CPU, &r.RAMSize, &r.TotalStorageTB,
			&r.NVMeCount, &r.DriveCount, &r.Datacenter, &r.Price, &r.Score, &scannedAt, &r.BreakdownJSON,
			&eccInt, &r.SetupPrice, &r.NextReduce, &fixedInt, &r.Bandwidth)
		if err != nil {
			return nil, fmt.Errorf("scanning top deal row: %w", err)
		}
		r.IsECC = eccInt == 1
		r.FixedPrice = fixedInt == 1
		r.ScannedAt = parseTimestamp(scannedAt)
		records = append(records, r)
	}
	return records, rows.Err()
}

// parseTimestamp parses a timestamp string trying the known write format first.
func parseTimestamp(s string) time.Time {
	// Our SaveScan writes "2006-01-02 15:04:05" — try this first
	t, err := time.Parse("2006-01-02 15:04:05", s)
	if err == nil {
		return t
	}
	t, err = time.Parse("2006-01-02T15:04:05Z", s)
	if err == nil {
		return t
	}
	return time.Time{}
}

func (s *SQLiteStore) Close() error {
	return s.db.Close()
}
