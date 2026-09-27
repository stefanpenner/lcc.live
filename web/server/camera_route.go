package server

import (
	"net/http"
	"strings"

	"github.com/labstack/echo/v4"
	"github.com/stefanpenner/lcc-live/web/metrics"
	"github.com/stefanpenner/lcc-live/web/store"
)

type CameraPageData struct {
	Camera         store.Camera
	CanyonName     string
	CanyonPath     string
	ImageURL       string
	WeatherStation *store.WeatherStation
}

func CameraRoute(s *store.Store) func(c echo.Context) error {
	return func(c echo.Context) error {
		slugOrID := cameraSlugOrID(c)
		entry, ok := s.Get(slugOrID)
		if !ok {
			return c.String(http.StatusNotFound, "Camera not found")
		}
		if entry.Camera == nil {
			return c.String(http.StatusInternalServerError, "Camera data is invalid")
		}
		if dest := cameraSlugRedirect(entry.Camera, slugOrID, wantsCameraJSON(c)); dest != "" {
			return c.Redirect(http.StatusMovedPermanently, dest)
		}

		metrics.PageViewsTotal.WithLabelValues("camera-" + entry.Camera.Canyon).Inc()

		page := cameraPage(s, entry)

		notModified, err := cacheCamera(c, entry.Image)
		if err != nil {
			return err
		}
		if notModified {
			return c.NoContent(http.StatusNotModified)
		}
		if c.Request().Method == http.MethodHead {
			return c.NoContent(http.StatusOK)
		}

		if wantsCameraJSON(c) {
			return c.JSON(http.StatusOK, page)
		}
		return c.Render(http.StatusOK, "camera.html.tmpl", page)
	}
}

func cameraSlugOrID(c echo.Context) string {
	return strings.TrimSuffix(c.Param("*"), ".json")
}

func wantsCameraJSON(c echo.Context) bool {
	return strings.HasSuffix(c.Request().URL.Path, ".json")
}

func cameraSlugRedirect(cam *store.Camera, slugOrID string, isJSON bool) string {
	if cam.Alt == "" {
		return ""
	}
	slug := slugify(cam.Alt)
	if slug == "" || slugOrID == slug || slugOrID != cam.ID {
		return ""
	}
	if isJSON {
		return "/camera/" + slug + ".json"
	}
	return "/camera/" + slug
}

func cameraPage(s *store.Store, entry store.EntrySnapshot) CameraPageData {
	canyonPath := "/"
	if link, ok := canyonLink(entry.Camera.Canyon); ok {
		canyonPath = link.Path
	}
	return CameraPageData{
		Camera:         *entry.Camera,
		CanyonName:     entry.Camera.Canyon,
		CanyonPath:     canyonPath,
		ImageURL:       "/image/" + entry.Camera.ID,
		WeatherStation: s.GetWeatherStation(entry.Camera.ID),
	}
}

func cacheCamera(c echo.Context, image *store.Image) (bool, error) {
	contentType := "text/html; charset=UTF-8"
	if wantsCameraJSON(c) {
		contentType = "application/json; charset=UTF-8"
	}
	c.Response().Header().Set("Content-Type", contentType)

	_, notModified, err := SetCacheHeaders(c, CacheConfig{
		Hash:    []any{image.ETag},
		DevMode: c.Get("_dev_mode") != nil,
	})
	return notModified, err
}
