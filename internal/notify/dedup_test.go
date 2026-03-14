package notify

import (
	"testing"
	"time"

	"github.com/madfam-org/server-auction-tracker/internal/scanner"
	"github.com/madfam-org/server-auction-tracker/internal/scorer"
	"github.com/stretchr/testify/assert"
)

func TestDedupNewServerPasses(t *testing.T) {
	d := NewDedupTracker(1 * time.Hour)
	servers := []scorer.ScoredServer{
		{Server: scanner.Server{ID: 1001}, Score: 80},
	}

	result := d.Filter(servers)
	assert.Len(t, result, 1)
	assert.Equal(t, 1001, result[0].Server.ID)
}

func TestDedupRecentlySeenBlocked(t *testing.T) {
	d := NewDedupTracker(1 * time.Hour)
	servers := []scorer.ScoredServer{
		{Server: scanner.Server{ID: 1001}, Score: 80},
	}

	// First call passes
	result := d.Filter(servers)
	assert.Len(t, result, 1)

	// Second call within window is blocked
	result = d.Filter(servers)
	assert.Len(t, result, 0)
}

func TestDedupExpiredServerPassesAgain(t *testing.T) {
	d := NewDedupTracker(10 * time.Millisecond)
	servers := []scorer.ScoredServer{
		{Server: scanner.Server{ID: 1001}, Score: 80},
	}

	result := d.Filter(servers)
	assert.Len(t, result, 1)

	// Wait for expiry
	time.Sleep(15 * time.Millisecond)

	result = d.Filter(servers)
	assert.Len(t, result, 1)
}

func TestDedupMixedServers(t *testing.T) {
	d := NewDedupTracker(1 * time.Hour)
	servers := []scorer.ScoredServer{
		{Server: scanner.Server{ID: 1001}, Score: 80},
		{Server: scanner.Server{ID: 1002}, Score: 75},
	}

	// First call: both pass
	result := d.Filter(servers)
	assert.Len(t, result, 2)

	// Second call with one new server
	servers2 := []scorer.ScoredServer{
		{Server: scanner.Server{ID: 1001}, Score: 80}, // seen
		{Server: scanner.Server{ID: 1003}, Score: 90}, // new
	}
	result = d.Filter(servers2)
	assert.Len(t, result, 1)
	assert.Equal(t, 1003, result[0].Server.ID)
}
