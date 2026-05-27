package logs

import (
	"context"
	"io"
	"os"
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

type OutputFormat int

const (
	HumanReadable OutputFormat = iota
	JSON
)

type Config struct {
	Level          Level
	Format         OutputFormat
	StackTraceFrom *Level
}

func StackFrom(level Level) *Level {
	return &level
}

type RedactFunc func(key string, value any) (newValue any, keep bool)

type contextKeyType struct{}

var stateKey = &contextKeyType{}

type state struct {
	config      Config
	fields      map[string]any
	scope       string
	console     io.Writer
	writers     []io.Writer
	suppressAll bool
	redact      RedactFunc
}

var defaultState = &state{
	config: Config{
		Level:          LevelInfo,
		StackTraceFrom: StackFrom(LevelError),
	},
	console: os.Stderr,
}

var defaultFormat atomic.Int32

func SetDefaultFormat(format OutputFormat) {
	defaultFormat.Store(int32(format))
}

func (s *state) format() OutputFormat {
	if s == defaultState {
		return OutputFormat(defaultFormat.Load())
	}
	return s.config.Format
}

func getState(ctx context.Context) *state {
	if ctx == nil {
		return defaultState
	}
	if s, ok := ctx.Value(stateKey).(*state); ok {
		return s
	}
	return defaultState
}

func Attach(ctx context.Context, config Config) context.Context {
	if config.StackTraceFrom == nil {
		config.StackTraceFrom = StackFrom(LevelError)
	}
	var s = getState(ctx).clone()
	s.config = config
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
