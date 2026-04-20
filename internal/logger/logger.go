package logger

import (
	"fmt"
	"io"
	"sync"
	"time"
)

type Level int
type ColorCode string

const (
	LevelTrace Level = iota
	LevelInfo
	LevelWarning
	LevelError
)

const (
	Red    ColorCode = "\033[31m"
	Green  ColorCode = "\033[32m"
	Yellow ColorCode = "\033[33m"
	Gray   ColorCode = "\033[90m"
	Reset  ColorCode = "\033[0m"
)

type Logger struct {
	out      io.Writer
	minLevel Level
	color    bool
	mutex    sync.Mutex
}

func New(w io.Writer, min Level, color bool) *Logger {
	return &Logger{
		out:      w,
		minLevel: min,
		color:    color,
	}
}

func (logger *Logger) log(level Level, label string, colorCode ColorCode, format string, args ...any) {
	if level < logger.minLevel {
		return
	}

	timestamp := time.Now().Format("2006-01-02 15:04:05.000 -07:00")

	logger.mutex.Lock()
	defer logger.mutex.Unlock()

	if logger.color && colorCode != "" {
		fmt.Fprint(logger.out, colorCode)
	}

	// max label size = 5, (ie, TRACE, ERROR)
	// so label size + ":" is at most 6 chars, padded to width 7 for alignment.
	// update appropriately if max label size is later increased,
	// like if something like CRITICAL is added.
	// so size = 8, + colon = 9, then space after, so %-10s
	fmt.Fprintf(logger.out, "[%s] %-7s", timestamp, label+":")
	fmt.Fprintf(logger.out, format, args...)
	fmt.Fprint(logger.out, "\n")

	if logger.color && colorCode != "" {
		fmt.Fprint(logger.out, Reset)
	}
}

func (logger *Logger) Trace(format string, args ...any) {
	logger.log(LevelTrace, "TRACE", Gray, format, args...)
}

func (logger *Logger) Info(format string, args ...any) {
	logger.log(LevelInfo, "INFO", Green, format, args...)
}

func (logger *Logger) Warn(format string, args ...any) {
	logger.log(LevelWarning, "WARN", Yellow, format, args...)
}

func (logger *Logger) Error(format string, args ...any) {
	logger.log(LevelError, "ERROR", Red, format, args...)
}
