package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"runtime"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/stefanpenner/lcc-live/logger"
	"github.com/stefanpenner/lcc-live/neon"
	"github.com/stefanpenner/lcc-live/server"
	"github.com/stefanpenner/lcc-live/store"
	"github.com/stefanpenner/lcc-live/ui"
)

const defaultSyncInterval = 3 * time.Second

type Config struct {
	Port         string
	SyncInterval time.Duration
	DevMode      bool
}

// keepCamerasInSync keeps the local store in-sync with image origins
func keepCamerasInSync(ctx context.Context, store *store.Store, interval time.Duration, totalSyncs *int) error {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			logger.Muted("Syncing cameras...")
			*totalSyncs++
			store.FetchImages(ctx)
		}
	}
}

func loadConfig() Config {
	port := os.Getenv("PORT")
	if port == "" {
		port = "3000"
	}

	syncIntervalStr := os.Getenv("SYNC_INTERVAL")
	syncInterval := defaultSyncInterval
	if syncIntervalStr != "" {
		if d, err := time.ParseDuration(syncIntervalStr); err == nil {
			syncInterval = d
		}
	}

	// Enable dev mode for hot reloading
	devMode := os.Getenv("DEV_MODE") == "1" || os.Getenv("DEV_MODE") == "true"

	return Config{
		Port:         port,
		SyncInterval: syncInterval,
		DevMode:      devMode,
	}
}

// getBaseDir returns the directory containing the binary or working directory in dev mode
func getBaseDir() (string, error) {
	// In dev mode, use working directory
	if os.Getenv("DEV_MODE") == "1" || os.Getenv("DEV_MODE") == "true" {
		return os.Getwd()
	}

	// In production/container, use binary directory
	exe, err := os.Executable()
	if err != nil {
		return "", err
	}
	exeDir := filepath.Dir(exe)

	// Check if files exist in the binary directory (container deployment)
	if _, err := os.Stat(filepath.Join(exeDir, "data.json")); err == nil {
		return exeDir, nil
	}

	// For Bazel runs, check the runfiles directory
	// Bazel creates a .runfiles directory next to the binary
	runfilesDir := filepath.Join(exeDir, filepath.Base(exe)+".runfiles", "_main")
	if _, err := os.Stat(filepath.Join(runfilesDir, "data.json")); err == nil {
		return runfilesDir, nil
	}

	// Fall back to working directory
	return os.Getwd()
}

// loadFilesystem loads files from disk (dev mode) or from bundled files (production)
func loadFilesystem(subdir string) (fs.FS, error) {
	baseDir, err := getBaseDir()
	if err != nil {
		return nil, fmt.Errorf("failed to get base directory: %w", err)
	}

	path := filepath.Join(baseDir, subdir)
	return os.DirFS(path), nil
}

// purgeCloudflareCache purges the Cloudflare cache for the configured zone
func purgeCloudflareCache() error {
	zoneID := os.Getenv("CLOUDFLARE_ZONE_ID")
	apiToken := os.Getenv("CLOUDFLARE_API_TOKEN")

	if zoneID == "" || apiToken == "" {
		logger.Warn("CLOUDFLARE_ZONE_ID or CLOUDFLARE_API_TOKEN not set. Skipping cache purge.")
		return nil
	}

	logger.Info("Purging Cloudflare cache for zone: %s", zoneID)

	// Prepare request body
	body := bytes.NewBufferString(`{"purge_everything":true}`)

	// Create request with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, "POST",
		fmt.Sprintf("https://api.cloudflare.com/client/v4/zones/%s/purge_cache", zoneID),
		body)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+apiToken)
	req.Header.Set("Content-Type", "application/json")

	// Make request
	client := &http.Client{
		Timeout: 15 * time.Second,
	}

	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to make request: %w", err)
	}
	defer resp.Body.Close()

	// Read response
	responseBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read response: %w", err)
	}

	// Parse response
	var result struct {
		Success bool     `json:"success"`
		Errors  []string `json:"errors"`
	}

	if err := json.Unmarshal(responseBody, &result); err != nil {
		return fmt.Errorf("failed to parse response: %w", err)
	}

	if result.Success {
		logger.Success("Cloudflare cache purged successfully")
		return nil
	}

	return fmt.Errorf("cache purge failed: %v", result.Errors)
}

func main() {
	// Handle subcommands
	if len(os.Args) > 1 {
		switch os.Args[1] {
		case "purge-cache":
			if err := purgeCloudflareCache(); err != nil {
				logger.Error("%v", err)
				os.Exit(1)
			}
			os.Exit(0)
		case "help", "--help", "-h":
			fmt.Println("LCC Live Camera Service")
			fmt.Println("")
			fmt.Println("Usage:")
			fmt.Println("  lcc-live              Start the web server (default)")
			fmt.Println("  lcc-live purge-cache  Purge Cloudflare cache")
			fmt.Println("  lcc-live help         Show this help message")
			return
		}
	}

	// Setup graceful shutdown with context
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Setup signal handling
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	config := loadConfig()

	// Setup filesystem - load from disk instead of embed
	staticFS, err := loadFilesystem("static")
	if err != nil {
		log.Fatalf("failed to load static files: %v", err)
	}

	tmplFS, err := loadFilesystem("templates")
	if err != nil {
		log.Fatalf("failed to load templates: %v", err)
	}

	dataFS, err := loadFilesystem(".")
	if err != nil {
		log.Fatalf("failed to load data directory: %v", err)
	}

	store, err := store.NewStoreFromFile(dataFS, "data.json")
	if err != nil {
		log.Fatalf("failed to create new store from file %s - %v", "data.json", err)
	}

	var adminRepo *neon.Repository
	if os.Getenv("NEON_DATABASE_URL") == "" {
		logger.Warn("NEON_DATABASE_URL not set; admin endpoints disabled")
	} else {
		neonConfig, err := neon.FromEnv()
		if err != nil {
			log.Fatalf("invalid Neon configuration: %v", err)
		}
		neonPool, err := neon.NewPool(ctx, neonConfig)
		if err != nil {
			log.Fatalf("failed to connect to Neon: %v", err)
		}
		defer neonPool.Close()
		adminRepo = neon.NewRepository(neonPool)
		logger.Info("Neon connection ready for admin endpoints")
	}

	// Count cameras
	cameraCount := len(store.Canyon("LCC").Cameras) + len(store.Canyon("BCC").Cameras)
	if store.Canyon("LCC").Status.Src != "" {
		cameraCount++
	}
	if store.Canyon("BCC").Status.Src != "" {
		cameraCount++
	}

	// Initialize TUI with HUD (before any logging)
	hasUI := ui.Initialize(server.Version, server.BuildTime, config.Port, config.SyncInterval, cameraCount)
	if hasUI {
		logger.SetUIMode(true)
		logger.Log = ui.AddLog
	} else {
		logger.PrintBanner(server.Version, server.BuildTime)
	}

	// Log startup info
	if config.DevMode {
		logger.Info("🔥 DEV MODE: Hot reload enabled - files served from disk")
	} else {
		logger.Info("Serving from embedded files")
	}

	// Track total syncs and requests
	totalSyncs := 0
	var requestCount int64
	var lastRequestCount int64
	var lastCheckTime = time.Now()

	// Set up store callbacks to update UI stats
	store.SetSyncCallback(func(duration time.Duration, changed, unchanged, errors int) {
		if !hasUI {
			return
		}

		// Calculate requests/sec
		currentReqs := atomic.LoadInt64(&requestCount)
		elapsed := time.Since(lastCheckTime).Seconds()
		reqPerSec := 0.0
		if elapsed > 0 {
			reqPerSec = float64(currentReqs-lastRequestCount) / elapsed
		}
		lastRequestCount = currentReqs
		lastCheckTime = time.Now()

		// Get memory stats
		var m runtime.MemStats
		runtime.ReadMemStats(&m)
		memMB := float64(m.Alloc) / 1024 / 1024

		ui.UpdateStats(ui.Stats{
			Cameras:         cameraCount,
			LastSyncTime:    time.Now(),
			SyncDuration:    duration,
			Changed:         changed,
			Unchanged:       unchanged,
			Errors:          errors,
			TotalSyncs:      totalSyncs,
			RequestsTotal:   int(currentReqs),
			RequestsPerSec:  reqPerSec,
			MemoryUsageMB:   memMB,
			CPUUsagePercent: 0, // TODO: Implement CPU tracking
			GoroutineCount:  runtime.NumGoroutine(),
		})
	})

	// Fetch initial images and start background sync
	logger.Info("Fetching initial camera images...")
	go store.FetchImages(ctx)
	go keepCamerasInSync(ctx, store, config.SyncInterval, &totalSyncs)

	// Configure server to use UI logger
	server.LogWriter = ui.AddLog

	// Start server
	server.RequestCounter = &requestCount
	app, err := server.Start(store, staticFS, tmplFS, config.DevMode, adminRepo)
	if err != nil {
		log.Fatal(err)
	}

	logger.Success("Server listening on http://localhost:%s", config.Port)
	if hasUI {
		logger.Info("Press Ctrl+C or 'q' to stop")
		ui.SetReady()
	} else {
		logger.Info("Press Ctrl+C to stop")
	}

	// Start HTTP server
	go func() {
		if err := app.Start(":" + config.Port); err != nil {
			logger.Error("Server error: %v", err)
			cancel()
		}
	}()

	// Wait for shutdown signal
	<-sigChan
	cancel()

	logger.Info("Shutting down gracefully...")
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer shutdownCancel()
	app.Shutdown(shutdownCtx)
	ui.Shutdown()
	time.Sleep(100 * time.Millisecond)

	logger.Success("Goodbye!")
	fmt.Println()
}
