package store

import (
	"database/sql"
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
	`
	_, err := s.db.Exec(schema)
	if err != nil {
		return fmt.Errorf("creating schema: %w", err)
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
		INSERT INTO scans (server_id, cpu, ram_size, total_storage_tb, nvme_count, drive_count, datacenter, price, score, scanned_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`)
	if err != nil {
		return fmt.Errorf("preparing statement: %w", err)
	}
	defer stmt.Close()

	now := time.Now().UTC().Format("2006-01-02 15:04:05")
	for _, ss := range servers {
		_, err := stmt.Exec(
			ss.Server.ID,
			ss.Server.CPU,
			ss.Server.RAMSize,
			ss.Server.TotalStorageTB,
			ss.Server.NVMeCount,
			ss.Server.DriveCount,
			ss.Server.Datacenter,
			ss.Server.Price,
			ss.Score,
			now,
		)
		if err != nil {
			return fmt.Errorf("inserting scan for server %d: %w", ss.Server.ID, err)
		}
	}

	return tx.Commit()
}

func (s *SQLiteStore) GetHistory(cpuModel string, limit int) ([]ScanRecord, error) {
	if limit <= 0 {
		limit = 50
	}

	rows, err := s.db.Query(`
		SELECT id, server_id, cpu, ram_size, total_storage_tb, nvme_count, drive_count, datacenter, price, score, scanned_at
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
		err := rows.Scan(&r.ID, &r.ServerID, &r.CPU, &r.RAMSize, &r.TotalStorageTB,
			&r.NVMeCount, &r.DriveCount, &r.Datacenter, &r.Price, &r.Score, &scannedAt)
		if err != nil {
			return nil, fmt.Errorf("scanning row: %w", err)
		}
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
