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
	BreakdownJSON  string  // JSON-encoded scorer.Breakdown
	IsECC          bool    `json:"is_ecc"`
	SetupPrice     float64 `json:"setup_price"`
	NextReduce     int     `json:"next_reduce"`
	FixedPrice     bool    `json:"fixed_price"`
	Bandwidth      int     `json:"bandwidth"`
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

// ServerTracker tracks how long individual servers stay listed on the auction.
type ServerTracker struct {
	ServerID          int       `json:"server_id"`
	CPU               string    `json:"cpu"`
	Price             float64   `json:"price"`
	Datacenter        string    `json:"datacenter"`
	FirstSeen         time.Time `json:"first_seen"`
	LastSeen          time.Time `json:"last_seen"`
	Status            string    `json:"status"` // "active" or "sold"
	ConsecutiveMisses int       `json:"-"`
}

// ServerTrackerEntry is the input for UpsertServerTracker.
type ServerTrackerEntry struct {
	ServerID   int
	CPU        string
	Price      float64
	Datacenter string
}

// MarketAnalytics contains aggregated market intelligence.
type MarketAnalytics struct {
	BrandTrends  []BrandTrend  `json:"brand_trends"`
	DCVolume     []DCVolume    `json:"dc_volume"`
	TopValueCPUs []CPUValue    `json:"top_value_cpus"`
	PriceBuckets []PriceBucket `json:"price_buckets"`
}

type BrandTrend struct {
	Date     string  `json:"date"`
	Brand    string  `json:"brand"`
	AvgPrice float64 `json:"avg_price"`
	Count    int     `json:"count"`
}

type DCVolume struct {
	Datacenter string `json:"datacenter"`
	Count      int    `json:"count"`
}

type CPUValue struct {
	CPU      string  `json:"cpu"`
	AvgScore float64 `json:"avg_score"`
	AvgPrice float64 `json:"avg_price"`
	Count    int     `json:"count"`
}

type PriceBucket struct {
	Bucket string `json:"bucket"`
	Count  int    `json:"count"`
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
	UpsertServerTracker(servers []ServerTrackerEntry) error
	MarkSoldServers(activeIDs []int) error
	GetServerTracker(serverID int) (*ServerTracker, error)
	GetAvgTimeOnMarket(cpu string) (float64, error) // returns hours
	GetMarketAnalytics() (*MarketAnalytics, error)
	GetTopDeals(since time.Time, limit int, minScore float64) ([]ScanRecord, error)
	Close() error
}
