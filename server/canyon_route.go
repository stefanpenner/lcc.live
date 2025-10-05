package server

import (
	"net/http"
	"strings"

	"github.com/labstack/echo/v4"
	"github.com/stefanpenner/lcc-live/metrics"
	"github.com/stefanpenner/lcc-live/store"
)

func CanyonRoute(store *store.Store, canyonID string) func(c echo.Context) error {
	return func(c echo.Context) error {
		// Track page view
		metrics.PageViewsTotal.WithLabelValues(canyonID).Inc()

		canyon := store.Canyon(canyonID)

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
		etag := canyon.ETag
		if isJSON {
			etag = canyon.ETag + "-json"
		} else {
			etag = canyon.ETag + "-html"
		}

		// Use no-cache to force revalidation while still allowing caching
		// This helps with cache busting - CDN will always check with origin
		// but can serve cached content if ETag matches
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
			return c.JSON(http.StatusOK, canyon)
		}

		return c.Render(http.StatusOK, "canyon.html.tmpl", canyon)
	}
}
