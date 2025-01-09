package server

import (
	"fmt"
	"log"
	"net/http"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/logger"
	"github.com/stefanpenner/lcc-live/cameras"
)

func Start(store *cameras.Store) (*fiber.App, error) {
	app := fiber.New()
	app.Use(logger.New())

	app.Get("/", func(c *fiber.Ctx) error {
		return c.SendFile("public/index.html")
	})

	app.Get("/bcc", func(c *fiber.Ctx) error {
		return c.SendFile("public/index.html")
	})

	app.Get("/lcc", func(c *fiber.Ctx) error {
		return c.SendFile("public/index.html")
	})
	app.Get("/image/:id", func(c *fiber.Ctx) error {
		id := c.Params("id")
		camera, exists := store.Get(id)

		status := fiber.StatusNotFound

		if exists {
			if camera.HTTPHeaders.Status == http.StatusOK {
				headers := &camera.HTTPHeaders

				c.Set("Content-Type", headers.ContentType)
				c.Set("Content-Length", fmt.Sprintf("%d", headers.ContentLength))

				log.Printf("Http(200): src: %s content-type: %s content-length: %d ", camera.Src, headers.ContentType, headers.ContentLength)
				return c.Send(camera.Image.Bytes)
			} else {
				status = camera.HTTPHeaders.Status
			}
		}

		log.Printf("http(%d): %s", status, id)
		return c.Status(status).SendString("image not found")
	})

	app.Static("/", "./public")
	return app, nil
}
