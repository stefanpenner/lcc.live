package main

import (
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoadConfig_Defaults(t *testing.T) {
	// Clear environment
	os.Unsetenv("PORT")
	os.Unsetenv("SYNC_INTERVAL")

	config := loadConfig()

	assert.Equal(t, "3000", config.Port)
	assert.Equal(t, defaultSyncInterval, config.SyncInterval)
}

func TestLoadConfig_CustomPort(t *testing.T) {
	os.Setenv("PORT", "8080")
	defer os.Unsetenv("PORT")

	config := loadConfig()

	assert.Equal(t, "8080", config.Port)
}

func TestLoadConfig_CustomSyncInterval(t *testing.T) {
	os.Setenv("SYNC_INTERVAL", "10s")
	defer os.Unsetenv("SYNC_INTERVAL")

	config := loadConfig()

	assert.Equal(t, 10*time.Second, config.SyncInterval)
}

func TestLoadConfig_InvalidSyncInterval(t *testing.T) {
	os.Setenv("SYNC_INTERVAL", "invalid")
	defer os.Unsetenv("SYNC_INTERVAL")

	config := loadConfig()

	// Should fall back to default
	assert.Equal(t, defaultSyncInterval, config.SyncInterval)
}

func TestLoadConfig_ZeroSyncInterval(t *testing.T) {
	os.Setenv("SYNC_INTERVAL", "0")
	defer os.Unsetenv("SYNC_INTERVAL")

	config := loadConfig()

	assert.Equal(t, time.Duration(0), config.SyncInterval)
}

func TestLoadConfig_MultipleCalls(t *testing.T) {
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
		"static/script.js",
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
