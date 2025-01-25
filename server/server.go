package server

import (
	"fmt"
	"html/template"
	"io"
	"io/fs"

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
	e.HideBanner = true
	e.Use(middleware.GzipWithConfig(middleware.GzipConfig{
		Level: 5,
	}))
	e.StaticFS("/s", staticFS)

	// make some nicer log output
	e.Use(middleware.LoggerWithConfig(middleware.LoggerConfig{
		Format: fmt.Sprintf("${time_rfc3339} %s %s %s %s\n",
			style.Method.Render("${method}"),
			style.URI.Render("${uri}"),
			"${status}",
			style.Duration.Render("${latency_human}")),
	}))

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
	// handleIndex handles both GET and HEAD requests for the index route

	e.GET("/", CanyonRoute(store, "LCC"))
	e.HEAD("/", CanyonRoute(store, "LCC"))

	e.GET("/bcc", CanyonRoute(store, "BCC"))
	e.HEAD("/bcc", CanyonRoute(store, "BCC"))

	e.GET("/image/:id", ImageRoute(store))
	e.HEAD("/image/:id", ImageRoute(store))

	e.GET("/healthcheck", HealthCheckRoute())

	return e, nil
}
