package server

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"

	"github.com/labstack/echo/v4"
	"github.com/stefanpenner/lcc-live/store"
)

func HealthCheckRoute(store *store.Store) func(c echo.Context) error {
	return func(c echo.Context) error {
		// Verify that the store is initialized and has completed
		// its initial image fetch before declaring the service healthy
		if !store.IsReady() {
			return c.String(http.StatusServiceUnavailable, "Service starting up - images not ready yet")
		}

		// Verify store has cameras loaded (basic sanity check)
		lcc := store.Canyon("LCC")
		bcc := store.Canyon("BCC")
		
		if len(lcc.Cameras) == 0 && len(bcc.Cameras) == 0 {
			return c.String(http.StatusServiceUnavailable, "No cameras configured")
		}

		// Smoke test: verify that LCC and BCC routes can render HTML
		// This catches template errors, data issues, and rendering pipeline problems
		e := c.Echo()
		
		// Test LCC route
		if err := testRoute(e, "/", "Little Cottonwood Canyon"); err != nil {
			return c.String(http.StatusServiceUnavailable, 
				fmt.Sprintf("Healthcheck failed - LCC route error: %v", err))
		}
		
		// Test BCC route
		if err := testRoute(e, "/bcc", "Big Cottonwood Canyon"); err != nil {
			return c.String(http.StatusServiceUnavailable, 
				fmt.Sprintf("Healthcheck failed - BCC route error: %v", err))
		}

		return c.String(http.StatusOK, "OK")
	}
}

// testRoute performs an internal HTTP request to verify a route can render successfully
func testRoute(e *echo.Echo, path string, expectedContent string) error {
	req := httptest.NewRequest(http.MethodGet, path, nil)
	rec := httptest.NewRecorder()
	
	e.ServeHTTP(rec, req)
	
	if rec.Code != http.StatusOK {
		return fmt.Errorf("returned status %d instead of 200", rec.Code)
	}
	
	body := rec.Body.String()
	
	// Verify it's HTML
	if !strings.Contains(body, "<!DOCTYPE") {
		return fmt.Errorf("response is not valid HTML (missing DOCTYPE)")
	}
	
	// Verify expected content is present
	if !strings.Contains(body, expectedContent) {
		return fmt.Errorf("response missing expected content '%s'", expectedContent)
	}
	
	return nil
}
