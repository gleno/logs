package logs

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"
)

func captureOutput(config Config, fn func(ctx context.Context)) string {
	var buf bytes.Buffer
	var ctx = Attach(context.Background(), config)
	ctx = SuppressConsoleOutput(ctx)
	ctx = AttachWriter(ctx, &buf)
	fn(ctx)
	return buf.String()
}

func TestInfo_WritesToWriter(t *testing.T) {
	var output = captureOutput(Config{Level: LevelInfo, Format: HumanReadable}, func(ctx context.Context) {
		Info(ctx, "hello world")
	})
	if !strings.Contains(output, "hello world") {
		t.Errorf("expected 'hello world' in output, got: %s", output)
	}
	if !strings.Contains(output, "INFO") {
		t.Errorf("expected INFO level, got: %s", output)
	}
}

func TestInfof_FormatsMessage(t *testing.T) {
	var output = captureOutput(Config{Level: LevelInfo, Format: HumanReadable}, func(ctx context.Context) {
		Infof(ctx, "batch %s started with %d items", "abc", 42)
	})
	if !strings.Contains(output, "batch abc started with 42 items") {
		t.Errorf("expected formatted message, got: %s", output)
	}
}

func TestDebug_FilteredAtInfoLevel(t *testing.T) {
	var output = captureOutput(Config{Level: LevelInfo, Format: HumanReadable}, func(ctx context.Context) {
		Debug(ctx, "should not appear")
	})
	if strings.Contains(output, "should not appear") {
		t.Errorf("debug message should be filtered at info level, got: %s", output)
	}
}

func TestDebug_ShownAtDebugLevel(t *testing.T) {
	var output = captureOutput(Config{Level: LevelDebug, Format: HumanReadable}, func(ctx context.Context) {
		Debug(ctx, "visible debug")
	})
	if !strings.Contains(output, "visible debug") {
		t.Errorf("expected debug message at debug level, got: %s", output)
	}
}

func TestError_WritesAtErrorLevel(t *testing.T) {
	var output = captureOutput(Config{Level: LevelInfo, Format: HumanReadable}, func(ctx context.Context) {
		Error(ctx, "something broke")
	})
	if !strings.Contains(output, "ERROR") {
		t.Errorf("expected ERROR level, got: %s", output)
	}
	if !strings.Contains(output, "something broke") {
		t.Errorf("expected error message, got: %s", output)
	}
}

func TestWarn_WritesAtWarnLevel(t *testing.T) {
	var output = captureOutput(Config{Level: LevelInfo, Format: HumanReadable}, func(ctx context.Context) {
		Warn(ctx, "caution")
	})
	if !strings.Contains(output, "WARNING") {
		t.Errorf("expected WARNING level, got: %s", output)
	}
}

func TestErr_CapturesError(t *testing.T) {
	var output = captureOutput(Config{Level: LevelInfo, Format: JSON}, func(ctx context.Context) {
		Err(ctx, fmt.Errorf("connection refused"))
	})

	var parsed map[string]any
	if err := json.Unmarshal([]byte(output), &parsed); err != nil {
		t.Fatalf("invalid JSON: %v\noutput: %s", err, output)
	}
	if parsed["error"] != "connection refused" {
		t.Errorf("expected error field, got %v", parsed["error"])
	}
	if parsed["severity"] != "ERROR" {
		t.Errorf("expected ERROR severity, got %v", parsed["severity"])
	}
}

func TestErrf_CapturesErrorWithContext(t *testing.T) {
	var output = captureOutput(Config{Level: LevelInfo, Format: JSON}, func(ctx context.Context) {
		Errf(ctx, fmt.Errorf("timeout"), "batch %s failed", "abc")
	})

	var parsed map[string]any
	if err := json.Unmarshal([]byte(output), &parsed); err != nil {
		t.Fatalf("invalid JSON: %v\noutput: %s", err, output)
	}
	if parsed["error"] != "timeout" {
		t.Errorf("expected error=timeout, got %v", parsed["error"])
	}
	if parsed["message"] != "batch abc failed" {
		t.Errorf("expected formatted message, got %v", parsed["message"])
	}
}

func TestWithFields_AppearsInOutput(t *testing.T) {
	var output = captureOutput(Config{Level: LevelInfo, Format: JSON}, func(ctx context.Context) {
		ctx = WithFields(ctx, map[string]any{"batch": "xyz"})
		Info(ctx, "processing")
	})

	var parsed map[string]any
	if err := json.Unmarshal([]byte(output), &parsed); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	data, ok := parsed["data"].(map[string]any)
	if !ok {
		t.Fatalf("expected data field, got %T", parsed["data"])
	}
	if data["batch"] != "xyz" {
		t.Errorf("expected batch=xyz, got %v", data["batch"])
	}
}

func TestSuppressConsoleOutput_WritersStillReceive(t *testing.T) {
	var buf bytes.Buffer
	var ctx = Attach(context.Background(), Config{Level: LevelInfo, Format: HumanReadable})
	ctx = SuppressConsoleOutput(ctx)
	ctx = AttachWriter(ctx, &buf)

	Info(ctx, "writer only")
	if !strings.Contains(buf.String(), "writer only") {
		t.Errorf("attached writer should receive output, got: %s", buf.String())
	}
}

func TestSuppressAllOutput_NothingWritten(t *testing.T) {
	var buf bytes.Buffer
	var ctx = Attach(context.Background(), Config{Level: LevelInfo, Format: HumanReadable})
	ctx = AttachWriter(ctx, &buf)
	ctx = SuppressAllOutput(ctx)

	Info(ctx, "should be silent")
	if buf.Len() > 0 {
		t.Errorf("expected no output with SuppressAllOutput, got: %s", buf.String())
	}
}

func TestMultipleWriters_AllReceive(t *testing.T) {
	var buf1, buf2 bytes.Buffer
	var ctx = Attach(context.Background(), Config{Level: LevelInfo, Format: HumanReadable})
	ctx = SuppressConsoleOutput(ctx)
	ctx = AttachWriter(ctx, &buf1)
	ctx = AttachWriter(ctx, &buf2)

	Info(ctx, "broadcast")
	if !strings.Contains(buf1.String(), "broadcast") {
		t.Error("first writer should receive output")
	}
	if !strings.Contains(buf2.String(), "broadcast") {
		t.Error("second writer should receive output")
	}
}

func TestJSON_Output(t *testing.T) {
	var output = captureOutput(Config{Level: LevelInfo, Format: JSON}, func(ctx context.Context) {
		Info(ctx, "json test")
	})

	var parsed map[string]any
	if err := json.Unmarshal([]byte(output), &parsed); err != nil {
		t.Fatalf("invalid JSON output: %v\noutput: %s", err, output)
	}
	if parsed["message"] != "json test" {
		t.Errorf("expected message 'json test', got %v", parsed["message"])
	}
}

func TestCallerInfo_AppearsInOutput(t *testing.T) {
	var output = captureOutput(Config{Level: LevelInfo, Format: JSON}, func(ctx context.Context) {
		Info(ctx, "caller test")
	})

	var parsed map[string]any
	if err := json.Unmarshal([]byte(output), &parsed); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	data, ok := parsed["data"].(map[string]any)
	if !ok {
		t.Fatalf("expected jsonPayload.data to be an object, got %T", parsed["data"])
	}
	code, ok := data["@code"].(string)
	if !ok || code == "" {
		t.Error("expected non-empty @code field in data")
	}
	if !strings.Contains(code, "log_test.go") {
		t.Errorf("expected @code to reference log_test.go, got: %s", code)
	}
}

func TestStackTrace_IncludedAtErrorLevel(t *testing.T) {
	var output = captureOutput(Config{
		Level:          LevelInfo,
		Format:         JSON,
		StackTraceFrom: LevelError,
	}, func(ctx context.Context) {
		Error(ctx, "with stack")
	})

	var parsed map[string]any
	if err := json.Unmarshal([]byte(output), &parsed); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if parsed["stack_trace"] == nil {
		t.Error("expected stack_trace at error level when StackTraceFrom=LevelError")
	}
}

func TestStackTrace_NotIncludedBelowThreshold(t *testing.T) {
	var output = captureOutput(Config{
		Level:          LevelInfo,
		Format:         JSON,
		StackTraceFrom: LevelError,
	}, func(ctx context.Context) {
		Warn(ctx, "no stack")
	})

	var parsed map[string]any
	if err := json.Unmarshal([]byte(output), &parsed); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if parsed["stack_trace"] != nil {
		t.Error("expected no stack_trace below StackTraceFrom threshold")
	}
}

func TestErr_NilError_NoOutput(t *testing.T) {
	var output = captureOutput(Config{Level: LevelInfo, Format: JSON}, func(ctx context.Context) {
		Err(ctx, nil)
	})
	if output != "" {
		t.Errorf("expected no output for nil error, got: %s", output)
	}
}

func TestWithScope_AppearsInOutput(t *testing.T) {
	var output = captureOutput(Config{Level: LevelInfo, Format: JSON}, func(ctx context.Context) {
		ctx = WithScope(ctx, "engine")
		ctx = WithScope(ctx, "ingestion")
		Info(ctx, "scoped msg")
	})

	var parsed map[string]any
	if err := json.Unmarshal([]byte(output), &parsed); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	data, _ := parsed["data"].(map[string]any)
	if data["scope"] != "engine.ingestion" {
		t.Errorf("expected scope engine.ingestion, got %v", data["scope"])
	}
}
