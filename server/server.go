package server

import (
	"fmt"
	"html/template"
	"io"
	"io/fs"
	"log"
	"net/http"

	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
	"github.com/stefanpenner/lcc-live/store"
	"github.com/stefanpenner/lcc-live/style"
)

// Template renderer for Echo
type TemplateRenderer struct {
	templates *template.Template
	fs        fs.FS
}

var templateFuncs = template.FuncMap{}

func (t *TemplateRenderer) Render(w io.Writer, name string, data interface{}, c echo.Context) error {
	return t.templates.ExecuteTemplate(w, name, data)
}

func Start(store *store.Store, staticFS fs.FS, tmplFS fs.FS) (*echo.Echo, error) {
	e := echo.New()

	e.StaticFS("/s", staticFS)

	// make some nicer log output
	e.Use(middleware.LoggerWithConfig(middleware.LoggerConfig{
		Format: fmt.Sprintf("${time_rfc3339} %s %s %s %s\n",
			style.Method.Render("${method}"),
			style.URI.Render("${uri}"),
			"${status}",
			style.Duration.Render("${latency_human}")),
	}))

	// lets make the output pretty
	e.Use(func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			err := next(c)

			// Color the status in the log based on the response code
			status := c.Response().Status
			if status >= 400 {
				fmt.Print("\033[1A\033[K") // Move up one line and clear it
				log.Printf(style.StatusError.Render(fmt.Sprintf("%d", status)))
			} else {
				fmt.Print("\033[1A\033[K") // Move up one line and clear it
				log.Printf(style.StatusSuccess.Render(fmt.Sprintf("%d", status)))
			}

			return err
		}
	})

	e.Use(middleware.Recover())
	// Custom Rendering Stuff [
	tmpl, err := template.New("").Funcs(templateFuncs).ParseFS(tmplFS, "*.tmpl")
	if err != nil {
		return nil, err
	}
	renderer := &TemplateRenderer{
		templates: tmpl,
	}
	e.Renderer = renderer
	// ]

	e.GET("/", func(c echo.Context) error {
		return c.Render(http.StatusOK, "canyon.html.tmpl", store.Canyon("LCC"))
	})

	e.GET("/healthcheck", func(c echo.Context) error {
		return c.String(http.StatusOK, "OK")
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

		return c.String(status, "image not found")
	})

	// TODO: leave incase I want to explore SSE again
	// e.GET("/events", func(c echo.Context) error {
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

	// return nil
	// })
	return e, nil
}
