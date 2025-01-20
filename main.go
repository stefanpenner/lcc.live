package main

import (
	"context"
	"embed"
	_ "embed"
	"fmt"
	"io/fs"
	"log"
	"time"

	"github.com/stefanpenner/lcc-live/server"
	"github.com/stefanpenner/lcc-live/store"
)

func keepCamerasInSync(ctx context.Context, store *store.Store) error {
	for {
		select {
		case <-ctx.Done():
			log.Println("cancelling sync")
			return ctx.Err()
		default:
			{
				log.Println("syncing cameras")
				store.FetchImages(ctx)
				time.Sleep(time.Second * 10)
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
	staticFS, _ := fs.Sub(staticFS, "static")
	tmplFS, _ := fs.Sub(tmplFS, "templates")

	fmt.Printf("Embedded File Systems:\n")
	printFS("- data", dataFS)
	printFS("- public", staticFS)
	printFS("- templates", tmplFS)

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
