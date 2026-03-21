package main

import (
	"fmt"
	"os"
	"text/tabwriter"

	"github.com/spf13/cobra"

	"github.com/madfam-org/server-auction-tracker/internal/config"
	"github.com/madfam-org/server-auction-tracker/internal/store"
)

var (
	historyCPU   string
	historyLimit int
)

var historyCmd = &cobra.Command{
	Use:   "history",
	Short: "Query price history for a CPU model",
	Long:  "Display historical scan results and price statistics for servers matching a CPU model.",
	RunE:  runHistory,
}

func init() {
	historyCmd.Flags().StringVar(&historyCPU, "cpu", "", "CPU model to search (e.g., 'Ryzen 5 3600')")
	historyCmd.Flags().IntVar(&historyLimit, "limit", 20, "Maximum number of records to display")
}

func runHistory(cmd *cobra.Command, args []string) error {
	cfg, err := config.Load(cfgFile)
	if err != nil {
		return fmt.Errorf("loading config: %w", err)
	}
	setupLogging(cfg.LogLevel)

	db, err := store.NewSQLite(cfg.Database.Path)
	if err != nil {
		return fmt.Errorf("opening database: %w", err)
	}
	defer db.Close()

	if err := db.Init(); err != nil {
		return fmt.Errorf("initializing database: %w", err)
	}

	// Stats
	if historyCPU != "" {
		stats, err := db.GetStats(historyCPU)
		if err != nil {
			return fmt.Errorf("querying stats: %w", err)
		}
		if stats == nil {
			fmt.Printf("No price history found for CPU matching '%s'.\n", historyCPU)
			return nil
		}

		fmt.Printf("Price Statistics for: %s\n", stats.CPU)
		fmt.Printf("  Observations: %d\n", stats.Count)
		fmt.Printf("  Min Price:    €%.2f\n", stats.MinPrice)
		fmt.Printf("  Max Price:    €%.2f\n", stats.MaxPrice)
		fmt.Printf("  Avg Price:    €%.2f\n", stats.AvgPrice)
		fmt.Printf("  First Seen:   %s\n", stats.FirstSeen.Format("2006-01-02 15:04"))
		fmt.Printf("  Last Seen:    %s\n", stats.LastSeen.Format("2006-01-02 15:04"))
		fmt.Println()
	}

	// Load all CPU stats for deal quality comparison
	allStats, err := db.GetAllCPUStats()
	if err != nil {
		return fmt.Errorf("querying CPU stats: %w", err)
	}

	// History records
	records, err := db.GetHistory(historyCPU, historyLimit)
	if err != nil {
		return fmt.Errorf("querying history: %w", err)
	}
	if len(records) == 0 {
		fmt.Println("No history records found.")
		return nil
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "DATE\tSERVER\tCPU\tRAM\tPRICE\tDEAL\tSCORE\tDC")   //nolint:errcheck
	fmt.Fprintln(w, "----\t------\t---\t---\t-----\t----\t-----\t--") //nolint:errcheck
	for _, r := range records {
		deal := dealQuality(r, allStats)
		fmt.Fprintf(w, "%s\t%d\t%s\t%dGB\t€%.2f\t%s\t%.1f\t%s\n",
			r.ScannedAt.Format("2006-01-02 15:04"),
			r.ServerID,
			truncate(r.CPU, 25),
			r.RAMSize,
			r.Price,
			deal,
			r.Score,
			r.Datacenter,
		)
	}
	w.Flush() //nolint:errcheck
	fmt.Printf("\n%d records shown.\n", len(records))
	return nil
}

// dealQuality returns a percentage indicator showing how a server's price
// compares to the average for that CPU model. Negative = below avg (good deal).
func dealQuality(r store.ScanRecord, allStats map[string]*store.PriceStats) string {
	stats, ok := allStats[r.CPU]
	if !ok || stats.AvgPrice == 0 {
		return "—"
	}
	pctDiff := ((r.Price - stats.AvgPrice) / stats.AvgPrice) * 100
	if pctDiff <= 0 {
		return fmt.Sprintf("%.1f%%", pctDiff)
	}
	return fmt.Sprintf("+%.1f%%", pctDiff)
}
