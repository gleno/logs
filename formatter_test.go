package logs

import (
	"context"
	"strings"
	"testing"
)

type funcFormatter func(Entry) []byte

func (f funcFormatter) Format(e Entry) []byte { return f(e) }

func TestFormatter_CustomImplViaConfig(t *testing.T) {
	var custom = funcFormatter(func(e Entry) []byte {
		return []byte("X> " + e.Message + "\n")
	})

	var out = captureOutput(Config{Level: LevelInfo, Formatter: custom}, func(ctx context.Context) {
		Info(ctx, "hello")
	})

	if out != "X> hello\n" {
		t.Errorf("expected custom formatter output, got %q", out)
	}
}

func TestFormatter_CustomImplReceivesEntry(t *testing.T) {
	var got Entry
	var capture = funcFormatter(func(e Entry) []byte {
		got = e
		return nil
	})

	captureOutput(Config{Level: LevelInfo, Formatter: capture}, func(ctx context.Context) {
		ctx = WithFields(ctx, map[string]any{"k": "v"})
		Info(ctx, "msg")
	})

	if got.Message != "msg" {
		t.Errorf("formatter should receive the message, got %q", got.Message)
	}
	if got.Fields["k"] != "v" {
		t.Errorf("formatter should receive the fields, got %v", got.Fields)
	}
}

func TestFormatter_NilDefaultsToHuman(t *testing.T) {
	var out = captureOutput(Config{Level: LevelInfo}, func(ctx context.Context) {
		Info(ctx, "bare")
	})

	if !strings.Contains(out, "bare") || strings.Contains(out, `"severity"`) {
		t.Errorf("nil Formatter should default to human-readable, got %q", out)
	}
}

func TestFormatter_GCPEmitsSourceLocation(t *testing.T) {
	var out = captureOutput(Config{Level: LevelInfo, Formatter: GCPLogFormatter{}}, func(ctx context.Context) {
		Info(ctx, "boom")
	})

	var parsed = parseJSON(t, out)
	if parsed["severity"] != "INFO" {
		t.Errorf("GCP formatter should emit top-level severity, got %v", parsed["severity"])
	}
	loc, ok := parsed["logging.googleapis.com/sourceLocation"].(map[string]any)
	if !ok {
		t.Fatalf("GCP formatter should emit the GCP-native sourceLocation field, got %T", parsed["logging.googleapis.com/sourceLocation"])
	}
	if _, ok := loc["file"].(string); !ok {
		t.Errorf("sourceLocation should carry a file, got %v", loc)
	}
}

func TestFormatter_SetFormatOverridesPerContext(t *testing.T) {
	var out = captureOutput(Config{Level: LevelInfo, Formatter: HumanLogFormatter{}}, func(ctx context.Context) {
		ctx = SetFormat(ctx, GCPLogFormatter{})
		Info(ctx, "switched")
	})

	if !strings.Contains(out, `"severity"`) {
		t.Errorf("SetFormat should switch this context to GCP JSON, got %q", out)
	}
}
