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

func devMode() bool {
	v := os.Getenv("DEV_MODE")
	return v == "1" || v == "true"
}

func envDuration(key string, fallback time.Duration) time.Duration {
	s := os.Getenv(key)
	if s == "" {
		return fallback
	}
	d, err := time.ParseDuration(s)
	if err != nil {
		return fallback
	}
	return d
}

// keepCamerasInSync keeps the local store in-sync with image origins
func keepCamerasInSync(ctx context.Context, live *store.Store, interval time.Duration, totalSyncs *atomic.Int64) error {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			logger.Muted("Syncing cameras...")
			totalSyncs.Add(1)
			live.FetchImages(ctx)
		}
	}
}

func loadConfig() Config {
	port := os.Getenv("PORT")
	if port == "" {
		port = "3000"
	}

	return Config{
		Port:         port,
		SyncInterval: envDuration("SYNC_INTERVAL", defaultSyncInterval),
		DevMode:      devMode(),
		UDOTAPIKey:   os.Getenv("UDOT_API_KEY"),
		UDOTInterval: envDuration("UDOT_FETCH_INTERVAL", defaultUDOTFetchInterval),
		// Empty token → free NWS station observations, same STIDs.
		SynopticToken:    os.Getenv("SYNOPTIC_TOKEN"),
		SynopticInterval: envDuration("SYNOPTIC_FETCH_INTERVAL", defaultSynopticInterval),
	}
}

// getBaseDir is the directory that holds data.json.
// Bazel test/run looks under TEST_SRCDIR or RUNFILES_DIR /_main.
// Dev uses the working directory. Otherwise the binary directory or its .runfiles.
func getBaseDir() (string, error) {
	if dir := firstDataDir(bazelMain("TEST_SRCDIR"), bazelMain("RUNFILES_DIR")); dir != "" {
		return dir, nil
	}
	if devMode() {
		return os.Getwd()
	}

	exe, err := os.Executable()
	if err != nil {
		return "", err
	}
	exeDir := filepath.Dir(exe)
	runfiles := filepath.Join(exeDir, filepath.Base(exe)+".runfiles", "_main")
	if dir := firstDataDir(exeDir, runfiles); dir != "" {
		return dir, nil
	}
	return os.Getwd()
}

func bazelMain(envKey string) string {
	root := os.Getenv(envKey)
	if root == "" {
		return ""
	}
	return filepath.Join(root, "_main")
}

func firstDataDir(dirs ...string) string {
	for _, dir := range dirs {
		if dir != "" && hasDataJSON(dir) {
			return dir
		}
	}
	return ""
}

func hasDataJSON(dir string) bool {
	_, err := os.Stat(filepath.Join(dir, "data.json"))
	return err == nil
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
	if devMode() {
		// bazel run sets BUILD_WORKSPACE_DIRECTORY; prefer it over the process cwd.
		wd, wdErr := os.Getwd()
		roots := []string{os.Getenv("BUILD_WORKSPACE_DIRECTORY")}
		if wdErr == nil {
			roots = append(roots, wd)
		}
		for _, root := range roots {
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
	for _, sub := range []string{"web/static/dist", "web/static"} {
		dir := filepath.Join(baseDir, sub)
		if _, err := os.Stat(filepath.Join(dir, "script.mjs")); err == nil {
			return os.DirFS(dir), nil
		}
	}
	return nil, fmt.Errorf("static files not found under %s (tried web/static/dist and web/static)", baseDir)
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

	body := bytes.NewBufferString(`{"purge_everything":true}`)

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

	responseBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read response: %w", err)
	}

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

// initSentry initializes Sentry if DSN is provided and not in dev mode.
// Returns true if Sentry was initialized.
func initSentry(dev bool) bool {
	dsn := os.Getenv("SENTRY_DSN")
	if dsn == "" || dev {
		return false
	}

	err := sentry.Init(sentry.ClientOptions{
		Dsn:              dsn,
		Environment:      "production",
		Release:          server.Version,
		EnableTracing:    true,
		TracesSampleRate: 1.0,
		AttachStacktrace: true,
	})
	if err != nil {
		log.Fatalf("sentry.Init: %s", err)
	}
	logger.SetSentryCaptureException(func(err error) interface{} {
		return sentry.CaptureException(err)
	})

	return true
}

func cameraCount(live *store.Store) int {
	n := 0
	for _, id := range live.CanyonIDs() {
		canyon := live.Canyon(id)
		n += len(canyon.Cameras)
		if canyon.Status.Src != "" {
			n++
		}
	}
	return n
}

func publishSyncStats(
	hasUI bool,
	cameras int,
	requestCount *int64,
	lastRequestCount *atomic.Int64,
	lastCheckUnix *atomic.Int64,
	totalSyncs *atomic.Int64,
	duration time.Duration,
	changed, unchanged, failed int,
) {
	if !hasUI {
		return
	}

	currentReqs := atomic.LoadInt64(requestCount)
	prevReqs := lastRequestCount.Swap(currentReqs)
	prevCheck := lastCheckUnix.Swap(time.Now().UnixNano())
	elapsed := float64(time.Now().UnixNano()-prevCheck) / 1e9
	reqPerSec := 0.0
	if elapsed > 0 {
		reqPerSec = float64(currentReqs-prevReqs) / elapsed
	}

	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	memMB := float64(m.Alloc) / 1024 / 1024

	ui.UpdateStats(ui.Stats{
		Cameras:         cameras,
		LastSyncTime:    time.Now(),
		SyncDuration:    duration,
		Changed:         changed,
		Unchanged:       unchanged,
		Errors:          failed,
		TotalSyncs:      int(totalSyncs.Load()),
		RequestsTotal:   int(currentReqs),
		RequestsPerSec:  reqPerSec,
		MemoryUsageMB:   memMB,
		CPUUsagePercent: 0, // TODO: Implement CPU tracking
		GoroutineCount:  runtime.NumGoroutine(),
	})
}

// startPollers runs image sync, UDOT, UAC, Alta, and mountain weather until ctx ends.
func startPollers(g *errgroup.Group, ctx context.Context, live *store.Store, config Config, totalSyncs *atomic.Int64) {
	g.Go(func() error {
		live.FetchImages(ctx)
		return nil
	})
	g.Go(func() error {
		return keepCamerasInSync(ctx, live, config.SyncInterval, totalSyncs)
	})

	udotClient := udot.NewClient(config.UDOTAPIKey)
	udotPoller := udot.NewPoller(udotClient, live, config.UDOTInterval)
	g.Go(func() error { return udotPoller.StartRoadConditions(ctx) })
	g.Go(func() error { return udotPoller.StartWeatherStations(ctx) })
	g.Go(func() error { return udotPoller.StartEvents(ctx) })

	// No API keys.
	uacPoller := uac.NewPoller(uac.NewClient(), live, 10*time.Minute)
	g.Go(func() error { return uacPoller.Start(ctx) })
	altaPoller := alta.NewPoller(alta.NewClient(), live, 3*time.Minute)
	g.Go(func() error { return altaPoller.Start(ctx) })

	// Synoptic when SYNOPTIC_TOKEN is set, else free NWS.
	synopticClient := synoptic.NewClient(config.SynopticToken)
	synopticPoller := synoptic.NewPoller(synopticClient, live, config.SynopticInterval)
	g.Go(func() error { return synopticPoller.Start(ctx) })
}

func main() {
	dev := devMode()
	sentryEnabled := initSentry(dev)

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

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	config := loadConfig()

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

	live, err := store.NewStoreFromFile(dataFS, "data.json")
	if err != nil {
		logger.Fatal(err, "failed to create new store from file %s - %v", "data.json", err)
	}

	cameras := cameraCount(live)

	// HUD before any log line, or the banner and the TUI both print.
	hasUI := ui.Initialize(server.Version, server.BuildTime, config.Port, config.SyncInterval, cameras)
	if hasUI {
		logger.SetUIMode(true)
		logger.Log = ui.AddLog
	} else {
		logger.PrintBanner(server.Version, server.BuildTime)
	}

	if config.DevMode {
		logger.Info("🔥 DEV MODE: Hot reload enabled - files served from disk")
	} else {
		logger.Info("Serving from embedded files")
	}

	var totalSyncs atomic.Int64
	var requestCount int64
	var errorCount int64
	var lastRequestCount atomic.Int64
	var lastCheckUnix atomic.Int64
	lastCheckUnix.Store(time.Now().UnixNano())

	live.SetSyncCallback(func(duration time.Duration, changed, unchanged, failed int) {
		publishSyncStats(
			hasUI, cameras,
			&requestCount, &lastRequestCount, &lastCheckUnix, &totalSyncs,
			duration, changed, unchanged, failed,
		)
	})

	logger.Info("Fetching initial camera images...")
	g, gCtx := errgroup.WithContext(ctx)
	startPollers(g, gCtx, live, config, &totalSyncs)

	server.LogWriter = ui.AddLog
	server.RequestCounter = &requestCount
	server.ErrorCounter = &errorCount
	app, err := server.Start(server.ServerConfig{
		Store:         live,
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

	go func() {
		if err := app.Start(":" + config.Port); err != nil && err != http.ErrServerClosed {
			logger.Error(err, "Server error: %v", err)
			cancel()
		}
	}()

	<-sigChan
	cancel()

	logger.Info("Shutting down gracefully...")
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 4*time.Second)
	defer shutdownCancel()
	if err := app.Shutdown(shutdownCtx); err != nil {
		logger.Error(err, "error during shutdown: %v", err)
	}

	if err := g.Wait(); err != nil && !errors.Is(err, context.Canceled) {
		logger.Error(err, "background task error: %v", err)
	}

	ui.Shutdown()
	server.CloseErrorLogger()
	time.Sleep(100 * time.Millisecond)

	sentry.Flush(2 * time.Second)

	logger.Success("Goodbye!")
	fmt.Println()
}
