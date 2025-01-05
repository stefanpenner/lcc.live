package main

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"time"

	"github.com/gofiber/fiber/v2"
)

var data struct {
	Cameras []Camera `json:"cameras"`
}

type Camera struct {
	Kind   string
	Src    string
	Alt    string
	Canyon string
	Image  Image
}

type Image struct {
	Status        int
	Src           string
	ContentType   string
	Bytes         []byte
	ContentLength int64
	Expires       int
}

type Store struct {
	path       string
	IDToCamera map[string]Camera
	Cameras    []Camera
}

func NewStore(cameras ...Camera) *Store {
	IDToCamera := make(map[string]Camera)

	for _, camera := range cameras {
		id := url.QueryEscape(camera.Src)
		IDToCamera[id] = camera
	}

	for _, camera := range cameras {
		fmt.Printf("FetchingCamera: %v\n", camera)
		client := &http.Client{
			Timeout: 30 * time.Second,
		}
		resp, err := client.Get(camera.Src)
		if err != nil {
			log.Fatalf("Error fetching image: %v err: %v\n", camera.Src, err)
		}

		if resp.StatusCode != http.StatusOK {
			log.Fatalf("Bad status code from image source: %d\n", resp.StatusCode)
		}

		contentType := resp.Header.Get("Content-Type")
		contentLength := resp.ContentLength

		// Read the full response body into memory
		imageBytes, err := io.ReadAll(resp.Body)
		if err != nil {
			log.Fatalf("Error reading image body: %v\n", err)
		}

		camera.Image = Image{
			Status:        http.StatusOK,
			Bytes:         imageBytes,
			ContentType:   contentType,
			ContentLength: contentLength,
		}
	}

	return &Store{
		Cameras:    cameras,
		IDToCamera: IDToCamera,
	}
}

func (s *Store) Get(cameraID string) (*Camera, bool) {
	camera, exists := s.IDToCamera[cameraID]
	return &camera, exists
}

func main() {
	app := fiber.New()

	app.Get("/", func(c *fiber.Ctx) error {
		return c.SendFile("public/index.html")
	})

	app.Get("/bcc", func(c *fiber.Ctx) error {
		return c.SendFile("public/index.html")
	})

	app.Get("/lcc", func(c *fiber.Ctx) error {
		return c.SendFile("public/index.html")
	})

	// Read cameras from JSON file
	jsonFile, err := os.ReadFile("cameras.json")
	if err != nil {
		log.Fatalf("Error reading cameras.json: %v", err)
	}
	if err := json.Unmarshal(jsonFile, &data); err != nil {
		log.Fatalf("Error parsing cameras.json: %v", err)
	}

	store := NewStore(data.Cameras...)

	app.Get("/image/:id", func(c *fiber.Ctx) error {
		id := c.Params("id")
		camera, exists := store.Get(id)

		status := fiber.StatusNotFound

		if exists {
			if camera.Image.Status == http.StatusOK {
				image := camera.Image

				c.Set("Content-Type", image.ContentType)
				c.Set("Content-Length", fmt.Sprintf("%d", image.ContentLength))

				log.Printf("Http(200): src: %s content-type: %s content-length: %d ", camera.Src, image.ContentType, image.ContentLength)
				return c.Send(image.Bytes)
			} else {
				status = camera.Image.Status
			}
		}

		log.Printf("http(%d): %s", status, id)
		return c.Status(status).SendString("image not found")
	})

	app.Static("/", "./public")
	log.Fatal(app.Listen(":3000"))
}
