package simulate

import (
	"testing"

	"github.com/madfam-org/server-auction-tracker/internal/config"
	"github.com/madfam-org/server-auction-tracker/internal/scanner"
	"github.com/stretchr/testify/assert"
)

func TestSimulateWithRealClusterValues(t *testing.T) {
	// Values from capacity.md: 2-node k3s cluster
	cluster := config.Cluster{
		CPUMillicores:  12000,
		CPURequested:   10460,
		RAMGB:          64,
		RAMRequestedGB: 25,
		DiskGB:         98,
		DiskUsedGB:     77,
		Nodes:          2,
	}

	server := scanner.Server{
		ID:             2873962,
		CPU:            "Intel Core i9-13900",
		RAMSize:        64,
		TotalStorageTB: 3.8,
		ParsedCores:    24,
		ParsedThreads:  32,
		Price:          85.00,
		Datacenter:     "HEL1-DC7",
	}

	r := Simulate(&cluster, &server, 55.00)

	// CPU: 10460 / 12000 = 87.2% -> 10460 / (12000 + 24000) = 29.1%
	assert.InDelta(t, 87.2, r.CPUBefore, 0.1)
	assert.InDelta(t, 29.1, r.CPUAfter, 0.1)

	// RAM: 25/64 = 39.1% -> 25/128 = 19.5%
	assert.InDelta(t, 39.1, r.RAMBefore, 0.1)
	assert.InDelta(t, 19.5, r.RAMAfter, 0.1)

	// Disk: 77/98 = 78.6% -> 77 / (98 + 3891) = ~1.9%
	assert.InDelta(t, 78.6, r.DiskBefore, 0.1)
	assert.True(t, r.DiskAfter < 5.0)

	assert.Equal(t, 2, r.NodesBefore)
	assert.Equal(t, 3, r.NodesAfter)
	assert.Equal(t, 55.0, r.MonthlyCostBefore)
	assert.Equal(t, 140.0, r.MonthlyCostAfter)

	// Disk has the biggest drop (78.6 → ~1.9)
	assert.Equal(t, "Disk", r.Bottleneck)
}

func TestSimulateSmallServer(t *testing.T) {
	cluster := config.Cluster{
		CPUMillicores:  12000,
		CPURequested:   10460,
		RAMGB:          64,
		RAMRequestedGB: 25,
		DiskGB:         98,
		DiskUsedGB:     77,
		Nodes:          2,
	}

	server := scanner.Server{
		ID:             1001,
		CPU:            "AMD Ryzen 5 3600",
		RAMSize:        64,
		TotalStorageTB: 1.0,
		ParsedCores:    6,
		Price:          39.00,
	}

	r := Simulate(&cluster, &server, 55.00)

	// CPU: 10460/12000 = 87.2% -> 10460/18000 = 58.1%
	assert.InDelta(t, 87.2, r.CPUBefore, 0.1)
	assert.InDelta(t, 58.1, r.CPUAfter, 0.1)
	assert.Equal(t, 94.0, r.MonthlyCostAfter)
}

func TestHealthLabel(t *testing.T) {
	assert.Equal(t, "CRITICAL", HealthLabel(90))
	assert.Equal(t, "CRITICAL", HealthLabel(85))
	assert.Equal(t, "WARNING", HealthLabel(75))
	assert.Equal(t, "MODERATE", HealthLabel(60))
	assert.Equal(t, "HEALTHY", HealthLabel(40))
	assert.Equal(t, "HEALTHY", HealthLabel(0))
}
