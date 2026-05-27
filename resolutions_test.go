package logs

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"math"
	"strings"
	"sync"
	"testing"
	"time"
)

type panicMarshaler struct{}

func (panicMarshaler) MarshalJSON() ([]byte, error) { panic("boom in MarshalJSON") }

type panicWriter struct{}

func (panicWriter) Write([]byte) (int, error) { panic("boom in Write") }

func parseJSON(t *testing.T, s string) map[string]any {
	t.Helper()
	var m map[string]any
	if err := json.Unmarshal([]byte(s), &m); err != nil {
		t.Fatalf("expected valid JSON, got error %v in: %s", err, s)
	}
	return m
}

func TestResolve_PanickyFieldDoesNotCrashAndEntrySurvives(t *testing.T) {
	var out = captureOutput(Config{Level: LevelInfo, Format: JSON}, func(ctx context.Context) {
		ctx = WithFields(ctx, map[string]any{"bad": panicMarshaler{}, "good": "kept"})
		Info(ctx, "still logs")
	})

	var parsed = parseJSON(t, out)
	if parsed["message"] != "still logs" {
		t.Errorf("message lost to a panicking field: %v", parsed["message"])
	}
	var data = parsed["data"].(map[string]any)
	if data["good"] != "kept" {
		t.Errorf("good field lost: %v", data["good"])
	}
	if s, _ := data["bad"].(string); !strings.Contains(s, "ERR") {
		t.Errorf("panicking field should degrade to a placeholder, got %v", data["bad"])
	}
}

func TestResolve_NaNInfStringified(t *testing.T) {
	var out = captureOutput(Config{Level: LevelInfo, Format: JSON}, func(ctx context.Context) {
		ctx = WithFields(ctx, map[string]any{
			"nan":    math.NaN(),
			"pinf":   math.Inf(1),
			"ninf":   math.Inf(-1),
			"f32inf": float32(math.Inf(1)),
			"normal": 1.5,
			"user":   "u123",
		})
		Info(ctx, "metrics")
	})

	var parsed = parseJSON(t, out)
	if parsed["message"] != "metrics" {
		t.Errorf("message lost to a NaN field: %v", parsed["message"])
	}
	var data = parsed["data"].(map[string]any)
	for k, want := range map[string]string{"nan": "NaN", "pinf": "+Inf", "ninf": "-Inf", "f32inf": "+Inf"} {
		if data[k] != want {
			t.Errorf("field %s: expected %q, got %v", k, want, data[k])
		}
	}
	if data["normal"] != 1.5 {
		t.Errorf("finite float should stay a number, got %v", data["normal"])
	}
	if data["user"] != "u123" {
		t.Errorf("good field lost: %v", data["user"])
	}
}

func TestResolve_WriterPanicDoesNotCrashOrDeadlock(t *testing.T) {
	var buf bytes.Buffer
	var ctx = Attach(context.Background(), Config{Level: LevelInfo})
	ctx = SuppressConsoleOutput(ctx)
	ctx = AttachWriter(ctx, &buf)
	ctx = AttachWriter(ctx, panicWriter{})

	Info(ctx, "first")
	Info(ctx, "second")

	if c := strings.Count(buf.String(), "INFO"); c != 2 {
		t.Errorf("a panicking writer must not crash, swallow, or deadlock; good writer got %d lines", c)
	}
}

func TestResolve_ConcurrentWritesAllArrive(t *testing.T) {
	var buf bytes.Buffer
	var ctx = Attach(context.Background(), Config{Level: LevelInfo})
	ctx = SuppressConsoleOutput(ctx)
	ctx = AttachWriter(ctx, &buf)

	var wg sync.WaitGroup
	for range 50 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			Info(ctx, "concurrent")
		}()
	}
	wg.Wait()

	if c := strings.Count(buf.String(), "INFO"); c != 50 {
		t.Errorf("expected 50 serialized lines, got %d", c)
	}
}

func TestResolve_AttachMergesPriorState(t *testing.T) {
	var buf bytes.Buffer
	var ctx = Attach(context.Background(), Config{Level: LevelInfo})
	ctx = AttachWriter(ctx, &buf)
	ctx = SuppressConsoleOutput(ctx)
	ctx = Attach(ctx, Config{Level: LevelDebug, Format: JSON})

	Info(ctx, "after reattach")

	if !strings.Contains(buf.String(), "after reattach") {
		t.Errorf("Attach should preserve the prior writer, got: %q", buf.String())
	}
	if !strings.Contains(buf.String(), `"severity"`) {
		t.Errorf("Attach should still apply the new config (JSON), got: %q", buf.String())
	}
}

func TestResolve_DefaultFormatNoRace(t *testing.T) {
	var original = defaultState.format()
	defer SetDefaultFormat(original)
	var origConsole = defaultState.console
	defaultState.console = io.Discard
	defer func() { defaultState.console = origConsole }()

	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		for range 100 {
			SetDefaultFormat(JSON)
		}
	}()
	go func() {
		defer wg.Done()
		for range 100 {
			Info(context.Background(), "bare")
		}
	}()
	wg.Wait()
}

func TestResolve_StackTraceFromNilDefaultsToError(t *testing.T) {
	var out = captureOutput(Config{Level: LevelDebug, Format: JSON}, func(ctx context.Context) {
		Error(ctx, "boom")
	})
	if parseJSON(t, out)["stack_trace"] == nil {
		t.Error("nil StackTraceFrom should default to LevelError, so Error gets a stack")
	}
}

func TestResolve_StackTraceFromDebugIsConfigurable(t *testing.T) {
	var out = captureOutput(Config{Level: LevelDebug, Format: JSON, StackTraceFrom: StackFrom(LevelDebug)}, func(ctx context.Context) {
		Debug(ctx, "trace me")
	})
	if parseJSON(t, out)["stack_trace"] == nil {
		t.Error("StackTraceFrom: LevelDebug should put a stack on Debug logs")
	}
}

func TestResolve_ScopeIsFirstClass(t *testing.T) {
	var out = captureOutput(Config{Level: LevelInfo, Format: JSON}, func(ctx context.Context) {
		ctx = WithScope(ctx, "engine")
		ctx = WithFields(ctx, map[string]any{"scope": "user-value"})
		Info(ctx, "msg")
	})

	var parsed = parseJSON(t, out)
	if parsed["scope"] != "engine" {
		t.Errorf("logger scope should be top-level 'engine', got %v", parsed["scope"])
	}
	var data = parsed["data"].(map[string]any)
	if data["scope"] != "user-value" {
		t.Errorf("a user field named 'scope' must survive in data, got %v", data["scope"])
	}
}

func TestResolve_ScopeInHumanHeader(t *testing.T) {
	var e = entry{Timestamp: time.Now(), Level: LevelInfo, Message: "started", Scope: "engine.ingestion"}
	var r = formatHuman(e)
	if !strings.Contains(r, "[engine.ingestion]") {
		t.Errorf("expected scope segment in human header, got: %s", r)
	}
}

func TestResolve_FatalAlwaysEmits(t *testing.T) {
	var origExit = exit
	exit = func(int) {}
	defer func() { exit = origExit }()

	var buf bytes.Buffer
	var ctx = Attach(context.Background(), Config{Level: LevelFatal + 1, Format: HumanReadable})
	ctx = SuppressConsoleOutput(ctx)
	ctx = AttachWriter(ctx, &buf)

	Fatal(ctx, "dying")

	if !strings.Contains(buf.String(), "dying") {
		t.Error("Fatal must emit even when Level is configured above LevelFatal")
	}
}
