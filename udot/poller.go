package udot

import (
	"context"
	"fmt"
	"time"

	"github.com/stefanpenner/lcc-live/logger"
	"github.com/stefanpenner/lcc-live/store"
)

// Poller handles periodic fetching and updating of UDOT data
type Poller struct {
	client   *Client
	store    *store.Store
	interval time.Duration
}

// NewPoller creates a new UDOT data poller
func NewPoller(client *Client, s *store.Store, interval time.Duration) *Poller {
	return &Poller{
		client:   client,
		store:    s,
		interval: interval,
	}
}

// StartRoadConditions starts polling road conditions
func (p *Poller) StartRoadConditions(ctx context.Context) error {
	if !p.client.IsConfigured() {
		logger.Warn("UDOT_API_KEY not set. Skipping road conditions fetching.")
		return nil
	}

	ticker := time.NewTicker(p.interval)
	defer ticker.Stop()

	// Fetch immediately on startup
	p.pollRoadConditions(ctx)

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			p.pollRoadConditions(ctx)
		}
	}
}

// StartWeatherStations starts polling weather stations
func (p *Poller) StartWeatherStations(ctx context.Context) error {
	if !p.client.IsConfigured() {
		logger.Warn("UDOT_API_KEY not set. Skipping weather stations fetching.")
		return nil
	}

	ticker := time.NewTicker(p.interval)
	defer ticker.Stop()

	// Fetch immediately on startup
	p.pollWeatherStations(ctx)

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			p.pollWeatherStations(ctx)
		}
	}
}

// StartEvents starts polling traffic events
func (p *Poller) StartEvents(ctx context.Context) error {
	if !p.client.IsConfigured() {
		logger.Warn("UDOT_API_KEY not set. Skipping events fetching.")
		return nil
	}

	ticker := time.NewTicker(p.interval)
	defer ticker.Stop()

	// Fetch immediately on startup
	p.pollEvents(ctx)

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			p.pollEvents(ctx)
		}
	}
}

func (p *Poller) pollRoadConditions(ctx context.Context) {
	conditions, err := p.client.FetchRoadConditions(ctx)
	if err != nil {
		logger.Error(err, "Failed to fetch road conditions: %v", err)
		return
	}

	// If conditions is nil, it means we got a 304 Not Modified - data hasn't changed
	if conditions == nil {
		logger.Muted("Road conditions unchanged (304 Not Modified)")
		return
	}

	lccConditions, bccConditions := FilterRoadConditionsByCanyon(conditions)
	p.store.UpdateRoadConditions("LCC", lccConditions)
	p.store.UpdateRoadConditions("BCC", bccConditions)
	logger.Muted("Updated road conditions: LCC=%d, BCC=%d", len(lccConditions), len(bccConditions))
}

func (p *Poller) pollWeatherStations(ctx context.Context) {
	stations, err := p.client.FetchWeatherStations(ctx)
	if err != nil {
		logger.Error(err, "Failed to fetch weather stations: %v", err)
		return
	}

	// If stations is nil, it means we got a 304 Not Modified - data hasn't changed
	if stations == nil {
		logger.Muted("Weather stations unchanged (304 Not Modified)")
		return
	}

	// Store all weather stations for later re-matching when camera coordinates are available
	p.store.StoreAllWeatherStations(stations)

	// First match by CameraSourceId (existing method)
	matched := 0
	for i := range stations {
		station := &stations[i]
		if station.CameraSourceId != nil && *station.CameraSourceId != "" {
			p.store.UpdateWeatherStation(*station.CameraSourceId, station)
			matched++
		}
	}

	// Then match by coordinates (new method)
	p.store.MatchWeatherStationsByCoordinates(stations)
	logger.Muted("Updated weather stations: %d matched by SourceId, additional matches by coordinates", matched)
}

func (p *Poller) pollEvents(ctx context.Context) {
	events, err := p.client.FetchEvents(ctx)
	if err != nil {
		logger.Error(err, "Failed to fetch events: %v", err)
		return
	}

	// If events is nil, it means we got a 304 Not Modified - data hasn't changed
	if events == nil {
		logger.Muted("Events unchanged (304 Not Modified)")
		return
	}

	lccEvents, bccEvents := FilterEventsByCanyon(events)
	p.store.UpdateEvents("LCC", lccEvents)
	p.store.UpdateEvents("BCC", bccEvents)
	logger.Muted("Updated events: LCC=%d, BCC=%d", len(lccEvents), len(bccEvents))
}

// StartCameraCoordinates starts polling camera coordinates from UDOT
func (p *Poller) StartCameraCoordinates(ctx context.Context) error {
	if !p.client.IsConfigured() {
		logger.Warn("UDOT_API_KEY not set. Skipping camera coordinates fetching.")
		return nil
	}

	ticker := time.NewTicker(p.interval)
	defer ticker.Stop()

	// Fetch immediately on startup
	p.pollCameraCoordinates(ctx)

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			p.pollCameraCoordinates(ctx)
		}
	}
}

func (p *Poller) pollCameraCoordinates(ctx context.Context) {
	cameras, err := p.client.FetchCameras(ctx)
	if err != nil {
		logger.Error(err, "Failed to fetch camera coordinates: %v", err)
		return
	}

	// If cameras is nil, it means we got a 304 Not Modified - data hasn't changed
	if cameras == nil {
		logger.Muted("Camera coordinates unchanged (304 Not Modified)")
		return
	}

	// Build map of camera ID (as string) -> coordinates
	// The camera ID from UDOT matches the SourceId extracted from camera URLs
	cameraIdToCoords := make(map[string]struct {
		Lat float64
		Lon float64
	})
	for _, cam := range cameras {
		// Convert camera Id to string to match SourceId from URLs
		cameraIdStr := fmt.Sprintf("%d", cam.Id)
		cameraIdToCoords[cameraIdStr] = struct {
			Lat float64
			Lon float64
		}{
			Lat: cam.Latitude,
			Lon: cam.Longitude,
		}
	}

	p.store.UpdateCameraCoordinates(cameraIdToCoords)
}
