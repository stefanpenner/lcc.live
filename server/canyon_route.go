package server

import (
	"net/http"
	"strings"

	"github.com/labstack/echo/v4"
	"github.com/stefanpenner/lcc-live/store"
)

func CanyonRoute(store *store.Store, canyonID string) func(c echo.Context) error {
	return func(c echo.Context) error {
		// Use no-cache to force revalidation while still allowing caching
		// This helps with cache busting - CDN will always check with origin
		// but can serve cached content if ETag matches
		c.Response().Header().Set("Cache-Control", "public, no-cache, must-revalidate")

		canyon := store.Canyon(canyonID)
		c.Response().Header().Set("ETAG", canyon.ETag)

		// Check if client has matching ETag
		if ifNoneMatch := c.Request().Header.Get("If-None-Match"); ifNoneMatch != "" {
			if ifNoneMatch == canyon.ETag {
				return c.NoContent(http.StatusNotModified)
			}
		}

		if c.Request().Method == http.MethodHead {
			return c.NoContent(http.StatusOK)
		}

		// Check Accept header to determine response format
		accept := c.Request().Header.Get("Accept")
		if strings.Contains(accept, "application/json") {
			return c.JSON(http.StatusOK, canyon)
		}

		return c.Render(http.StatusOK, "canyon.html.tmpl", canyon)
	}
}
