package server

import (
	"net/http"
	"strings"

	"github.com/labstack/echo/v4"
	"github.com/stefanpenner/lcc-live/web/metrics"
	"github.com/stefanpenner/lcc-live/web/store"
)

type CanyonPageData struct {
	*store.Canyon
	Title           string
	Path            string
	Nav             []CanyonLink
	RoadConditions  []store.RoadCondition
	Events          []store.Event
	WeatherStations map[string]*store.WeatherStation
	AvalancheDanger *store.AvalancheDanger
	AltaStatus      *store.AltaStatus
	Dev             bool
}

func CanyonRoute(s *store.Store, canyonID string) func(c echo.Context) error {
	return func(c echo.Context) error {
		metrics.PageViewsTotal.WithLabelValues(canyonID).Inc()

		page := loadCanyonPage(s, canyonID)

		notModified, err := cacheCanyon(c, page)
		if err != nil {
			return err
		}
		if notModified {
			return c.NoContent(http.StatusNotModified)
		}

		if c.Request().Method == http.MethodHead {
			return c.NoContent(http.StatusOK)
		}

		if wantsCanyonJSON(c) {
			return c.JSON(http.StatusOK, proxiedCanyon(c, page.Canyon))
		}
		page.Dev = c.Get("_dev_mode") != nil
		return c.Render(http.StatusOK, "canyon.html.tmpl", page)
	}
}

func loadCanyonPage(s *store.Store, canyonID string) CanyonPageData {
	canyon := s.Canyon(canyonID)
	roadConditions := FilterRoadConditions(s.GetRoadConditions(canyonID))
	events := SortEvents(s.GetEvents(canyonID))
	weatherStations := s.GetWeatherStationsForCanyon(canyon)
	avalancheDanger := s.GetAvalancheDanger()
	var altaStatus *store.AltaStatus
	if canyonID == "LCC" {
		altaStatus = s.GetAltaStatus()
	}
	link, _ := canyonLink(canyonID)
	return CanyonPageData{
		Canyon:          canyon,
		Title:           link.Title,
		Path:            link.Path,
		Nav:             canyonCatalog(),
		RoadConditions:  roadConditions,
		Events:          events,
		WeatherStations: weatherStations,
		AvalancheDanger: avalancheDanger,
		AltaStatus:      altaStatus,
	}
}

func cacheCanyon(c echo.Context, page CanyonPageData) (bool, error) {
	contentType := "text/html; charset=UTF-8"
	if wantsCanyonJSON(c) {
		contentType = "application/json; charset=UTF-8"
	}
	c.Response().Header().Set("Content-Type", contentType)

	_, notModified, err := SetCacheHeaders(c, CacheConfig{
		ETag: page.ETag,
		Hash: []any{
			page.RoadConditions,
			page.WeatherStations,
			page.Events,
			page.AvalancheDanger,
			page.AltaStatus,
		},
		DevMode: c.Get("_dev_mode") != nil,
	})
	return notModified, err
}

func wantsCanyonJSON(c echo.Context) bool {
	return strings.HasSuffix(c.Request().URL.Path, ".json")
}

// Image cameras go through /image so clients never hit UDOT, which blocks non-US IPs.
func proxiedCanyon(c echo.Context, canyon *store.Canyon) *store.Canyon {
	proxied := *canyon
	proxied.Cameras = make([]store.Camera, len(canyon.Cameras))
	for i, cam := range canyon.Cameras {
		if cam.Kind == "img" {
			cam.Src = c.Scheme() + "://" + c.Request().Host + "/image/" + cam.ID
		}
		proxied.Cameras[i] = cam
	}
	return &proxied
}
