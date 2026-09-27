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
		if _, ok := canyonLink(canyonID); !ok {
			return c.String(http.StatusBadRequest, "Invalid canyon")
		}

		data := loadUDOT(s, canyonID)

		notModified, err := cacheUDOT(c, data)
		if err != nil {
			return err
		}
		if notModified {
			return c.NoContent(http.StatusNotModified)
		}

		c.Response().Header().Set("X-Content-Type-Options", "nosniff")
		return c.JSON(http.StatusOK, data)
	}
}

func loadUDOT(s *store.Store, canyonID string) UDOTData {
	roads := SortRoadConditions(FilterRoadConditions(s.GetRoadConditions(canyonID)))
	events := SortEvents(s.GetEvents(canyonID))
	canyon := s.Canyon(canyonID)
	stations := s.GetWeatherStationsForCanyon(canyon)
	avalanche := s.GetAvalancheDanger()
	var alta *store.AltaStatus
	if canyonID == "LCC" {
		alta = s.GetAltaStatus()
	}
	return UDOTData{
		RoadConditions:  roads,
		WeatherStations: stations,
		Events:          events,
		AvalancheDanger: avalanche,
		AltaStatus:      alta,
		LastUpdated:     latestUDOT(roads, events, avalanche, alta),
	}
}

func latestUDOT(roads []store.RoadCondition, events []store.Event, avalanche *store.AvalancheDanger, alta *store.AltaStatus) int64 {
	latest := time.Now().Unix()
	for _, cond := range roads {
		if cond.LastUpdated > latest {
			latest = cond.LastUpdated
		}
	}
	for _, ev := range events {
		if ev.LastUpdated > latest {
			latest = ev.LastUpdated
		}
	}
	if avalanche != nil && avalanche.Updated > latest {
		latest = avalanche.Updated
	}
	if alta != nil && alta.Updated > latest {
		latest = alta.Updated
	}
	return latest
}

func cacheUDOT(c echo.Context, data UDOTData) (bool, error) {
	c.Response().Header().Set("Content-Type", "application/json; charset=UTF-8")
	_, notModified, err := SetCacheHeaders(c, CacheConfig{
		Hash:    []any{data},
		DevMode: c.Get("_dev_mode") != nil,
	})
	return notModified, err
}
