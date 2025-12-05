package server

import (
	"errors"
	"reflect"
	"strings"

	"github.com/labstack/echo/v4"
)

// ETagger is an interface for types that have their own ETag
type ETagger interface {
	ETag() string
}

// CacheConfig holds configuration for cache headers and ETag generation
type CacheConfig struct {
	// Components are all the data components to include in the ETag
	// Components can be:
	// - Objects implementing ETagger interface - will call ETag() method
	// - Other objects - will be hashed using StableJSONHash
	Components []interface{}

	// DevMode disables caching when true
	DevMode bool
}

// SetCacheHeaders sets consistent cache headers and ETag based on the config
// Returns the generated ETag and whether the request should return 304 Not Modified
// Returns an error if Content-Type is not already set
func SetCacheHeaders(c echo.Context, config CacheConfig) (string, bool, error) {
	// Ensure Content-Type is already set
	if c.Response().Header().Get("Content-Type") == "" {
		return "", false, errors.New("Content-Type must be set before calling SetCacheHeaders")
	}

	// Determine format from request path
	isJSON := strings.HasSuffix(c.Request().URL.Path, ".json")
	formatSuffix := "html"
	if isJSON {
		formatSuffix = "json"
	}

	// Build composite ETag from all components
	etag := buildCompositeETag(config, formatSuffix)

	// In dev mode, disable caching completely
	if config.DevMode {
		c.Response().Header().Set("Cache-Control", "no-cache, no-store, must-revalidate, private")
		c.Response().Header().Set("Pragma", "no-cache")
		c.Response().Header().Set("Expires", "0")
		c.Response().Header().Set("Vary", "*")
		return etag, false, nil
	}

	// Set standard cache headers
	c.Response().Header().Set("Cache-Control", "public, max-age=30, stale-while-revalidate=60, must-revalidate")
	c.Response().Header().Set("ETag", etag)
	c.Response().Header().Set("Vary", "Accept")

	// Check if client has matching ETag
	if ifNoneMatch := c.Request().Header.Get("If-None-Match"); ifNoneMatch != "" {
		if ifNoneMatch == etag {
			return etag, true, nil // Return 304 Not Modified
		}
	}

	return etag, false, nil
}

// buildCompositeETag builds a composite ETag from version + all components
func buildCompositeETag(config CacheConfig, formatSuffix string) string {
	version := GetVersionString()

	// Start with version
	parts := []string{version}

	// Add hash/ETag of each component
	for _, component := range config.Components {
		if component == nil {
			continue
		}

		var hashValue string

		// Check if component implements ETagger interface
		if etagger, ok := component.(ETagger); ok {
			hashValue = strings.Trim(etagger.ETag(), "\"")
		} else if etag := getETagFromStruct(component); etag != "" {
			// Check if component has an ETag field (like store.Canyon)
			hashValue = strings.Trim(etag, "\"")
		} else {
			// Fall back to StableJSONHash
			hash, err := StableJSONHash(component)
			if err == nil {
				hashValue = strings.Trim(hash, "\"")
			} else {
				continue // Skip component if hashing fails
			}
		}

		if hashValue != "" {
			parts = append(parts, hashValue)
		}
	}

	// Add format suffix if specified
	if formatSuffix != "" {
		parts = append(parts, formatSuffix)
	}

	// Join all parts with hyphens
	return "\"" + strings.Join(parts, "-") + "\""
}

// getETagFromStruct extracts ETag field from a struct using reflection
func getETagFromStruct(component interface{}) string {
	v := reflect.ValueOf(component)
	if v.Kind() == reflect.Ptr {
		v = v.Elem()
	}
	if v.Kind() != reflect.Struct {
		return ""
	}

	etagField := v.FieldByName("ETag")
	if etagField.IsValid() && etagField.Kind() == reflect.String {
		return etagField.String()
	}

	return ""
}

