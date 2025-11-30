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
		slugOrID := strings.TrimSuffix(path, ".json")
		isJSON := strings.HasSuffix(c.Request().URL.Path, ".json")

		entry, exists := store.Get(slugOrID)

		if !exists {
			return c.String(http.StatusNotFound, "Camera not found")
		}

		// Check if Camera is nil (defensive programming)
		if entry.Camera == nil {
			return c.String(http.StatusInternalServerError, "Camera data is invalid")
		}

		// If accessed via ID, redirect to slug-based URL for canonical URLs
		// Check if this was accessed via ID (not slug) and redirect to slug if available
		if entry.Camera.Alt != "" {
			expectedSlug := slugify(entry.Camera.Alt)
			// Only redirect if:
			// 1. The path doesn't match the expected slug (i.e., it's an ID or wrong slug)
			// 2. The path matches this camera's ID (confirming it was accessed via ID)
			// 3. The expected slug is not empty
			if expectedSlug != "" && slugOrID != expectedSlug && slugOrID == entry.Camera.ID {
				// Redirect ID-based URLs to slug-based URLs
				redirectPath := "/camera/" + expectedSlug
				if isJSON {
					redirectPath += ".json"
				}
				return c.Redirect(http.StatusMovedPermanently, redirectPath)
			}
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
		// Use the actual camera ID for image URL, not the path parameter (which might be a slug)
		data := CameraPageData{
			Camera:     *entry.Camera,
			CanyonName: canyonName,
			CanyonPath: canyonPath,
			ImageURL:   "/image/" + entry.Camera.ID,
		}

		// Determine response format and set appropriate headers BEFORE caching headers
		// (isJSON already determined above)

		// Set Content-Type early so Cloudflare knows what we're caching
		if isJSON {
			c.Response().Header().Set("Content-Type", "application/json; charset=UTF-8")
		} else {
			c.Response().Header().Set("Content-Type", "text/html; charset=UTF-8")
		}

		// Include version in ETag so deploys automatically bust cache
		// Use different ETags for JSON vs HTML to prevent cache confusion
		version := GetVersionString()
		etag := entry.Image.ETag + "-" + version
		if isJSON {
			etag = etag + "-json"
		} else {
			etag = etag + "-html"
		}

		// Use max-age with stale-while-revalidate for better performance
		// When version changes, ETag changes automatically, so no manual purge needed
		c.Response().Header().Set("Cache-Control", "public, max-age=30, stale-while-revalidate=60, must-revalidate")
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
