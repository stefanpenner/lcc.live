package server

import (
	"io"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/stefanpenner/lcc-live/web/logger"
)

func mountDevLog(e *echo.Echo) {
	path := os.Getenv("DEV_CLIENT_LOG")
	if path == "" {
		root := os.Getenv("BUILD_WORKSPACE_DIRECTORY")
		if root == "" {
			root = "."
		}
		path = filepath.Join(root, "tmp", "client.log")
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		logger.Error(err, "Client log dir: %v", err)
		return
	}
	logger.Info("Client log: %s", path)
	var mu sync.Mutex
	e.POST("/api/dev/log", func(c echo.Context) error {
		body, err := io.ReadAll(io.LimitReader(c.Request().Body, 16*1024))
		if err != nil {
			return c.NoContent(400)
		}
		line := time.Now().Format(time.RFC3339Nano) + " " + c.RealIP() + " " + string(body) + "\n"
		mu.Lock()
		defer mu.Unlock()
		f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
		if err != nil {
			logger.Error(err, "Client log write: %v", err)
			return c.NoContent(500)
		}
		defer f.Close()
		_, _ = f.WriteString(line)
		return c.NoContent(204)
	})
}
