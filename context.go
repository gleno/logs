package logs

import (
	"context"
	"io"
)

func copyFields(src map[string]any) map[string]any {
	if len(src) == 0 {
		return nil
	}
	var dst = make(map[string]any, len(src))
	for k, v := range src {
		dst[k] = v
	}
	return dst
}

func copyHidden(src map[string]bool) map[string]bool {
	if len(src) == 0 {
		return nil
	}
	var dst = make(map[string]bool, len(src))
	for k := range src {
		dst[k] = true
	}
	return dst
}

func (s *state) clone() *state {
	return &state{
		config:      s.config,
		fields:      copyFields(s.fields),
		hidden:      copyHidden(s.hidden),
		scope:       s.scope,
		console:     s.console,
		writers:     append([]io.Writer(nil), s.writers...),
		suppressAll: s.suppressAll,
	}
}

func WithFields(ctx context.Context, fields map[string]any) context.Context {
	if len(fields) == 0 {
		return ctx
	}

	var s = getState(ctx).clone()

	if s.fields == nil {
		s.fields = make(map[string]any, len(fields))
	}
	for k, v := range fields {
		if v == nil {
			continue
		}
		s.fields[k] = v
	}

	return context.WithValue(ctx, stateKey, s)
}

func HideFields(ctx context.Context, keys ...string) context.Context {
	if len(keys) == 0 {
		return ctx
	}

	var s = getState(ctx).clone()

	if s.hidden == nil {
		s.hidden = make(map[string]bool, len(keys))
	}
	for _, k := range keys {
		s.hidden[k] = true
	}

	return context.WithValue(ctx, stateKey, s)
}

func WithScope(ctx context.Context, scope string) context.Context {
	var s = getState(ctx).clone()

	if s.scope == "" {
		s.scope = scope
	} else {
		s.scope = s.scope + "." + scope
	}

	if s.fields == nil {
		s.fields = make(map[string]any, 1)
	}
	s.fields["scope"] = s.scope

	return context.WithValue(ctx, stateKey, s)
}

func SuppressConsoleOutput(ctx context.Context) context.Context {
	var s = getState(ctx).clone()
	s.console = nil
	return context.WithValue(ctx, stateKey, s)
}

func AttachWriter(ctx context.Context, w io.Writer) context.Context {
	var s = getState(ctx).clone()
	s.writers = append(s.writers, w)
	return context.WithValue(ctx, stateKey, s)
}

func SuppressAllOutput(ctx context.Context) context.Context {
	var s = getState(ctx).clone()
	s.suppressAll = true
	return context.WithValue(ctx, stateKey, s)
}
