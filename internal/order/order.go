package order

import (
	"context"

	"github.com/madfam-org/server-auction-tracker/internal/config"
	"github.com/madfam-org/server-auction-tracker/internal/scanner"
)

// Result represents the outcome of an order attempt.
type Result struct {
	ServerID    int
	TransID     string
	Success     bool
	Message     string
}

// Check represents the eligibility check result.
type Check struct {
	Eligible bool
	Reasons  []string
}

// Orderer handles automated server ordering via the Hetzner Robot API.
type Orderer interface {
	Order(ctx context.Context, serverID int) (*Result, error)
	CheckEligibility(server scanner.Server, score float64, cfg config.Order) *Check
}
