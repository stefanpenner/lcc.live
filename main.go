package main

import (
	"bytes"
	"context"
	"embed"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"log"
	"net/http"
	"os"
	"os/signal"
	"runtime"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/stefanpenner/lcc-live/logger"
	"github.com/stefanpenner/lcc-live/server"
	"github.com/stefanpenner/lcc-live/store"
	"github.com/stefanpenner/lcc-live/ui"
)

const defaultSyncInterval = 3 * time.Second

type Config struct {
	Port         string
	SyncInterval time.Duration
}

// keeps the local store in-sync with image origins.
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

// All assets are provided as part of the same binary using go:embed
// to keep stuff organized, we provide 3 seperate embedded file systems

// seed data
//
//go:embed data.json
var dataFS embed.FS

// assets for web serving (css, images etc)
//
//go:embed static/**
var staticFS embed.FS

// templates
//
//go:embed templates/**
var tmplFS embed.FS

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

	return Config{
		Port:         port,
		SyncInterval: syncInterval,
	}
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
	// Check for subcommands
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

	// lets do cancellation, man it's nice to see a language that builds this in
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Setup signal handling
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	// Setup filesystem
	static, err := fs.Sub(staticFS, "static")
	if err != nil {
		log.Fatalf("failed to setup static filesystem: %v", err)
	}

	tmpl, err := fs.Sub(tmplFS, "templates")
	if err != nil {
		log.Fatalf("failed to setup template filesystem: %v", err)
	}

	store, err := store.NewStoreFromFile(dataFS, "data.json")
	if err != nil {
		log.Fatalf("failed to create new store from file %s - %v", "data.json", err)
	}

	config := loadConfig()

	// Count cameras
	cameraCount := len(store.Canyon("LCC").Cameras) + len(store.Canyon("BCC").Cameras)
	if store.Canyon("LCC").Status.Src != "" {
		cameraCount++
	}
	if store.Canyon("BCC").Status.Src != "" {
		cameraCount++
	}

	// Initialize TUI with HUD (do this BEFORE any logging)
	hasUI := ui.Initialize(server.Version, server.BuildTime, config.Port, config.SyncInterval, cameraCount)

	if hasUI {
		// Configure logger to use UI
		logger.SetUIMode(true)
		logger.Log = ui.AddLog

		// Now log the initialization messages
		logger.Info("Embedded filesystems: Data (1), Public (4), Templates (2)")
	} else {
		// No TTY, print startup info normally
		logger.PrintBanner(server.Version, server.BuildTime)
		logger.Section("Embedded File Systems")
		logger.Info("Data (1), Public (4), Templates (2)")
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

	// Start initial image fetch in background
	logger.Info("Fetching initial camera images...")
	go store.FetchImages(ctx)

	// kick-off camera syncing background thread
	go keepCamerasInSync(ctx, store, config.SyncInterval, &totalSyncs)

	// Configure server to use UI logger
	server.LogWriter = ui.AddLog

	// Set up request counter middleware
	server.RequestCounter = &requestCount

	app, err := server.Start(store, static, tmpl)
	if err != nil {
		log.Fatal(err)
	}

	// Start server
	logger.Success("Server listening on http://localhost:%s", config.Port)
	if hasUI {
		logger.Info("Press Ctrl+C or 'q' to stop")
		ui.SetReady()
	} else {
		logger.Info("Press Ctrl+C to stop")
	}

	go func() {
		if err := app.Start(":" + config.Port); err != nil {
			logger.Error("Server error: %v", err)
			cancel() // Cancel context on server error
		}
	}()

	// Wait for shutdown signal
	<-sigChan
	cancel() // Cancel root context

	logger.Info("Shutting down gracefully...")

	// Give the server a short grace period to finish requests
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer shutdownCancel()
	app.Shutdown(shutdownCtx)

	// Shutdown UI
	ui.Shutdown()
	time.Sleep(100 * time.Millisecond)

	logger.Success("Goodbye!")
	fmt.Println()
}
