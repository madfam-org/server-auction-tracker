package main

import (
	"fmt"

	"github.com/spf13/cobra"
)

var watchCmd = &cobra.Command{
	Use:   "watch",
	Short: "Poll auction listings on an interval",
	Long:  "Continuously poll Hetzner Server Auction and send notifications for matching servers.",
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Println("watch: planned for future milestone (M2 — Notifications)")
		return nil
	},
}
