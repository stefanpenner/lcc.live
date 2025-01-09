package main

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/stefanpenner/lcc-live/cameras"
	"github.com/stefanpenner/lcc-live/server"
)

func keepCamerasInSync(ctx context.Context, store *cameras.Store) error {
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
	store, err := cameras.NewStoreFromFile("cameras.json")
	if err != nil {
		log.Fatalf("failed to create new store from file %s - %v", "cameras.json", err)
	}

	go keepCamerasInSync(context.Background(), store)

	app, err := server.Start(store)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Println("Serving")
	log.Fatal(app.Listen(":3000"))
}
