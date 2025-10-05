package server

import (
	"net/http"
	"strings"

	"github.com/labstack/echo/v4"
	"github.com/stefanpenner/lcc-live/metrics"
	"github.com/stefanpenner/lcc-live/store"
)

type CameraPageData struct {
	Camera     store.Camera
	CanyonName string
	CanyonPath string
	ImageURL   string
}

func CameraRoute(store *store.Store) func(c echo.Context) error {
	return func(c echo.Context) error {
		id := c.Param("id")
		entry, exists := store.Get(id)

		if !exists {
			return c.String(http.StatusNotFound, "Camera not found")
		}

		// Check if Camera is nil (defensive programming)
		if entry.Camera == nil {
			return c.String(http.StatusInternalServerError, "Camera data is invalid")
		}

		// Track camera page view
		cameraName := entry.Camera.Alt
		if cameraName == "" {
			cameraName = entry.Camera.ID
		}
		metrics.PageViewsTotal.WithLabelValues("camera-" + entry.Camera.Canyon).Inc()

		// Determine canyon name and path
		canyonName := entry.Camera.Canyon
		canyonPath := "/"
		if strings.ToUpper(canyonName) == "BCC" {
			canyonPath = "/bcc"
		}

		// Build the data for the template
		data := CameraPageData{
			Camera:     *entry.Camera,
			CanyonName: canyonName,
			CanyonPath: canyonPath,
			ImageURL:   "/image/" + id,
		}

		// Set cache headers similar to canyon pages
		c.Response().Header().Set("Cache-Control", "public, no-cache, must-revalidate")
		c.Response().Header().Set("ETAG", entry.Image.ETag)

		// Check if client has matching ETag
		if ifNoneMatch := c.Request().Header.Get("If-None-Match"); ifNoneMatch != "" {
			if ifNoneMatch == entry.Image.ETag {
				return c.NoContent(http.StatusNotModified)
			}
		}

		if c.Request().Method == http.MethodHead {
			return c.NoContent(http.StatusOK)
		}

		// Check if request path ends with .json to determine response format
		if strings.HasSuffix(c.Request().URL.Path, ".json") {
			return c.JSON(http.StatusOK, data)
		}

		return c.Render(http.StatusOK, "camera.html.tmpl", data)
	}
}
