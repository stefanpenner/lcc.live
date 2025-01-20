package style

import "github.com/charmbracelet/lipgloss"

var (
	// Style definitions
	Title = lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("#FF69B4")).
		MarginBottom(1)

	Section = lipgloss.NewStyle().
		Foreground(lipgloss.Color("#5F9EA0")).
		Bold(true)

	File = lipgloss.NewStyle().
		Foreground(lipgloss.Color("#98FB98"))

	Dir = lipgloss.NewStyle().
		Foreground(lipgloss.Color("#DDA0DD")).
		Bold(true)

	Info = lipgloss.NewStyle().
		Italic(true).
		Foreground(lipgloss.Color("#FFD700"))

	Sync = lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("#00ADD8"))

	Cancel = lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("#FF6B6B"))

	Method = lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("#00ADD8"))

	URI = lipgloss.NewStyle().
		Foreground(lipgloss.Color("#98FB98"))

	StatusSuccess = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#00FF00"))

	StatusError = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#FF0000"))

	Duration = lipgloss.NewStyle().
			Italic(true).
			Foreground(lipgloss.Color("#FFA500"))

	Error = lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("#FF0000"))

	Success = lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("#00FF00"))

	URL = lipgloss.NewStyle().
		Italic(true).
		Foreground(lipgloss.Color("#FFA500"))
)
