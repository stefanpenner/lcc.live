package server

import (
	_ "embed"
	"net/http"

	"github.com/labstack/echo/v4"
	"github.com/stefanpenner/lcc-live/web/push"
)

//go:embed sw.js
var serviceWorkerJS []byte

const webManifest = `{
  "name": "LCC Live",
  "short_name": "LCC",
  "id": "/",
  "start_url": "/",
  "scope": "/",
  "display": "standalone",
  "background_color": "#0c0d0f",
  "theme_color": "#0c0d0f",
  "icons": [
    { "src": "/s/apple-touch-icon.png", "sizes": "180x180", "type": "image/png", "purpose": "any" }
  ]
}`

// PushCanyons is the canyon id, title, and path used in a road alert.
func PushCanyons() map[string]push.Canyon {
	out := make(map[string]push.Canyon, len(canyonCatalog()))
	for _, link := range canyonCatalog() {
		out[link.ID] = push.Canyon{Title: link.Title, Path: link.Path}
	}
	return out
}

func mountPush(e *echo.Echo, n *push.Notifier, dev bool) {
	e.GET("/manifest.webmanifest", func(c echo.Context) error {
		c.Response().Header().Set("Cache-Control", "no-cache")
		return c.Blob(http.StatusOK, "application/manifest+json", []byte(webManifest))
	})
	e.GET("/sw.js", func(c echo.Context) error {
		c.Response().Header().Set("Service-Worker-Allowed", "/")
		c.Response().Header().Set("Cache-Control", "no-cache")
		return c.Blob(http.StatusOK, "application/javascript", serviceWorkerJS)
	})
	if n == nil || !n.Enabled() {
		return
	}
	e.GET("/api/push/key", func(c echo.Context) error {
		return c.JSON(http.StatusOK, map[string]string{"publicKey": n.PublicKey()})
	})
	e.POST("/api/push/subscribe", func(c echo.Context) error {
		var sub push.Subscription
		if err := c.Bind(&sub); err != nil {
			return c.NoContent(http.StatusBadRequest)
		}
		if _, ok := canyonLink(sub.Canyon); !ok || sub.Endpoint == "" || sub.P256dh == "" || sub.Auth == "" {
			return c.NoContent(http.StatusBadRequest)
		}
		if err := n.Save(sub); err != nil {
			return c.NoContent(http.StatusInternalServerError)
		}
		return c.NoContent(http.StatusNoContent)
	})
	e.DELETE("/api/push/subscribe", func(c echo.Context) error {
		var body struct {
			Endpoint string `json:"endpoint"`
		}
		if err := c.Bind(&body); err != nil || body.Endpoint == "" {
			return c.NoContent(http.StatusBadRequest)
		}
		if err := n.Remove(body.Endpoint); err != nil {
			return c.NoContent(http.StatusInternalServerError)
		}
		return c.NoContent(http.StatusNoContent)
	})
	if !dev {
		return
	}
	e.POST("/api/push/test", func(c echo.Context) error {
		var body struct {
			Canyon string `json:"canyon"`
		}
		if err := c.Bind(&body); err != nil {
			return c.NoContent(http.StatusBadRequest)
		}
		if _, ok := canyonLink(body.Canyon); !ok {
			return c.NoContent(http.StatusBadRequest)
		}
		if n.SendTest(c.Request().Context(), body.Canyon) == 0 {
			return c.NoContent(http.StatusConflict)
		}
		return c.NoContent(http.StatusNoContent)
	})
}
