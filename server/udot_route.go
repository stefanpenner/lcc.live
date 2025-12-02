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

		// Generate ETag based on content hash using helper
		etag, err := StableJSONHash(data)
		if err != nil {
			return c.String(http.StatusInternalServerError, "Failed to generate ETag")
		}

		// Set appropriate headers for polling endpoint
		c.Response().Header().Set("Content-Type", "application/json; charset=UTF-8")
		c.Response().Header().Set("Cache-Control", "public, max-age=55")
		c.Response().Header().Set("ETag", etag)
		c.Response().Header().Set("X-Content-Type-Options", "nosniff")

		// Check if client has matching ETag
		if ifNoneMatch := c.Request().Header.Get("If-None-Match"); ifNoneMatch != "" {
			if ifNoneMatch == etag {
				return c.NoContent(http.StatusNotModified)
			}
		}

		return c.JSON(http.StatusOK, data)
	}
}
