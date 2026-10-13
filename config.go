package logs

import (
	"context"
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

type Config struct {
	Level          Level
	Formatter      LoggerFormatter
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
	console     *sink
	writers     []*sink
	suppressAll bool
	redact      RedactFunc
}

var defaultState = &state{
	config: Config{
		Level:          LevelInfo,
		StackTraceFrom: StackFrom(LevelError),
	},
	console: &sink{w: os.Stderr},
}

type formatterBox struct{ formatter LoggerFormatter }

var defaultFormatter atomic.Value

func SetDefaultFormat(formatter LoggerFormatter) {
	defaultFormatter.Store(formatterBox{formatter})
}

func loadDefaultFormatter() LoggerFormatter {
	if b, ok := defaultFormatter.Load().(formatterBox); ok && b.formatter != nil {
		return b.formatter
	}
	return HumanLogFormatter{}
}

func (s *state) formatter() LoggerFormatter {
	if s.config.Formatter != nil {
		return s.config.Formatter
	}
	return loadDefaultFormatter()
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

func Flush(ctx context.Context) error {
	var s = getState(ctx)
	if s.console != nil {
		_ = s.console.sync()
	}
	for _, w := range s.writers {
		if err := w.sync(); err != nil {
			return err
		}
	}
	return nil
}

func SetFormat(ctx context.Context, formatter LoggerFormatter) context.Context {
	var s = getState(ctx).clone()
	s.config.Formatter = formatter
	return context.WithValue(ctx, stateKey, s)
}
