package notify

import "github.com/madfam-org/server-auction-tracker/internal/scorer"

// Notifier sends alerts when high-scoring servers are found.
// Planned for M2 milestone (Slack/Discord webhook integration).
type Notifier interface {
	Notify(servers []scorer.ScoredServer) error
}
