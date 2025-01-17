package server

import (
	"bufio"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stefanpenner/lcc-live/store"
)

func Start(store *store.Store) (*gin.Engine, error) {
	router := gin.New()

	router.Use(gin.Logger())
	router.Use(gin.Recovery()) // TODO: what?

	router.LoadHTMLGlob("./views/*")

	router.GET("/", func(c *gin.Context) {
		c.HTML(http.StatusOK, "canyon.html.tmpl", store.Canyon("LCC"))
	})

	// Define route for "/bcc"
	router.GET("/bcc", func(c *gin.Context) {
		c.HTML(http.StatusOK, "canyon.html.tmpl", store.Canyon("BCC"))
	})

	// Define route for "/image/:id"
	router.GET("/image/:id", func(c *gin.Context) {
		id := c.Param("id")
		fmt.Printf("get(%s)", id)
		entry, exists := store.Get(id)
		fmt.Printf("did get")

		status := http.StatusNotFound

		if exists {
			fmt.Printf("exists")
			if entry.HTTPHeaders.Status == http.StatusOK {
				headers := entry.HTTPHeaders

				c.Header("Content-Type", headers.ContentType)
				c.Header("Content-Length", fmt.Sprintf("%d", headers.ContentLength))
				// TODO: provide exact cache control headers

				log.Printf("Http(200): src: %s content-type: %s content-length: %d ", entry.Image.Src, headers.ContentType, headers.ContentLength)
				c.Data(http.StatusOK, headers.ContentType, entry.Image.Bytes)
				return
			} else {
				status = entry.HTTPHeaders.Status
			}
		} else {
			fmt.Printf("nope:")
		}

		log.Printf("http(%d): %s", status, id)
		c.String(status, "image not found")
	})

	router.GET("/events", func(c *gin.Context) {
		c.Header("Content-Type", "text/event-stream")
		c.Header("Cache-Control", "no-cache")
		c.Header("Connection", "keep-alive")
		c.Header("Transfer-Encoding", "chunked")

		writer := bufio.NewWriter(c.Writer)
		flusher, ok := c.Writer.(http.Flusher)
		if !ok {
			http.Error(c.Writer, "Streaming unsupported!", http.StatusInternalServerError)
			return
		}

		c.Status(http.StatusOK)

		var i int
		for {
			select {
			case <-c.Writer.CloseNotify():
				return
			default:
			}

			i++
			msg := fmt.Sprintf("%d - the time is %v", i, time.Now())

			fmt.Fprintf(writer, "data: Message: %s\n\n", msg)
			fmt.Println(msg)

			if err := writer.Flush(); err != nil {
				fmt.Printf("Error while flushing: %v. Closing http connection.\n", err)
				break
			}
			flusher.Flush()
			time.Sleep(10 * time.Second)
		}
	})

	router.Static("/s/", "./public")

	return router, nil
}
