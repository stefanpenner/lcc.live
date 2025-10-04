package server

import (
	"strconv"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/stefanpenner/lcc-live/metrics"
)

// MetricsMiddleware records HTTP request metrics for Prometheus
func MetricsMiddleware() echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			// Track in-flight requests
			metrics.HTTPRequestsInFlight.Inc()
			defer metrics.HTTPRequestsInFlight.Dec()

			// Record start time
			start := time.Now()

			// Process request
			err := next(c)

			// Record duration and counts
			duration := time.Since(start).Seconds()
			status := c.Response().Status
			method := c.Request().Method
			path := c.Path()

			// Normalize paths to avoid high cardinality
			// Replace :id params with the placeholder
			if path == "" {
				path = c.Request().URL.Path
			}

			statusStr := strconv.Itoa(status)

			// Record metrics
			metrics.HTTPRequestDuration.WithLabelValues(method, path, statusStr).Observe(duration)
			metrics.HTTPRequestsTotal.WithLabelValues(method, path, statusStr).Inc()

			return err
		}
	}
}

