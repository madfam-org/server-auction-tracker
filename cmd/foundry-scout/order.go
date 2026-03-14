package main

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"strings"

	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"

	"github.com/madfam-org/server-auction-tracker/internal/config"
	"github.com/madfam-org/server-auction-tracker/internal/order"
	"github.com/madfam-org/server-auction-tracker/internal/scanner"
	"github.com/madfam-org/server-auction-tracker/internal/scorer"
	"github.com/madfam-org/server-auction-tracker/internal/store"
)

var (
	orderServerID int
	orderYes      bool
)

var orderCmd = &cobra.Command{
	Use:   "order",
	Short: "Order a server from the auction",
	Long:  "Place an order for a specific server via the Hetzner Robot API, subject to safety gates.",
	RunE:  runOrder,
}

func init() {
	orderCmd.Flags().IntVar(&orderServerID, "server-id", 0, "Server ID to order")
	orderCmd.MarkFlagRequired("server-id") //nolint:errcheck
	orderCmd.Flags().BoolVar(&orderYes, "yes", false, "Skip interactive confirmation")
}

func runOrder(cmd *cobra.Command, args []string) error {
	cfg, err := config.Load(cfgFile)
	if err != nil {
		return fmt.Errorf("loading config: %w", err)
	}
	setupLogging(cfg.LogLevel)

	// Initialize store for audit logging
	db, err := store.NewSQLite(cfg.Database.Path)
	if err != nil {
		return fmt.Errorf("opening database: %w", err)
	}
	defer db.Close()
	if err := db.Init(); err != nil {
		return fmt.Errorf("initializing database: %w", err)
	}

	// Re-fetch from live auction to confirm availability and current price
	sc := scanner.New(nil)
	servers, err := sc.Fetch()
	if err != nil {
		return fmt.Errorf("fetching live auction: %w", err)
	}

	var target *scanner.Server
	for i := range servers {
		if servers[i].ID == orderServerID {
			target = &servers[i]
			break
		}
	}
	if target == nil {
		return fmt.Errorf("server %d not found in live auction (may have been sold)", orderServerID)
	}

	// Score the server
	scored := scorer.Score([]scanner.Server{*target}, cfg.Scoring, cfg.Filters.DatacenterPrefix)
	if len(scored) == 0 {
		return fmt.Errorf("scoring failed")
	}
	serverScore := scored[0].Score

	// Eligibility check
	client := order.NewRobotClient(cfg.Order)
	check := client.CheckEligibility(*target, serverScore, cfg.Order)
	if !check.Eligible {
		fmt.Println("Order NOT eligible:")
		for _, reason := range check.Reasons {
			fmt.Printf("  - %s\n", reason)
		}
		db.SaveOrderAttempt(orderServerID, serverScore, target.Price, false, "eligibility: "+strings.Join(check.Reasons, "; ")) //nolint:errcheck
		return nil
	}

	// Display server details
	fmt.Printf("Server #%d: %s | %dGB | €%.2f/mo | Score: %.1f | %s\n",
		target.ID, target.CPU, target.RAMSize, target.Price, serverScore, target.Datacenter)

	// Interactive confirmation
	if cfg.Order.RequireApproval && !orderYes {
		fmt.Print("\nProceed with order? [y/N]: ")
		reader := bufio.NewReader(os.Stdin)
		answer, _ := reader.ReadString('\n')
		answer = strings.TrimSpace(strings.ToLower(answer))
		if answer != "y" && answer != "yes" {
			fmt.Println("Order cancelled.")
			db.SaveOrderAttempt(orderServerID, serverScore, target.Price, false, "cancelled by user") //nolint:errcheck
			return nil
		}
	}

	// Place order
	log.WithField("server_id", orderServerID).Info("Placing order via Robot API")
	result, err := client.Order(context.Background(), orderServerID)
	if err != nil {
		db.SaveOrderAttempt(orderServerID, serverScore, target.Price, false, err.Error()) //nolint:errcheck
		return fmt.Errorf("order failed: %w", err)
	}

	db.SaveOrderAttempt(orderServerID, serverScore, target.Price, result.Success, result.Message) //nolint:errcheck

	if result.Success {
		fmt.Printf("Order placed successfully: %s\n", result.Message)
	} else {
		fmt.Printf("Order failed: %s\n", result.Message)
	}

	return nil
}
