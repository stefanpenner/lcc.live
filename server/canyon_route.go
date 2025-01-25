package server

import (
	"net/http"

	"github.com/labstack/echo/v4"
	"github.com/stefanpenner/lcc-live/store"
)

func CanyonRoute(store *store.Store, canyonID string) func(c echo.Context) error {
	return func(c echo.Context) error {
		c.Response().Header().Set("Cache-Control", "public, max-age=60")

		canyon := store.Canyon(canyonID)
		c.Response().Header().Set("ETAG", canyon.ETag)
		if c.Request().Method == http.MethodHead {
			return c.NoContent(http.StatusOK)
		}
		return c.Render(http.StatusOK, "canyon.html.tmpl", canyon)
	}
}
