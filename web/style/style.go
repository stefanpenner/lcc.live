// Package style provides lipgloss styles for terminal UI rendering
package style

import "github.com/charmbracelet/lipgloss"

var (
	// Title is a style for title text
	Title = lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("#FF69B4")).
		MarginBottom(1)

	// Section is a style for section headers
	Section = lipgloss.NewStyle().
		Foreground(lipgloss.Color("#5F9EA0")).
		Bold(true)

	// File is a style for file names
	File = lipgloss.NewStyle().
		Foreground(lipgloss.Color("#98FB98"))

	// Dir is a style for directory names
	Dir = lipgloss.NewStyle().
		Foreground(lipgloss.Color("#DDA0DD")).
		Bold(true)

	// Info is a style for informational messages
	Info = lipgloss.NewStyle().
		Italic(true).
		Foreground(lipgloss.Color("#FFD700"))

	// Sync is a style for sync operation messages
	Sync = lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("#00ADD8"))

	// Cancel is a style for cancellation messages
	Cancel = lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("#FF6B6B"))

	// Method is a style for HTTP method names
	Method = lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("#00ADD8"))

	// URI is a style for URI paths
	URI = lipgloss.NewStyle().
		Foreground(lipgloss.Color("#98FB98"))

	// StatusSuccess is a style for successful status codes
	StatusSuccess = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#00FF00"))

	// StatusError is a style for error status codes
	StatusError = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#FF0000"))

	// Duration is a style for duration values
	Duration = lipgloss.NewStyle().
			Italic(true).
			Foreground(lipgloss.Color("#FFA500"))

	// Error is a style for error messages
	Error = lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("#FF0000"))

	// Success is a style for success messages
	Success = lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("#00FF00"))

	// URL is a style for URL display
	URL = lipgloss.NewStyle().
		Italic(true).
		Foreground(lipgloss.Color("#FFA500"))
)
