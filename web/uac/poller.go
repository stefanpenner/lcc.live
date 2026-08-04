package uac

import (
	"context"
	"time"

	"github.com/stefanpenner/lcc-live/web/logger"
	"github.com/stefanpenner/lcc-live/web/store"
)

// Poller periodically fetches UAC Salt Lake danger.
type Poller struct {
	client   *Client
	store    *store.Store
	interval time.Duration
}

// NewPoller creates a UAC poller. Default interval ~10 minutes if zero.
func NewPoller(client *Client, s *store.Store, interval time.Duration) *Poller {
	if interval <= 0 {
		interval = 10 * time.Minute
	}
	return &Poller{client: client, store: s, interval: interval}
}

// Start polls until ctx is cancelled.
func (p *Poller) Start(ctx context.Context) error {
	ticker := time.NewTicker(p.interval)
	defer ticker.Stop()

	p.poll(ctx)

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			p.poll(ctx)
		}
	}
}

func (p *Poller) poll(ctx context.Context) {
	d, err := p.client.FetchSaltLakeDanger(ctx)
	if err != nil {
		logger.Error(err, "Failed to fetch UAC avalanche danger: %v", err)
		return
	}
	p.store.UpdateAvalancheDanger(*d)
	logger.Muted("Updated UAC Salt Lake danger: %s (level %d)", d.Danger, d.DangerLevel)
}
