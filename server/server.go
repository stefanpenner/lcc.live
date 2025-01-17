package server

import (
	"fmt"
	"html/template"
	"io"
	"log"
	"net/http"

	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
	"github.com/stefanpenner/lcc-live/store"
)

// Template renderer for Echo
type TemplateRenderer struct {
	templates *template.Template
}

var templateFuncs = template.FuncMap{}

func (t *TemplateRenderer) Render(w io.Writer, name string, data interface{}, c echo.Context) error {
	return t.templates.ExecuteTemplate(w, name, data)
}

func Start(store *store.Store) (*echo.Echo, error) {
	e := echo.New()

	// Middleware
	e.Use(middleware.LoggerWithConfig(middleware.LoggerConfig{
		Format: "method=${method}, uri=${uri}, status=${status}\n",
	}))
	e.Use(middleware.Recover())

	tmpl := template.New("").Funcs(templateFuncs)
	// Template renderer
	renderer := &TemplateRenderer{
		templates: template.Must(tmpl.ParseGlob("./views/*")),
	}
	e.Renderer = renderer

	// Error handler
	e.HTTPErrorHandler = func(err error, c echo.Context) {
		c.Render(http.StatusInternalServerError, "error.tmpl", map[string]interface{}{
			"title": "Error",
			"err":   err,
		})
	}

	// Routes
	e.GET("/", func(c echo.Context) error {
		return c.Render(http.StatusOK, "canyon.html.tmpl", store.Canyon("LCC"))
	})

	e.GET("/bcc", func(c echo.Context) error {
		return c.Render(http.StatusOK, "canyon.html.tmpl", store.Canyon("BCC"))
	})

	e.GET("/image/:id", func(c echo.Context) error {
		id := c.Param("id")
		entry, exists := store.Get(id)

		status := http.StatusNotFound

		if exists {
			if entry.HTTPHeaders.Status == http.StatusOK {
				headers := entry.HTTPHeaders

				c.Response().Header().Set("Content-Type", headers.ContentType)
				c.Response().Header().Set("Content-Length", fmt.Sprintf("%d", headers.ContentLength))

				return c.Blob(http.StatusOK, headers.ContentType, entry.Image.Bytes)
			}
			status = entry.HTTPHeaders.Status
		}

		log.Printf("http(%d): %s", status, id)
		return c.String(status, "image not found")
	})

	e.GET("/events", func(c echo.Context) error {
		// c.Response().Header().Set("Content-Type", "text/event-stream")
		// c.Response().Header().Set("Cache-Control", "no-cache")
		// c.Response().Header().Set("Connection", "keep-alive")
		// c.Response().Header().Set("Transfer-Encoding", "chunked")
		//
		// writer := bufio.NewWriter(c.Response().Writer)
		// flusher, ok := c.Response().Writer.(http.Flusher)
		// if !ok {
		// 	return echo.NewHTTPError(http.StatusInternalServerError, "Streaming unsupported!")
		// }
		//
		// c.Response().WriteHeader(http.StatusOK)
		//
		// var i int
		// for {
		// 	select {
		// 	case <-c.Request().Context().Done():
		// 		return nil
		// 	default:
		// 	}
		//
		// 	i++
		// 	msg := fmt.Sprintf("%d - the time is %v", i, time.Now())
		//
		// 	fmt.Fprintf(writer, "data: Message: %s\n\n", msg)
		// 	fmt.Println(msg)
		//
		// 	if err := writer.Flush(); err != nil {
		// 		fmt.Printf("Error while flushing: %v. Closing http connection.\n", err)
		// 		break
		// 	}
		// 	flusher.Flush()
		// 	time.Sleep(10 * time.Second)
		// }

		return nil
	})

	// Static files
	e.Static("/s", "public")

	return e, nil
}
