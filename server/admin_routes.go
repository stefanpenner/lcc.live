package server

import (
	"context"
	"net/http"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/stefanpenner/lcc-live/logger"
	"github.com/stefanpenner/lcc-live/neon"
)

// AdminUnavailableRoute returns a static 503 when Neon is not configured.
var AdminUnavailableRoute = func(c echo.Context) error {
	return c.JSON(http.StatusServiceUnavailable, map[string]string{
		"error": "admin dataset is unavailable",
	})
}

// AdminCanyonsRoute responds with canyon metadata sourced from Neon.
func AdminCanyonsRoute(repo *neon.Repository) echo.HandlerFunc {
	if repo == nil {
		return AdminUnavailableRoute
	}

	return func(c echo.Context) error {
		ctx, cancel := context.WithTimeout(c.Request().Context(), 2*time.Second)
		defer cancel()

		data, err := repo.ListCanyons(ctx)
		if err != nil {
			logger.Error("admin neon query failed: %v", err)
			return c.JSON(http.StatusInternalServerError, map[string]string{
				"error": "failed to load admin data",
			})
		}

		return c.JSON(http.StatusOK, map[string]any{
			"data": data,
		})
	}
}

// AdminPageRoute renders the admin SPA shell.
func AdminPageRoute() echo.HandlerFunc {
	return func(c echo.Context) error {
		return c.Render(http.StatusOK, "admin.html.tmpl", nil)
	}
}
