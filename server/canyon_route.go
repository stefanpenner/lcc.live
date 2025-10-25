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

		// Check if dev mode is enabled via environment variable
		devMode := c.Get("_dev_mode") != nil

		// Include version in ETag so deploys automatically bust cache
		// Use different ETags for JSON vs HTML to prevent cache confusion
		version := GetVersionString()
		etag := canyon.ETag + "-" + version
		if isJSON {
			etag = etag + "-json"
		} else {
			etag = etag + "-html"
		}

		// In dev mode, disable caching completely
		if devMode {
			c.Response().Header().Set("Cache-Control", "no-cache, no-store, must-revalidate, private")
			c.Response().Header().Set("Pragma", "no-cache")
			c.Response().Header().Set("Expires", "0")
			c.Response().Header().Set("Vary", "*")
			// Don't set ETag in dev mode to prevent conditional requests
		} else {
			// Longer max-age with stale-while-revalidate for better performance
			// Cloudflare will serve from cache for 30s, then revalidate in background
			// When version changes, ETag changes automatically, so no manual purge needed
			c.Response().Header().Set("Cache-Control", "public, max-age=30, stale-while-revalidate=60, must-revalidate")
			c.Response().Header().Set("ETag", etag)
			// Add Vary header to ensure Cloudflare caches by Content-Type
			c.Response().Header().Set("Vary", "Accept")
		}

		// Check if client has matching ETag (skip in dev mode)
		if !devMode {
			if ifNoneMatch := c.Request().Header.Get("If-None-Match"); ifNoneMatch != "" {
				if ifNoneMatch == etag {
					return c.NoContent(http.StatusNotModified)
				}
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
