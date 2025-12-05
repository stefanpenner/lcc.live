// Package metrics provides helper functions for Prometheus metrics
package metrics

import (
	"net/url"
	"runtime"
)

// ExtractOrigin extracts the domain from a URL for origin tracking
func ExtractOrigin(urlStr string) string {
	parsed, err := url.Parse(urlStr)
	if err != nil {
		return "unknown"
	}
	return parsed.Host
}

// RecordMemoryUsage updates memory usage metrics
func RecordMemoryUsage() {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	MemoryUsageBytes.Set(float64(m.Alloc))
}

// ErrorRateStats holds error rate statistics
type ErrorRateStats struct {
	TotalRequests float64
	ErrorRequests float64
	ErrorRate     float64 // Percentage
	ErrorsPerSec  float64
}

// CalculateErrorRate calculates error rate from request and error counts
func CalculateErrorRate(totalRequests, errorRequests, lastErrors float64, elapsedSeconds float64) ErrorRateStats {
	// Calculate error rate percentage
	errorRate := 0.0
	if totalRequests > 0 {
		errorRate = (errorRequests / totalRequests) * 100.0
	}

	// Calculate errors per second
	errorsPerSec := 0.0
	if elapsedSeconds > 0 {
		errorsPerSec = (errorRequests - lastErrors) / elapsedSeconds
	}

	return ErrorRateStats{
		TotalRequests: totalRequests,
		ErrorRequests: errorRequests,
		ErrorRate:     errorRate,
		ErrorsPerSec:  errorsPerSec,
	}
}
