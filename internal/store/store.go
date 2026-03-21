package store

import (
	"time"

	"github.com/madfam-org/server-auction-tracker/internal/scorer"
)

type ScanRecord struct {
	ID             int
	ServerID       int
	CPU            string
	RAMSize        int
	TotalStorageTB float64
	NVMeCount      int
	DriveCount     int
	Datacenter     string
	Price          float64
	Score          float64
	ScannedAt      time.Time
	BreakdownJSON  string // JSON-encoded scorer.Breakdown
}

type PriceStats struct {
	CPU       string
	Count     int
	MinPrice  float64
	MaxPrice  float64
	AvgPrice  float64
	LastSeen  time.Time
	FirstSeen time.Time
}

// OrderAttempt represents a single server order attempt record.
type OrderAttempt struct {
	ID          int       `json:"id"`
	ServerID    int       `json:"server_id"`
	Score       float64   `json:"score"`
	Price       float64   `json:"price"`
	Success     bool      `json:"success"`
	Message     string    `json:"message"`
	AttemptedAt time.Time `json:"attempted_at"`
}

type Store interface {
	Init() error
	SaveScan(servers []scorer.ScoredServer) error
	GetHistory(cpuModel string, limit int) ([]ScanRecord, error)
	GetByServerID(serverID int) (*ScanRecord, error)
	GetStats(cpuModel string) (*PriceStats, error)
	GetAllCPUStats() (map[string]*PriceStats, error)
	SaveOrderAttempt(serverID int, score, price float64, success bool, message string) error
	GetOrderAttempts(limit int) ([]OrderAttempt, error)
	PruneOldScans(retentionDays int) (int64, error)
	Close() error
}
