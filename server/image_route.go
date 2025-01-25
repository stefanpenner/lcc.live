package server

import (
	"fmt"
	"net/http"

	"github.com/labstack/echo/v4"
	"github.com/stefanpenner/lcc-live/store"
)

func ImageRoute(store *store.Store) func(c echo.Context) error {
	return func(c echo.Context) error {
		id := c.Param("id")
		entry, exists := store.Get(id)

		status := http.StatusNotFound

		if exists {
			if entry.HTTPHeaders.Status == http.StatusOK {
				headers := entry.HTTPHeaders

				c.Response().Header().Set("Content-Type", headers.ContentType)
				c.Response().Header().Set("Cache-Control", "Public, max-age=5")
				c.Response().Header().Set("ETag", entry.Image.ETag)
				c.Response().Header().Set("Content-Length", fmt.Sprintf("%d", headers.ContentLength))

				if ifNoneMatch := c.Request().Header.Get("If-None-Match"); ifNoneMatch != "" {
					if ifNoneMatch == entry.Image.ETag {
						return c.NoContent(http.StatusNotModified)
					}
				}
				if c.Request().Method == http.MethodHead {
					c.NoContent(http.StatusOK)
				} else {
					return c.Blob(http.StatusOK, headers.ContentType, entry.Image.Bytes)
				}
			}
			status = entry.HTTPHeaders.Status
		}

		return c.String(status, "image not found")
	}
}
