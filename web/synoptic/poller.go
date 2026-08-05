package synoptic

import (
	"context"
	"os"
	"time"

	"github.com/stefanpenner/lcc-live/web/logger"
	"github.com/stefanpenner/lcc-live/web/store"
)

// Poller periodically refreshes mountain weather for cameras with synopticStid.
type Poller struct {
	client   *Client
	store    *store.Store
	interval time.Duration
}

// NewPoller creates a poller. Default interval 10 minutes if zero
// (station reports are typically 5–15+ min; no need to hammer free tier).
func NewPoller(client *Client, s *store.Store, interval time.Duration) *Poller {
	if interval <= 0 {
		interval = 10 * time.Minute
	}
	return &Poller{client: client, store: s, interval: interval}
}

// Start polls until ctx is cancelled.
func (p *Poller) Start(ctx context.Context) error {
	if p.client.HasSynopticToken() {
		logger.Info("Synoptic weather: using SYNOPTIC_TOKEN (MesoWest API)")
	} else {
		logger.Info("Synoptic weather: no SYNOPTIC_TOKEN — using free NWS station observations")
	}

	// DEV seed only when neither path can run live... we always can hit NWS,
	// but seed helps offline / flaky network demos.
	if os.Getenv("DEV_MODE") == "1" || os.Getenv("DEV_MODE") == "true" {
		// Still try live first below; seed is only if first poll returns nothing.
	}

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
	stids := p.store.SynopticStids()
	if len(stids) == 0 {
		logger.Muted("No synopticStid values on cameras; skipping mountain weather poll")
		return
	}

	stations, err := p.client.FetchLatest(ctx, stids)
	if err != nil {
		logger.Error(err, "Failed to fetch mountain weather: %v", err)
		if os.Getenv("DEV_MODE") == "1" || os.Getenv("DEV_MODE") == "true" {
			p.seedDev(stids)
		}
		return
	}
	if len(stations) == 0 {
		logger.Muted("Mountain weather poll returned 0 stations for %d stids", len(stids))
		if os.Getenv("DEV_MODE") == "1" || os.Getenv("DEV_MODE") == "true" {
			p.seedDev(stids)
		}
		return
	}
	p.store.StoreWeatherStationsByStid(stations)
	src := "nws"
	if p.client.HasSynopticToken() {
		src = "synoptic"
	}
	logger.Muted("Updated mountain weather (%s): %d stations", src, len(stations))
}

func (p *Poller) seedDev(stids []string) {
	now := time.Now().Unix()
	str := func(s string) *string { return &s }
	// Representative summer-ish samples keyed by known canyon stids
	samples := map[string]store.WeatherStation{
		"CLN":   {StationName: "ALTA - COLLINS", AirTemperature: str("52.0"), WindSpeedAvg: str("8.0"), WindDirection: str("W"), Source: "dev", LastUpdated: now},
		"ATB":   {StationName: "ALTA - BASE", AirTemperature: str("58.0"), WindSpeedAvg: str("4.0"), WindDirection: str("SW"), Source: "dev", LastUpdated: now},
		"AMB":   {StationName: "ALTA - MT BALDY", AirTemperature: str("48.0"), WindSpeedAvg: str("18.0"), WindDirection: str("NW"), Source: "dev", LastUpdated: now},
		"AGD":   {StationName: "ALTA - GUARD HOUSE", AirTemperature: str("55.0"), WindSpeedAvg: str("6.0"), WindDirection: str("S"), Source: "dev", LastUpdated: now},
		"ALT":   {StationName: "ALTA - TOP OF COLLINS", AirTemperature: str("50.0"), WindSpeedAvg: str("14.0"), WindDirection: str("W"), Source: "dev", LastUpdated: now},
		"SBDU1": {StationName: "SNOWBIRD", AirTemperature: str("54.0"), WindSpeedAvg: str("10.0"), WindDirection: str("W"), Source: "dev", LastUpdated: now},
		"BRIU1": {StationName: "BRIGHTON", AirTemperature: str("56.0"), WindSpeedAvg: str("5.0"), WindDirection: str("S"), Source: "dev", LastUpdated: now},
		"BRC":   {StationName: "BRIGHTON CREST", AirTemperature: str("51.0"), WindSpeedAvg: str("12.0"), WindDirection: str("W"), Source: "dev", LastUpdated: now},
		"REY":   {StationName: "Reynolds Peak", AirTemperature: str("53.0"), WindSpeedAvg: str("15.0"), WindDirection: str("NW"), Source: "dev", LastUpdated: now},
		"MLDU1": {StationName: "MILL-D NORTH", AirTemperature: str("60.0"), WindSpeedAvg: str("4.0"), WindDirection: str("SE"), Source: "dev", LastUpdated: now},
	}
	var out []store.WeatherStation
	for _, stid := range stids {
		if s, ok := samples[stid]; ok {
			s.CameraSourceId = str(stid)
			out = append(out, s)
		} else {
			out = append(out, store.WeatherStation{
				StationName:    stid,
				AirTemperature: str("50.0"),
				Source:         "dev",
				CameraSourceId: str(stid),
				LastUpdated:    now,
			})
		}
	}
	p.store.StoreWeatherStationsByStid(out)
	logger.Warn("Mountain weather: seeded dev stations (%d)", len(out))
}
