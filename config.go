package logs

import (
	"context"
	"fmt"
	"io"
	"os"
	"strings"
	"sync/atomic"
)

type Level int

const (
	LevelDebug Level = iota
	LevelInfo
	LevelWarning
	LevelError
	LevelFatal
)

func (l Level) String() string {
	switch l {
	case LevelDebug:
		return "DEBUG"
	case LevelInfo:
		return "INFO"
	case LevelWarning:
		return "WARNING"
	case LevelError:
		return "ERROR"
	case LevelFatal:
		return "FATAL"
	default:
		return "UNKNOWN"
	}
}

func ParseLevel(text string) (Level, error) {
	switch strings.ToLower(strings.TrimSpace(text)) {
	case "debug":
		return LevelDebug, nil
	case "info":
		return LevelInfo, nil
	case "warn", "warning":
		return LevelWarning, nil
	case "error":
		return LevelError, nil
	case "fatal":
		return LevelFatal, nil
	default:
		return LevelInfo, fmt.Errorf("unknown log level %q: want debug, info, warning, error or fatal", text)
	}
}

type OutputFormat int

const (
	HumanReadable OutputFormat = iota
	JSON
)

type Config struct {
	Level          Level
	Format         OutputFormat
	StackTraceFrom Level
}

type contextKeyType struct{}

var stateKey = &contextKeyType{}

type state struct {
	config      Config
	fields      map[string]any
	hidden      map[string]bool
	scope       string
	console     io.Writer
	writers     []io.Writer
	suppressAll bool
}

var defaultState atomic.Pointer[state]

func init() {
	defaultState.Store(&state{
		config: Config{
			Level:          LevelInfo,
			Format:         HumanReadable,
			StackTraceFrom: LevelError,
		},
		console: os.Stderr,
	})
}

func SetDefaultFormat(format OutputFormat) {
	for {
		var current = defaultState.Load()
		var next = current.clone()
		next.config.Format = format
		if defaultState.CompareAndSwap(current, next) {
			return
		}
	}
}

func getState(ctx context.Context) *state {
	if ctx == nil {
		return defaultState.Load()
	}
	if s, ok := ctx.Value(stateKey).(*state); ok {
		return s
	}
	return defaultState.Load()
}

func Enabled(ctx context.Context, level Level) bool {
	var s = getState(ctx)
	return !s.suppressAll && level >= s.config.Level
}

func Attach(ctx context.Context, config Config) context.Context {
	if config.StackTraceFrom == 0 {
		config.StackTraceFrom = LevelError
	}
	var s = &state{
		config:  config,
		console: os.Stderr,
	}
	return context.WithValue(ctx, stateKey, s)
}

type syncer interface {
	Sync() error
}

func Flush(ctx context.Context) error {
	var s = getState(ctx)
	if f, ok := s.console.(syncer); ok {
		_ = f.Sync()
	}
	for _, w := range s.writers {
		if f, ok := w.(syncer); ok {
			if err := f.Sync(); err != nil {
				return err
			}
		}
	}
	return nil
}

func SetFormat(ctx context.Context, format OutputFormat) context.Context {
	var s = getState(ctx).clone()
	s.config.Format = format
	return context.WithValue(ctx, stateKey, s)
}
