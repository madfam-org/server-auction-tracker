package scorer

import (
	"math"
	"sort"

	"github.com/madfam-org/server-auction-tracker/internal/config"
	cpupkg "github.com/madfam-org/server-auction-tracker/internal/cpu"
	"github.com/madfam-org/server-auction-tracker/internal/scanner"
)

type ScoredServer struct {
	Server scanner.Server
	Score  float64
	Breakdown
}

type Breakdown struct {
	CPUPerDollar       float64
	RAMPerDollar       float64
	StoragePerDollar   float64
	NVMeBonus          float64
	CPUGenBonus        float64
	LocalityBonus      float64
	BenchmarkPerDollar float64
	ECCBonus           float64
}

func Score(servers []scanner.Server, scoring config.Scoring, dcPrefix string) []ScoredServer {
	if len(servers) == 0 {
		return nil
	}

	// Compute raw metrics for normalization
	type rawMetrics struct {
		cpuPerDollar       float64
		ramPerDollar       float64
		storagePerDollar   float64
		nvmeRatio          float64
		cpuGenScore        float64
		dcMatch            float64
		benchmarkPerDollar float64
	}

	metrics := make([]rawMetrics, len(servers))
	var maxCPU, maxRAM, maxStorage, maxBenchmark float64

	for i := range servers {
		price := servers[i].Price
		if price <= 0 {
			price = 1 // avoid division by zero
		}

		cpuInfo := cpupkg.Parse(servers[i].CPU, servers[i].ParsedCores, servers[i].ParsedThreads, 0)

		cores := float64(cpuInfo.Cores)
		if cores == 0 {
			cores = float64(servers[i].CPUCount)
		}
		threads := float64(cpuInfo.Threads)
		if threads == 0 {
			threads = cores * 2
		}
		ghz := cpuInfo.BaseGHz
		if ghz == 0 {
			ghz = 3.0 // reasonable default
		}

		cpuVal := cores * threads * ghz / price
		ramVal := float64(servers[i].RAMSize) / price
		storageVal := servers[i].TotalStorageTB / price

		var nvmeRatio float64
		if servers[i].DriveCount > 0 {
			nvmeRatio = float64(servers[i].NVMeCount) / float64(servers[i].DriveCount)
		}

		var dcMatch float64
		if dcPrefix != "" && len(servers[i].Datacenter) >= len(dcPrefix) && servers[i].Datacenter[:len(dcPrefix)] == dcPrefix {
			dcMatch = 1.0
		}

		var benchmarkPD float64
		if cpuInfo.BenchmarkScore > 0 {
			benchmarkPD = float64(cpuInfo.BenchmarkScore) / price
		}

		metrics[i] = rawMetrics{
			cpuPerDollar:       cpuVal,
			ramPerDollar:       ramVal,
			storagePerDollar:   storageVal,
			nvmeRatio:          nvmeRatio,
			cpuGenScore:        cpupkg.GenerationScore(cpuInfo.Generation),
			dcMatch:            dcMatch,
			benchmarkPerDollar: benchmarkPD,
		}

		maxCPU = math.Max(maxCPU, cpuVal)
		maxRAM = math.Max(maxRAM, ramVal)
		maxStorage = math.Max(maxStorage, storageVal)
		maxBenchmark = math.Max(maxBenchmark, benchmarkPD)
	}

	// Normalize and score
	scored := make([]ScoredServer, len(servers))
	for i := range servers {
		m := metrics[i]

		cpuNorm := safeNormalize(m.cpuPerDollar, maxCPU)
		ramNorm := safeNormalize(m.ramPerDollar, maxRAM)
		storageNorm := safeNormalize(m.storagePerDollar, maxStorage)
		benchmarkNorm := safeNormalize(m.benchmarkPerDollar, maxBenchmark)

		var eccBonus float64
		if servers[i].IsECC {
			eccBonus = 1.0
		}

		rawScore := cpuNorm*scoring.CPUWeight +
			ramNorm*scoring.RAMWeight +
			storageNorm*scoring.StorageWeight +
			m.nvmeRatio*scoring.NVMeWeight +
			m.cpuGenScore*scoring.CPUGenWeight +
			m.dcMatch*scoring.LocalityWeight +
			benchmarkNorm*scoring.BenchmarkWeight +
			eccBonus*scoring.ECCWeight

		scored[i] = ScoredServer{
			Server: servers[i],
			Score:  math.Round(rawScore*10000) / 100, // 0-100 scale
			Breakdown: Breakdown{
				CPUPerDollar:       cpuNorm,
				RAMPerDollar:       ramNorm,
				StoragePerDollar:   storageNorm,
				NVMeBonus:          m.nvmeRatio,
				CPUGenBonus:        m.cpuGenScore,
				LocalityBonus:      m.dcMatch,
				BenchmarkPerDollar: benchmarkNorm,
				ECCBonus:           eccBonus,
			},
		}
	}

	sort.Slice(scored, func(i, j int) bool {
		return scored[i].Score > scored[j].Score
	})

	return scored
}

func safeNormalize(val, max float64) float64 {
	if max == 0 {
		return 0
	}
	return val / max
}
