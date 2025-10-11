package server

import (
	"fmt"
	"html/template"
	"io"
	"io/fs"
	"net/http"
	"strings"
	"sync/atomic"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/stefanpenner/lcc-live/store"
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

// LogWriter is used to capture Echo logs and route them through our logger
var LogWriter func(string)

// RequestCounter tracks total requests (for UI stats)
var RequestCounter *int64

// Charm colors for HTTP logs
var (
	methodGET    = lipgloss.NewStyle().Foreground(lipgloss.Color("#42D9C8")).Bold(true)
	methodPOST   = lipgloss.NewStyle().Foreground(lipgloss.Color("#73F59F")).Bold(true)
	methodPUT    = lipgloss.NewStyle().Foreground(lipgloss.Color("#FFE66D")).Bold(true)
	methodDELETE = lipgloss.NewStyle().Foreground(lipgloss.Color("#FF6B9D")).Bold(true)
	status2xx    = lipgloss.NewStyle().Foreground(lipgloss.Color("#73F59F"))
	status3xx    = lipgloss.NewStyle().Foreground(lipgloss.Color("#42D9C8"))
	status4xx    = lipgloss.NewStyle().Foreground(lipgloss.Color("#FFE66D"))
	status5xx    = lipgloss.NewStyle().Foreground(lipgloss.Color("#FF6B9D"))
	mutedStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("#626262"))
)

// customLogWriter implements io.Writer for Echo
type customLogWriter struct{}

func (w customLogWriter) Write(p []byte) (n int, err error) {
	if LogWriter != nil {
		msg := strings.TrimSpace(string(p))
		if msg != "" {
			// Parse and colorize the log message
			LogWriter(msg)
		}
	}
	return len(p), nil
}

func Start(store *store.Store, staticFS fs.FS, tmplFS fs.FS) (*echo.Echo, error) {
	e := echo.New()
	e.HideBanner = true
	e.HidePort = true

	// Use our custom log writer if available
	if LogWriter != nil {
		e.Logger.SetOutput(customLogWriter{})
	}

	// Add version header to all responses
	e.Use(func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			c.Response().Header().Set("X-Version", GetVersionString())
			return next(c)
		}
	})

	// Add metrics middleware early to track all requests
	e.Use(MetricsMiddleware())

	// Increment request counter for UI stats
	e.Use(func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			if RequestCounter != nil {
				atomic.AddInt64(RequestCounter, 1)
			}
			return next(c)
		}
	})

	e.Use(middleware.GzipWithConfig(middleware.GzipConfig{
		Level: 5,
	}))

	// Serve static files with long-term caching
	// These files (CSS, JS, images) are versioned via their URLs or rarely change
	e.GET("/s/*", func(c echo.Context) error {
		// Set aggressive caching for static assets
		// Long cache time is safe because:
		// 1. Static files rarely change
		// 2. HTML pages are already cache-busted via version ETags
		// 3. When HTML changes, it references new/updated static files
		c.Response().Header().Set("Cache-Control", "public, max-age=86400, immutable")
		return echo.WrapHandler(http.StripPrefix("/s", http.FileServer(http.FS(staticFS))))(c)
	})

	// Custom logger middleware that routes through our UI
	e.Use(func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			err := next(c)

			if LogWriter != nil {
				req := c.Request()
				res := c.Response()

				// Format method with color
				var methodStyled string
				switch req.Method {
				case "GET":
					methodStyled = methodGET.Render(req.Method)
				case "POST":
					methodStyled = methodPOST.Render(req.Method)
				case "PUT":
					methodStyled = methodPUT.Render(req.Method)
				case "DELETE":
					methodStyled = methodDELETE.Render(req.Method)
				default:
					methodStyled = mutedStyle.Render(req.Method)
				}

				// Format status with color
				statusCode := res.Status
				var statusStyled string
				switch {
				case statusCode >= 200 && statusCode < 300:
					statusStyled = status2xx.Render(fmt.Sprintf("%d", statusCode))
				case statusCode >= 300 && statusCode < 400:
					statusStyled = status3xx.Render(fmt.Sprintf("%d", statusCode))
				case statusCode >= 400 && statusCode < 500:
					statusStyled = status4xx.Render(fmt.Sprintf("%d", statusCode))
				case statusCode >= 500:
					statusStyled = status5xx.Render(fmt.Sprintf("%d", statusCode))
				default:
					statusStyled = mutedStyle.Render(fmt.Sprintf("%d", statusCode))
				}

				// Calculate latency
				latency := c.Get("request_latency")

				// Format URI with clickable link (even when truncated)
				uri := req.RequestURI

				// Build full URL with proper scheme, host, and port
				scheme := "http"
				if req.TLS != nil {
					scheme = "https"
				}
				host := req.Host
				if host == "" {
					host = "localhost"
				}
				fullURL := fmt.Sprintf("%s://%s%s", scheme, host, uri)

				var uriStyled string
				if len(uri) > 60 {
					// Truncate but keep it clickable using ANSI hyperlink escape codes
					truncated := uri[:57] + "..."
					// Format: \e]8;;URL\e\\TEXT\e]8;;\e\\
					uriStyled = fmt.Sprintf("\x1b]8;;%s\x1b\\%s\x1b]8;;\x1b\\", fullURL, mutedStyle.Render(truncated))
				} else {
					// Make full URI clickable
					uriStyled = fmt.Sprintf("\x1b]8;;%s\x1b\\%s\x1b]8;;\x1b\\", fullURL, mutedStyle.Render(uri))
				}

				// Format duration with color coding
				var durationStyled string
				if latency != nil {
					if dur, ok := latency.(time.Duration); ok {
						ms := dur.Milliseconds()
						// Color code based on latency
						var durStyle lipgloss.Style
						if ms < 50 {
							durStyle = status2xx // Green - fast
						} else if ms < 200 {
							durStyle = status3xx // Cyan - acceptable
						} else if ms < 500 {
							durStyle = status4xx // Yellow - slow
						} else {
							durStyle = status5xx // Red - very slow
						}

						// Format duration nicely
						if ms < 1000 {
							durationStyled = durStyle.Render(fmt.Sprintf("%dms", ms))
						} else {
							durationStyled = durStyle.Render(fmt.Sprintf("%.2fs", dur.Seconds()))
						}
					} else {
						durationStyled = mutedStyle.Render(fmt.Sprintf("%v", latency))
					}
				} else {
					durationStyled = mutedStyle.Render("-")
				}

				// Log the request
				msg := fmt.Sprintf("  %s %s %s %s",
					methodStyled,
					uriStyled,
					statusStyled,
					durationStyled)

				LogWriter(msg)
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
