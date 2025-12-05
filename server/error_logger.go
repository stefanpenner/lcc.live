package server

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// ErrorLogEntry represents a single error log entry
type ErrorLogEntry struct {
	Timestamp time.Time `json:"timestamp"`
	Status    int       `json:"status"`
	Method    string    `json:"method"`
	Path      string    `json:"path"`
	URL       string    `json:"url"`
	IP        string    `json:"ip"`
	UserAgent string    `json:"user_agent"`
	Duration  string    `json:"duration"`
	Error     string    `json:"error,omitempty"`
}

var (
	errorLogFile   *os.File
	errorLogMutex  sync.Mutex
	errorLogPath   string
	errorLogWriter *json.Encoder
)

// InitErrorLogger initializes the error log file
func InitErrorLogger(logDir string) error {
	errorLogMutex.Lock()
	defer errorLogMutex.Unlock()

	if logDir == "" {
		// Default to temp directory
		logDir = os.TempDir()
	}

	// Create log directory if it doesn't exist
	if err := os.MkdirAll(logDir, 0755); err != nil {
		return fmt.Errorf("failed to create log directory: %w", err)
	}

	errorLogPath = filepath.Join(logDir, "lcc-live-errors.jsonl")

	// Open file in append mode
	file, err := os.OpenFile(errorLogPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return fmt.Errorf("failed to open error log file: %w", err)
	}

	errorLogFile = file
	errorLogWriter = json.NewEncoder(file)

	return nil
}

// LogError logs an HTTP error to the error log file
func LogError(status int, method, path, url, ip, userAgent string, duration time.Duration, err error) {
	errorLogMutex.Lock()
	defer errorLogMutex.Unlock()

	if errorLogWriter == nil {
		return
	}

	entry := ErrorLogEntry{
		Timestamp: time.Now(),
		Status:    status,
		Method:    method,
		Path:      path,
		URL:       url,
		IP:        ip,
		UserAgent: userAgent,
		Duration:  duration.String(),
	}

	if err != nil {
		entry.Error = err.Error()
	}

	_ = errorLogWriter.Encode(entry)
	_ = errorLogFile.Sync() // Flush to disk
}

// GetErrorLogPath returns the path to the error log file
func GetErrorLogPath() string {
	errorLogMutex.Lock()
	defer errorLogMutex.Unlock()
	return errorLogPath
}

// CloseErrorLogger closes the error log file
func CloseErrorLogger() error {
	errorLogMutex.Lock()
	defer errorLogMutex.Unlock()

	if errorLogFile != nil {
		err := errorLogFile.Close()
		errorLogFile = nil
		errorLogWriter = nil
		return err
	}
	return nil
}


