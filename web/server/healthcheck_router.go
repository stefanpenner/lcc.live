package server

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"

	"github.com/labstack/echo/v4"
	"github.com/stefanpenner/lcc-live/web/store"
)

// HealthCheckRoute is 503 until the store is live and both canyon pages render.
func HealthCheckRoute(live *store.Store) func(c echo.Context) error {
	return func(c echo.Context) error {
		if msg := unavailable(live); msg != "" {
			return c.String(http.StatusServiceUnavailable, msg)
		}

		if err := proveCanyonHTML(c.Echo()); err != nil {
			return c.String(http.StatusServiceUnavailable, err.Error())
		}

		return c.String(http.StatusOK, "OK")
	}
}

func unavailable(live *store.Store) string {
	if !live.IsReady() {
		return "Service starting up - images not ready yet"
	}

	lcc := live.Canyon("LCC")
	bcc := live.Canyon("BCC")
	if len(lcc.Cameras) == 0 && len(bcc.Cameras) == 0 {
		return "No cameras configured"
	}

	if !live.HasAnyLiveImage() {
		return "No live camera images"
	}
	return ""
}

func proveCanyonHTML(e *echo.Echo) error {
	if err := proveHTML(e, "/", "Little Cottonwood Canyon"); err != nil {
		return fmt.Errorf("Healthcheck failed - LCC route error: %v", err)
	}
	if err := proveHTML(e, "/bcc", "Big Cottonwood Canyon"); err != nil {
		return fmt.Errorf("Healthcheck failed - BCC route error: %v", err)
	}
	return nil
}

func proveHTML(e *echo.Echo, path, needle string) error {
	req := httptest.NewRequest(http.MethodGet, path, nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		return fmt.Errorf("returned status %d instead of 200", rec.Code)
	}

	body := rec.Body.String()
	if !strings.Contains(body, "<!DOCTYPE") {
		return fmt.Errorf("response is not valid HTML (missing DOCTYPE)")
	}
	if !strings.Contains(body, needle) {
		return fmt.Errorf("response missing expected content '%s'", needle)
	}
	return nil
}
