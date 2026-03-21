package notify

import (
	"context"
	"fmt"
	"testing"

	"github.com/madfam-org/server-auction-tracker/internal/scanner"
	"github.com/madfam-org/server-auction-tracker/internal/scorer"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type mockNotifier struct {
	called  bool
	servers []scorer.ScoredServer
	err     error
}

func (m *mockNotifier) Notify(_ context.Context, servers []scorer.ScoredServer) error {
	m.called = true
	m.servers = servers
	return m.err
}

func TestMultiNotifier(t *testing.T) {
	n1 := &mockNotifier{}
	n2 := &mockNotifier{}
	multi := NewMultiNotifier(n1, n2)

	servers := []scorer.ScoredServer{{
		Server: scanner.Server{ID: 1, CPU: "test", RAMSize: 64, Price: 39.00},
		Score:  80.0,
	}}

	err := multi.Notify(context.Background(), servers)
	require.NoError(t, err)
	assert.True(t, n1.called)
	assert.True(t, n2.called)
	assert.Len(t, n1.servers, 1)
	assert.Len(t, n2.servers, 1)
}

func TestMultiNotifierPartialFailure(t *testing.T) {
	n1 := &mockNotifier{}
	n2 := &mockNotifier{err: fmt.Errorf("discord down")}
	n3 := &mockNotifier{}
	multi := NewMultiNotifier(n1, n2, n3)

	servers := []scorer.ScoredServer{{
		Server: scanner.Server{ID: 1, CPU: "test", RAMSize: 64, Price: 39.00},
		Score:  80.0,
	}}

	err := multi.Notify(context.Background(), servers)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "discord down")
	// All notifiers should still be called
	assert.True(t, n1.called)
	assert.True(t, n2.called)
	assert.True(t, n3.called)
}
