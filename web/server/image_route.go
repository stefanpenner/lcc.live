package server

import (
	"fmt"
	"net/http"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/stefanpenner/lcc-live/web/metrics"
	"github.com/stefanpenner/lcc-live/web/store"
)

func ImageRoute(store *store.Store) func(c echo.Context) error {
	return func(c echo.Context) error {
		id := c.Param("id")
		entry, exists := store.Get(id)

		status := http.StatusNotFound

		if exists {
			// Track image view
			cameraName := entry.Camera.Alt
			if cameraName == "" {
				cameraName = entry.Camera.ID
			}
			metrics.ImageViewsTotal.WithLabelValues(cameraName, entry.Camera.Canyon).Inc()
			if entry.HTTPHeaders.Status == http.StatusOK {
				headers := entry.HTTPHeaders

				c.Response().Header().Set("Content-Type", headers.ContentType)
				// max-age=0: every request is "stale" so CF always revalidates
				// in the background, keeping images maximally fresh.
				// stale-while-revalidate=120: CF still serves instantly from
				// edge cache during spikes — origin sees at most 1 req/POP.
				c.Response().Header().Set("Cache-Control", "public, max-age=0, stale-while-revalidate=120")
				c.Response().Header().Set("ETag", entry.Image.ETag)
				c.Response().Header().Set("Content-Length", fmt.Sprintf("%d", headers.ContentLength))
				if !entry.FetchedAt.IsZero() {
					c.Response().Header().Set("Last-Modified", entry.FetchedAt.UTC().Format(time.RFC1123))
				}

				if ifNoneMatch := c.Request().Header.Get("If-None-Match"); ifNoneMatch != "" {
					if ifNoneMatch == entry.Image.ETag {
						// Track cache hit
						metrics.CacheHits.WithLabelValues(c.Path()).Inc()
						return c.NoContent(http.StatusNotModified)
					}
				}
				if c.Request().Method == http.MethodHead {
					return c.NoContent(http.StatusOK)
				} else {
					// Track response size
					metrics.ResponseSizeBytes.WithLabelValues(c.Path()).Observe(float64(len(entry.Image.Bytes)))
					return c.Blob(http.StatusOK, headers.ContentType, entry.Image.Bytes)
				}
			}
			status = entry.HTTPHeaders.Status
		}

		// Ensure we have a valid HTTP status code
		if status == 0 {
			status = http.StatusNotFound
		}
		return c.String(status, "image not found")
	}
}
