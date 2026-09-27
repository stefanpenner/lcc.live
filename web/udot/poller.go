package udot

import (
	"context"
	"os"
	"time"

	"github.com/stefanpenner/lcc-live/web/logger"
	"github.com/stefanpenner/lcc-live/web/store"
)

// Poller handles periodic fetching and updating of UDOT data
type Poller struct {
	client   *Client
	store    *store.Store
	interval time.Duration
	after    func()
}

// AfterUpdate runs after a road or event poll writes the store.
func (p *Poller) AfterUpdate(fn func()) {
	p.after = fn
}

func (p *Poller) changed() {
	if p.after != nil {
		p.after()
	}
}

// NewPoller creates a new UDOT data poller
func NewPoller(client *Client, s *store.Store, interval time.Duration) *Poller {
	return &Poller{
		client:   client,
		store:    s,
		interval: interval,
	}
}

func (p *Poller) pollNowAndEvery(ctx context.Context, poll func(context.Context)) error {
	ticker := time.NewTicker(p.interval)
	defer ticker.Stop()

	poll(ctx)

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			poll(ctx)
		}
	}
}

// seedDevRoadConditions injects sample chips so local UI work is visible without a UDOT key.
func (p *Poller) seedDevRoadConditions() {
	now := time.Now().Unix()
	p.store.UpdateRoadConditions("LCC", []store.RoadCondition{{
		Id:               205,
		SourceId:         "dev",
		RoadCondition:    "Dry",
		WeatherCondition: "Fair",
		Restriction:      "none",
		RoadwayName:      "SR-210 Upper Little Cottonwood Canyon",
		LastUpdated:      now,
	}})
	p.store.UpdateRoadConditions("BCC", []store.RoadCondition{{
		Id:               206,
		SourceId:         "dev",
		RoadCondition:    "Wet",
		WeatherCondition: "Cloudy",
		Restriction:      "Traction law",
		RoadwayName:      "SR-190 Big Cottonwood Canyon",
		LastUpdated:      now,
	}})
	logger.Warn("UDOT_API_KEY not set — seeded dev road conditions for UI")
}

// seedDevEvents injects sample event chips so local UI work is visible without a UDOT key.
func (p *Poller) seedDevEvents() {
	now := time.Now().Unix()
	p.store.UpdateEvents("LCC", []store.Event{{
		ID:            "dev-lcc-1",
		SourceId:      "dev",
		RoadwayName:   "SR-210",
		Name:          "Avalanche control",
		Description:   "Intermittent delays due to avalanche mitigation work near White Pine",
		IsFullClosure: false,
		Severity:      "Moderate",
		LastUpdated:   now,
	}, {
		ID:            "dev-lcc-2",
		SourceId:      "dev",
		RoadwayName:   "SR-210",
		Name:          "Full canyon closure",
		Description:   "SR-210 closed at mouth for emergency response",
		IsFullClosure: true,
		Severity:      "High",
		LastUpdated:   now,
	}})
	p.store.UpdateEvents("BCC", []store.Event{{
		ID:            "dev-bcc-1",
		SourceId:      "dev",
		RoadwayName:   "SR-190",
		Name:          "Construction",
		Description:   "Lane restrictions near Spruces campground",
		IsFullClosure: false,
		Severity:      "Low",
		LastUpdated:   now,
	}})
	logger.Warn("UDOT_API_KEY not set — seeded dev events for UI")
}

// StartRoadConditions starts polling road conditions
func (p *Poller) StartRoadConditions(ctx context.Context) error {
	if !p.client.IsConfigured() {
		logger.Warn("UDOT_API_KEY not set. Skipping road conditions fetching.")
		if os.Getenv("DEV_MODE") == "1" || os.Getenv("DEV_MODE") == "true" {
			p.seedDevRoadConditions()
		}
		return nil
	}

	return p.pollNowAndEvery(ctx, p.pollRoadConditions)
}

// seedDevWeatherStations injects fresh sample temps for known data.json station IDs
// so local UI (Powerhouse, White Pine, Baldy, S Curves, …) is visible without a UDOT key.
func (p *Poller) seedDevWeatherStations() {
	now := time.Now().Unix()
	str := func(s string) *string { return &s }
	p.store.StoreWeatherStationsById([]store.WeatherStation{
		{Id: 1650225, StationName: "SR-210 @ Powerhouse", AirTemperature: str("72.4"), WindSpeedAvg: str("4.2"), WindDirection: str("SW"), Source: "dev", LastUpdated: now},
		{Id: 1650226, StationName: "SR-210 @ White Pine", AirTemperature: str("68.1"), WindSpeedAvg: str("6.0"), WindDirection: str("W"), Source: "dev", LastUpdated: now},
		{Id: 1650160, StationName: "Alta - Mt Baldy", AirTemperature: str("51.0"), WindSpeedAvg: str("12.0"), WindDirection: str("NW"), Source: "dev", LastUpdated: now},
		{Id: 1650231, StationName: "Alta - Collins", AirTemperature: str("48.5"), WindSpeedAvg: str("3.1"), WindDirection: str("N"), Source: "dev", LastUpdated: now},
		{Id: 1844865, StationName: "SR-210 @ Alta", AirTemperature: str("51.4"), WindSpeedAvg: str("1.8"), WindDirection: str("W"), Source: "dev", LastUpdated: now},
		{Id: 1650085, StationName: "SR-190 @ S-Turns", AirTemperature: str("74.0"), WindSpeedAvg: str("5.5"), WindDirection: str("S"), Source: "dev", LastUpdated: now},
		{Id: 1650218, StationName: "SR-190 @ Cardiff", AirTemperature: str("48.7"), WindSpeedAvg: str("0.0"), WindDirection: str("N"), Source: "dev", LastUpdated: now},
		{Id: 1650159, StationName: "Spruces", AirTemperature: str("47.5"), WindSpeedAvg: str("0.1"), WindDirection: str("N"), Source: "dev", LastUpdated: now},
		{Id: 1650206, StationName: "SR-190 @ Brighton", AirTemperature: str("48.5"), WindSpeedAvg: str("1.9"), WindDirection: str("W"), Source: "dev", LastUpdated: now},
		{Id: 1650069, StationName: "US-189 Provo Canyon", AirTemperature: str("61.8"), WindSpeedAvg: str("4.0"), WindDirection: str("W"), Source: "dev", LastUpdated: now},
		{Id: 1650162, StationName: "US-189 @ Lower Provo Canyon", AirTemperature: str("62.2"), WindSpeedAvg: str("3.0"), WindDirection: str("SW"), Source: "dev", LastUpdated: now},
		{Id: 1650163, StationName: "US-189 @ Deer Creek Dam", AirTemperature: str("59.2"), WindSpeedAvg: str("5.0"), WindDirection: str("W"), Source: "dev", LastUpdated: now},
		{Id: 1650253, StationName: "SR-92 @ American Fork Canyon", AirTemperature: str("53.4"), WindSpeedAvg: str("2.0"), WindDirection: str("N"), Source: "dev", LastUpdated: now},
		{Id: 1650136, StationName: "I-80 @ Mouth of Parleys", AirTemperature: str("67.8"), WindSpeedAvg: str("6.0"), WindDirection: str("W"), Source: "dev", LastUpdated: now},
		{Id: 1650118, StationName: "I-80 @ East Canyon", AirTemperature: str("53.8"), WindSpeedAvg: str("8.0"), WindDirection: str("W"), Source: "dev", LastUpdated: now},
		{Id: 1650059, StationName: "I-80 @ Parleys Summit", AirTemperature: str("58.9"), WindSpeedAvg: str("10.0"), WindDirection: str("W"), Source: "dev", LastUpdated: now},
		{Id: 1650263, StationName: "I-15 @ MP 29 Browse NB", AirTemperature: str("67.4"), WindSpeedAvg: str("6.0"), WindDirection: str("S"), Source: "dev", LastUpdated: now},
		{Id: 1650110, StationName: "I-15 @ MP 37 Ash Creek", AirTemperature: str("57.6"), WindSpeedAvg: str("8.0"), WindDirection: str("SW"), Source: "dev", LastUpdated: now},
		{Id: 1650056, StationName: "I-15 @ Black Ridge", AirTemperature: str("64.2"), WindSpeedAvg: str("7.0"), WindDirection: str("S"), Source: "dev", LastUpdated: now},
		{Id: 1650247, StationName: "I-15 @ MP 54 Hamiltons Fort", AirTemperature: str("59.3"), WindSpeedAvg: str("5.0"), WindDirection: str("W"), Source: "dev", LastUpdated: now},
		{Id: 1650076, StationName: "SR-143 @ Brian Head", AirTemperature: str("47.6"), WindSpeedAvg: str("9.0"), WindDirection: str("SW"), Source: "dev", LastUpdated: now},
	})
	logger.Warn("UDOT_API_KEY not set — seeded dev weather stations for UI")
}

// StartWeatherStations starts polling weather stations
func (p *Poller) StartWeatherStations(ctx context.Context) error {
	if !p.client.IsConfigured() {
		logger.Warn("UDOT_API_KEY not set. Skipping weather stations fetching.")
		if os.Getenv("DEV_MODE") == "1" || os.Getenv("DEV_MODE") == "true" {
			p.seedDevWeatherStations()
		}
		return nil
	}

	return p.pollNowAndEvery(ctx, p.pollWeatherStations)
}

// StartEvents starts polling traffic events
func (p *Poller) StartEvents(ctx context.Context) error {
	if !p.client.IsConfigured() {
		logger.Warn("UDOT_API_KEY not set. Skipping events fetching.")
		if os.Getenv("DEV_MODE") == "1" || os.Getenv("DEV_MODE") == "true" {
			p.seedDevEvents()
		}
		return nil
	}

	return p.pollNowAndEvery(ctx, p.pollEvents)
}

func (p *Poller) pollRoadConditions(ctx context.Context) {
	conditions, notModified, err := p.client.FetchRoadConditions(ctx)
	if err != nil {
		logger.Error(err, "Failed to fetch road conditions: %v", err)
		return
	}
	if notModified {
		logger.Muted("Road conditions unchanged (304 Not Modified)")
		return
	}

	roads := RoadsByCanyon(conditions)
	for _, id := range p.store.CanyonIDs() {
		p.store.UpdateRoadConditions(id, roads[id])
	}
	logger.Muted("Updated road conditions for %d canyons", len(p.store.CanyonIDs()))
	p.changed()
}

func (p *Poller) pollWeatherStations(ctx context.Context) {
	stations, notModified, err := p.client.FetchWeatherStations(ctx)
	if err != nil {
		logger.Error(err, "Failed to fetch weather stations: %v", err)
		return
	}
	if notModified {
		logger.Muted("Weather stations unchanged (304 Not Modified)")
		return
	}

	p.store.StoreWeatherStationsById(stations)
}

func (p *Poller) pollEvents(ctx context.Context) {
	events, notModified, err := p.client.FetchEvents(ctx)
	if err != nil {
		logger.Error(err, "Failed to fetch events: %v", err)
		return
	}
	if notModified {
		logger.Muted("Events unchanged (304 Not Modified)")
		return
	}

	grouped := EventsByCanyon(events)
	for _, id := range p.store.CanyonIDs() {
		p.store.UpdateEvents(id, grouped[id])
	}
	logger.Muted("Updated events for %d canyons", len(p.store.CanyonIDs()))
	p.changed()
}
