package udot

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/stefanpenner/lcc-live/store"
)

const (
	baseURL   = "https://www.udottraffic.utah.gov/api/v2"
	userAgent = "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36"
)

// Client provides access to UDOT API endpoints
type Client struct {
	apiKey  string
	client  *http.Client
	timeout time.Duration
	// ETags for conditional requests
	etags   map[string]string // Maps endpoint -> ETag
	etagsMu sync.RWMutex
}

// NewClient creates a new UDOT API client
func NewClient(apiKey string) *Client {
	return &Client{
		apiKey:  apiKey,
		client:  &http.Client{Timeout: 10 * time.Second},
		timeout: 10 * time.Second,
		etags:   make(map[string]string),
	}
}

// IsConfigured returns true if the client has an API key
func (c *Client) IsConfigured() bool {
	return c.apiKey != ""
}

// FetchRoadConditions fetches all road conditions from the UDOT API
func (c *Client) FetchRoadConditions(ctx context.Context) ([]store.RoadCondition, error) {
	if !c.IsConfigured() {
		return nil, fmt.Errorf("UDOT_API_KEY not set")
	}

	url := fmt.Sprintf("%s/get/roadconditions?key=%s&format=json", baseURL, c.apiKey)
	return fetchJSON[store.RoadCondition](ctx, c, url, "roadconditions")
}

// FetchWeatherStations fetches all weather stations from the UDOT API
func (c *Client) FetchWeatherStations(ctx context.Context) ([]store.WeatherStation, error) {
	if !c.IsConfigured() {
		return nil, fmt.Errorf("UDOT_API_KEY not set")
	}

	url := fmt.Sprintf("%s/get/weatherstations?key=%s&format=json", baseURL, c.apiKey)
	return fetchJSON[store.WeatherStation](ctx, c, url, "weatherstations")
}

// FetchEvents fetches all traffic events from the UDOT API
func (c *Client) FetchEvents(ctx context.Context) ([]store.Event, error) {
	if !c.IsConfigured() {
		return nil, fmt.Errorf("UDOT_API_KEY not set")
	}

	url := fmt.Sprintf("%s/get/event?key=%s&format=json", baseURL, c.apiKey)
	return fetchJSON[store.Event](ctx, c, url, "events")
}

// UDOTCamera represents a camera from the UDOT cameras API
type UDOTCamera struct {
	Id        int     `json:"Id"`
	SourceId  string  `json:"SourceId"`
	Latitude  float64 `json:"Latitude"`
	Longitude float64 `json:"Longitude"`
}

// FetchCameras fetches all cameras from the UDOT API
func (c *Client) FetchCameras(ctx context.Context) ([]UDOTCamera, error) {
	if !c.IsConfigured() {
		return nil, fmt.Errorf("UDOT_API_KEY not set")
	}

	url := fmt.Sprintf("%s/get/cameras?key=%s&format=json", baseURL, c.apiKey)
	return fetchJSON[UDOTCamera](ctx, c, url, "cameras")
}

// fetchJSON is a generic helper to fetch and decode JSON from the API
// It respects ETags and caching headers for conditional requests
func fetchJSON[T any](ctx context.Context, client *Client, url string, endpoint string) ([]T, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("User-Agent", userAgent)

	// Add If-None-Match header if we have a cached ETag
	client.etagsMu.RLock()
	if etag, exists := client.etags[endpoint]; exists && etag != "" {
		req.Header.Set("If-None-Match", etag)
	}
	client.etagsMu.RUnlock()

	resp, err := client.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch: %w", err)
	}
	defer resp.Body.Close()

	// Handle 304 Not Modified - data hasn't changed
	if resp.StatusCode == http.StatusNotModified {
		// Return nil to indicate no update needed
		// Caller should keep using existing data
		return nil, nil
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("API returned status %d", resp.StatusCode)
	}

	// Store ETag from response for next request
	if etag := resp.Header.Get("ETag"); etag != "" {
		client.etagsMu.Lock()
		client.etags[endpoint] = etag
		client.etagsMu.Unlock()
	}

	var results []T
	if err := json.NewDecoder(resp.Body).Decode(&results); err != nil {
		return nil, fmt.Errorf("failed to decode JSON: %w", err)
	}

	return results, nil
}
