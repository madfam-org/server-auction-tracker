package simulate

import (
	"github.com/madfam-org/server-auction-tracker/internal/config"
	"github.com/madfam-org/server-auction-tracker/internal/scanner"
)

// Result holds the before/after comparison of adding a server to the cluster.
type Result struct {
	Server scanner.Server

	CPUBefore  float64
	CPUAfter   float64
	RAMBefore  float64
	RAMAfter   float64
	DiskBefore float64
	DiskAfter  float64

	NodesBefore int
	NodesAfter  int

	MonthlyCostBefore float64
	MonthlyCostAfter  float64

	Bottleneck string
}

// Simulate calculates the impact of adding a server to the cluster.
func Simulate(cluster config.Cluster, server scanner.Server, currentMonthlyCost float64) *Result {
	r := &Result{
		Server:      server,
		NodesBefore: cluster.Nodes,
		NodesAfter:  cluster.Nodes + 1,
	}

	// CPU utilization (millicores)
	if cluster.CPUMillicores > 0 {
		r.CPUBefore = float64(cluster.CPURequested) / float64(cluster.CPUMillicores) * 100
		// Estimate server CPU capacity: cores * 1000 millicores
		newCPU := cluster.CPUMillicores + server.ParsedCores*1000
		r.CPUAfter = float64(cluster.CPURequested) / float64(newCPU) * 100
	}

	// RAM utilization
	if cluster.RAMGB > 0 {
		r.RAMBefore = float64(cluster.RAMRequestedGB) / float64(cluster.RAMGB) * 100
		newRAM := cluster.RAMGB + server.RAMSize
		r.RAMAfter = float64(cluster.RAMRequestedGB) / float64(newRAM) * 100
	}

	// Disk utilization
	if cluster.DiskGB > 0 {
		r.DiskBefore = float64(cluster.DiskUsedGB) / float64(cluster.DiskGB) * 100
		serverDiskGB := int(server.TotalStorageTB * 1024)
		newDisk := cluster.DiskGB + serverDiskGB
		r.DiskAfter = float64(cluster.DiskUsedGB) / float64(newDisk) * 100
	}

	// Monthly cost
	r.MonthlyCostBefore = currentMonthlyCost
	r.MonthlyCostAfter = currentMonthlyCost + server.Price

	// Determine which bottleneck is most relieved
	cpuDrop := r.CPUBefore - r.CPUAfter
	ramDrop := r.RAMBefore - r.RAMAfter
	diskDrop := r.DiskBefore - r.DiskAfter

	switch {
	case cpuDrop >= ramDrop && cpuDrop >= diskDrop:
		r.Bottleneck = "CPU"
	case ramDrop >= cpuDrop && ramDrop >= diskDrop:
		r.Bottleneck = "RAM"
	default:
		r.Bottleneck = "Disk"
	}

	return r
}

// HealthLabel returns a status label for a utilization percentage.
func HealthLabel(pct float64) string {
	switch {
	case pct >= 85:
		return "CRITICAL"
	case pct >= 70:
		return "WARNING"
	case pct >= 50:
		return "MODERATE"
	default:
		return "HEALTHY"
	}
}
