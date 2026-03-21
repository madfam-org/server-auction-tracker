package notify

import (
	"sync"
	"time"

	"github.com/madfam-org/server-auction-tracker/internal/scorer"
)

// DedupTracker suppresses notifications for recently-seen server IDs.
type DedupTracker struct {
	mu     sync.Mutex
	seen   map[int]time.Time
	window time.Duration
}

// NewDedupTracker creates a tracker with the given dedup window.
func NewDedupTracker(window time.Duration) *DedupTracker {
	return &DedupTracker{
		seen:   make(map[int]time.Time),
		window: window,
	}
}

// Filter returns only servers that haven't been seen within the dedup window.
func (d *DedupTracker) Filter(servers []scorer.ScoredServer) []scorer.ScoredServer {
	d.mu.Lock()
	defer d.mu.Unlock()

	now := time.Now()
	var result []scorer.ScoredServer

	for i := range servers {
		lastSeen, exists := d.seen[servers[i].Server.ID]
		if exists && now.Sub(lastSeen) < d.window {
			continue
		}
		d.seen[servers[i].Server.ID] = now
		result = append(result, servers[i])
	}

	// Purge expired entries
	for id, t := range d.seen {
		if now.Sub(t) >= d.window {
			delete(d.seen, id)
		}
	}

	return result
}
