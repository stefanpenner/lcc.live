package server

import (
	"fmt"
	"html/template"
	"io"
	"io/fs"
	"math"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/charmbracelet/lipgloss"
	sentryecho "github.com/getsentry/sentry-go/echo"
	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/stefanpenner/lcc-live/web/store"
)

// TemplateRenderer is a template renderer for Echo
type TemplateRenderer struct {
	templates *template.Template
	fs        fs.FS
	devMode   bool
	mu        sync.Mutex // Protects template reloading in dev mode
}

// Pre-compiled regexes for slugify (avoid recompiling on every call)
var (
	reSpacesUnderscores = regexp.MustCompile(`[\s_]+`)
	reNonAlphanumeric   = regexp.MustCompile(`[^a-z0-9-]`)
	reMultipleHyphens   = regexp.MustCompile(`-+`)
)

// slugify converts a camera name to a URL-safe slug
func slugify(name string) string {
	if name == "" {
		return ""
	}
	slug := strings.ToLower(name)
	slug = reSpacesUnderscores.ReplaceAllString(slug, "-")
	slug = reNonAlphanumeric.ReplaceAllString(slug, "")
	slug = reMultipleHyphens.ReplaceAllString(slug, "-")
	slug = strings.Trim(slug, "-")
	return slug
}

func formatUnixTime(timestamp int64) string {
	if timestamp == 0 {
		return "Unknown"
	}
	t := time.Unix(timestamp, 0)
	return t.Format("Jan 2, 2006 3:04 PM MST")
}

func formatTimeAgo(timestamp int64) string {
	if timestamp == 0 {
		return "unknown"
	}
	now := time.Now().Unix()
	diff := now - timestamp

	if diff < 60 {
		return "just now"
	} else if diff < 3600 {
		minutes := diff / 60
		return fmt.Sprintf("%dm", minutes)
	} else if diff < 86400 {
		hours := diff / 3600
		return fmt.Sprintf("%dh", hours)
	} else if diff < 604800 {
		days := diff / 86400
		return fmt.Sprintf("%dd", days)
	} else if diff < 31536000 {
		weeks := diff / 604800
		return fmt.Sprintf("%dw", weeks)
	} else {
		years := diff / 31536000
		return fmt.Sprintf("%dy", years)
	}
}

// isStale marks weather/road samples older than 2h as unusable for display.
// UDOT RWIS often updates on a ~15–60m cadence; 30m was too aggressive and
// hid nearly everything. Multi-day/month-old Alta ADX feeds stay hidden.
func isStale(timestamp int64) bool {
	return timestamp == 0 || time.Now().Unix()-timestamp > 7200
}

func roundTemp(temp *string) string {
	if temp == nil {
		return ""
	}
	f, err := strconv.ParseFloat(*temp, 64)
	if err != nil {
		return *temp
	}
	return strconv.Itoa(int(math.Round(f)))
}

const (
	svgSnow  = `<svg class="precip-icon" width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round"><line x1="12" y1="2" x2="12" y2="22"/><line x1="2" y1="12" x2="22" y2="12"/><line x1="5" y1="5" x2="19" y2="19"/><line x1="19" y1="5" x2="5" y2="19"/></svg>`
	svgRain  = `<svg class="precip-icon" width="14" height="14" viewBox="0 0 24 24" fill="currentColor" stroke="none"><path d="M12 2.69l5.66 5.66a8 8 0 1 1-11.31 0z"/></svg>`
	svgMixed = `<svg class="precip-icon" width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round"><line x1="6" y1="2" x2="6" y2="12"/><line x1="1" y1="7" x2="11" y2="7"/><line x1="3" y1="4" x2="9" y2="10"/><line x1="9" y1="4" x2="3" y2="10"/><path d="M17 11l4 4a5.5 5.5 0 1 1-8 0z" fill="currentColor" stroke="none"/></svg>`
)

func precipIcon(airTemp *string) template.HTML {
	if airTemp == nil {
		return template.HTML(svgRain)
	}
	temp, err := strconv.ParseFloat(*airTemp, 64)
	if err != nil {
		return template.HTML(svgRain)
	}
	if temp < 35 {
		return template.HTML(svgSnow)
	}
	if temp <= 40 {
		return template.HTML(svgMixed)
	}
	return template.HTML(svgRain)
}

func hasActiveRestriction(restriction string) bool {
	r := strings.TrimSpace(strings.ToLower(restriction))
	return r != "" && r != "none" && r != "no restrictions" && r != "n/a"
}

// eventLabel prefers Name, falls back to Description; truncates for chip UI.
func eventLabel(name, description string) string {
	label := strings.TrimSpace(name)
	if label == "" {
		label = strings.TrimSpace(description)
	}
	const max = 40
	if len(label) <= max {
		return label
	}
	// Prefer rune-safe truncate
	r := []rune(label)
	if len(r) <= max {
		return label
	}
	return string(r[:max-1]) + "…"
}

func eventIsWarn(isFullClosure bool, severity string) bool {
	if isFullClosure {
		return true
	}
	s := strings.ToLower(strings.TrimSpace(severity))
	return s == "high" || s == "severe" || s == "critical" || s == "major"
}

func uacDangerClass(level int, danger string) string {
	d := strings.ToLower(strings.TrimSpace(danger))
	if level <= 0 && (d == "" || d == "no rating" || d == "none" || d == "n/a") {
		return "uac-none"
	}
	switch level {
	case 1:
		return "uac-low"
	case 2:
		return "uac-moderate"
	case 3:
		return "uac-considerable"
	case 4, 5:
		return "uac-high"
	default:
		// Fall back on danger string
		switch d {
		case "low":
			return "uac-low"
		case "moderate":
			return "uac-moderate"
		case "considerable":
			return "uac-considerable"
		case "high", "extreme":
			return "uac-high"
		default:
			return "uac-none"
		}
	}
}

func uacDangerLabel(danger string) string {
	d := strings.TrimSpace(danger)
	if d == "" {
		return "No rating"
	}
	// Title-case common values lightly
	if strings.EqualFold(d, "no rating") {
		return "No rating"
	}
	if len(d) == 0 {
		return "No rating"
	}
	// Capitalize first letter of each word
	parts := strings.Fields(d)
	for i, p := range parts {
		if p == "" {
			continue
		}
		parts[i] = strings.ToUpper(p[:1]) + strings.ToLower(p[1:])
	}
	return strings.Join(parts, " ")
}

func altaParkingWarn(status string) bool {
	s := strings.ToLower(strings.TrimSpace(status))
	return s == "full" || s == "closed" || s == "limited" || strings.Contains(s, "full") || strings.Contains(s, "closed")
}

var templateFuncs = template.FuncMap{
	"slugify":              slugify,
	"formatUnixTime":       formatUnixTime,
	"formatTimeAgo":        formatTimeAgo,
	"isStale":              isStale,
	"roundTemp":            roundTemp,
	"precipIcon":           precipIcon,
	"version":              GetVersionString,
	"hasActiveRestriction": hasActiveRestriction,
	"eventLabel":           eventLabel,
	"eventIsWarn":          eventIsWarn,
	"uacDangerClass":       uacDangerClass,
	"uacDangerLabel":       uacDangerLabel,
	"altaParkingWarn":      altaParkingWarn,
	"sub":                  func(a, b int) int { return a - b },
}

// Render renders a template with the given data
func (t *TemplateRenderer) Render(w io.Writer, name string, data interface{}, _ echo.Context) error {
	// In dev mode, reload templates on every request for hot reloading
	if t.devMode {
		t.mu.Lock()
		defer t.mu.Unlock()

		tmpl, err := template.New("").Funcs(templateFuncs).ParseFS(t.fs, "*.html.tmpl")
		if err != nil {
			return err
		}
		return tmpl.ExecuteTemplate(w, name, data)
	}
	return t.templates.ExecuteTemplate(w, name, data)
}

// LogWriter is used to capture Echo logs and route them through our logger
var LogWriter func(string)

// RequestCounter tracks total requests (for UI stats)
var RequestCounter *int64

// ErrorCounter tracks error requests (status >= 400) for UI stats
var ErrorCounter *int64

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
			LogWriter(msg)
		}
	}
	return len(p), nil
}

// ServerConfig holds configuration for starting the HTTP server
type ServerConfig struct {
	Store         *store.Store
	StaticFS      fs.FS
	TemplateFS    fs.FS
	DevMode       bool
	SentryEnabled bool
}

// Start serves canyon pages, camera images, and health.
func Start(cfg ServerConfig) (*echo.Echo, error) {
	e := echo.New()
	e.HideBanner = true
	e.HidePort = true

	warnErrorLogger()
	if LogWriter != nil {
		e.Logger.SetOutput(customLogWriter{})
	}

	// First, so it wraps every later middleware.
	usePanicGuard(e, cfg.SentryEnabled)
	e.Use(securityHeaders(cfg.DevMode))
	e.Use(middleware.TimeoutWithConfig(middleware.TimeoutConfig{
		Timeout: 30 * time.Second,
	}))
	e.Use(versionHeader)
	e.Use(MetricsMiddleware())
	e.Use(countRequests)
	e.Use(middleware.GzipWithConfig(middleware.GzipConfig{
		Level: 5,
	}))
	e.GET("/s/*", staticHandler(cfg.StaticFS, cfg.DevMode))
	e.Use(requestLog)

	if err := mountTemplates(e, cfg.TemplateFS, cfg.DevMode); err != nil {
		return nil, err
	}
	if cfg.DevMode {
		e.Use(devNoCache)
	}

	mountPublic(e, cfg.Store)
	mountInternal(e)
	return e, nil
}

func warnErrorLogger() {
	err := InitErrorLogger("")
	if err != nil && LogWriter != nil {
		LogWriter(fmt.Sprintf("Warning: Failed to initialize error logger: %v", err))
	}
}

func usePanicGuard(e *echo.Echo, sentryEnabled bool) {
	if sentryEnabled {
		e.Use(sentryecho.New(sentryecho.Options{
			Repanic: true,
		}))
		return
	}
	e.Use(middleware.RecoverWithConfig(middleware.RecoverConfig{
		DisableStackAll:   false,
		DisablePrintStack: false,
		StackSize:         4 << 10, // 4 KB
		LogLevel:          0,       // Log all panics
	}))
}

func securityHeaders(devMode bool) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			h := c.Response().Header()
			h.Set("X-Content-Type-Options", "nosniff")
			h.Set("X-Frame-Options", "DENY")
			h.Set("Referrer-Policy", "strict-origin-when-cross-origin")
			h.Set("Permissions-Policy", "camera=(), microphone=(), geolocation=()")
			if !devMode {
				h.Set("Strict-Transport-Security", "max-age=31536000; includeSubDomains")
			}
			return next(c)
		}
	}
}

func versionHeader(next echo.HandlerFunc) echo.HandlerFunc {
	return func(c echo.Context) error {
		c.Response().Header().Set("X-Version", GetVersionString())
		return next(c)
	}
}

func countRequests(next echo.HandlerFunc) echo.HandlerFunc {
	return func(c echo.Context) error {
		if RequestCounter != nil {
			atomic.AddInt64(RequestCounter, 1)
		}
		start := time.Now()
		err := next(c)
		status := c.Response().Status
		if ErrorCounter != nil && status >= 400 {
			atomic.AddInt64(ErrorCounter, 1)
			LogError(
				status,
				c.Request().Method,
				c.Path(),
				c.Request().URL.String(),
				c.RealIP(),
				c.Request().UserAgent(),
				time.Since(start),
				err,
			)
		}
		return err
	}
}

func staticHandler(staticFS fs.FS, devMode bool) echo.HandlerFunc {
	files := echo.WrapHandler(http.StripPrefix("/s", http.FileServer(http.FS(staticFS))))
	return func(c echo.Context) error {
		h := c.Response().Header()
		if devMode {
			h.Set("Cache-Control", "no-cache, no-store, must-revalidate")
			h.Set("Pragma", "no-cache")
			h.Set("Expires", "0")
		} else {
			// Not content-hashed. HTML etags move on deploy; these files rarely change.
			h.Set("Cache-Control", "public, max-age=86400, immutable")
		}
		return files(c)
	}
}

func requestLog(next echo.HandlerFunc) echo.HandlerFunc {
	return func(c echo.Context) error {
		err := next(c)
		if LogWriter == nil {
			return err
		}

		req := c.Request()
		line := fmt.Sprintf("  %s %s %s %s",
			styledMethod(req.Method),
			styledURI(req),
			styledStatus(c.Response().Status),
			styledDuration(c.Get("request_latency")),
		)
		LogWriter(line)
		return err
	}
}

func styledMethod(method string) string {
	switch method {
	case "GET": //nolint:goconst // HTTP method string used for readability
		return methodGET.Render(method)
	case "POST":
		return methodPOST.Render(method)
	case "PUT":
		return methodPUT.Render(method)
	case "DELETE":
		return methodDELETE.Render(method)
	default:
		return mutedStyle.Render(method)
	}
}

func styledStatus(code int) string {
	text := fmt.Sprintf("%d", code)
	switch {
	case code >= 200 && code < 300:
		return status2xx.Render(text)
	case code >= 300 && code < 400:
		return status3xx.Render(text)
	case code >= 400 && code < 500:
		return status4xx.Render(text)
	case code >= 500:
		return status5xx.Render(text)
	default:
		return mutedStyle.Render(text)
	}
}

func styledURI(req *http.Request) string {
	uri := req.RequestURI
	scheme := "http"
	if req.TLS != nil {
		scheme = "https"
	}
	host := req.Host
	if host == "" {
		host = "localhost"
	}
	fullURL := fmt.Sprintf("%s://%s%s", scheme, host, uri)

	text := uri
	if len(uri) > 60 {
		text = uri[:57] + "..."
	}
	return fmt.Sprintf("\x1b]8;;%s\x1b\\%s\x1b]8;;\x1b\\", fullURL, mutedStyle.Render(text))
}

func styledDuration(latency interface{}) string {
	if latency == nil {
		return mutedStyle.Render("-")
	}
	dur, ok := latency.(time.Duration)
	if !ok {
		return mutedStyle.Render(fmt.Sprintf("%v", latency))
	}

	ms := dur.Milliseconds()
	var durStyle lipgloss.Style
	switch {
	case ms < 50:
		durStyle = status2xx
	case ms < 200:
		durStyle = status3xx
	case ms < 500:
		durStyle = status4xx
	default:
		durStyle = status5xx
	}
	if ms < 1000 {
		return durStyle.Render(fmt.Sprintf("%dms", ms))
	}
	return durStyle.Render(fmt.Sprintf("%.2fs", dur.Seconds()))
}

func mountTemplates(e *echo.Echo, templateFS fs.FS, devMode bool) error {
	tmpl, err := template.New("").Funcs(templateFuncs).ParseFS(templateFS, "*.html.tmpl")
	if err != nil {
		return err
	}
	e.Renderer = &TemplateRenderer{
		templates: tmpl,
		fs:        templateFS,
		devMode:   devMode,
	}
	return nil
}

func devNoCache(next echo.HandlerFunc) echo.HandlerFunc {
	return func(c echo.Context) error {
		c.Set("_dev_mode", true)

		h := c.Response().Header()
		h.Set("Cache-Control", "no-cache, no-store, must-revalidate, private")
		h.Set("Pragma", "no-cache")
		h.Set("Expires", "0")
		h.Set("Vary", "*")
		return next(c)
	}
}

// CanyonLink is one entry in the canyon switcher.
type CanyonLink struct {
	ID, Label, Path, Title string
}

func canyonCatalog() []CanyonLink {
	return []CanyonLink{
		{ID: "LCC", Label: "LCC", Path: "/", Title: "Little Cottonwood Canyon"},
		{ID: "BCC", Label: "BCC", Path: "/bcc", Title: "Big Cottonwood Canyon"},
		{ID: "Provo", Label: "Provo", Path: "/provo", Title: "Provo Canyon"},
		{ID: "AFC", Label: "AF", Path: "/afc", Title: "American Fork Canyon"},
		{ID: "Parleys", Label: "Parleys", Path: "/parleys", Title: "Parleys Canyon"},
	}
}

func canyonLink(id string) (CanyonLink, bool) {
	for _, link := range canyonCatalog() {
		if link.ID == id {
			return link, true
		}
	}
	return CanyonLink{}, false
}

func mountPublic(e *echo.Echo, live *store.Store) {
	for _, link := range canyonCatalog() {
		mountCanyon(e, live, link.Path, link.ID)
		if link.Path == "/" {
			mountCanyon(e, live, "/.json", link.ID)
			mountCanyon(e, live, "/lcc", link.ID)
			mountCanyon(e, live, "/lcc.json", link.ID)
			continue
		}
		mountCanyon(e, live, link.Path+".json", link.ID)
	}

	e.GET("/image/:id", ImageRoute(live))
	e.HEAD("/image/:id", ImageRoute(live))

	e.GET("/camera/*", CameraRoute(live))
	e.HEAD("/camera/*", CameraRoute(live))

	e.GET("/api/canyon/:canyon/udot", UDOTRoute(live))
	e.GET("/healthcheck", HealthCheckRoute(live))
}

func mountCanyon(e *echo.Echo, live *store.Store, path, id string) {
	e.GET(path, CanyonRoute(live, id))
	e.HEAD(path, CanyonRoute(live, id))
}

func mountInternal(e *echo.Echo) {
	internal := e.Group("/_")
	internal.Use(noStore)
	internal.GET("/version", VersionRoute())
	internal.GET("/metrics", echo.WrapHandler(promhttp.Handler()))
}

func noStore(next echo.HandlerFunc) echo.HandlerFunc {
	return func(c echo.Context) error {
		h := c.Response().Header()
		h.Set("Cache-Control", "no-store, no-cache, must-revalidate, private, max-age=0")
		h.Set("Pragma", "no-cache")
		h.Set("Expires", "0")
		return next(c)
	}
}
