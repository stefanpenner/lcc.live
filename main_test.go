package main

import (
	"io/fs"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/stefanpenner/lcc-live/server"
	"github.com/stefanpenner/lcc-live/store"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoadConfig(t *testing.T) {
	tests := []struct {
		name             string
		envPort          string
		envSyncInterval  string
		expectedPort     string
		expectedInterval time.Duration
	}{
		{
			name:             "defaults when no env vars",
			envPort:          "",
			envSyncInterval:  "",
			expectedPort:     "3000",
			expectedInterval: defaultSyncInterval,
		},
		{
			name:             "custom port",
			envPort:          "8080",
			envSyncInterval:  "",
			expectedPort:     "8080",
			expectedInterval: defaultSyncInterval,
		},
		{
			name:             "custom sync interval",
			envPort:          "",
			envSyncInterval:  "10s",
			expectedPort:     "3000",
			expectedInterval: 10 * time.Second,
		},
		{
			name:             "invalid sync interval falls back to default",
			envPort:          "",
			envSyncInterval:  "invalid",
			expectedPort:     "3000",
			expectedInterval: defaultSyncInterval,
		},
		{
			name:             "zero sync interval",
			envPort:          "",
			envSyncInterval:  "0",
			expectedPort:     "3000",
			expectedInterval: 0,
		},
		{
			name:             "both custom values",
			envPort:          "4000",
			envSyncInterval:  "5s",
			expectedPort:     "4000",
			expectedInterval: 5 * time.Second,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Clear and set environment
			os.Unsetenv("PORT")
			os.Unsetenv("SYNC_INTERVAL")
			if tt.envPort != "" {
				os.Setenv("PORT", tt.envPort)
				defer os.Unsetenv("PORT")
			}
			if tt.envSyncInterval != "" {
				os.Setenv("SYNC_INTERVAL", tt.envSyncInterval)
				defer os.Unsetenv("SYNC_INTERVAL")
			}

			config := loadConfig()

			assert.Equal(t, tt.expectedPort, config.Port)
			assert.Equal(t, tt.expectedInterval, config.SyncInterval)
		})
	}

	// Test consistency across multiple calls
	t.Run("multiple calls return same values", func(t *testing.T) {
		os.Setenv("PORT", "4000")
		os.Setenv("SYNC_INTERVAL", "5s")
		defer func() {
			os.Unsetenv("PORT")
			os.Unsetenv("SYNC_INTERVAL")
		}()

		config1 := loadConfig()
		config2 := loadConfig()

		assert.Equal(t, config1.Port, config2.Port)
		assert.Equal(t, config1.SyncInterval, config2.SyncInterval)
	})
}

func TestDefaultSyncInterval(t *testing.T) {
	assert.Equal(t, 3*time.Second, defaultSyncInterval)
}

func TestConfig_Structure(t *testing.T) {
	config := Config{
		Port:         "3000",
		SyncInterval: 5 * time.Second,
	}

	assert.Equal(t, "3000", config.Port)
	assert.Equal(t, 5*time.Second, config.SyncInterval)
}

// Test embedded FS compilation (compile-time check)
func TestEmbeddedFS_Exists(t *testing.T) {
	// These should compile - if embed directives are wrong, compilation fails
	assert.NotNil(t, dataFS)
	assert.NotNil(t, staticFS)
	assert.NotNil(t, tmplFS)
}

func TestEmbeddedFS_DataFile(t *testing.T) {
	// Verify data.json is embedded
	data, err := dataFS.ReadFile("data.json")
	require.NoError(t, err)
	assert.NotEmpty(t, data)
	assert.Contains(t, string(data), "lcc")
	assert.Contains(t, string(data), "bcc")
}

func TestEmbeddedFS_StaticFiles(t *testing.T) {
	// Verify static files are embedded
	files := []string{
		"static/script.mjs",
		"static/style.css",
		"static/favicon.png",
	}

	for _, file := range files {
		data, err := staticFS.ReadFile(file)
		require.NoError(t, err, "File %s should be embedded", file)
		assert.NotEmpty(t, data, "File %s should not be empty", file)
	}
}

func TestEmbeddedFS_Templates(t *testing.T) {
	// Verify template is embedded
	data, err := tmplFS.ReadFile("templates/canyon.html.tmpl")
	require.NoError(t, err)
	assert.NotEmpty(t, data)
	assert.Contains(t, string(data), "<!DOCTYPE")
}

// Benchmark config loading
func BenchmarkLoadConfig(b *testing.B) {
	os.Setenv("PORT", "3000")
	os.Setenv("SYNC_INTERVAL", "5s")
	defer func() {
		os.Unsetenv("PORT")
		os.Unsetenv("SYNC_INTERVAL")
	}()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = loadConfig()
	}
}

// Integration test that validates the full application startup sequence
func TestApplicationStartup(t *testing.T) {
	// Test that embedded FS files load correctly
	t.Run("Embedded FS loads", func(t *testing.T) {
		assert.NotNil(t, dataFS)
		assert.NotNil(t, staticFS)
		assert.NotNil(t, tmplFS)

		// Verify data.json is present
		data, err := dataFS.ReadFile("data.json")
		require.NoError(t, err)
		assert.NotEmpty(t, data)
	})

	// Test that store initializes from data.json
	t.Run("Store initializes from data.json", func(t *testing.T) {
		testStore, err := store.NewStoreFromFile(dataFS, "data.json")
		require.NoError(t, err)
		assert.NotNil(t, testStore)

		// Verify canyons are loaded
		lcc := testStore.Canyon("LCC")
		bcc := testStore.Canyon("BCC")
		assert.NotEmpty(t, lcc.Name)
		assert.NotEmpty(t, bcc.Name)
	})

	// Test that server starts without errors
	t.Run("Server starts successfully", func(t *testing.T) {
		// Setup filesystem
		static, err := fs.Sub(staticFS, "static")
		require.NoError(t, err)

		tmpl, err := fs.Sub(tmplFS, "templates")
		require.NoError(t, err)

		// Create store
		testStore, err := store.NewStoreFromFile(dataFS, "data.json")
		require.NoError(t, err)

		// Start server
		app, err := server.Start(testStore, static, tmpl)
		require.NoError(t, err)
		assert.NotNil(t, app)
	})

	// Integration test: full startup and basic route
	t.Run("Full startup and basic route works", func(t *testing.T) {
		// Setup filesystem
		static, err := fs.Sub(staticFS, "static")
		require.NoError(t, err)

		tmpl, err := fs.Sub(tmplFS, "templates")
		require.NoError(t, err)

		// Create and initialize store
		testStore, err := store.NewStoreFromFile(dataFS, "data.json")
		require.NoError(t, err)

		// Start server
		app, err := server.Start(testStore, static, tmpl)
		require.NoError(t, err)

		// Test that routes are accessible (even if images not yet fetched)
		// The healthcheck should return 503 if images aren't ready
		req := httptest.NewRequest("GET", "/healthcheck", nil)
		rec := httptest.NewRecorder()
		app.ServeHTTP(rec, req)

		// Should be 503 since we haven't fetched images yet
		assert.Equal(t, http.StatusServiceUnavailable, rec.Code)
	})
}
