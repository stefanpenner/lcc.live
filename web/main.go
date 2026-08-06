// Package main is the entry point for the LCC Live webcam server application
package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
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

	"github.com/getsentry/sentry-go"
	"github.com/stefanpenner/lcc-live/web/alta"
	"github.com/stefanpenner/lcc-live/web/logger"
	"github.com/stefanpenner/lcc-live/web/server"
	"github.com/stefanpenner/lcc-live/web/store"
	"github.com/stefanpenner/lcc-live/web/synoptic"
	"github.com/stefanpenner/lcc-live/web/uac"
	"github.com/stefanpenner/lcc-live/web/udot"
	"github.com/stefanpenner/lcc-live/web/ui"
	"golang.org/x/sync/errgroup"
)

const (
	defaultSyncInterval      = 3 * time.Second
	defaultUDOTFetchInterval = 75 * time.Second
	defaultSynopticInterval  = 10 * time.Minute
)

type Config struct {
	Port             string
	SyncInterval     time.Duration
	DevMode          bool
	UDOTAPIKey       string
	UDOTInterval     time.Duration
	SynopticToken    string
	SynopticInterval time.Duration
}

// keepCamerasInSync keeps the local store in-sync with image origins
func keepCamerasInSync(ctx context.Context, store *store.Store, interval time.Duration, totalSyncs *atomic.Int64) error {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			logger.Muted("Syncing cameras...")
			totalSyncs.Add(1)
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

	udotIntervalStr := os.Getenv("UDOT_FETCH_INTERVAL")
	udotInterval := defaultUDOTFetchInterval
	if udotIntervalStr != "" {
		if d, err := time.ParseDuration(udotIntervalStr); err == nil {
			udotInterval = d
		}
	}

	// Enable dev mode for hot reloading
	devMode := os.Getenv("DEV_MODE") == "1" || os.Getenv("DEV_MODE") == "true"

	// Get UDOT API key from environment only
	udotAPIKey := os.Getenv("UDOT_API_KEY")

	// Optional Synoptic/MesoWest token; empty → free NWS station observations
	synopticToken := os.Getenv("SYNOPTIC_TOKEN")
	synopticInterval := defaultSynopticInterval
	if s := os.Getenv("SYNOPTIC_FETCH_INTERVAL"); s != "" {
		if d, err := time.ParseDuration(s); err == nil {
			synopticInterval = d
		}
	}

	return Config{
		Port:             port,
		SyncInterval:     syncInterval,
		DevMode:          devMode,
		UDOTAPIKey:       udotAPIKey,
		UDOTInterval:     udotInterval,
		SynopticToken:    synopticToken,
		SynopticInterval: synopticInterval,
	}
}

// getBaseDir returns the directory containing the binary or working directory in dev mode
func getBaseDir() (string, error) {
	// For Bazel test/run: check TEST_SRCDIR (set by Bazel for tests)
	if testSrcDir := os.Getenv("TEST_SRCDIR"); testSrcDir != "" {
		candidate := filepath.Join(testSrcDir, "_main")
		if _, err := os.Stat(filepath.Join(candidate, "data.json")); err == nil {
			return candidate, nil
		}
	}

	// For Bazel run: check RUNFILES_DIR
	if runfilesDir := os.Getenv("RUNFILES_DIR"); runfilesDir != "" {
		candidate := filepath.Join(runfilesDir, "_main")
		if _, err := os.Stat(filepath.Join(candidate, "data.json")); err == nil {
			return candidate, nil
		}
	}

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

// loadStaticFilesystem prefers Bazel-minified dist/ when present; in DEV_MODE
// loads unminified web/static sources from the workspace for hot-reload.
func loadStaticFilesystem() (fs.FS, error) {
	devMode := os.Getenv("DEV_MODE") == "1" || os.Getenv("DEV_MODE") == "true"
	if devMode {
		// bazel run sets BUILD_WORKSPACE_DIRECTORY; prefer it over runfiles.
		for _, root := range []string{os.Getenv("BUILD_WORKSPACE_DIRECTORY"), mustGetwd()} {
			if root == "" {
				continue
			}
			src := filepath.Join(root, "web/static")
			if _, err := os.Stat(filepath.Join(src, "script.mjs")); err == nil {
				return os.DirFS(src), nil
			}
		}
	}

	baseDir, err := getBaseDir()
	if err != nil {
		return nil, fmt.Errorf("failed to get base directory: %w", err)
	}
	// Minified package first (//web/static:static_files → dist/), then raw static.
	for _, sub := range []string{"web/static/dist", "web/static"} {
		dir := filepath.Join(baseDir, sub)
		if _, err := os.Stat(filepath.Join(dir, "script.mjs")); err == nil {
			return os.DirFS(dir), nil
		}
	}
	return nil, fmt.Errorf("static files not found under %s (tried web/static/dist and web/static)", baseDir)
}

func mustGetwd() string {
	wd, err := os.Getwd()
	if err != nil {
		return ""
	}
	return wd
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
	defer func() {
		_ = resp.Body.Close()
	}()

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

// initSentry initializes Sentry if DSN is provided and not in dev mode
// Returns true if Sentry was initialized
func initSentry(devMode bool) bool {
	dsn := os.Getenv("SENTRY_DSN")
	if dsn == "" || devMode {
		return false
	}

	err := sentry.Init(sentry.ClientOptions{
		Dsn:         dsn,
		Environment: "production",
		Release:     server.Version,
		// Enable performance monitoring
		EnableTracing: true,
		// Set sample rate for performance monitoring
		TracesSampleRate: 1.0,
		// Capture panics
		AttachStacktrace: true,
	})
	if err != nil {
		log.Fatalf("sentry.Init: %s", err)
	}
	// Configure logger to send errors to Sentry
	logger.SetSentryCaptureException(func(err error) interface{} {
		return sentry.CaptureException(err)
	})

	return true
}

func main() {
	// Check dev mode early
	devMode := os.Getenv("DEV_MODE") == "1" || os.Getenv("DEV_MODE") == "true"

	// Initialize Sentry early, before any other operations
	sentryEnabled := initSentry(devMode)

	// Handle subcommands
	if len(os.Args) > 1 {
		switch os.Args[1] {
		case "purge-cache":
			if err := purgeCloudflareCache(); err != nil {
				logger.Fatal(err)
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
	staticFS, err := loadStaticFilesystem()
	if err != nil {
		logger.Fatal(err, "failed to load static files: %v", err)
	}

	tmplFS, err := loadFilesystem("web/templates")
	if err != nil {
		logger.Fatal(err, "failed to load templates: %v", err)
	}

	dataFS, err := loadFilesystem(".")
	if err != nil {
		logger.Fatal(err, "failed to load data directory: %v", err)
	}

	store, err := store.NewStoreFromFile(dataFS, "data.json")
	if err != nil {
		logger.Fatal(err, "failed to create new store from file %s - %v", "data.json", err)
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
	var totalSyncs atomic.Int64
	var requestCount int64
	var errorCount int64
	var lastRequestCount atomic.Int64
	var lastCheckUnix atomic.Int64
	lastCheckUnix.Store(time.Now().UnixNano())

	// Set up store callbacks to update UI stats
	store.SetSyncCallback(func(duration time.Duration, changed, unchanged, errors int) {
		if !hasUI {
			return
		}

		// Calculate requests/sec
		currentReqs := atomic.LoadInt64(&requestCount)
		prevReqs := lastRequestCount.Swap(currentReqs)
		prevCheck := lastCheckUnix.Swap(time.Now().UnixNano())
		elapsed := float64(time.Now().UnixNano()-prevCheck) / 1e9
		reqPerSec := 0.0
		if elapsed > 0 {
			reqPerSec = float64(currentReqs-prevReqs) / elapsed
		}

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
			TotalSyncs:      int(totalSyncs.Load()),
			RequestsTotal:   int(currentReqs),
			RequestsPerSec:  reqPerSec,
			MemoryUsageMB:   memMB,
			CPUUsagePercent: 0, // TODO: Implement CPU tracking
			GoroutineCount:  runtime.NumGoroutine(),
		})
	})

	// Fetch initial images and start background sync
	logger.Info("Fetching initial camera images...")
	g, gCtx := errgroup.WithContext(ctx)

	g.Go(func() error {
		store.FetchImages(gCtx)
		return nil
	})
	g.Go(func() error {
		return keepCamerasInSync(gCtx, store, config.SyncInterval, &totalSyncs)
	})

	// Start UDOT API fetchers
	udotClient := udot.NewClient(config.UDOTAPIKey)
	udotPoller := udot.NewPoller(udotClient, store, config.UDOTInterval)
	g.Go(func() error { return udotPoller.StartRoadConditions(gCtx) })
	g.Go(func() error { return udotPoller.StartWeatherStations(gCtx) })
	g.Go(func() error { return udotPoller.StartEvents(gCtx) })

	// Public HTTP pollers (no API keys)
	uacPoller := uac.NewPoller(uac.NewClient(), store, 10*time.Minute)
	g.Go(func() error { return uacPoller.Start(gCtx) })
	altaPoller := alta.NewPoller(alta.NewClient(), store, 3*time.Minute)
	g.Go(func() error { return altaPoller.Start(gCtx) })

	// Mountain weather: Synoptic when SYNOPTIC_TOKEN set, else free NWS (same STIDs)
	synopticClient := synoptic.NewClient(config.SynopticToken)
	synopticPoller := synoptic.NewPoller(synopticClient, store, config.SynopticInterval)
	g.Go(func() error { return synopticPoller.Start(gCtx) })

	// Configure server to use UI logger
	server.LogWriter = ui.AddLog

	// Start server
	server.RequestCounter = &requestCount
	server.ErrorCounter = &errorCount
	app, err := server.Start(server.ServerConfig{
		Store:         store,
		StaticFS:      staticFS,
		TemplateFS:    tmplFS,
		DevMode:       config.DevMode,
		SentryEnabled: sentryEnabled,
	})
	if err != nil {
		logger.Fatal(err)
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
		if err := app.Start(":" + config.Port); err != nil && err != http.ErrServerClosed {
			logger.Error(err, "Server error: %v", err)
			cancel()
		}
	}()

	// Wait for shutdown signal
	<-sigChan
	cancel()

	logger.Info("Shutting down gracefully...")
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 4*time.Second)
	defer shutdownCancel()
	if err := app.Shutdown(shutdownCtx); err != nil {
		logger.Error(err, "error during shutdown: %v", err)
	}

	// Wait for background goroutines to finish
	if err := g.Wait(); err != nil && !errors.Is(err, context.Canceled) {
		logger.Error(err, "background task error: %v", err)
	}

	ui.Shutdown()
	server.CloseErrorLogger()
	time.Sleep(100 * time.Millisecond)

	// Flush Sentry before exiting
	sentry.Flush(2 * time.Second)

	logger.Success("Goodbye!")
	fmt.Println()
}
