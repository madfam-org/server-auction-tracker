package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"

	"github.com/madfam-org/server-auction-tracker/internal/config"
	"github.com/madfam-org/server-auction-tracker/internal/notify"
	"github.com/madfam-org/server-auction-tracker/internal/scanner"
	"github.com/madfam-org/server-auction-tracker/internal/scorer"
	"github.com/madfam-org/server-auction-tracker/internal/store"
)

var (
	watchInterval string
	watchOnce     bool
)

var watchCmd = &cobra.Command{
	Use:   "watch",
	Short: "Poll auction listings on an interval",
	Long:  "Continuously poll Hetzner Server Auction and send notifications for matching servers.",
	RunE:  runWatch,
}

func init() {
	watchCmd.Flags().StringVar(&watchInterval, "interval", "", "Poll interval (overrides config, e.g. '5m')")
	watchCmd.Flags().BoolVar(&watchOnce, "once", false, "Run a single iteration then exit (for K8s CronJob mode)")
}

func runWatch(cmd *cobra.Command, args []string) error {
	cfg, err := config.Load(cfgFile)
	if err != nil {
		return fmt.Errorf("loading config: %w", err)
	}
	setupLogging(cfg.LogLevel)

	// Determine interval
	intervalStr := cfg.Watch.Interval
	if watchInterval != "" {
		intervalStr = watchInterval
	}
	interval, err := time.ParseDuration(intervalStr)
	if err != nil {
		return fmt.Errorf("parsing interval %q: %w", intervalStr, err)
	}

	// Parse dedup window
	dedupWindow, err := time.ParseDuration(cfg.Watch.DedupWindow)
	if err != nil {
		return fmt.Errorf("parsing dedup_window %q: %w", cfg.Watch.DedupWindow, err)
	}

	// Initialize notifier
	notifier, err := notify.NewNotifier(&cfg.Notify)
	if err != nil {
		return fmt.Errorf("creating notifier: %w", err)
	}

	// Initialize store
	db, err := store.NewSQLite(cfg.Database.Path)
	if err != nil {
		return fmt.Errorf("opening database: %w", err)
	}
	defer db.Close() //nolint:errcheck

	if err := db.Init(); err != nil {
		return fmt.Errorf("initializing database: %w", err)
	}

	// Create scanner and dedup tracker
	sc := scanner.New(nil)
	dedup := notify.NewDedupTracker(dedupWindow)
	var lastDigestSent time.Time

	// Graceful shutdown
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	log.WithFields(log.Fields{
		"interval":     interval,
		"dedup_window": dedupWindow,
		"once":         watchOnce,
		"notify_type":  cfg.Notify.Type,
	}).Info("Starting watch loop")

	for {
		if err := watchIteration(ctx, sc, db, cfg, notifier, dedup); err != nil {
			log.WithError(err).Error("Watch iteration failed")
		}

		// Check if digest is due
		if cfg.Digest.Enabled {
			lastDigestSent = checkAndSendDigest(ctx, db, cfg, notifier, lastDigestSent)
		}

		if watchOnce {
			log.Info("Single iteration complete (--once), exiting")
			return nil
		}

		select {
		case <-ctx.Done():
			log.Info("Shutting down watch loop")
			return nil
		case <-time.After(interval):
		}
	}
}

func watchIteration(ctx context.Context, sc *scanner.Scanner, db store.Store, cfg *config.Config, notifier notify.Notifier, dedup *notify.DedupTracker) error {
	// Fetch
	servers, err := sc.Fetch()
	if err != nil {
		return fmt.Errorf("fetching: %w", err)
	}

	// Track all servers for time-on-market analysis (pre-filter)
	trackerEntries := make([]store.ServerTrackerEntry, len(servers))
	activeIDs := make([]int, len(servers))
	for i := range servers {
		trackerEntries[i] = store.ServerTrackerEntry{
			ServerID:   servers[i].ID,
			CPU:        servers[i].CPU,
			Price:      servers[i].Price,
			Datacenter: servers[i].Datacenter,
		}
		activeIDs[i] = servers[i].ID
	}
	if err := db.UpsertServerTracker(trackerEntries); err != nil {
		log.WithError(err).Warn("Failed to upsert server tracker")
	}
	if err := db.MarkSoldServers(activeIDs); err != nil {
		log.WithError(err).Warn("Failed to mark sold servers")
	}

	// Filter
	filtered := sc.Filter(servers, cfg.Filters)
	if len(filtered) == 0 {
		log.Info("No servers match filters")
		return nil
	}

	// Score
	scored := scorer.Score(filtered, cfg.Scoring, cfg.Filters.DatacenterPrefix)

	// Dedup — only notify for new servers
	newServers := dedup.Filter(scored)

	// Apply minimum score filter for notifications
	if cfg.Notify.MinScore > 0 {
		var filtered []scorer.ScoredServer
		for i := range newServers {
			if newServers[i].Score >= cfg.Notify.MinScore {
				filtered = append(filtered, newServers[i])
			}
		}
		newServers = filtered
	}

	if len(newServers) > 0 {
		log.WithField("new_servers", len(newServers)).Info("New servers found, sending notification")
		if err := notifier.Notify(ctx, newServers); err != nil {
			log.WithError(err).Error("Notification failed")
		}
	} else {
		log.Info("No new servers since last check")
	}

	// Store all matched servers (not just new)
	if err := db.SaveScan(scored); err != nil {
		log.WithError(err).Warn("Failed to save scan results")
	} else {
		log.WithField("count", len(scored)).Info("Saved scan results")
	}

	// Prune old scan data
	if cfg.Database.RetentionDays > 0 {
		pruned, err := db.PruneOldScans(cfg.Database.RetentionDays)
		if err != nil {
			log.WithError(err).Warn("Failed to prune old scans")
		} else if pruned > 0 {
			log.WithField("pruned", pruned).Info("Pruned old scan records")
		}
	}

	return nil
}

func checkAndSendDigest(ctx context.Context, db store.Store, cfg *config.Config, notifier notify.Notifier, lastSent time.Time) time.Time {
	var digestInterval time.Duration
	switch cfg.Digest.Schedule {
	case "weekly":
		digestInterval = 7 * 24 * time.Hour
	default: // "daily"
		digestInterval = 24 * time.Hour
	}

	if !lastSent.IsZero() && time.Since(lastSent) < digestInterval {
		return lastSent
	}

	since := time.Now().Add(-digestInterval)
	topN := cfg.Digest.TopN
	if topN <= 0 {
		topN = 5
	}

	deals, err := db.GetTopDeals(since, topN, cfg.Digest.MinScore)
	if err != nil {
		log.WithError(err).Warn("Failed to query top deals for digest")
		return lastSent
	}
	if len(deals) == 0 {
		log.Info("No deals for digest")
		return time.Now()
	}

	text := notify.FormatDigest(deals, cfg.Digest.Schedule)
	// Send digest via the same notifier by wrapping as a ScoredServer list
	// For simplicity, log the digest and send a minimal notification
	log.WithFields(log.Fields{
		"deals":  len(deals),
		"period": cfg.Digest.Schedule,
	}).Info("Sending digest notification")

	// Convert top deals to ScoredServer for notifier compatibility
	digestServers := make([]scorer.ScoredServer, len(deals))
	for i, d := range deals {
		digestServers[i] = scorer.ScoredServer{
			Server: scanner.Server{
				ID:             d.ServerID,
				CPU:            d.CPU,
				RAMSize:        d.RAMSize,
				TotalStorageTB: d.TotalStorageTB,
				NVMeCount:      d.NVMeCount,
				DriveCount:     d.DriveCount,
				Datacenter:     d.Datacenter,
				Price:          d.Price,
			},
			Score: d.Score,
		}
	}

	if err := notifier.Notify(ctx, digestServers); err != nil {
		log.WithError(err).Error("Digest notification failed")
		return lastSent
	}

	_ = text // text is available for channels that support custom formatting
	return time.Now()
}
