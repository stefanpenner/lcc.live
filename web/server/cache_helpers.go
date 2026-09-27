package server

import (
	"errors"
	"strings"

	"github.com/labstack/echo/v4"
)

// CacheConfig is the cache key for one response.
// ETag is used as-is. Each Hash value is reduced with StableJSONHash.
type CacheConfig struct {
	ETag    string
	Hash    []any
	DevMode bool
}

// SetCacheHeaders writes cache headers and reports whether to return 304.
// Content-Type must already be set.
func SetCacheHeaders(c echo.Context, config CacheConfig) (string, bool, error) {
	if c.Response().Header().Get("Content-Type") == "" {
		return "", false, errors.New("Content-Type must be set before calling SetCacheHeaders")
	}

	format := "html"
	if strings.HasSuffix(c.Request().URL.Path, ".json") {
		format = "json"
	}

	etag := compositeETag(config.ETag, config.Hash, format)

	if config.DevMode {
		h := c.Response().Header()
		h.Set("Cache-Control", "no-cache, no-store, must-revalidate, private")
		h.Set("Pragma", "no-cache")
		h.Set("Expires", "0")
		h.Set("Vary", "*")
		return etag, false, nil
	}

	h := c.Response().Header()
	h.Set("Cache-Control", "public, max-age=30, stale-while-revalidate=120, must-revalidate")
	h.Set("ETag", etag)
	h.Set("Vary", "Accept")

	if match := c.Request().Header.Get("If-None-Match"); match != "" && match == etag {
		return etag, true, nil
	}
	return etag, false, nil
}

func compositeETag(ready string, values []any, format string) string {
	parts := []string{GetVersionString()}
	if token := strings.Trim(ready, `"`); token != "" {
		parts = append(parts, token)
	}
	for _, v := range values {
		if token := hashPart(v); token != "" {
			parts = append(parts, token)
		}
	}
	if format != "" {
		parts = append(parts, format)
	}
	return `"` + strings.Join(parts, "-") + `"`
}

func hashPart(v any) string {
	if v == nil {
		return ""
	}
	hash, err := StableJSONHash(v)
	if err != nil {
		return ""
	}
	return strings.Trim(hash, `"`)
}
