package main

import (
	"fmt"
	"os"

	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

var (
	version   = "dev"
	cfgFile   string
)

var rootCmd = &cobra.Command{
	Use:     "foundry-scout",
	Short:   "Hetzner Server Auction intelligence",
	Long:    "Automated scoring, price history, and notifications for Hetzner Server Auction capacity expansion.",
	Version: version,
}

func init() {
	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default: scout.yaml)")
	rootCmd.AddCommand(scanCmd)
	rootCmd.AddCommand(watchCmd)
	rootCmd.AddCommand(historyCmd)
	rootCmd.AddCommand(simulateCmd)
	rootCmd.AddCommand(orderCmd)
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func setupLogging(level string) {
	lvl, err := log.ParseLevel(level)
	if err != nil {
		lvl = log.InfoLevel
	}
	log.SetLevel(lvl)
	log.SetFormatter(&log.TextFormatter{
		FullTimestamp: true,
	})
}
