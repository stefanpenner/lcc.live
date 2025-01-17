package main

import (
	"context"
	"fmt"
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

func main() {
	store, err := store.NewStoreFromFile("data.json")
	if err != nil {
		log.Fatalf("failed to create new store from file %s - %v", "cameras.json", err)
	}

	// block server starting until images are ready
	// TODO: just hold requests until these is done
	store.FetchImages(context.Background())
	go keepCamerasInSync(context.Background(), store)

	app, err := server.Start(store)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Println("Serving")
	log.Fatal(app.Listen(":3000"))
}
