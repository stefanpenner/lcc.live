// Package logger provides structured logging with styled output
package logger

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/log"
)

var useUI bool

// SetUIMode enables UI mode (logs go to TUI instead of stdout)
func SetUIMode(enabled bool) {
	useUI = enabled
}

var (
	// Box drawing characters for clean borders
	horizontalLine = "─"
	verticalLine   = "│"
	topLeft        = "┌"
	topRight       = "┐"
	bottomLeft     = "└"
	bottomRight    = "┘"
	leftT          = "├"
	rightT         = "┤"

	// Charm color palette - professional and cohesive
	charmPink   = lipgloss.Color("#FF69B4") // Charm's signature pink
	charmCyan   = lipgloss.Color("#42D9C8") // Bright cyan
	charmGreen  = lipgloss.Color("#73F59F") // Success green
	charmYellow = lipgloss.Color("#FFE66D") // Warning yellow
	charmRed    = lipgloss.Color("#FF6B9D") // Error pink-red
	charmPurple = lipgloss.Color("#B794F6") // Accent purple
	charmGray   = lipgloss.Color("#626262") // Muted gray
	charmWhite  = lipgloss.Color("#ECEFF4") // Clean white

	// Styles
	titleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(charmPink).
			MarginTop(1)

	headerStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(charmCyan)

	infoStyle = lipgloss.NewStyle().
			Foreground(charmWhite)

	warnStyle = lipgloss.NewStyle().
			Foreground(charmYellow)

	errorStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(charmRed)

	successStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(charmGreen)

	mutedStyle = lipgloss.NewStyle().
			Foreground(charmGray)

	keyStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(charmPurple)

	valueStyle = lipgloss.NewStyle().
			Foreground(charmCyan)

	borderStyle = lipgloss.NewStyle().
			Foreground(charmPink)

	// Structured logger for HTTP requests
	httpLogger *log.Logger
)

func init() {
	// Initialize HTTP logger with Charm's log
	httpLogger = log.NewWithOptions(os.Stdout, log.Options{
		ReportTimestamp: true,
		TimeFormat:      "15:04:05",
		Prefix:          "🌐 ",
	})
	httpLogger.SetLevel(log.InfoLevel)
	// Use a more subtle style for HTTP logs
	styles := log.DefaultStyles()
	styles.Levels[log.InfoLevel] = lipgloss.NewStyle().
		Foreground(charmGray)
	styles.Keys["method"] = lipgloss.NewStyle().
		Foreground(charmCyan).
		Bold(true)
	styles.Values["method"] = lipgloss.NewStyle().
		Foreground(charmCyan)
	httpLogger.SetStyles(styles)
}

// PrintBanner displays the startup banner
func PrintBanner(version, buildTime string) {
	width := 62

	// Create gradient effect with box drawing
	topBorder := borderStyle.Render(
		topLeft + strings.Repeat(horizontalLine, width-2) + topRight,
	)
	fmt.Println(topBorder)

	// Title with better centering
	title := "🌄  LCC.LIVE Camera Service"
	titleRendered := titleStyle.Render(title)
	titleWidth := lipgloss.Width(title)
	leftPad := (width - titleWidth - 2) / 2
	rightPad := width - titleWidth - leftPad - 2

	fmt.Print(borderStyle.Render(verticalLine))
	fmt.Print(strings.Repeat(" ", leftPad))
	fmt.Print(titleRendered)
	fmt.Print(strings.Repeat(" ", rightPad))
	fmt.Println(borderStyle.Render(verticalLine))

	// Separator
	fmt.Println(borderStyle.Render(leftT + strings.Repeat(horizontalLine, width-2) + rightT))

	// Info lines with better formatting
	printInfoLine("Version", version, width)
	if buildTime != "" {
		printInfoLine("Built", buildTime, width)
	}
	printInfoLine("URL", "https://lcc.live", width)

	// Bottom border
	fmt.Println(borderStyle.Render(bottomLeft + strings.Repeat(horizontalLine, width-2) + bottomRight))
	fmt.Println()
}

func printInfoLine(key, value string, width int) {
	keyRendered := keyStyle.Render(key + ":")
	valueRendered := valueStyle.Render(value)
	// Account for ANSI codes in width calculation
	lineWidth := 2 + lipgloss.Width(key+":") + 1 + lipgloss.Width(value)
	padding := width - lineWidth - 2
	if padding < 0 {
		padding = 0
	}
	fmt.Print(borderStyle.Render(verticalLine))
	fmt.Print("  ")
	fmt.Print(keyRendered)
	fmt.Print(" ")
	fmt.Print(valueRendered)
	fmt.Print(strings.Repeat(" ", padding))
	fmt.Println(borderStyle.Render(verticalLine))
}

// Section prints a section header with a decorative divider
func Section(title string) {
	fmt.Println()
	divider := mutedStyle.Render("━━━━")
	header := headerStyle.Render("▸ " + title)
	fmt.Printf("%s %s\n", divider, header)
}

// Log is the interface for sending logs (will be set by main if using UI)
var Log func(string)

func logOrPrint(msg string) {
	if Log != nil && useUI {
		Log(msg)
	} else {
		fmt.Println(msg)
	}
}

// Info prints an info message
func Info(format string, args ...interface{}) {
	msg := fmt.Sprintf(format, args...)
	logOrPrint(infoStyle.Render("  " + msg))
}

// Success prints a success message
func Success(format string, args ...interface{}) {
	msg := fmt.Sprintf(format, args...)
	logOrPrint(successStyle.Render("  ✓ " + msg))
}

// Warn prints a warning message
func Warn(format string, args ...interface{}) {
	msg := fmt.Sprintf(format, args...)
	logOrPrint(warnStyle.Render("  ⚠ " + msg))
}

// Error logs an error message. If the first argument is an error, it will be sent to Sentry.
// Usage:
//   logger.Error("something went wrong")
//   logger.Error(err)  // logs error and sends to Sentry
//   logger.Error(err, "failed to load: %v", err)  // logs formatted message and sends to Sentry
func Error(args ...interface{}) {
	var err error
	var msg string

	// Check if first argument is an error
	if len(args) > 0 {
		if e, ok := args[0].(error); ok {
			err = e
			// If there are more args, use them as format string
			if len(args) > 1 {
				format, ok := args[1].(string)
				if ok {
					msg = fmt.Sprintf(format, args[2:]...)
				} else {
					msg = fmt.Sprintf("%v", err)
				}
			} else {
				msg = fmt.Sprintf("%v", err)
			}
		} else {
			// First arg is not an error, treat all as format string
			format, ok := args[0].(string)
			if ok && len(args) > 1 {
				msg = fmt.Sprintf(format, args[1:]...)
			} else {
				msg = fmt.Sprintf("%v", args[0])
			}
		}
	}

	// Log the error nicely
	logOrPrint(errorStyle.Render("  ✗ " + msg))

	// Send to Sentry if error was provided and Sentry is configured
	if err != nil && captureException != nil {
		captureException(err)
	}
}

// Fatal logs an error message and exits the program. If an error is provided, it will be sent to Sentry.
// Usage:
//   logger.Fatal("critical error occurred")
//   logger.Fatal(err)  // logs error, sends to Sentry, and exits
//   logger.Fatal(err, "failed to start: %v", err)  // logs formatted message, sends to Sentry, and exits
func Fatal(args ...interface{}) {
	var err error
	var msg string

	// Check if first argument is an error
	if len(args) > 0 {
		if e, ok := args[0].(error); ok {
			err = e
			// If there are more args, use them as format string
			if len(args) > 1 {
				format, ok := args[1].(string)
				if ok {
					msg = fmt.Sprintf(format, args[2:]...)
				} else {
					msg = fmt.Sprintf("%v", err)
				}
			} else {
				msg = fmt.Sprintf("%v", err)
			}
		} else {
			// First arg is not an error, treat all as format string
			format, ok := args[0].(string)
			if ok && len(args) > 1 {
				msg = fmt.Sprintf(format, args[1:]...)
			} else {
				msg = fmt.Sprintf("%v", args[0])
			}
		}
	}

	// Log the error nicely
	logOrPrint(errorStyle.Render("  ✗ " + msg))

	// Send to Sentry if error was provided and Sentry is configured
	if err != nil && captureException != nil {
		captureException(err)
	}

	// Exit the program
	os.Exit(1)
}

// captureException is a function pointer that can be set to capture exceptions
// This allows us to avoid importing sentry-go in the logger package
// The function signature matches sentry.CaptureException which returns *sentry.EventID
var captureException func(error) interface{}

// SetSentryCaptureException sets the function to use for capturing exceptions to Sentry
func SetSentryCaptureException(fn func(error) interface{}) {
	captureException = fn
}

// Muted prints a muted/debug message
func Muted(format string, args ...interface{}) {
	msg := fmt.Sprintf(format, args...)
	logOrPrint(mutedStyle.Render("  " + msg))
}

// FetchSummary prints a summary of image fetch results
type FetchSummary struct {
	Duration  time.Duration
	Changed   int
	Unchanged int
	Errors    int
	Total     int
}

// Print displays a formatted summary of the fetch operation
func (f FetchSummary) Print() {
	duration := f.Duration.Round(time.Millisecond)
	total := f.Changed + f.Unchanged + f.Errors

	// Determine overall status with visual indicators
	var icon string
	var statusStyle lipgloss.Style
	switch {
	case f.Errors == 0:
		icon = "✓"
		statusStyle = successStyle
	case f.Errors < total/2:
		icon = "⚠"
		statusStyle = warnStyle
	default:
		icon = "✗"
		statusStyle = errorStyle
	}

	// Create a nicely formatted summary with color-coded numbers
	iconRendered := statusStyle.Render(icon)
	durationRendered := mutedStyle.Render(fmt.Sprintf("(%v)", duration))
	changedRendered := successStyle.Render(fmt.Sprintf("%d", f.Changed))
	unchangedRendered := mutedStyle.Render(fmt.Sprintf("%d", f.Unchanged))

	summary := fmt.Sprintf("  %s Sync complete %s • %s changed • %s unchanged",
		iconRendered, durationRendered, changedRendered, unchangedRendered)

	if f.Errors > 0 {
		errorsRendered := errorStyle.Render(fmt.Sprintf("%d", f.Errors))
		summary += fmt.Sprintf(" • %s errors", errorsRendered)
	}

	logOrPrint(summary)
}

// ServerInfo prints server startup information
type ServerInfo struct {
	Port         string
	SyncInterval time.Duration
	Cameras      int
}

// Print displays formatted server configuration information
func (s ServerInfo) Print() {
	Section("Configuration")
	// Format with icons and color-coded values
	portIcon := "🔌"
	timerIcon := "⏱"
	cameraIcon := "📷"

	fmt.Printf("  %s %s %s\n",
		mutedStyle.Render(portIcon),
		keyStyle.Render("Port:"),
		valueStyle.Render(s.Port))
	fmt.Printf("  %s %s %s\n",
		mutedStyle.Render(timerIcon),
		keyStyle.Render("Sync:"),
		valueStyle.Render(s.SyncInterval.String()))
	fmt.Printf("  %s %s %s\n",
		mutedStyle.Render(cameraIcon),
		keyStyle.Render("Cameras:"),
		valueStyle.Render(fmt.Sprintf("%d", s.Cameras)))
}

// Shutdown prints shutdown message
func Shutdown() {
	fmt.Println()
	shutdownMsg := lipgloss.NewStyle().
		Foreground(charmYellow).
		Bold(true).
		Render("  ⏸  Shutting down gracefully...")
	fmt.Println(shutdownMsg)
}

// HTTPLogger returns the configured HTTP logger for middleware
func HTTPLogger() *log.Logger {
	return httpLogger
}
