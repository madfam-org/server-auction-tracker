package notify

import (
	"context"
	"errors"

	"github.com/madfam-org/server-auction-tracker/internal/scorer"
)

// MultiNotifier dispatches to multiple notifiers, collecting errors.
type MultiNotifier struct {
	notifiers []Notifier
}

// NewMultiNotifier wraps multiple notifiers into one.
func NewMultiNotifier(notifiers ...Notifier) *MultiNotifier {
	return &MultiNotifier{notifiers: notifiers}
}

func (m *MultiNotifier) Notify(ctx context.Context, servers []scorer.ScoredServer) error {
	var errs []error
	for _, n := range m.notifiers {
		if err := n.Notify(ctx, servers); err != nil {
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
}
