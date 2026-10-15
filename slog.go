package logs

import (
	"context"
	"log/slog"
	"time"
)

type slogHandler struct {
	ctx    context.Context
	fields map[string]any
	group  string
}

func NewSlogHandler(ctx context.Context) slog.Handler {
	return &slogHandler{ctx: ctx}
}

func levelOfSlog(level slog.Level) Level {
	switch {
	case level >= slog.LevelError:
		return LevelError
	case level >= slog.LevelWarn:
		return LevelWarning
	case level >= slog.LevelInfo:
		return LevelInfo
	default:
		return LevelDebug
	}
}

func (h *slogHandler) Enabled(_ context.Context, level slog.Level) bool {
	return Enabled(h.ctx, levelOfSlog(level))
}

func (h *slogHandler) Handle(_ context.Context, record slog.Record) error {
	if !Enabled(h.ctx, levelOfSlog(record.Level)) {
		return nil
	}
	var fields = copyFields(h.fields)
	if fields == nil {
		fields = make(map[string]any, record.NumAttrs())
	}
	record.Attrs(func(attr slog.Attr) bool {
		addSlogAttr(fields, h.group, attr)
		return true
	})

	var e = entry{
		Timestamp: record.Time,
		Level:     levelOfSlog(record.Level),
		Message:   record.Message,
		Fields:    fields,
		Caller:    getCallerInfoAt(record.PC),
	}
	if e.Timestamp.IsZero() {
		e.Timestamp = time.Now()
	}
	write(getState(h.ctx), e)
	return nil
}

func (h *slogHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	if len(attrs) == 0 {
		return h
	}
	var fields = copyFields(h.fields)
	if fields == nil {
		fields = make(map[string]any, len(attrs))
	}
	for _, attr := range attrs {
		addSlogAttr(fields, h.group, attr)
	}
	return &slogHandler{ctx: h.ctx, fields: fields, group: h.group}
}

func (h *slogHandler) WithGroup(name string) slog.Handler {
	if name == "" {
		return h
	}
	return &slogHandler{ctx: h.ctx, fields: h.fields, group: qualifySlogKey(h.group, name)}
}

func qualifySlogKey(group, key string) string {
	if group == "" {
		return key
	}
	return group + "." + key
}

func addSlogAttr(fields map[string]any, group string, attr slog.Attr) {
	var value = attr.Value.Resolve()
	if attr.Equal(slog.Attr{}) {
		return
	}
	if value.Kind() == slog.KindGroup {
		var nested = group
		if attr.Key != "" {
			nested = qualifySlogKey(group, attr.Key)
		}
		for _, member := range value.Group() {
			addSlogAttr(fields, nested, member)
		}
		return
	}
	fields[qualifySlogKey(group, attr.Key)] = fieldValueOfSlog(value)
}

func fieldValueOfSlog(value slog.Value) any {
	var v = value.Any()
	if err, isError := v.(error); isError {
		return err.Error()
	}
	return v
}
