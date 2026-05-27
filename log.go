package logs

import (
	"context"
	"fmt"
	"os"
	"sync"
	"time"
)

const callerSkipDepth = 3

var exit = os.Exit

var outputMu sync.Mutex

func emit(ctx context.Context, level Level, msg string, err error) {
	defer func() { _ = recover() }()

	var s = getState(ctx)

	if level < s.config.Level && level != LevelFatal {
		return
	}

	if s.suppressAll {
		return
	}

	var fields = s.fields
	if s.redact != nil {
		fields = applyRedaction(s.redact, s.fields)
	}

	var e = entry{
		Timestamp: time.Now(),
		Level:     level,
		Message:   msg,
		Fields:    fields,
		Scope:     s.scope,
		Caller:    getCallerInfo(callerSkipDepth),
		Error:     err,
	}

	if s.config.StackTraceFrom != nil && level >= *s.config.StackTraceFrom {
		e.Stack = captureStack()
	}

	var formatted string
	if s.format() == JSON {
		formatted = formatJSON(e)
	} else {
		formatted = formatHuman(e)
	}

	writeOutput(s, []byte(formatted))
}

func applyRedaction(fn RedactFunc, src map[string]any) map[string]any {
	if len(src) == 0 {
		return nil
	}
	var out = make(map[string]any, len(src))
	for k, v := range src {
		if nv, keep := fn(k, v); keep {
			out[k] = nv
		}
	}
	return out
}

func writeOutput(s *state, output []byte) {
	outputMu.Lock()
	defer outputMu.Unlock()

	if s.console != nil {
		s.console.Write(output)
	}

	for _, w := range s.writers {
		w.Write(output)
	}
}

func Debug(ctx context.Context, msg string) {
	emit(ctx, LevelDebug, msg, nil)
}

func Debugf(ctx context.Context, format string, args ...any) {
	emit(ctx, LevelDebug, fmt.Sprintf(format, args...), nil)
}

func Info(ctx context.Context, msg string) {
	emit(ctx, LevelInfo, msg, nil)
}

func Infof(ctx context.Context, format string, args ...any) {
	emit(ctx, LevelInfo, fmt.Sprintf(format, args...), nil)
}

func Warn(ctx context.Context, msg string) {
	emit(ctx, LevelWarning, msg, nil)
}

func Warnf(ctx context.Context, format string, args ...any) {
	emit(ctx, LevelWarning, fmt.Sprintf(format, args...), nil)
}

func Error(ctx context.Context, msg string) {
	emit(ctx, LevelError, msg, nil)
}

func Errorf(ctx context.Context, format string, args ...any) {
	emit(ctx, LevelError, fmt.Sprintf(format, args...), nil)
}

func Fatal(ctx context.Context, msg string) {
	emit(ctx, LevelFatal, msg, nil)
	exit(1)
}

func Fatalf(ctx context.Context, format string, args ...any) {
	emit(ctx, LevelFatal, fmt.Sprintf(format, args...), nil)
	exit(1)
}

func Err(ctx context.Context, err error) {
	if err == nil {
		return
	}
	emit(ctx, LevelError, err.Error(), err)
}

func Errf(ctx context.Context, err error, format string, args ...any) {
	if err == nil {
		return
	}
	emit(ctx, LevelError, fmt.Sprintf(format, args...), err)
}
