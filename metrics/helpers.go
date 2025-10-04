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
