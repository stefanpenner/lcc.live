package server

import (
	"fmt"
	"html/template"
	"io"
	"io/fs"

	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
	"github.com/prometheus/client_golang/prometheus/promhttp"
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

	// Add version header to all responses
	e.Use(func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			c.Response().Header().Set("X-Version", GetVersionString())
			return next(c)
		}
	})

	// Add metrics middleware early to track all requests
	e.Use(MetricsMiddleware())

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
	e.GET("/.json", CanyonRoute(store, "LCC"))
	e.HEAD("/.json", CanyonRoute(store, "LCC"))

	e.GET("/lcc", CanyonRoute(store, "LCC"))
	e.HEAD("/lcc", CanyonRoute(store, "LCC"))
	e.GET("/lcc.json", CanyonRoute(store, "LCC"))
	e.HEAD("/lcc.json", CanyonRoute(store, "LCC"))

	e.GET("/bcc", CanyonRoute(store, "BCC"))
	e.HEAD("/bcc", CanyonRoute(store, "BCC"))
	e.GET("/bcc.json", CanyonRoute(store, "BCC"))
	e.HEAD("/bcc.json", CanyonRoute(store, "BCC"))

	e.GET("/image/:id", ImageRoute(store))
	e.HEAD("/image/:id", ImageRoute(store))

	e.GET("/camera/*", CameraRoute(store))
	e.HEAD("/camera/*", CameraRoute(store))

	e.GET("/healthcheck", HealthCheckRoute(store))

	// Internal/admin endpoints under /_/
	// These endpoints should never be cached
	internal := e.Group("/_")
	internal.Use(func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			// Prevent any caching of internal endpoints
			c.Response().Header().Set("Cache-Control", "no-store, no-cache, must-revalidate, private, max-age=0")
			c.Response().Header().Set("Pragma", "no-cache")
			c.Response().Header().Set("Expires", "0")
			return next(c)
		}
	})
	internal.GET("/version", VersionRoute())
	internal.GET("/metrics", echo.WrapHandler(promhttp.Handler()))

	return e, nil
}
