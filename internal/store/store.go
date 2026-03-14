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

type Store interface {
	Init() error
	SaveScan(servers []scorer.ScoredServer) error
	GetHistory(cpuModel string, limit int) ([]ScanRecord, error)
	GetStats(cpuModel string) (*PriceStats, error)
	GetAllCPUStats() (map[string]*PriceStats, error)
	Close() error
}
