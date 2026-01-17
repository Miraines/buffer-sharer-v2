package logging

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// LogLevel represents the severity of a log entry
type LogLevel string

const (
	LevelDebug LogLevel = "DEBUG"
	LevelInfo  LogLevel = "INFO"
	LevelWarn  LogLevel = "WARN"
	LevelError LogLevel = "ERROR"
)

// LogEntry represents a single log entry for UI display
type LogEntry struct {
	Timestamp time.Time `json:"timestamp"`
	Level     LogLevel  `json:"level"`
	Message   string    `json:"message"`
	Source    string    `json:"source,omitempty"`
}

// Logger wraps zap logger with UI-friendly features
type Logger struct {
	zap        *zap.Logger
	sugar      *zap.SugaredLogger
	entries    []LogEntry
	maxEntries int
	mu         sync.RWMutex
	enabled    bool
	listeners  []func(LogEntry)
	logFile    *os.File
	role       string
}

// Config holds logger configuration
type Config struct {
	Enabled    bool
	MaxEntries int
	Level      LogLevel
	Role       string // "client" or "controller" - for log file naming
	LogToFile  bool   // Enable file logging
}

// NewLogger creates a new logger instance
func NewLogger(cfg Config) (*Logger, error) {
	// Determine log level
	var level zapcore.Level
	switch cfg.Level {
	case LevelDebug:
		level = zapcore.DebugLevel
	case LevelInfo:
		level = zapcore.InfoLevel
	case LevelWarn:
		level = zapcore.WarnLevel
	case LevelError:
		level = zapcore.ErrorLevel
	default:
		level = zapcore.DebugLevel // Log everything by default
	}

	// Create encoder config
	encoderConfig := zapcore.EncoderConfig{
		TimeKey:        "timestamp",
		LevelKey:       "level",
		NameKey:        "logger",
		CallerKey:      "caller",
		FunctionKey:    zapcore.OmitKey,
		MessageKey:     "msg",
		StacktraceKey:  "stacktrace",
		LineEnding:     zapcore.DefaultLineEnding,
		EncodeLevel:    zapcore.CapitalLevelEncoder,
		EncodeTime:     zapcore.ISO8601TimeEncoder,
		EncodeDuration: zapcore.SecondsDurationEncoder,
		EncodeCaller:   zapcore.ShortCallerEncoder,
	}

	var logFile *os.File
	var cores []zapcore.Core

	// Console output
	consoleEncoder := zapcore.NewConsoleEncoder(encoderConfig)
	consoleCore := zapcore.NewCore(consoleEncoder, zapcore.AddSync(os.Stdout), level)
	cores = append(cores, consoleCore)

	// File output - always enabled, log everything
	if cfg.LogToFile {
		// Get executable directory
		exePath, err := os.Executable()
		if err == nil {
			exeDir := filepath.Dir(exePath)
			// On macOS, go up from MacOS folder to app bundle level
			if filepath.Base(filepath.Dir(exeDir)) == "Contents" {
				exeDir = filepath.Dir(filepath.Dir(filepath.Dir(exeDir)))
			}

			role := cfg.Role
			if role == "" {
				role = "app"
			}

			// Create log filename with date
			dateStr := time.Now().Format("2006-01-02_15-04-05")
			logFileName := fmt.Sprintf("logs_%s_%s.txt", role, dateStr)
			logPath := filepath.Join(exeDir, logFileName)

			logFile, err = os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
			if err == nil {
				fileEncoder := zapcore.NewConsoleEncoder(encoderConfig)
				fileCore := zapcore.NewCore(fileEncoder, zapcore.AddSync(logFile), zapcore.DebugLevel)
				cores = append(cores, fileCore)
			}
		}
	}

	// Combine cores
	core := zapcore.NewTee(cores...)
	zapLogger := zap.New(core, zap.AddCaller(), zap.AddStacktrace(zapcore.ErrorLevel))

	maxEntries := cfg.MaxEntries
	if maxEntries <= 0 {
		maxEntries = 1000
	}

	return &Logger{
		zap:        zapLogger,
		sugar:      zapLogger.Sugar(),
		entries:    make([]LogEntry, 0, maxEntries),
		maxEntries: maxEntries,
		enabled:    cfg.Enabled,
		listeners:  make([]func(LogEntry), 0),
		logFile:    logFile,
		role:       cfg.Role,
	}, nil
}

// ListenerID is a unique identifier for a registered listener
type ListenerID int

// listenerEntry holds a listener function with its ID
type listenerEntry struct {
	id       ListenerID
	listener func(LogEntry)
}

// AddListener adds a listener function that will be called for each new log entry.
// Returns a ListenerID that can be used to remove the listener later.
func (l *Logger) AddListener(listener func(LogEntry)) ListenerID {
	l.mu.Lock()
	defer l.mu.Unlock()
	id := ListenerID(len(l.listeners))
	l.listeners = append(l.listeners, listener)
	return id
}

// RemoveListener removes a previously registered listener by its ID.
// This prevents listener accumulation and memory leaks.
func (l *Logger) RemoveListener(id ListenerID) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if int(id) >= 0 && int(id) < len(l.listeners) {
		// Set to nil instead of removing to preserve indices
		l.listeners[id] = nil
	}
}

// addEntry adds a log entry to the buffer and notifies listeners
func (l *Logger) addEntry(level LogLevel, source, message string) {
	if !l.enabled {
		return
	}

	entry := LogEntry{
		Timestamp: time.Now(),
		Level:     level,
		Message:   message,
		Source:    source,
	}

	l.mu.Lock()
	// Maintain max entries limit
	if len(l.entries) >= l.maxEntries {
		l.entries = l.entries[1:]
	}
	l.entries = append(l.entries, entry)
	listeners := make([]func(LogEntry), len(l.listeners))
	copy(listeners, l.listeners)
	l.mu.Unlock()

	// Notify listeners outside of lock (skip nil listeners)
	for _, listener := range listeners {
		if listener != nil {
			listener(entry)
		}
	}
}

// Debug logs a debug message
func (l *Logger) Debug(source, msg string, args ...interface{}) {
	formatted := fmt.Sprintf(msg, args...)
	l.sugar.Debugw(formatted, "source", source)
	l.addEntry(LevelDebug, source, formatted)
}

// Info logs an info message
func (l *Logger) Info(source, msg string, args ...interface{}) {
	formatted := fmt.Sprintf(msg, args...)
	l.sugar.Infow(formatted, "source", source)
	l.addEntry(LevelInfo, source, formatted)
}

// Warn logs a warning message
func (l *Logger) Warn(source, msg string, args ...interface{}) {
	formatted := fmt.Sprintf(msg, args...)
	l.sugar.Warnw(formatted, "source", source)
	l.addEntry(LevelWarn, source, formatted)
}

// Error logs an error message
func (l *Logger) Error(source, msg string, args ...interface{}) {
	formatted := fmt.Sprintf(msg, args...)
	l.sugar.Errorw(formatted, "source", source)
	l.addEntry(LevelError, source, formatted)
}

// GetEntries returns a copy of all log entries
func (l *Logger) GetEntries() []LogEntry {
	l.mu.RLock()
	defer l.mu.RUnlock()

	entries := make([]LogEntry, len(l.entries))
	copy(entries, l.entries)
	return entries
}

// GetRecentEntries returns the most recent n entries
func (l *Logger) GetRecentEntries(n int) []LogEntry {
	l.mu.RLock()
	defer l.mu.RUnlock()

	if n <= 0 || n >= len(l.entries) {
		entries := make([]LogEntry, len(l.entries))
		copy(entries, l.entries)
		return entries
	}

	start := len(l.entries) - n
	entries := make([]LogEntry, n)
	copy(entries, l.entries[start:])
	return entries
}

// Clear clears all log entries
func (l *Logger) Clear() {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.entries = l.entries[:0]
}

// Close syncs and closes the logger
func (l *Logger) Close() error {
	err := l.zap.Sync()
	if l.logFile != nil {
		l.logFile.Close()
	}
	return err
}

// SetRole updates the role for logging purposes
func (l *Logger) SetRole(role string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.role = role
}

// GetLogFilePath returns the path to the current log file
func (l *Logger) GetLogFilePath() string {
	if l.logFile != nil {
		return l.logFile.Name()
	}
	return ""
}

// FormatEntry formats a log entry for display
func FormatEntry(entry LogEntry) string {
	return fmt.Sprintf("[%s] [%s] %s: %s",
		entry.Timestamp.Format("15:04:05"),
		entry.Level,
		entry.Source,
		entry.Message,
	)
}
