package alta

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/stefanpenner/lcc-live/web/store"
)

const (
	weatherURL = "https://www.alta.com/weather"
	userAgent  = "lcc.live (https://lcc.live; canyon conditions)"
)

// Client fetches Alta weather page notices.
type Client struct {
	http *http.Client
}

// NewClient creates an Alta HTTP client.
func NewClient() *Client {
	return &Client{
		http: &http.Client{Timeout: 20 * time.Second},
	}
}

var windowAltaRe = regexp.MustCompile(`window\.Alta\s*=\s*\{`)

type altaBootstrap struct {
	Alerts []altaAlert `json:"alerts"`
}

type altaAlert struct {
	Name    string  `json:"name"`
	Slug    string  `json:"slug"`
	Type    string  `json:"type"`
	Status  string  `json:"status"`
	Message *string `json:"message"`
	Enabled bool    `json:"enabled"`
}

// FetchStatus fetches Alta parking/road status from the weather page.
func (c *Client) FetchStatus(ctx context.Context) (*store.AltaStatus, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, weatherURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", userAgent)
	req.Header.Set("Accept", "text/html")

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return nil, fmt.Errorf("alta weather: HTTP %d: %s", resp.StatusCode, string(body))
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	if err != nil {
		return nil, err
	}

	return ParseAltaStatus(body)
}

// ParseAltaStatus extracts parking/road status from Alta weather HTML.
func ParseAltaStatus(html []byte) (*store.AltaStatus, error) {
	loc := windowAltaRe.FindIndex(html)
	if loc == nil {
		return nil, fmt.Errorf("alta weather: window.Alta bootstrap not found")
	}
	// Find opening brace (regex ends at '{')
	start := loc[1] - 1
	if start < 0 || start >= len(html) || html[start] != '{' {
		return nil, fmt.Errorf("alta weather: window.Alta JSON start not found")
	}
	raw, err := extractJSONObject(html[start:])
	if err != nil {
		return nil, fmt.Errorf("alta weather: %w", err)
	}
	return statusFromBootstrap(raw)
}

func statusFromBootstrap(raw []byte) (*store.AltaStatus, error) {
	var boot altaBootstrap
	if err := json.Unmarshal(raw, &boot); err != nil {
		return nil, fmt.Errorf("alta weather json: %w", err)
	}

	st := &store.AltaStatus{
		Updated: time.Now().Unix(),
	}

	for _, a := range boot.Alerts {
		if !a.Enabled {
			continue
		}
		typ := strings.ToLower(strings.TrimSpace(a.Type))
		slug := strings.ToLower(strings.TrimSpace(a.Slug))
		name := strings.ToLower(strings.TrimSpace(a.Name))
		msg := ""
		if a.Message != nil {
			msg = strings.TrimSpace(*a.Message)
		}

		if typ == "parking-status" || slug == "parking-status" {
			st.ParkingStatus = a.Status
			st.ParkingMessage = msg
			continue
		}
		if typ == "summer-road-status" ||
			slug == "summer-road-status" ||
			strings.Contains(name, "road status") {
			st.RoadStatus = a.Status
			st.RoadMessage = msg
		}
	}

	if st.ParkingStatus == "" && st.RoadStatus == "" {
		return nil, fmt.Errorf("alta weather: no parking/road alerts found")
	}
	return st, nil
}

// extractJSONObject returns the first balanced JSON object from b.
func extractJSONObject(b []byte) ([]byte, error) {
	if len(b) == 0 || b[0] != '{' {
		return nil, fmt.Errorf("expected JSON object")
	}
	depth := 0
	inString := false
	escape := false
	for i := 0; i < len(b); i++ {
		c := b[i]
		if inString {
			if escape {
				escape = false
				continue
			}
			if c == '\\' {
				escape = true
				continue
			}
			if c == '"' {
				inString = false
			}
			continue
		}
		switch c {
		case '"':
			inString = true
		case '{':
			depth++
		case '}':
			depth--
			if depth == 0 {
				return b[:i+1], nil
			}
		}
	}
	return nil, fmt.Errorf("unbalanced JSON object")
}
