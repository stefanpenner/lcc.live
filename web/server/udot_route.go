package server

import (
	"net/http"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/stefanpenner/lcc-live/web/store"
)

type UDOTData struct {
	RoadConditions  []store.RoadCondition            `json:"roadConditions"`
	WeatherStations map[string]*store.WeatherStation `json:"weatherStations,omitempty"`
	Events          []store.Event                    `json:"events"`
	AvalancheDanger *store.AvalancheDanger           `json:"avalancheDanger,omitempty"`
	AltaStatus      *store.AltaStatus                `json:"altaStatus,omitempty"`
	LastUpdated     int64                            `json:"lastUpdated"`
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

		// Events for this canyon
		sortedEvents := SortEvents(s.GetEvents(canyonID))

		// Get weather stations for all cameras in this canyon
		canyon := s.Canyon(canyonID)
		weatherStations := s.GetWeatherStationsForCanyon(canyon)

		// Global UAC danger (both canyons)
		avalancheDanger := s.GetAvalancheDanger()

		// Alta parking only meaningful for LCC; omit for BCC
		var altaStatus *store.AltaStatus
		if canyonID == "LCC" {
			altaStatus = s.GetAltaStatus()
		}

		// Calculate LastUpdated as max of all timestamps, or current time if no data
		lastUpdated := time.Now().Unix()
		for _, cond := range sortedRoadConditions {
			if cond.LastUpdated > lastUpdated {
				lastUpdated = cond.LastUpdated
			}
		}
		for _, ev := range sortedEvents {
			if ev.LastUpdated > lastUpdated {
				lastUpdated = ev.LastUpdated
			}
		}
		if avalancheDanger != nil && avalancheDanger.Updated > lastUpdated {
			lastUpdated = avalancheDanger.Updated
		}
		if altaStatus != nil && altaStatus.Updated > lastUpdated {
			lastUpdated = altaStatus.Updated
		}

		data := UDOTData{
			RoadConditions:  sortedRoadConditions,
			WeatherStations: weatherStations,
			Events:          sortedEvents,
			AvalancheDanger: avalancheDanger,
			AltaStatus:      altaStatus,
			LastUpdated:     lastUpdated,
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
