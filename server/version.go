package server

import (
	"fmt"
	"runtime"
	"time"
)

// Build information that should be set at compile time via ldflags
var (
	// Version is the git commit hash
	Version = "dev"
	// BuildTime is when the binary was built
	BuildTime = "unknown"
	// GoVersion is the version of Go used to build
	GoVersion = runtime.Version()
)

// VersionInfo contains information about the current build
type VersionInfo struct {
	Version   string    `json:"version"`
	BuildTime string    `json:"build_time"`
	GoVersion string    `json:"go_version"`
	Uptime    string    `json:"uptime"`
	StartTime time.Time `json:"-"`
}

var startTime = time.Now()

// GetVersionInfo returns the current version information
func GetVersionInfo() VersionInfo {
	uptime := time.Since(startTime)
	return VersionInfo{
		Version:   Version,
		BuildTime: BuildTime,
		GoVersion: GoVersion,
		Uptime:    uptime.String(),
		StartTime: startTime,
	}
}

// GetVersionString returns a short version string for headers
func GetVersionString() string {
	if Version == "dev" {
		// In dev mode, use server start time for cache busting
		// This ensures CSS cache busting works in dev mode, changing on each server restart
		return fmt.Sprintf("dev-%d-%s", startTime.Unix(), GoVersion)
	}
	return Version
}
