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
		// Get the wildcard parameter (everything after /camera/)
		path := c.Param("*")
		// Remove .json suffix if present
		id := strings.TrimSuffix(path, ".json")

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

		// Determine response format and set appropriate headers BEFORE caching headers
		isJSON := strings.HasSuffix(c.Request().URL.Path, ".json")

		// Set Content-Type early so Cloudflare knows what we're caching
		if isJSON {
			c.Response().Header().Set("Content-Type", "application/json; charset=UTF-8")
		} else {
			c.Response().Header().Set("Content-Type", "text/html; charset=UTF-8")
		}

		// Use different ETags for JSON vs HTML to prevent cache confusion
		// This ensures Cloudflare caches them separately
		etag := entry.Image.ETag
		if isJSON {
			etag = entry.Image.ETag + "-json"
		} else {
			etag = entry.Image.ETag + "-html"
		}

		// Set cache headers similar to canyon pages
		c.Response().Header().Set("Cache-Control", "public, no-cache, must-revalidate")
		c.Response().Header().Set("ETag", etag)

		// Add Vary header to ensure Cloudflare caches by Content-Type
		c.Response().Header().Set("Vary", "Accept")

		// Check if client has matching ETag
		if ifNoneMatch := c.Request().Header.Get("If-None-Match"); ifNoneMatch != "" {
			if ifNoneMatch == etag {
				return c.NoContent(http.StatusNotModified)
			}
		}

		if c.Request().Method == http.MethodHead {
			return c.NoContent(http.StatusOK)
		}

		// Return appropriate response format
		if isJSON {
			return c.JSON(http.StatusOK, data)
		}

		return c.Render(http.StatusOK, "camera.html.tmpl", data)
	}
}
