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

// Test filesystem loading from disk
func TestFilesystemLoading(t *testing.T) {
	// Set up dev mode for testing
	os.Setenv("DEV_MODE", "1")
	defer os.Unsetenv("DEV_MODE")

	t.Run("Data file loads", func(t *testing.T) {
		dataFS, err := loadFilesystem(".")
		require.NoError(t, err)
		data, err := fs.ReadFile(dataFS, "seed.json")
		require.NoError(t, err)
		assert.NotEmpty(t, data)
		assert.Contains(t, string(data), "lcc")
		assert.Contains(t, string(data), "bcc")
	})

	t.Run("Static files load", func(t *testing.T) {
		staticFS, err := loadFilesystem("static")
		require.NoError(t, err)

		files := []string{
			"script.mjs",
			"style.css",
			"favicon.png",
		}

		for _, file := range files {
			data, err := fs.ReadFile(staticFS, file)
			require.NoError(t, err, "File %s should load", file)
			assert.NotEmpty(t, data, "File %s should not be empty", file)
		}
	})

	t.Run("Templates load", func(t *testing.T) {
		tmplFS, err := loadFilesystem("templates")
		require.NoError(t, err)
		data, err := fs.ReadFile(tmplFS, "canyon.html.tmpl")
		require.NoError(t, err)
		assert.NotEmpty(t, data)
		assert.Contains(t, string(data), "<!DOCTYPE")
	})
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
	// Set up dev mode for testing
	os.Setenv("DEV_MODE", "1")
	defer os.Unsetenv("DEV_MODE")

	// Test that store initializes from seed.json
	t.Run("Store initializes from seed.json", func(t *testing.T) {
		dataFS, err := loadFilesystem(".")
		require.NoError(t, err)

		testStore, err := store.NewStoreFromFile(dataFS, "seed.json")
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
		staticFS, err := loadFilesystem("static")
		require.NoError(t, err)

		tmplFS, err := loadFilesystem("templates")
		require.NoError(t, err)

		dataFS, err := loadFilesystem(".")
		require.NoError(t, err)

		// Create store
		testStore, err := store.NewStoreFromFile(dataFS, "seed.json")
		require.NoError(t, err)

		// Start server
		app, err := server.Start(testStore, staticFS, tmplFS, false, nil)
		require.NoError(t, err)
		assert.NotNil(t, app)
	})

	// Integration test: full startup and basic route
	t.Run("Full startup and basic route works", func(t *testing.T) {
		// Setup filesystem
		staticFS, err := loadFilesystem("static")
		require.NoError(t, err)

		tmplFS, err := loadFilesystem("templates")
		require.NoError(t, err)

		dataFS, err := loadFilesystem(".")
		require.NoError(t, err)

		// Create and initialize store
		testStore, err := store.NewStoreFromFile(dataFS, "seed.json")
		require.NoError(t, err)

		// Start server
		app, err := server.Start(testStore, staticFS, tmplFS, false, nil)
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
