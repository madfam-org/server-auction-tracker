package scanner

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/madfam-org/server-auction-tracker/internal/config"
	cpupkg "github.com/madfam-org/server-auction-tracker/internal/cpu"
	log "github.com/sirupsen/logrus"
)

const defaultURL = "https://www.hetzner.com/_resources/app/data/app/live_data_sb_EUR.json"

type ServerDiskData struct {
	NVMe    []int `json:"nvme"`
	SATA    []int `json:"sata"`
	HDD     []int `json:"hdd"`
	General []int `json:"general"`
}

type Server struct {
	ID             int            `json:"id"`
	Key            int            `json:"key"`
	Name           string         `json:"name"`
	CPU            string         `json:"cpu"`
	CPUCount       int            `json:"cpu_count"` // socket count (usually 1)
	RAMSize        int            `json:"ram_size"`
	HDDs           []string       `json:"hdd_arr"`
	HDDCount       int            `json:"hdd_count"`
	HDDSize        int            `json:"hdd_size"`
	DiskData       ServerDiskData `json:"serverDiskData"`
	Datacenter     string         `json:"datacenter"`
	Price          float64        `json:"price"`
	SetupPrice     float64        `json:"setup_price"`
	Specials       []string       `json:"specials"`
	IsECC          bool           `json:"is_ecc"`
	Traffic        string         `json:"traffic"`
	Bandwidth      int            `json:"bandwidth"`
	NextReduce     int            `json:"next_reduce"`
	FixedPrice     bool           `json:"fixed_price"`

	// Parsed/derived fields
	TotalStorageTB float64
	NVMeCount      int
	DriveCount     int
	ParsedCores    int
	ParsedThreads  int
}

type hetznerResponse struct {
	Server []Server `json:"server"`
}

type HTTPClient interface {
	Do(req *http.Request) (*http.Response, error)
}

type Scanner struct {
	url    string
	client HTTPClient
}

func New(client HTTPClient) *Scanner {
	if client == nil {
		client = NewRetryClient(nil, 3, 500*time.Millisecond)
	}
	return &Scanner{
		url:    defaultURL,
		client: client,
	}
}

func NewWithURL(url string, client HTTPClient) *Scanner {
	s := New(client)
	s.url = url
	return s
}

func (s *Scanner) Fetch() ([]Server, error) {
	req, err := http.NewRequest("GET", s.url, nil)
	if err != nil {
		return nil, fmt.Errorf("creating request: %w", err)
	}
	req.Header.Set("User-Agent", "foundry-scout/1.0")

	resp, err := s.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetching auction data: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("reading response body: %w", err)
	}

	var data hetznerResponse
	if err := json.Unmarshal(body, &data); err != nil {
		return nil, fmt.Errorf("parsing JSON: %w", err)
	}

	for i := range data.Server {
		enrichServer(&data.Server[i])
	}

	log.WithField("count", len(data.Server)).Info("Fetched auction listings")
	return data.Server, nil
}

func enrichServer(s *Server) {
	// Use serverDiskData for accurate NVMe detection
	s.NVMeCount = len(s.DiskData.NVMe)
	s.DriveCount = len(s.DiskData.NVMe) + len(s.DiskData.SATA) + len(s.DiskData.HDD)

	// Compute total storage from disk data
	for _, size := range s.DiskData.NVMe {
		s.TotalStorageTB += float64(size) / 1024.0
	}
	for _, size := range s.DiskData.SATA {
		s.TotalStorageTB += float64(size) / 1024.0
	}
	for _, size := range s.DiskData.HDD {
		s.TotalStorageTB += float64(size) / 1024.0
	}

	// Parse CPU info from model string for cores/threads
	cpuInfo := cpupkg.Parse(s.CPU, 0, 0, 0)
	s.ParsedCores = cpuInfo.Cores
	s.ParsedThreads = cpuInfo.Threads

	// Fallback if CPU parser didn't extract cores
	if s.ParsedCores == 0 {
		s.ParsedCores = estimateCoresFromModel(s.CPU)
		s.ParsedThreads = s.ParsedCores * 2
	}
}

// estimateCoresFromModel provides fallback core estimates for common CPUs.
func estimateCoresFromModel(model string) int {
	upper := strings.ToUpper(model)
	switch {
	// AMD Ryzen
	case strings.Contains(upper, "RYZEN 9 7950"):
		return 16
	case strings.Contains(upper, "RYZEN 9 5950"), strings.Contains(upper, "RYZEN 9 3950"):
		return 16
	case strings.Contains(upper, "RYZEN 9 5900"), strings.Contains(upper, "RYZEN 9 3900"):
		return 12
	case strings.Contains(upper, "RYZEN 7"):
		return 8
	case strings.Contains(upper, "RYZEN 5"):
		return 6
	// Intel 13th gen
	case strings.Contains(upper, "I9-13"):
		return 24
	case strings.Contains(upper, "I7-13"):
		return 16
	case strings.Contains(upper, "I5-13"):
		return 14
	// Intel 12th gen
	case strings.Contains(upper, "I7-12"):
		return 12
	case strings.Contains(upper, "I5-12"):
		return 10
	// Older Intel
	case strings.Contains(upper, "I7-8"), strings.Contains(upper, "I7-9"):
		return 8
	case strings.Contains(upper, "I7-6"), strings.Contains(upper, "I7-7"):
		return 4
	case strings.Contains(upper, "I5-"):
		return 6
	case strings.Contains(upper, "XEON"):
		return 8 // conservative default
	default:
		return 4
	}
}

func (s *Scanner) Filter(servers []Server, filters config.Filters) []Server {
	var result []Server
	for _, srv := range servers {
		if !passesFilters(srv, filters) {
			continue
		}
		result = append(result, srv)
	}
	log.WithFields(log.Fields{
		"total":    len(servers),
		"filtered": len(result),
	}).Info("Applied filters")
	return result
}

func passesFilters(s Server, f config.Filters) bool {
	if s.RAMSize < f.MinRAMGB {
		return false
	}
	if f.MinCPUCores > 0 && s.ParsedCores < f.MinCPUCores {
		return false
	}
	if s.DriveCount < f.MinDrives {
		return false
	}
	if f.MaxPriceEUR > 0 && s.Price > f.MaxPriceEUR {
		return false
	}
	if f.DatacenterPrefix != "" && !strings.HasPrefix(s.Datacenter, f.DatacenterPrefix) {
		return false
	}
	if f.MinDriveSizeGB > 0 {
		hasQualifyingDrive := false
		for _, size := range s.DiskData.NVMe {
			if size >= f.MinDriveSizeGB {
				hasQualifyingDrive = true
				break
			}
		}
		if !hasQualifyingDrive {
			for _, size := range s.DiskData.SATA {
				if size >= f.MinDriveSizeGB {
					hasQualifyingDrive = true
					break
				}
			}
		}
		if !hasQualifyingDrive {
			for _, size := range s.DiskData.HDD {
				if size >= f.MinDriveSizeGB {
					hasQualifyingDrive = true
					break
				}
			}
		}
		if !hasQualifyingDrive {
			return false
		}
	}
	return true
}
