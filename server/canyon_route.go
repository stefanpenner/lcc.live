package server

import (
	"net/http"
	"strings"

	"github.com/labstack/echo/v4"
	"github.com/stefanpenner/lcc-live/metrics"
	"github.com/stefanpenner/lcc-live/store"
)

type CanyonPageData struct {
	*store.Canyon
	RoadConditions []store.RoadCondition
	Events         []store.Event
}

func CanyonRoute(s *store.Store, canyonID string) func(c echo.Context) error {
	return func(c echo.Context) error {
		// Track page view
		metrics.PageViewsTotal.WithLabelValues(canyonID).Inc()

		canyon := s.Canyon(canyonID)
		roadConditions := s.GetRoadConditions(canyonID)
		// Filter out unwanted road conditions
		roadConditions = FilterRoadConditions(roadConditions)
		events := s.GetEvents(canyonID)

		// Determine response format
		isJSON := strings.HasSuffix(c.Request().URL.Path, ".json")

		// Set Content-Type before calling SetCacheHeaders
		if isJSON {
			c.Response().Header().Set("Content-Type", "application/json; charset=UTF-8")
		} else {
			c.Response().Header().Set("Content-Type", "text/html; charset=UTF-8")
		}

		// Check if dev mode is enabled
		devMode := c.Get("_dev_mode") != nil

		// Build cache config - include all components that affect the response
		config := CacheConfig{
			Components: []interface{}{
				canyon,         // Canyon data (cameras, etc.) - uses ETag() method
				roadConditions, // Road conditions - hashed with StableJSONHash
			},
			DevMode: devMode,
		}

		// Set cache headers and check for 304
		_, shouldReturn304, err := SetCacheHeaders(c, config)
		if err != nil {
			return err
		}
		if shouldReturn304 {
			return c.NoContent(http.StatusNotModified)
		}

		if c.Request().Method == http.MethodHead {
			return c.NoContent(http.StatusOK)
		}

		// Return appropriate response format
		if isJSON {
			return c.JSON(http.StatusOK, canyon)
		}

		pageData := CanyonPageData{
			Canyon:         canyon,
			RoadConditions: roadConditions,
			Events:         events,
		}
		return c.Render(http.StatusOK, "canyon.html.tmpl", pageData)
	}
}
