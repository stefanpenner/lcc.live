package main

import (
	"fmt"
	"log"
	"time"

	"github.com/stefanpenner/lcc-live/cameras"
	"github.com/stefanpenner/lcc-live/server"
)

func keepCamerasInSync(store *cameras.Store) {
	// TODO: add cancellation
	for {
		fmt.Println("fetching")
		store.FetchImages()
		time.Sleep(time.Second * 10)
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
