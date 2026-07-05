// Package mlog is a minimal structured logger for the Marco engine.
//
// Level is controlled by the $MARCO_LOG environment variable:
//
//	MARCO_LOG=debug   all messages
//	MARCO_LOG=info    info + warn + error
//	MARCO_LOG=warn    warn + error (good default when debugging specific issues)
//	MARCO_LOG=error   errors only
//	MARCO_LOG=off     nothing
//	(unset)           info (the default)
//
// Format: [HH:MM:SS][LEVEL] component - message key=value
// (messages use a "component: detail" convention, rendered as "component - detail").
// Output: stderr (never stdout — stdout is the engine's data channel).
package mlog

import (
	"fmt"
	"io"
	"os"
	"strings"
	"sync"
	"time"
)

// Level represents a log severity.
type Level int8

const (
	LevelDebug Level = iota
	LevelInfo
	LevelWarn
	LevelError
	LevelOff
)

var levelLabel = [...]string{"DEBUG", "INFO", "WARN", "ERROR"}

// Logger writes structured log lines to w, suppressing lines below lvl.
type Logger struct {
	mu  sync.Mutex
	w   io.Writer
	lvl Level
}

// New returns a Logger writing to w at the given level.
func New(w io.Writer, lvl Level) *Logger { return &Logger{w: w, lvl: lvl} }

func (l *Logger) log(lvl Level, msg string, kv []any) {
	if lvl < l.lvl {
		return
	}
	var b strings.Builder
	b.WriteString(time.Now().Format("[15:04:05]"))
	b.WriteString("[")
	b.WriteString(levelLabel[lvl])
	b.WriteString("] ")
	// Messages follow a "component: detail" convention; render it as "component -
	// detail" for the [time][LEVEL] component - detail style.
	b.WriteString(strings.Replace(msg, ": ", " - ", 1))
	for i := 0; i+1 < len(kv); i += 2 {
		fmt.Fprintf(&b, " %v=%v", kv[i], kv[i+1])
	}
	b.WriteByte('\n')
	l.mu.Lock()
	_, _ = l.w.Write([]byte(b.String()))
	l.mu.Unlock()
}

func (l *Logger) Debug(msg string, kv ...any) { l.log(LevelDebug, msg, kv) }
func (l *Logger) Info(msg string, kv ...any)  { l.log(LevelInfo, msg, kv) }
func (l *Logger) Warn(msg string, kv ...any)  { l.log(LevelWarn, msg, kv) }
func (l *Logger) Error(msg string, kv ...any) { l.log(LevelError, msg, kv) }

// Default is the package-level logger, initialised from $MARCO_LOG.
var Default = New(os.Stderr, ParseLevel(os.Getenv("MARCO_LOG")))

// Package-level convenience wrappers that write to Default.
func Debug(msg string, kv ...any) { Default.Debug(msg, kv...) }
func Info(msg string, kv ...any)  { Default.Info(msg, kv...) }
func Warn(msg string, kv ...any)  { Default.Warn(msg, kv...) }
func Error(msg string, kv ...any) { Default.Error(msg, kv...) }

// ParseLevel converts a $MARCO_LOG string to a Level. Unknown or empty → LevelInfo: a
// quiet production default that still surfaces notable events, warnings and errors. Set
// MARCO_LOG=debug for the full per-click scoring/coordinate trace while diagnosing.
func ParseLevel(s string) Level {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "debug":
		return LevelDebug
	case "info":
		return LevelInfo
	case "warn", "warning":
		return LevelWarn
	case "error", "err":
		return LevelError
	case "off", "none", "silent":
		return LevelOff
	default:
		return LevelInfo
	}
}
