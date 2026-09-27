package server

import (
	"fmt"
	"net/http"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/stefanpenner/lcc-live/web/metrics"
	"github.com/stefanpenner/lcc-live/web/store"
)

func ImageRoute(s *store.Store) func(c echo.Context) error {
	return func(c echo.Context) error {
		entry, ok := s.Get(c.Param("id"))
		if !ok {
			return imageNotFound(c, http.StatusNotFound)
		}

		metrics.ImageViewsTotal.WithLabelValues(imageViewName(entry.Camera), entry.Camera.Canyon).Inc()
		if entry.HTTPHeaders.Status != http.StatusOK {
			return imageNotFound(c, entry.HTTPHeaders.Status)
		}

		setImageHeaders(c, entry)
		if imageNotModified(c, entry.Image.ETag) {
			metrics.CacheHits.WithLabelValues(c.Path()).Inc()
			return c.NoContent(http.StatusNotModified)
		}
		if c.Request().Method == http.MethodHead {
			return c.NoContent(http.StatusOK)
		}

		metrics.ResponseSizeBytes.WithLabelValues(c.Path()).Observe(float64(len(entry.Image.Bytes)))
		return c.Blob(http.StatusOK, entry.HTTPHeaders.ContentType, entry.Image.Bytes)
	}
}

func imageViewName(cam *store.Camera) string {
	if cam.Alt != "" {
		return cam.Alt
	}
	return cam.ID
}

func imageNotFound(c echo.Context, status int) error {
	if status == 0 {
		status = http.StatusNotFound
	}
	return c.String(status, "image not found")
}

func setImageHeaders(c echo.Context, entry store.EntrySnapshot) {
	h := c.Response().Header()
	h.Set("Content-Type", entry.HTTPHeaders.ContentType)
	// See web/docs/caching.md for analysis of max-age tradeoffs.
	h.Set("Cache-Control", "public, max-age=3, stale-while-revalidate=120")
	h.Set("ETag", entry.Image.ETag)
	// Body length is SSOT (store also sets HTTPHeaders.ContentLength = len(bytes))
	h.Set("Content-Length", fmt.Sprintf("%d", len(entry.Image.Bytes)))
	if !entry.FetchedAt.IsZero() {
		h.Set("Last-Modified", entry.FetchedAt.UTC().Format(time.RFC1123))
	}
}

func imageNotModified(c echo.Context, etag string) bool {
	match := c.Request().Header.Get("If-None-Match")
	return match != "" && match == etag
}
