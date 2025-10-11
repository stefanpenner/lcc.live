package ui

import (
	"context"
	"fmt"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/mattn/go-isatty"
)

// Colors
var (
	pink   = lipgloss.Color("#FF69B4")
	cyan   = lipgloss.Color("#42D9C8")
	green  = lipgloss.Color("#73F59F")
	red    = lipgloss.Color("#FF6B9D")
	orange = lipgloss.Color("#FF9F43")
	gray   = lipgloss.Color("#626262")
)

// Styles
var (
	hudStyle = lipgloss.NewStyle().
			BorderStyle(lipgloss.RoundedBorder()).
			BorderForeground(pink).
			Padding(0, 1)

	titleStyle   = lipgloss.NewStyle().Bold(true).Foreground(pink)
	valueStyle   = lipgloss.NewStyle().Foreground(cyan)
	statStyle    = lipgloss.NewStyle().Foreground(green)
	errorStyle   = lipgloss.NewStyle().Foreground(red)
	warningStyle = lipgloss.NewStyle().Foreground(orange)
	mutedStyle   = lipgloss.NewStyle().Foreground(gray)
	helpStyle    = lipgloss.NewStyle().Foreground(gray).Italic(true).PaddingLeft(1)
)

type Stats struct {
	Cameras         int
	LastSyncTime    time.Time
	SyncDuration    time.Duration
	Changed         int
	Unchanged       int
	Errors          int
	TotalSyncs      int
	RequestsTotal   int
	RequestsPerSec  float64
	MemoryUsageMB   float64
	CPUUsagePercent float64
	GoroutineCount  int
}

type model struct {
	viewport  viewport.Model
	logs      []string
	stats     Stats
	version   string
	port      string
	startTime time.Time
	ready     bool
	width     int
	height    int
}

var (
	globalModel  *model
	program      *tea.Program
	uiEnabled    bool
	shutdownCtx  context.Context
	shutdownFunc context.CancelFunc
	shutdownOnce sync.Once
)

const (
	maxLogs     = 1000
	avgLogChars = 100
)

// IsTTY checks if stdout is a terminal
func IsTTY() bool {
	return isatty.IsTerminal(os.Stdout.Fd()) || isatty.IsCygwinTerminal(os.Stdout.Fd())
}

// Initialize starts the TUI (only if TTY is available)
func Initialize(version, buildTime, port string, syncInterval time.Duration, cameras int) bool {
	if !IsTTY() {
		return false
	}

	uiEnabled = true
	shutdownCtx, shutdownFunc = context.WithCancel(context.Background())

	globalModel = &model{
		version:   version,
		port:      port,
		startTime: time.Now(),
		stats:     Stats{Cameras: cameras},
		logs:      make([]string, 0, maxLogs),
	}

	program = tea.NewProgram(globalModel, tea.WithAltScreen())

	go func() { program.Run() }()

	time.Sleep(100 * time.Millisecond)
	go startMetricsUpdater(shutdownCtx)

	return true
}

func startMetricsUpdater(ctx context.Context) {
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if program != nil {
				program.Send(tickMsg{})
			}
		}
	}
}

// AddLog adds a log line to the scrolling area (or prints to stdout if no TUI)
func AddLog(msg string) {
	if !uiEnabled {
		fmt.Println(msg)
		return
	}
	if program != nil {
		program.Send(logMsg{msg})
	}
}

// UpdateStats updates the stats in the HUD
func UpdateStats(stats Stats) {
	if uiEnabled && program != nil {
		program.Send(statsMsg{stats})
	}
}

// SetReady marks the server as ready
func SetReady() {
	if uiEnabled && program != nil {
		program.Send(readyMsg{})
	}
}

// Shutdown stops the TUI
func Shutdown() {
	if !uiEnabled {
		return
	}

	shutdownOnce.Do(func() {
		if shutdownFunc != nil {
			shutdownFunc()
		}
		if program != nil {
			program.Quit()
			time.Sleep(100 * time.Millisecond)
		}
		program = nil
		globalModel = nil
	})
}

// Messages
type (
	logMsg   struct{ msg string }
	statsMsg struct{ stats Stats }
	readyMsg struct{}
	tickMsg  struct{}
)

func (m *model) Init() tea.Cmd {
	return nil
}

func (m *model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd

	switch msg := msg.(type) {
	case tea.KeyMsg:
		if msg.String() == "ctrl+c" || msg.String() == "q" {
			return m, tea.Quit
		}

	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		viewportHeight := m.height - 12 // HUD + separator + footer

		if !m.ready {
			m.viewport = viewport.New(msg.Width, viewportHeight)
			m.viewport.YPosition = 10
			m.ready = true
		} else {
			m.viewport.Width = msg.Width
			m.viewport.Height = viewportHeight
		}

		if len(m.logs) > 0 {
			m.viewport.SetContent(m.buildLogContent())
			m.viewport.GotoBottom()
		}

	case logMsg:
		m.logs = append(m.logs, msg.msg)
		if len(m.logs) > maxLogs {
			copy(m.logs, m.logs[len(m.logs)-maxLogs:])
			m.logs = m.logs[:maxLogs]
		}
		m.viewport.SetContent(m.buildLogContent())
		m.viewport.GotoBottom()

	case statsMsg:
		m.stats = msg.stats

	case readyMsg, tickMsg:
		// Trigger re-render
	}

	m.viewport, cmd = m.viewport.Update(msg)
	return m, cmd
}

func (m *model) View() string {
	if !m.ready {
		spinner := []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}
		frame := int(time.Since(m.startTime).Milliseconds()/100) % len(spinner)
		loading := titleStyle.Render(spinner[frame] + " Initializing LCC.LIVE...")
		return lipgloss.NewStyle().Padding(2).Render(loading)
	}

	return lipgloss.JoinVertical(lipgloss.Left,
		m.renderHUD(),
		mutedStyle.Bold(true).Render(strings.Repeat("─", m.width)),
		m.viewport.View(),
		m.renderFooter(),
	)
}

func (m *model) buildLogContent() string {
	if len(m.logs) == 0 {
		return ""
	}

	var b strings.Builder
	b.Grow(len(m.logs) * avgLogChars)
	for i, log := range m.logs {
		if i > 0 {
			b.WriteByte('\n')
		}
		b.WriteString(log)
	}
	return b.String()
}

func (m *model) renderHUD() string {
	uptime := formatDuration(time.Since(m.startTime))

	rows := []string{
		fmt.Sprintf("%s %s  %s",
			titleStyle.Render("🌄 LCC.LIVE"),
			mutedStyle.Render("v"+m.version),
			mutedStyle.Render("⏱ "+uptime)),

		fmt.Sprintf("%s %s  %s %s",
			mutedStyle.Render("🔌"), valueStyle.Render(m.port),
			mutedStyle.Render("🌐"), mutedStyle.Render("http://localhost:"+m.port)),

		fmt.Sprintf("%s %s  %s %s",
			mutedStyle.Render("📷"), statStyle.Render(fmt.Sprintf("%d", m.stats.Cameras)),
			mutedStyle.Render("🔄"), statStyle.Render(fmt.Sprintf("%d", m.stats.TotalSyncs))),

		m.renderSyncInfo(),
		m.renderPerfMetrics(),
	}

	return hudStyle.Render(lipgloss.JoinVertical(lipgloss.Left, rows...))
}

func (m *model) renderSyncInfo() string {
	if m.stats.LastSyncTime.IsZero() {
		return mutedStyle.Render("⏱ Waiting for first sync...")
	}

	elapsed := time.Since(m.stats.LastSyncTime)
	timeAgo := formatTimeAgo(elapsed)

	changed := colorizeIfNonZero(m.stats.Changed, statStyle)
	unchanged := mutedStyle.Render(fmt.Sprintf("%d", m.stats.Unchanged))
	status := colorizeErrors(m.stats.Errors)

	return fmt.Sprintf("%s %s • %s↑ %s→ %s",
		mutedStyle.Render("⏱"), mutedStyle.Render(timeAgo),
		changed, unchanged, status)
}

func (m *model) renderPerfMetrics() string {
	if m.stats.RequestsTotal == 0 {
		return mutedStyle.Render("📊 No requests yet")
	}

	reqTotal := statStyle.Render(fmt.Sprintf("%d", m.stats.RequestsTotal))
	reqRate := colorizeRate(m.stats.RequestsPerSec)
	memory := colorizeMemory(m.stats.MemoryUsageMB)
	memBar := renderMemBar(m.stats.MemoryUsageMB)
	cpu := statStyle.Render(fmt.Sprintf("%.1f%%", m.stats.CPUUsagePercent))
	goroutines := colorizeGoroutines(m.stats.GoroutineCount)

	return fmt.Sprintf("%s %s (%s)  %s %s %s  %s %s  %s %s",
		mutedStyle.Render("📊"), reqTotal, reqRate,
		mutedStyle.Render("💾"), memory, memBar,
		mutedStyle.Render("⚡"), cpu,
		mutedStyle.Render("🔀"), goroutines)
}

func (m *model) renderFooter() string {
	scrollPos := ""
	if len(m.logs) > 0 && m.viewport.TotalLineCount() > m.viewport.Height {
		pct := int(float64(m.viewport.YOffset) / float64(m.viewport.TotalLineCount()-m.viewport.Height) * 100)
		if pct > 100 {
			pct = 100
		}
		scrollPos = fmt.Sprintf("(%d%%)", pct)
	}
	return helpStyle.Render(fmt.Sprintf("↑↓ scroll %s • q/ctrl+c quit", scrollPos))
}

// Helper functions
func colorizeIfNonZero(val int, style lipgloss.Style) string {
	if val > 0 {
		return style.Render(fmt.Sprintf("%d", val))
	}
	return mutedStyle.Render("0")
}

func colorizeErrors(errors int) string {
	if errors > 0 {
		return errorStyle.Render(fmt.Sprintf("%d ⚠", errors))
	}
	return mutedStyle.Render("0") + " " + statStyle.Render("✓")
}

func colorizeRate(rate float64) string {
	style := statStyle
	if rate > 100 {
		style = errorStyle
	} else if rate > 50 {
		style = warningStyle
	}
	return style.Render(fmt.Sprintf("%.1f/s", rate))
}

func colorizeMemory(memMB float64) string {
	if memMB > 1024 {
		gb := memMB / 1024
		style := statStyle
		if gb > 2 {
			style = errorStyle
		} else if gb > 1 {
			style = warningStyle
		}
		return style.Render(fmt.Sprintf("%.1fGB", gb))
	}

	style := statStyle
	if memMB > 500 {
		style = warningStyle
	}
	return style.Render(fmt.Sprintf("%.0fMB", memMB))
}

func renderMemBar(memMB float64) string {
	barLen := int(memMB / 102.4) // Each char = ~100MB
	if barLen > 10 {
		barLen = 10
	}
	return mutedStyle.Render("[" + strings.Repeat("▓", barLen) + strings.Repeat("░", 10-barLen) + "]")
}

func colorizeGoroutines(count int) string {
	style := mutedStyle
	if count > 1000 {
		style = errorStyle
	} else if count > 500 {
		style = warningStyle
	}
	return style.Render(fmt.Sprintf("%d", count))
}

func formatDuration(d time.Duration) string {
	d = d.Round(time.Second)
	h := d / time.Hour
	d -= h * time.Hour
	m := d / time.Minute
	d -= m * time.Minute
	s := d / time.Second

	if h > 0 {
		return fmt.Sprintf("%dh%dm%ds", h, m, s)
	}
	if m > 0 {
		return fmt.Sprintf("%dm%ds", m, s)
	}
	return fmt.Sprintf("%ds", s)
}

func formatTimeAgo(d time.Duration) string {
	if d < time.Minute {
		return fmt.Sprintf("%ds ago", int(d.Seconds()))
	}
	if d < time.Hour {
		return fmt.Sprintf("%dm ago", int(d.Minutes()))
	}
	return fmt.Sprintf("%s ago", d.Round(time.Minute))
}
