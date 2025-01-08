package main

import (
	"fmt"
	"log"
	"time"

	"github.com/stefanpenner/lcc-live/cameras"
	"github.com/stefanpenner/lcc-live/server"
)

func keepCamerasInSync(store *cameras.Store) {
	fmt.Println("fetching")
	store.FetchImages()
	for range time.Tick(time.Second * 1) {
		fmt.Println("fetching")
		store.FetchImages()
	}
}

func main() {
	store, err := cameras.NewStoreFromFile("cameras.json")
	if err != nil {
		log.Fatalf("failed to create new store from file %s - %v", "cameras.json", err)
	}

	go keepCamerasInSync(store)

	app, err := server.Start(store)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Println("Serving")
	log.Fatal(app.Listen(":3000"))
}
