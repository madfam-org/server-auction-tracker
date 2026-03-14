package main

import (
	"fmt"
	"os"
	"text/tabwriter"

	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"

	"github.com/madfam-org/server-auction-tracker/internal/config"
	"github.com/madfam-org/server-auction-tracker/internal/scanner"
	"github.com/madfam-org/server-auction-tracker/internal/scorer"
	"github.com/madfam-org/server-auction-tracker/internal/store"
)

var scanCmd = &cobra.Command{
	Use:   "scan",
	Short: "One-shot scan of Hetzner Server Auction",
	Long:  "Fetch current auction listings, filter by requirements, score, persist to database, and display results.",
	RunE:  runScan,
}

func runScan(cmd *cobra.Command, args []string) error {
	cfg, err := config.Load(cfgFile)
	if err != nil {
		return fmt.Errorf("loading config: %w", err)
	}
	setupLogging(cfg.LogLevel)

	// Fetch
	sc := scanner.New(nil)
	servers, err := sc.Fetch()
	if err != nil {
		return fmt.Errorf("fetching auction data: %w", err)
	}

	// Filter
	filtered := sc.Filter(servers, cfg.Filters)
	if len(filtered) == 0 {
		fmt.Println("No servers match the configured filters.")
		return nil
	}

	// Score
	scored := scorer.Score(filtered, cfg.Scoring, cfg.Filters.DatacenterPrefix)

	// Store
	db, err := store.NewSQLite(cfg.Database.Path)
	if err != nil {
		return fmt.Errorf("opening database: %w", err)
	}
	defer db.Close()

	if err := db.Init(); err != nil {
		return fmt.Errorf("initializing database: %w", err)
	}

	if err := db.SaveScan(scored); err != nil {
		log.WithError(err).Warn("Failed to save scan results")
	} else {
		log.WithField("count", len(scored)).Info("Saved scan results to database")
	}

	// Print
	printResults(scored)
	return nil
}

func printResults(servers []scorer.ScoredServer) {
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "SCORE\tID\tCPU\tRAM\tSTORAGE\tNVMe\tDC\tPRICE")
	fmt.Fprintln(w, "-----\t--\t---\t---\t-------\t----\t--\t-----")

	for _, ss := range servers {
		fmt.Fprintf(w, "%.1f\t%d\t%s\t%dGB\t%.1fTB\t%d/%d\t%s\t€%.2f\n",
			ss.Score,
			ss.Server.ID,
			truncate(ss.Server.CPU, 30),
			ss.Server.RAMSize,
			ss.Server.TotalStorageTB,
			ss.Server.NVMeCount,
			ss.Server.DriveCount,
			ss.Server.Datacenter,
			ss.Server.Price,
		)
	}
	w.Flush()
	fmt.Printf("\n%d servers found matching filters.\n", len(servers))
}

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
}
