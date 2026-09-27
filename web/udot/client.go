// Package udot fetches road conditions, weather stations, and events from the UDOT API.
// notModified is HTTP 304: keep the stored copy. The slice is not a body.
package udot

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"

	"github.com/stefanpenner/lcc-live/web/store"
)

const (
	baseURL   = "https://www.udottraffic.utah.gov/api/v2"
	userAgent = "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36"
)

// Client fetches UDOT feeds. ETags make the next request conditional.
type Client struct {
	apiKey  string
	client  *http.Client
	etags   map[string]string
	etagsMu sync.RWMutex
}

// NewClient returns a client for apiKey. An empty key is not configured.
func NewClient(apiKey string) *Client {
	if len(apiKey) > 0 && len(apiKey) < 32 {
		fmt.Printf("WARNING: UDOT_API_KEY seems too short (%d chars). Expecting ~32 characters.\n", len(apiKey))
	}
	return &Client{
		apiKey: apiKey,
		client: &http.Client{Timeout: 30 * time.Second},
		etags:  make(map[string]string),
	}
}

// IsConfigured reports whether an API key is set.
func (c *Client) IsConfigured() bool {
	return c.apiKey != ""
}

// FetchRoadConditions returns the road-condition feed. notModified is HTTP 304.
func (c *Client) FetchRoadConditions(ctx context.Context) ([]store.RoadCondition, bool, error) {
	return fetchJSON[store.RoadCondition](ctx, c, "get/roadconditions", "roadconditions")
}

// FetchWeatherStations returns the weather-station feed. notModified is HTTP 304.
func (c *Client) FetchWeatherStations(ctx context.Context) ([]store.WeatherStation, bool, error) {
	return fetchJSON[store.WeatherStation](ctx, c, "get/weatherstations", "weatherstations")
}

// FetchEvents returns the traffic-event feed. notModified is HTTP 304.
func (c *Client) FetchEvents(ctx context.Context) ([]store.Event, bool, error) {
	return fetchJSON[store.Event](ctx, c, "get/event", "events")
}

func fetchJSON[T any](ctx context.Context, c *Client, path, endpoint string) ([]T, bool, error) {
	if !c.IsConfigured() {
		return nil, false, fmt.Errorf("UDOT_API_KEY not set")
	}

	url := fmt.Sprintf("%s/%s?key=%s&format=json", baseURL, path, c.apiKey)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, false, fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("User-Agent", userAgent)
	if etag := c.cachedETag(endpoint); etag != "" {
		req.Header.Set("If-None-Match", etag)
	}

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, false, fmt.Errorf("failed to fetch: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotModified {
		return nil, true, nil
	}
	if resp.StatusCode != http.StatusOK {
		return nil, false, statusError(resp)
	}

	c.rememberETag(endpoint, resp.Header.Get("ETag"))

	var results []T
	if err := json.NewDecoder(resp.Body).Decode(&results); err != nil {
		return nil, false, fmt.Errorf("failed to decode JSON: %w", err)
	}
	return results, false, nil
}

func (c *Client) cachedETag(endpoint string) string {
	c.etagsMu.RLock()
	defer c.etagsMu.RUnlock()
	return c.etags[endpoint]
}

func (c *Client) rememberETag(endpoint, etag string) {
	if etag == "" {
		return
	}
	c.etagsMu.Lock()
	c.etags[endpoint] = etag
	c.etagsMu.Unlock()
}

func statusError(resp *http.Response) error {
	body, _ := io.ReadAll(resp.Body)
	if len(body) > 0 {
		return fmt.Errorf("API returned status %d: %s", resp.StatusCode, string(body))
	}
	return fmt.Errorf("API returned status %d", resp.StatusCode)
}
