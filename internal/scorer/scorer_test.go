package scorer

import (
	"testing"

	"github.com/madfam-org/server-auction-tracker/internal/config"
	"github.com/madfam-org/server-auction-tracker/internal/scanner"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestScoreEmpty(t *testing.T) {
	result := Score(nil, config.Scoring{}, "")
	assert.Nil(t, result)
}

func TestScoreSingleServer(t *testing.T) {
	servers := []scanner.Server{
		{
			ID:             1001,
			CPU:            "AMD Ryzen 5 3600 6-Core Processor",
			CPUCount:       1,
			RAMSize:        64,
			Price:         39.00,
			Datacenter:     "HEL1-DC7",
			HDDs:           []string{"512 GB NVMe SSD", "512 GB NVMe SSD"},
			DriveCount:     2,
			NVMeCount:      2,
			TotalStorageTB: 1.0,
			ParsedCores:    6,
			ParsedThreads:  12,
		},
	}

	scoring := config.Scoring{
		CPUWeight:      0.30,
		RAMWeight:      0.25,
		StorageWeight:  0.20,
		NVMeWeight:     0.10,
		CPUGenWeight:   0.10,
		LocalityWeight: 0.05,
	}

	result := Score(servers, scoring, "HEL1")
	require.Len(t, result, 1)
	assert.Greater(t, result[0].Score, 0.0)
	assert.LessOrEqual(t, result[0].Score, 100.0)
	assert.Equal(t, 1.0, result[0].Breakdown.LocalityBonus)
	assert.Equal(t, 1.0, result[0].Breakdown.NVMeBonus)
}

func TestScoreOrdering(t *testing.T) {
	servers := []scanner.Server{
		{
			ID:             1,
			CPU:            "Intel Xeon E-2136",
			CPUCount:       4,
			RAMSize:        32,
			Price:         60.00,
			Datacenter:     "FSN1-DC14",
			HDDs:           []string{"256 GB SATA SSD"},
			DriveCount:     1,
			NVMeCount:      0,
			TotalStorageTB: 0.25,
			ParsedCores:    4,
			ParsedThreads:  8,
		},
		{
			ID:             2,
			CPU:            "AMD Ryzen 5 3600 6-Core Processor",
			CPUCount:       6,
			RAMSize:        64,
			Price:         39.00,
			Datacenter:     "HEL1-DC7",
			HDDs:           []string{"512 GB NVMe SSD", "512 GB NVMe SSD"},
			DriveCount:     2,
			NVMeCount:      2,
			TotalStorageTB: 1.0,
			ParsedCores:    6,
			ParsedThreads:  12,
		},
	}

	scoring := config.Scoring{
		CPUWeight:      0.30,
		RAMWeight:      0.25,
		StorageWeight:  0.20,
		NVMeWeight:     0.10,
		CPUGenWeight:   0.10,
		LocalityWeight: 0.05,
	}

	result := Score(servers, scoring, "HEL1")
	require.Len(t, result, 2)
	// Ryzen server should score higher (more RAM, NVMe, lower price, HEL1 match)
	assert.Equal(t, 2, result[0].Server.ID)
	assert.Greater(t, result[0].Score, result[1].Score)
}

func TestScoreLocalityBonus(t *testing.T) {
	servers := []scanner.Server{
		{
			ID:             1,
			CPU:            "AMD Ryzen 5 3600",
			CPUCount:       6,
			RAMSize:        64,
			Price:         40.00,
			Datacenter:     "HEL1-DC7",
			HDDs:           []string{"512 GB NVMe SSD"},
			DriveCount:     1,
			NVMeCount:      1,
			TotalStorageTB: 0.5,
		},
	}

	scoring := config.Scoring{LocalityWeight: 0.05}

	// With matching prefix
	result := Score(servers, scoring, "HEL1")
	assert.Equal(t, 1.0, result[0].Breakdown.LocalityBonus)

	// Without matching prefix
	result = Score(servers, scoring, "FSN1")
	assert.Equal(t, 0.0, result[0].Breakdown.LocalityBonus)
}
