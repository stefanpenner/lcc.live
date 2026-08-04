package uac

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/stefanpenner/lcc-live/web/store"
)

const (
	mapLayerURL = "https://api.avalanche.org/v2/public/products/map-layer/UAC"
	userAgent   = "lcc.live (https://lcc.live; canyon conditions)"
	saltLake    = "Salt Lake"
)

// Client fetches UAC map-layer products.
type Client struct {
	http *http.Client
}

// NewClient creates a UAC HTTP client.
func NewClient() *Client {
	return &Client{
		http: &http.Client{Timeout: 20 * time.Second},
	}
}

type mapLayerResponse struct {
	Features []struct {
		Properties struct {
			Name         string `json:"name"`
			Danger       string `json:"danger"`
			DangerLevel  int    `json:"danger_level"`
			Link         string `json:"link"`
			TravelAdvice string `json:"travel_advice"`
		} `json:"properties"`
	} `json:"features"`
}

// FetchSaltLakeDanger fetches UAC Salt Lake avalanche danger.
func (c *Client) FetchSaltLakeDanger(ctx context.Context) (*store.AvalancheDanger, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, mapLayerURL, nil)
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

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return nil, fmt.Errorf("uac map-layer: HTTP %d: %s", resp.StatusCode, string(body))
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, 2<<20))
	if err != nil {
		return nil, err
	}

	return ParseSaltLakeDanger(body)
}

// ParseSaltLakeDanger extracts Salt Lake danger from map-layer JSON.
func ParseSaltLakeDanger(data []byte) (*store.AvalancheDanger, error) {
	var layer mapLayerResponse
	if err := json.Unmarshal(data, &layer); err != nil {
		return nil, fmt.Errorf("uac map-layer json: %w", err)
	}

	for _, f := range layer.Features {
		p := f.Properties
		if p.Name != saltLake {
			continue
		}
		danger := p.Danger
		if danger == "" {
			danger = "no rating"
		}
		link := p.Link
		if link == "" {
			link = "https://utahavalanchecenter.org/forecast/salt-lake"
		}
		return &store.AvalancheDanger{
			Danger:       danger,
			DangerLevel:  p.DangerLevel,
			Link:         link,
			TravelAdvice: p.TravelAdvice,
			Updated:      time.Now().Unix(),
		}, nil
	}
	return nil, fmt.Errorf("uac map-layer: Salt Lake feature not found")
}
