package logs

import (
	"context"
	"io"
	"os"
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
	StackTraceFrom Level
}

type contextKeyType struct{}

var stateKey = &contextKeyType{}

type state struct {
	config      Config
	fields      map[string]any
	scope       string
	console     io.Writer
	writers     []io.Writer
	suppressAll bool
}

var defaultState = &state{
	config: Config{
		Level:          LevelInfo,
		Format:         HumanReadable,
		StackTraceFrom: LevelError,
	},
	console: os.Stderr,
}

func SetDefaultFormat(format OutputFormat) {
	defaultState.config.Format = format
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
		if err := f.Sync(); err != nil {
			return err
		}
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
	var s = getState(ctx)
	s.config.Format = format
	return context.WithValue(ctx, stateKey, s)
}
