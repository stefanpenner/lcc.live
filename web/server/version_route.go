package server

import (
	"net/http"

	"github.com/labstack/echo/v4"
)

// VersionRoute returns version information about the service
func VersionRoute() func(c echo.Context) error {
	return func(c echo.Context) error {
		info := GetVersionInfo()
		return c.JSON(http.StatusOK, info)
	}
}
