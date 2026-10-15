package logs

import (
	"context"
	"fmt"
	"os"
	"time"
)

const callerSkipDepth = 3

var exit = os.Exit

func emit(ctx context.Context, level Level, msg string, err error) {
	if !Enabled(ctx, level) {
		return
	}
	write(getState(ctx), entry{
		Timestamp: time.Now(),
		Level:     level,
		Message:   msg,
		Caller:    getCallerInfo(callerSkipDepth),
		Error:     err,
	})
}

func write(s *state, e entry) {
	e.Fields = mergeFields(s.fields, e.Fields)
	e.Hidden = s.hidden
	e.Scope = s.scope

	if e.Level >= s.config.StackTraceFrom {
		e.Stack = captureStack()
	}

	var formatted string
	if s.config.Format == JSON {
		formatted = formatJSON(e)
	} else {
		formatted = formatHuman(e)
	}

	var output = []byte(formatted)

	if s.console != nil {
		s.console.Write(output)
	}

	for _, w := range s.writers {
		w.Write(output)
	}
}

func mergeFields(base, extra map[string]any) map[string]any {
	if len(extra) == 0 {
		return base
	}
	var merged = copyFields(base)
	if merged == nil {
		merged = make(map[string]any, len(extra))
	}
	for k, v := range extra {
		merged[k] = v
	}
	return merged
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
