package main

import (
	"fmt"

	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"

	"github.com/madfam-org/server-auction-tracker/internal/config"
	"github.com/madfam-org/server-auction-tracker/internal/scanner"
	"github.com/madfam-org/server-auction-tracker/internal/simulate"
	"github.com/madfam-org/server-auction-tracker/internal/store"
)

var simulateServerID int

var simulateCmd = &cobra.Command{
	Use:   "simulate",
	Short: "Simulate cluster impact of adding a server",
	Long:  "Analyze how adding a specific auction server would affect cluster capacity and resource distribution.",
	RunE:  runSimulate,
}

func init() {
	simulateCmd.Flags().IntVar(&simulateServerID, "server-id", 0, "Server ID from auction listing or database")
	simulateCmd.MarkFlagRequired("server-id") //nolint:errcheck
}

func runSimulate(cmd *cobra.Command, args []string) error {
	cfg, err := config.Load(cfgFile)
	if err != nil {
		return fmt.Errorf("loading config: %w", err)
	}
	setupLogging(cfg.LogLevel)

	if cfg.Cluster.Nodes == 0 {
		return fmt.Errorf("cluster config is required for simulation (set cluster.nodes, cpu_millicores, etc.)")
	}

	// Try to find the server from live auction first
	server, err := findServer(cfg, simulateServerID)
	if err != nil {
		return err
	}

	// Estimate current monthly cost from cluster config
	// (simple heuristic: divide by nodes for per-node cost isn't available,
	//  so we use 0 and let the user see the increment)
	result := simulate.Simulate(cfg.Cluster, *server, 0)
	printSimulationResult(result)
	return nil
}

func findServer(cfg *config.Config, serverID int) (*scanner.Server, error) {
	// Try live auction
	sc := scanner.New(nil)
	servers, err := sc.Fetch()
	if err != nil {
		log.WithError(err).Warn("Could not fetch live auction, trying database")
	} else {
		for i := range servers {
			if servers[i].ID == serverID {
				log.Info("Found server in live auction")
				return &servers[i], nil
			}
		}
		log.Info("Server not in live auction, trying database")
	}

	// Fallback to database
	db, dbErr := store.NewSQLite(cfg.Database.Path)
	if dbErr != nil {
		if err != nil {
			return nil, fmt.Errorf("server %d not found (auction fetch failed: %v, db open failed: %v)", serverID, err, dbErr)
		}
		return nil, fmt.Errorf("server %d not in auction, and db open failed: %w", serverID, dbErr)
	}
	defer db.Close()

	if initErr := db.Init(); initErr != nil {
		return nil, fmt.Errorf("initializing database: %w", initErr)
	}

	records, dbErr := db.GetHistory("", 1000)
	if dbErr != nil {
		return nil, fmt.Errorf("querying database: %w", dbErr)
	}

	for _, r := range records {
		if r.ServerID == serverID {
			log.Info("Found server in database history")
			return &scanner.Server{
				ID:             r.ServerID,
				CPU:            r.CPU,
				RAMSize:        r.RAMSize,
				TotalStorageTB: r.TotalStorageTB,
				NVMeCount:      r.NVMeCount,
				DriveCount:     r.DriveCount,
				Datacenter:     r.Datacenter,
				Price:          r.Price,
			}, nil
		}
	}

	return nil, fmt.Errorf("server %d not found in live auction or database", serverID)
}

func printSimulationResult(r *simulate.Result) {
	fmt.Printf("Cluster Impact: Adding Server #%d\n", r.Server.ID)
	fmt.Println("═══════════════════════════════════════")
	fmt.Printf("Server: %s | %dGB | %.1fTB | €%.2f/mo\n\n",
		r.Server.CPU, r.Server.RAMSize, r.Server.TotalStorageTB, r.Server.Price)

	fmt.Printf("%-10s %-14s %-14s %s\n", "Resource", "Before", "After", "Change")
	fmt.Printf("%-10s %-14s %-14s %s\n", "────────", "──────", "─────", "──────")
	fmt.Printf("%-10s %-14s %-14s %.1fpp ▼\n", "CPU",
		fmt.Sprintf("%.1f%%", r.CPUBefore),
		fmt.Sprintf("%.1f%%", r.CPUAfter),
		r.CPUBefore-r.CPUAfter)
	fmt.Printf("%-10s %-14s %-14s %.1fpp ▼\n", "RAM",
		fmt.Sprintf("%.1f%%", r.RAMBefore),
		fmt.Sprintf("%.1f%%", r.RAMAfter),
		r.RAMBefore-r.RAMAfter)
	fmt.Printf("%-10s %-14s %-14s %.1fpp ▼\n", "Disk",
		fmt.Sprintf("%.1f%%", r.DiskBefore),
		fmt.Sprintf("%.1f%%", r.DiskAfter),
		r.DiskBefore-r.DiskAfter)
	fmt.Printf("%-10s %-14d %-14d +%d\n", "Nodes", r.NodesBefore, r.NodesAfter, r.NodesAfter-r.NodesBefore)
	fmt.Printf("%-10s %-14s %-14s +€%.2f\n", "Monthly",
		fmt.Sprintf("€%.0f", r.MonthlyCostBefore),
		fmt.Sprintf("€%.0f", r.MonthlyCostAfter),
		r.Server.Price)

	fmt.Printf("\nBottleneck Relief: %s drops from %s → %s\n",
		r.Bottleneck,
		simulate.HealthLabel(getBefore(r)),
		simulate.HealthLabel(getAfter(r)))
}

func getBefore(r *simulate.Result) float64 {
	switch r.Bottleneck {
	case "CPU":
		return r.CPUBefore
	case "RAM":
		return r.RAMBefore
	default:
		return r.DiskBefore
	}
}

func getAfter(r *simulate.Result) float64 {
	switch r.Bottleneck {
	case "CPU":
		return r.CPUAfter
	case "RAM":
		return r.RAMAfter
	default:
		return r.DiskAfter
	}
}
