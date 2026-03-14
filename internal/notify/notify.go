package notify

import (
	"context"

	"github.com/madfam-org/server-auction-tracker/internal/scorer"
)

// Notifier sends alerts when high-scoring servers are found.
type Notifier interface {
	Notify(ctx context.Context, servers []scorer.ScoredServer) error
}
