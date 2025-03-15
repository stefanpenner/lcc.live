package main

import (
	"context"
	"embed"
	"fmt"
	"io/fs"
	"log"
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

func main() {
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
