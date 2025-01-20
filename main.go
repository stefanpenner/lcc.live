package main

import (
	"context"
	"embed"
	_ "embed"
	"fmt"
	"io/fs"
	"log"
	"time"

	fs_helper "github.com/stefanpenner/lcc-live/fs"
	"github.com/stefanpenner/lcc-live/server"
	"github.com/stefanpenner/lcc-live/store"
	"github.com/stefanpenner/lcc-live/style"
)

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

//go:embed data.json
var dataFS embed.FS

//go:embed static/**
var staticFS embed.FS

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
	// TODO: just hold requests until these is done
	store.FetchImages(context.Background())
	go keepCamerasInSync(context.Background(), store)

	app, err := server.Start(store, staticFS, tmplFS)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Println("Serving")
	// Listen and serve on port 3000
	//
	if err := app.Start(":3000"); err != nil {
		log.Fatal(err)
	}
}
