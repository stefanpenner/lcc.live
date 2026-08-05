// Package synoptic fetches latest mountain weather from Synoptic (MesoWest)
// when SYNOPTIC_TOKEN is set, otherwise from free NWS station observations
// (same MesoWest STIDs: CLN, ATB, AMB, SBDU1, BRC, …).
package synoptic

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/stefanpenner/lcc-live/web/store"
)

const (
	synopticLatestURL = "https://api.synopticdata.com/v2/stations/latest"
	nwsStationURL     = "https://api.weather.gov/stations/%s/observations/latest"
	userAgent         = "lcc.live (https://lcc.live; canyon conditions)"
)

// Client fetches latest observations for MesoWest-style station IDs.
type Client struct {
	http  *http.Client
	token string // Synoptic public token; empty → NWS free path
}

// NewClient creates a client. Empty token uses free NWS observations.
func NewClient(token string) *Client {
	return &Client{
		http:  &http.Client{Timeout: 25 * time.Second},
		token: strings.TrimSpace(token),
	}
}

// HasSynopticToken reports whether Synoptic API auth is configured.
func (c *Client) HasSynopticToken() bool {
	return c != nil && c.token != ""
}

// FetchLatest returns weather for the given STIDs (e.g. CLN, SBDU1).
func (c *Client) FetchLatest(ctx context.Context, stids []string) ([]store.WeatherStation, error) {
	stids = uniqueNonEmpty(stids)
	if len(stids) == 0 {
		return nil, nil
	}
	if c.HasSynopticToken() {
		return c.fetchSynoptic(ctx, stids)
	}
	return c.fetchNWS(ctx, stids)
}

func uniqueNonEmpty(in []string) []string {
	seen := make(map[string]struct{}, len(in))
	var out []string
	for _, s := range in {
		s = strings.TrimSpace(s)
		if s == "" {
			continue
		}
		key := strings.ToUpper(s)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, key)
	}
	return out
}

// ---- Synoptic API ----

type synopticResponse struct {
	Summary struct {
		ResponseCode    int    `json:"RESPONSE_CODE"`
		ResponseMessage string `json:"RESPONSE_MESSAGE"`
	} `json:"SUMMARY"`
	Station []synopticStation `json:"STATION"`
}

type synopticStation struct {
	Stid         string            `json:"STID"`
	Name         string            `json:"NAME"`
	Latitude     json.RawMessage   `json:"LATITUDE"`
	Longitude    json.RawMessage   `json:"LONGITUDE"`
	Observations map[string]obsVal `json:"OBSERVATIONS"`
}

type obsVal struct {
	Value     *float64 `json:"value"`
	DateTime  string   `json:"date_time"`
}

func (c *Client) fetchSynoptic(ctx context.Context, stids []string) ([]store.WeatherStation, error) {
	q := url.Values{}
	q.Set("token", c.token)
	q.Set("stid", strings.Join(stids, ","))
	q.Set("units", "english")
	q.Set("vars", "air_temp,relative_humidity,wind_speed,wind_gust,wind_direction,dew_point_temperature")
	q.Set("within", "180") // minutes

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, synopticLatestURL+"?"+q.Encode(), nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", userAgent)
	req.Header.Set("Accept", "application/json")

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()

	body, err := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("synoptic latest: HTTP %d: %s", resp.StatusCode, truncate(string(body), 200))
	}

	return ParseSynopticLatest(body)
}

// ParseSynopticLatest maps Synoptic JSON into store.WeatherStation slices.
func ParseSynopticLatest(data []byte) ([]store.WeatherStation, error) {
	var root synopticResponse
	if err := json.Unmarshal(data, &root); err != nil {
		return nil, fmt.Errorf("synoptic json: %w", err)
	}
	if root.Summary.ResponseCode != 1 && root.Summary.ResponseCode != 0 {
		// RESPONSE_CODE 1 = OK in current Synoptic docs; some versions use 0
		if root.Summary.ResponseMessage != "" && !strings.EqualFold(root.Summary.ResponseMessage, "OK") {
			return nil, fmt.Errorf("synoptic: %s (code %d)", root.Summary.ResponseMessage, root.Summary.ResponseCode)
		}
	}

	out := make([]store.WeatherStation, 0, len(root.Station))
	for _, st := range root.Station {
		if st.Stid == "" {
			continue
		}
		ws := store.WeatherStation{
			StationName:    nonEmpty(st.Name, st.Stid),
			Latitude:       rawFloat(st.Latitude),
			Longitude:      rawFloat(st.Longitude),
			Source:         "synoptic",
			CameraSourceId: strPtr(strings.ToUpper(st.Stid)),
		}
		obs := st.Observations
		now := time.Now().Unix()
		ws.AirTemperature = floatStrFresh(obs, "air_temp_value_1", now)
		ws.RelativeHumidity = floatStrFresh(obs, "relative_humidity_value_1", now)
		ws.DewpointTemp = floatStrFresh(obs, "dew_point_temperature_value_1", now)
		// Wind: drop if that variable's timestamp is stale (don't mix old wind with fresh temp)
		ws.WindSpeedAvg = floatStrFresh(obs, "wind_speed_value_1", now)
		ws.WindSpeedGust = floatStrFresh(obs, "wind_gust_value_1", now)
		if dir := floatStrFresh(obs, "wind_direction_value_1", now); dir != nil {
			if deg, err := strconv.ParseFloat(*dir, 64); err == nil {
				ws.WindDirection = strPtr(degreesToCompass(deg))
			}
		} else {
			// No fresh wind direction without fresh wind
			ws.WindDirection = nil
		}
		// If wind was dropped as stale, clear direction too when speed missing
		if ws.WindSpeedAvg == nil {
			ws.WindDirection = nil
			ws.WindSpeedGust = nil
		}
		ws.LastUpdated = latestFreshObsUnix(obs, now)
		if ws.LastUpdated == 0 {
			ws.LastUpdated = now
		}
		out = append(out, ws)
	}
	return out, nil
}

// obsStaleSec: same 2h window as server isStale — older samples are dropped.
const obsStaleSec int64 = 7200

func floatStrFresh(obs map[string]obsVal, key string, now int64) *string {
	if obs == nil {
		return nil
	}
	v, ok := obs[key]
	if !ok || v.Value == nil {
		return nil
	}
	if ts := parseObsTime(v.DateTime); ts > 0 && now-ts > obsStaleSec {
		return nil // stale field — omit
	}
	s := strconv.FormatFloat(*v.Value, 'f', 1, 64)
	return &s
}

func parseObsTime(s string) int64 {
	if s == "" {
		return 0
	}
	t, err := time.Parse(time.RFC3339, s)
	if err != nil {
		t, err = time.Parse("2006-01-02T15:04:05Z", s)
	}
	if err != nil {
		return 0
	}
	return t.Unix()
}

func latestFreshObsUnix(obs map[string]obsVal, now int64) int64 {
	var best int64
	for _, v := range obs {
		u := parseObsTime(v.DateTime)
		if u == 0 || now-u > obsStaleSec {
			continue
		}
		if u > best {
			best = u
		}
	}
	return best
}

// ---- NWS free path (no token) ----

type nwsObservation struct {
	Properties struct {
		Timestamp         string `json:"timestamp"`
		Station           string `json:"station"`
		Temperature       nwsQty `json:"temperature"`
		Dewpoint          nwsQty `json:"dewpoint"`
		WindDirection     nwsQty `json:"windDirection"`
		WindSpeed         nwsQty `json:"windSpeed"`
		WindGust          nwsQty `json:"windGust"`
		RelativeHumidity  nwsQty `json:"relativeHumidity"`
	} `json:"properties"`
}

type nwsQty struct {
	Value          *float64 `json:"value"`
	UnitCode       string   `json:"unitCode"`
}

type nwsStationMeta struct {
	Properties struct {
		Name      string `json:"name"`
		Elevation struct {
			Value *float64 `json:"value"`
		} `json:"elevation"`
	} `json:"properties"`
	Geometry struct {
		Coordinates []float64 `json:"coordinates"` // [lon, lat]
	} `json:"geometry"`
}

func (c *Client) fetchNWS(ctx context.Context, stids []string) ([]store.WeatherStation, error) {
	var out []store.WeatherStation
	var errs []string
	for _, stid := range stids {
		ws, err := c.fetchNWSOne(ctx, stid)
		if err != nil {
			errs = append(errs, fmt.Sprintf("%s: %v", stid, err))
			continue
		}
		if ws != nil {
			out = append(out, *ws)
		}
	}
	if len(out) == 0 && len(errs) > 0 {
		return nil, fmt.Errorf("nws stations: %s", strings.Join(errs, "; "))
	}
	return out, nil
}

func (c *Client) fetchNWSOne(ctx context.Context, stid string) (*store.WeatherStation, error) {
	obsURL := fmt.Sprintf(nwsStationURL, url.PathEscape(stid))
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, obsURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", userAgent)
	req.Header.Set("Accept", "application/geo+json")

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return nil, err
	}
	if resp.StatusCode == http.StatusNotFound {
		return nil, fmt.Errorf("no latest observation")
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HTTP %d: %s", resp.StatusCode, truncate(string(body), 120))
	}

	ws, err := ParseNWSObservation(stid, body)
	if err != nil {
		return nil, err
	}

	// Optional station name/coords from metadata (best-effort)
	metaURL := fmt.Sprintf("https://api.weather.gov/stations/%s", url.PathEscape(stid))
	if mreq, err := http.NewRequestWithContext(ctx, http.MethodGet, metaURL, nil); err == nil {
		mreq.Header.Set("User-Agent", userAgent)
		mreq.Header.Set("Accept", "application/geo+json")
		if mresp, err := c.http.Do(mreq); err == nil {
			mb, _ := io.ReadAll(io.LimitReader(mresp.Body, 1<<20))
			_ = mresp.Body.Close()
			if mresp.StatusCode == http.StatusOK {
				var meta nwsStationMeta
				if json.Unmarshal(mb, &meta) == nil {
					if meta.Properties.Name != "" {
						ws.StationName = meta.Properties.Name
					}
					if len(meta.Geometry.Coordinates) >= 2 {
						lon := meta.Geometry.Coordinates[0]
						lat := meta.Geometry.Coordinates[1]
						ws.Longitude = &lon
						ws.Latitude = &lat
					}
				}
			}
		}
	}
	return ws, nil
}

// ParseNWSObservation converts an NWS /observations/latest payload.
func ParseNWSObservation(stid string, data []byte) (*store.WeatherStation, error) {
	var root nwsObservation
	if err := json.Unmarshal(data, &root); err != nil {
		return nil, fmt.Errorf("nws json: %w", err)
	}
	p := root.Properties
	ws := store.WeatherStation{
		StationName:    stid,
		Source:         "nws",
		CameraSourceId: strPtr(strings.ToUpper(stid)),
	}
	if p.Timestamp != "" {
		if t, err := time.Parse(time.RFC3339, p.Timestamp); err == nil {
			ws.LastUpdated = t.Unix()
		}
	}
	if ws.LastUpdated == 0 {
		ws.LastUpdated = time.Now().Unix()
	}

	// Whole-observation staleness (NWS has one timestamp for all fields)
	if ws.LastUpdated > 0 && time.Now().Unix()-ws.LastUpdated > obsStaleSec {
		// Keep structure but strip weather values — caller may still want station id
		return &ws, nil
	}

	// Temperature: typically °C
	if p.Temperature.Value != nil {
		f := nwsTempToF(*p.Temperature.Value, p.Temperature.UnitCode)
		ws.AirTemperature = strPtr(fmt.Sprintf("%.1f", f))
	}
	if p.Dewpoint.Value != nil {
		f := nwsTempToF(*p.Dewpoint.Value, p.Dewpoint.UnitCode)
		ws.DewpointTemp = strPtr(fmt.Sprintf("%.1f", f))
	}
	if p.RelativeHumidity.Value != nil {
		ws.RelativeHumidity = strPtr(fmt.Sprintf("%.0f", *p.RelativeHumidity.Value))
	}
	// Wind: honor unitCode (NWS often sends km_h-1, not m_s-1)
	if p.WindSpeed.Value != nil {
		mph := nwsSpeedToMPH(*p.WindSpeed.Value, p.WindSpeed.UnitCode)
		ws.WindSpeedAvg = strPtr(fmt.Sprintf("%.1f", mph))
	}
	if p.WindGust.Value != nil {
		mph := nwsSpeedToMPH(*p.WindGust.Value, p.WindGust.UnitCode)
		ws.WindSpeedGust = strPtr(fmt.Sprintf("%.1f", mph))
	}
	if p.WindDirection.Value != nil && ws.WindSpeedAvg != nil {
		ws.WindDirection = strPtr(degreesToCompass(*p.WindDirection.Value))
	}
	return &ws, nil
}

// nwsTempToF converts NWS temperature to °F using unitCode when present.
func nwsTempToF(v float64, unit string) float64 {
	u := strings.ToLower(unit)
	switch {
	case strings.Contains(u, "degf") || strings.Contains(u, "fahrenheit"):
		return v
	default: // degC / kelvin rare here
		if strings.Contains(u, "kelvin") || strings.Contains(u, "degk") {
			return (v-273.15)*9/5 + 32
		}
		return v*9/5 + 32 // degC
	}
}

// nwsSpeedToMPH converts NWS wind to mph. NWS mountain stations commonly use km_h-1.
func nwsSpeedToMPH(v float64, unit string) float64 {
	u := strings.ToLower(unit)
	switch {
	case strings.Contains(u, "km_h") || strings.Contains(u, "km/h") || strings.Contains(u, "kilometr"):
		return v * 0.621371
	case strings.Contains(u, "mi_h") || strings.Contains(u, "mph") || strings.Contains(u, "mile"):
		return v
	case strings.Contains(u, "kn") || strings.Contains(u, "kt"):
		return v * 1.15078
	default:
		// wmoUnit:m_s-1 and unspecified SI
		return v * 2.236936
	}
}

func degreesToCompass(deg float64) string {
	// 16-point compass
	dirs := []string{"N", "NNE", "NE", "ENE", "E", "ESE", "SE", "SSE",
		"S", "SSW", "SW", "WSW", "W", "WNW", "NW", "NNW"}
	deg = math.Mod(deg, 360)
	if deg < 0 {
		deg += 360
	}
	idx := int(math.Floor((deg+11.25)/22.5)) % 16
	return dirs[idx]
}

func strPtr(s string) *string { return &s }

// rawFloat accepts JSON number or string.
func rawFloat(raw json.RawMessage) *float64 {
	if len(raw) == 0 || string(raw) == "null" {
		return nil
	}
	var f float64
	if err := json.Unmarshal(raw, &f); err == nil {
		return &f
	}
	var s string
	if err := json.Unmarshal(raw, &s); err == nil {
		if v, err := strconv.ParseFloat(s, 64); err == nil {
			return &v
		}
	}
	return nil
}

func nonEmpty(a, b string) string {
	if strings.TrimSpace(a) != "" {
		return a
	}
	return b
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}
