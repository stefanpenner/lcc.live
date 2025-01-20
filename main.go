package main

import (
	"context"
	"embed"
	"fmt"
	"io/fs"
	"log"
	"time"

	fs_helper "github.com/stefanpenner/lcc-live/fs"
	"github.com/stefanpenner/lcc-live/server"
	"github.com/stefanpenner/lcc-live/store"
	"github.com/stefanpenner/lcc-live/style"
)

// keeps the local store in-sync with image origins.
func keepCamerasInSync(ctx context.Context, store *store.Store) error {
	for {
		select {
		case <-ctx.Done():
			fmt.Println(style.Cancel.Render("🛑 Cancelling camera sync"))
			return ctx.Err()
		default:
			{
				fmt.Println(style.Sync.Render("🔄 Starting camera sync..."))
				store.FetchImages(ctx)
				fmt.Println(style.Sync.Render("💤 Waiting 5 seconds before next sync"))
				time.Sleep(time.Second * 5)
			}
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

func main() {
	fmt.Println(style.Title.Render("🌄 Starting LCC Live Camera Service"))
	fmt.Println(style.Info.Render("https://lcc.live/\n"))

	staticFS, _ := fs.Sub(staticFS, "static")
	tmplFS, _ := fs.Sub(tmplFS, "templates")

	fmt.Println(style.Section.Render("Embedded File Systems:"))
	fs_helper.Print("📄 Data", dataFS)
	fs_helper.Print("🌐 Public", staticFS)
	fs_helper.Print("📑 Templates", tmplFS)

	store, err := store.NewStoreFromFile(dataFS, "data.json")
	if err != nil {
		log.Fatalf("failed to create new store from file %s - %v", "data.json", err)
	}

	// block server starting until images are ready
	store.FetchImages(context.Background())

	// kick-off camera syncing background thread, this one just runs forever
	go keepCamerasInSync(context.Background(), store)

	app, err := server.Start(store, staticFS, tmplFS)
	if err != nil {
		log.Fatal(err)
	}

	if err := app.Start("0.0.0.0:3000"); err != nil {
		log.Fatal(err)
	}
}
