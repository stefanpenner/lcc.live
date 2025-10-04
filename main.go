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
	"syscall"
	"time"

	fs_helper "github.com/stefanpenner/lcc-live/fs"
	"github.com/stefanpenner/lcc-live/server"
	"github.com/stefanpenner/lcc-live/store"
	"github.com/stefanpenner/lcc-live/style"
)

const defaultSyncInterval = 3 * time.Second

type Config struct {
	Port         string
	SyncInterval time.Duration
}

// keeps the local store in-sync with image origins.
func keepCamerasInSync(ctx context.Context, store *store.Store, interval time.Duration) error {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			fmt.Println(style.Cancel.Render("🛑 Cancelling camera sync"))
			return ctx.Err()
		case <-ticker.C:
			fmt.Println(style.Sync.Render("🔄 Starting camera sync..."))
			store.FetchImages(ctx)
			fmt.Printf(style.Sync.Render("💤 Waiting %s before next sync\n"), interval)
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
		fmt.Println("⚠️  Warning: CLOUDFLARE_ZONE_ID or CLOUDFLARE_API_TOKEN not set. Skipping cache purge.")
		return nil
	}

	fmt.Printf("🔄 Purging Cloudflare cache for zone: %s\n", zoneID)

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
		fmt.Println("✅ Cloudflare cache purged successfully")
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
				fmt.Fprintf(os.Stderr, "❌ Error: %v\n", err)
				os.Exit(1)
			}
			return
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

	fmt.Println(style.Title.Render("🌄 Starting LCC Live Camera Service"))
	fmt.Println(style.Info.Render("https://lcc.live/\n"))

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

	fmt.Println(style.Section.Render("Embedded File Systems:"))
	fs_helper.Print("📄 Data", dataFS)
	fs_helper.Print("🌐 Public", static)
	fs_helper.Print("📑 Templates", tmpl)

	store, err := store.NewStoreFromFile(dataFS, "data.json")
	if err != nil {
		log.Fatalf("failed to create new store from file %s - %v", "data.json", err)
	}

	config := loadConfig()

	// Start initial image fetch in background
	go store.FetchImages(ctx)

	// kick-off camera syncing background thread
	go keepCamerasInSync(ctx, store, config.SyncInterval)

	app, err := server.Start(store, static, tmpl)
	if err != nil {
		log.Fatal(err)
	}

	// Start server
	go func() {
		if err := app.Start(":" + config.Port); err != nil {
			log.Printf("server error: %v", err)
			cancel() // Cancel context on server error
		}
	}()

	// Wait for shutdown signal
	<-sigChan
	cancel() // Cancel root context

	// Give the server a short grace period to finish requests
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer shutdownCancel()
	app.Shutdown(shutdownCtx)
}
