package server

import (
	"bufio"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/compress"
	"github.com/gofiber/fiber/v2/middleware/logger"
	"github.com/stefanpenner/lcc-live/store"
	"github.com/valyala/fasthttp"

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
		fmt.Printf("get(%s)", id)
		entry, exists := store.Get(id)
		fmt.Printf("did get")

		status := fiber.StatusNotFound

		if exists {

			fmt.Printf("exists")
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
		} else {
			fmt.Printf("nope:")
		}

		log.Printf("http(%d): %s", status, id)
		return c.Status(status).SendString("image not found")
	})

	// SSE endpoint
	app.Get("/events", func(c *fiber.Ctx) error {
		c.Set("Content-Type", "text/event-stream")
		c.Set("Cache-Control", "no-cache")
		c.Set("Connection", "keep-alive")
		c.Set("Transfer-Encoding", "chunked")

		c.Status(fiber.StatusOK).Context().SetBodyStreamWriter(fasthttp.StreamWriter(func(w *bufio.Writer) {
			// TODO: limit max number of connections
			// TODO: cleanup when connections close
			var i int
			for {
				if c.Context().Done() {
					return
				}
				i++
				msg := fmt.Sprintf("%d - the time is %v", i, time.Now())
				fmt.Fprintf(w, "data: Message: %s\n\n", msg)
				fmt.Println(msg)

				err := w.Flush()
				if err != nil {
					// Refreshing page in web browser will establish a new
					// SSE connection, but only (the last) one is alive, so
					// dead connections must be closed here.
					fmt.Printf("Error while flushing: %v. Closing http connection.\n", err)

					break
				}
				time.Sleep(10 * time.Second)
			}
		}))
		return nil
	})
	app.Static("/", "./public")
	return app, nil
}
