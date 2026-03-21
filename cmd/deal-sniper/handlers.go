package main

import (
	"fmt"
	"time"

	"github.com/madfam-org/server-auction-tracker/internal/scanner"
	"github.com/madfam-org/server-auction-tracker/internal/store"
)

// scanRecordToServer converts a ScanRecord back into a scanner.Server
// with enough data for the simulate package.
func scanRecordToServer(r store.ScanRecord) scanner.Server {
	return scanner.Server{
		ID:             r.ServerID,
		CPU:            r.CPU,
		RAMSize:        r.RAMSize,
		TotalStorageTB: r.TotalStorageTB,
		NVMeCount:      r.NVMeCount,
		DriveCount:     r.DriveCount,
		Datacenter:     r.Datacenter,
		Price:          r.Price,
		ParsedCores:    estimateCoresFromRAM(r.RAMSize),
	}
}

// estimateCoresFromRAM provides a rough core estimate when we only have scan records.
// Hetzner servers with 128GB+ typically have 16+ cores.
func estimateCoresFromRAM(ramGB int) int {
	switch {
	case ramGB >= 256:
		return 32
	case ramGB >= 128:
		return 16
	case ramGB >= 64:
		return 8
	default:
		return 4
	}
}

// getOrderAttempts queries the order_attempts table directly.
// The Store interface doesn't expose a read method, so we access the underlying DB.
func getOrderAttempts(s *store.SQLiteStore) ([]orderAttempt, error) {
	rows, err := s.DB().Query(`
		SELECT id, server_id, score, price, success, message, attempted_at
		FROM order_attempts
		ORDER BY attempted_at DESC
		LIMIT 100
	`)
	if err != nil {
		return nil, fmt.Errorf("querying order attempts: %w", err)
	}
	defer rows.Close()

	var orders []orderAttempt
	for rows.Next() {
		var o orderAttempt
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

// parseTimestamp parses timestamps in the formats used by the store.
func parseTimestamp(s string) time.Time {
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
