package main

import (
	"context"
	"embed"
	_ "embed"
	"fmt"
	"io/fs"
	"log"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/stefanpenner/lcc-live/server"
	"github.com/stefanpenner/lcc-live/store"
)

var (
	// Style definitions
	titleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#FF69B4")).
			MarginBottom(1)

	sectionStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#5F9EA0")).
			Bold(true)

	fileStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#98FB98"))

	dirStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#DDA0DD")).
			Bold(true)

	infoStyle = lipgloss.NewStyle().
			Italic(true).
			Foreground(lipgloss.Color("#FFD700"))
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
	fmt.Println(titleStyle.Render("🌄 Starting LCC Live Camera Service"))
	fmt.Println(infoStyle.Render("https://lcc.live/"))

	staticFS, _ := fs.Sub(staticFS, "static")
	tmplFS, _ := fs.Sub(tmplFS, "templates")

	fmt.Println(sectionStyle.Render("Embedded File Systems:"))
	printFS("📄 Data", dataFS)
	printFS("🌐 Public", staticFS)
	printFS("📑 Templates", tmplFS)

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
