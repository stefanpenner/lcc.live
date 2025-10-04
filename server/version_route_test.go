package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestVersionRoute(t *testing.T) {
	srv, _ := setupTestServer(t)

	req := httptest.NewRequest("GET", "/_/version", nil)
	rec := httptest.NewRecorder()

	srv.Handler.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)

	var info VersionInfo
	err := json.Unmarshal(rec.Body.Bytes(), &info)
	require.NoError(t, err)

	// Check that version info is populated
	assert.NotEmpty(t, info.Version)
	assert.NotEmpty(t, info.GoVersion)
	assert.NotEmpty(t, info.BuildTime)
	assert.NotEmpty(t, info.Uptime)
}

func TestVersionHeader(t *testing.T) {
	srv, _ := setupTestServer(t)

	tests := []struct {
		name string
		path string
	}{
		{"index", "/"},
		{"bcc", "/bcc"},
		{"healthcheck", "/healthcheck"},
		{"version", "/_/version"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", tt.path, nil)
			rec := httptest.NewRecorder()

			srv.Handler.ServeHTTP(rec, req)

			// All responses should have X-Version header
			assert.NotEmpty(t, rec.Header().Get("X-Version"))
		})
	}
}

func TestGetVersionString(t *testing.T) {
	// Save original values
	origVersion := Version
	origGoVersion := GoVersion
	defer func() {
		Version = origVersion
		GoVersion = origGoVersion
	}()

	// Test dev version
	Version = "dev"
	GoVersion = "go1.23.3"
	result := GetVersionString()
	assert.Contains(t, result, "dev")
	assert.Contains(t, result, GoVersion)

	// Test with actual version
	Version = "abc1234"
	result = GetVersionString()
	assert.Equal(t, "abc1234", result)
}

func TestGetVersionInfo(t *testing.T) {
	info := GetVersionInfo()

	assert.NotEmpty(t, info.Version)
	assert.NotEmpty(t, info.GoVersion)
	assert.NotEmpty(t, info.BuildTime)
	assert.NotEmpty(t, info.Uptime)
}
