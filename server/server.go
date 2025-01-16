package server

import (
	"fmt"
	"log"
	"net/http"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/compress"
	"github.com/gofiber/fiber/v2/middleware/logger"
	"github.com/stefanpenner/lcc-live/store"

	"github.com/gofiber/template/html/v2"
)

func Start(store *store.Store) (*fiber.App, error) {
	app := fiber.New(fiber.Config{
		Views: html.New("./views", ".html.tmpl"),
	})
	app.Use(logger.New())
	app.Use(compress.New())

	app.Get("/", func(c *fiber.Ctx) error {
		return c.Render("canyon", store.Canyon("LCC"))
	})

	app.Get("/bcc", func(c *fiber.Ctx) error {
		return c.Render("canyon", store.Canyon("BCC"))
	})

	app.Get("/image/:id", func(c *fiber.Ctx) error {
		// TODO: add http caching
		id := c.Params("id")
		entry, exists := store.Get(id)

		status := fiber.StatusNotFound

		if exists {
			if entry.HTTPHeaders.Status == http.StatusOK {
				headers := entry.HTTPHeaders

				c.Set("Content-Type", headers.ContentType)
				c.Set("Content-Length", fmt.Sprintf("%d", headers.ContentLength))
				// TODO: provide exact cache control headers

				log.Printf("Http(200): src: %s content-type: %s content-length: %d ", entry.Image.Src, headers.ContentType, headers.ContentLength)
				return c.Send(entry.Image.Bytes)
			} else {
				status = entry.HTTPHeaders.Status
			}
		}

		log.Printf("http(%d): %s", status, id)
		return c.Status(status).SendString("image not found")
	})

	app.Static("/", "./public")
	return app, nil
}
