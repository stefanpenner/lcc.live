package server

import (
	"net/http"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/stefanpenner/lcc-live/store"
)

type UDOTData struct {
	RoadConditions []store.RoadCondition `json:"roadConditions"`
	LastUpdated    int64                 `json:"lastUpdated"`
}

func UDOTRoute(s *store.Store) func(c echo.Context) error {
	return func(c echo.Context) error {
		canyonID := c.Param("canyon")
		if canyonID != "LCC" && canyonID != "BCC" {
			return c.String(http.StatusBadRequest, "Invalid canyon. Must be LCC or BCC")
		}

		roadConditions := s.GetRoadConditions(canyonID)

		// Filter out unwanted road conditions
		filteredRoadConditions := FilterRoadConditions(roadConditions)

		// Sort road conditions for stable JSON hashing
		sortedRoadConditions := SortRoadConditions(filteredRoadConditions)

		// Calculate LastUpdated as max of all timestamps, or current time if no data
		lastUpdated := time.Now().Unix()
		for _, cond := range sortedRoadConditions {
			if cond.LastUpdated > lastUpdated {
				lastUpdated = cond.LastUpdated
			}
		}

		data := UDOTData{
			RoadConditions: sortedRoadConditions,
			LastUpdated:    lastUpdated,
		}

		// Set Content-Type before calling SetCacheHeaders
		c.Response().Header().Set("Content-Type", "application/json; charset=UTF-8")

		// Check if dev mode is enabled
		devMode := c.Get("_dev_mode") != nil

		// Build cache config - pass the data itself as the component
		config := CacheConfig{
			Components: []interface{}{data},
			DevMode:    devMode,
		}

		// Set cache headers and check for 304
		_, shouldReturn304, err := SetCacheHeaders(c, config)
		if err != nil {
			return err
		}
		if shouldReturn304 {
			return c.NoContent(http.StatusNotModified)
		}

		// Set additional headers specific to API endpoint
		c.Response().Header().Set("X-Content-Type-Options", "nosniff")

		return c.JSON(http.StatusOK, data)
	}
}
