package logs

import (
	"bytes"
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"
)

func TestSetDefaultFormat_AppliesToDerivedContexts(t *testing.T) {
	var original = defaultState.formatter()
	defer SetDefaultFormat(original)
	SetDefaultFormat(GCPLogFormatter{})

	var derivations = map[string]func(context.Context) context.Context{
		"WithScope":     func(ctx context.Context) context.Context { return WithScope(ctx, "svc") },
		"WithFields":    func(ctx context.Context) context.Context { return WithFields(ctx, map[string]any{"k": "v"}) },
		"WithRedaction": func(ctx context.Context) context.Context { return WithRedaction(ctx, keepAll) },
		"Attach without formatter": func(ctx context.Context) context.Context {
			return Attach(ctx, Config{Level: LevelDebug})
		},
		"writer and console only": func(ctx context.Context) context.Context { return ctx },
	}

	for name, derive := range derivations {
		var buf bytes.Buffer
		var ctx = AttachWriter(SuppressConsoleOutput(derive(context.Background())), &buf)

		Info(ctx, "hello")

		if !json.Valid(bytes.TrimSpace(buf.Bytes())) {
			t.Errorf("%s: default format ignored, got non-JSON output: %q", name, buf.String())
		}
	}
}

func TestSetDefaultFormat_AppliesToContextsDerivedEarlier(t *testing.T) {
	var original = defaultState.formatter()
	defer SetDefaultFormat(original)

	var buf bytes.Buffer
	var ctx = AttachWriter(SuppressConsoleOutput(WithScope(context.Background(), "svc")), &buf)
	SetDefaultFormat(GCPLogFormatter{})

	Info(ctx, "hello")

	if !json.Valid(bytes.TrimSpace(buf.Bytes())) {
		t.Errorf("default format should apply at log time, got non-JSON output: %q", buf.String())
	}
}

func TestSetFormat_OverridesDefaultFormat(t *testing.T) {
	var original = defaultState.formatter()
	defer SetDefaultFormat(original)
	SetDefaultFormat(GCPLogFormatter{})

	var buf bytes.Buffer
	var ctx = AttachWriter(SuppressConsoleOutput(context.Background()), &buf)
	ctx = SetFormat(ctx, HumanLogFormatter{})

	Info(ctx, "hello")

	if json.Valid(bytes.TrimSpace(buf.Bytes())) {
		t.Errorf("explicit per-context formatter should win over the default, got: %q", buf.String())
	}
}

func TestAttachWriter_ChainedLogWriterDoesNotDeadlock(t *testing.T) {
	var inner bytes.Buffer
	var innerCtx = AttachWriter(SuppressConsoleOutput(Attach(context.Background(), Config{Level: LevelInfo})), &inner)
	var outerCtx = AttachWriter(SuppressConsoleOutput(context.Background()), NewWriter(innerCtx, LevelWarning))

	var done = make(chan struct{})
	go func() {
		Info(outerCtx, "forwarded")
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("logging into a writer that logs to another context deadlocked")
	}

	if !strings.Contains(inner.String(), "forwarded") {
		t.Errorf("chained writer should forward the line, got: %q", inner.String())
	}
}

func keepAll(_ string, value any) (any, bool) {
	return value, true
}
