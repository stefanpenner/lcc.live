package alta

import (
	"context"
	"time"

	"github.com/stefanpenner/lcc-live/web/logger"
	"github.com/stefanpenner/lcc-live/web/store"
)

// Poller periodically fetches Alta parking/road status.
type Poller struct {
	client   *Client
	store    *store.Store
	interval time.Duration
}

// NewPoller creates an Alta poller. Default interval 3 minutes if zero.
func NewPoller(client *Client, s *store.Store, interval time.Duration) *Poller {
	if interval <= 0 {
		interval = 3 * time.Minute
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
	st, err := p.client.FetchStatus(ctx)
	if err != nil {
		logger.Error(err, "Failed to fetch Alta status: %v", err)
		return
	}
	p.store.UpdateAltaStatus(*st)
	logger.Muted("Updated Alta status: parking=%s road=%s", st.ParkingStatus, st.RoadStatus)
}
