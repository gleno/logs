package logs

import (
	"bytes"
	"context"
	"strings"
	"testing"
)

func redactKeysTo(replacement any, keys ...string) RedactFunc {
	var set = make(map[string]bool, len(keys))
	for _, k := range keys {
		set[k] = true
	}
	return func(key string, value any) (any, bool) {
		if set[key] {
			return replacement, true
		}
		return value, true
	}
}

func TestRedaction_MaskAppliesToBothFormats(t *testing.T) {
	var fields = map[string]any{"token": "secret", "user": "u1"}

	var jsonOut = captureOutput(Config{Level: LevelInfo, Format: JSON}, func(ctx context.Context) {
		ctx = WithRedaction(ctx, redactKeysTo("[REDACTED]", "token"))
		ctx = WithFields(ctx, fields)
		Info(ctx, "msg")
	})
	var data = parseJSON(t, jsonOut)["data"].(map[string]any)
	if data["token"] != "[REDACTED]" {
		t.Errorf("JSON: token not masked, got %v", data["token"])
	}
	if data["user"] != "u1" {
		t.Errorf("JSON: non-redacted field changed, got %v", data["user"])
	}

	var humanOut = captureOutput(Config{Level: LevelInfo, Format: HumanReadable}, func(ctx context.Context) {
		ctx = WithRedaction(ctx, redactKeysTo("[REDACTED]", "token"))
		ctx = WithFields(ctx, fields)
		Info(ctx, "msg")
	})
	if strings.Contains(humanOut, "secret") {
		t.Errorf("human: leaked the secret value: %s", humanOut)
	}
	if !strings.Contains(humanOut, "[REDACTED]") {
		t.Errorf("human: expected masked value, got: %s", humanOut)
	}
}

func TestRedaction_KeepFalseOmitsField(t *testing.T) {
	var dropToken = func(key string, value any) (any, bool) {
		if key == "token" {
			return nil, false
		}
		return value, true
	}
	var out = captureOutput(Config{Level: LevelInfo, Format: JSON}, func(ctx context.Context) {
		ctx = WithRedaction(ctx, dropToken)
		ctx = WithFields(ctx, map[string]any{"token": "secret", "user": "u1"})
		Info(ctx, "msg")
	})
	var data = parseJSON(t, out)["data"].(map[string]any)
	if _, exists := data["token"]; exists {
		t.Errorf("token should be omitted, got %v", data["token"])
	}
	if data["user"] != "u1" {
		t.Errorf("kept field lost: %v", data["user"])
	}
}

func TestRedaction_PassThroughPreservesType(t *testing.T) {
	var passAll = func(key string, value any) (any, bool) { return value, true }
	var out = captureOutput(Config{Level: LevelInfo, Format: JSON}, func(ctx context.Context) {
		ctx = WithRedaction(ctx, passAll)
		ctx = WithFields(ctx, map[string]any{"count": 42})
		Info(ctx, "msg")
	})
	var data = parseJSON(t, out)["data"].(map[string]any)
	if data["count"] != float64(42) {
		t.Errorf("passed-through number should stay a JSON number, got %T %v", data["count"], data["count"])
	}
}

func TestRedaction_OnlySeesUserFields(t *testing.T) {
	var seen = map[string]bool{}
	var record = func(key string, value any) (any, bool) {
		seen[key] = true
		return value, true
	}
	var out = captureOutput(Config{Level: LevelInfo, Format: JSON}, func(ctx context.Context) {
		ctx = WithScope(ctx, "engine")
		ctx = WithRedaction(ctx, record)
		ctx = WithFields(ctx, map[string]any{"user": "u1"})
		Info(ctx, "the message")
	})

	var parsed = parseJSON(t, out)
	if parsed["scope"] != "engine" {
		t.Errorf("scope must not be redacted, got %v", parsed["scope"])
	}
	if parsed["message"] != "the message" {
		t.Errorf("message must not be redacted, got %v", parsed["message"])
	}
	if !seen["user"] {
		t.Error("redact func should be called for the user field")
	}
	for _, k := range []string{"@code", "scope", "message"} {
		if seen[k] {
			t.Errorf("redact func should never see %q", k)
		}
	}
}

func TestRedaction_NilIsNoOp(t *testing.T) {
	var out = captureOutput(Config{Level: LevelInfo, Format: JSON}, func(ctx context.Context) {
		ctx = WithFields(ctx, map[string]any{"a": "b"})
		Info(ctx, "msg")
	})
	var data = parseJSON(t, out)["data"].(map[string]any)
	if data["a"] != "b" {
		t.Errorf("no redaction should pass fields through, got %v", data["a"])
	}
}

func TestRedaction_PanicDoesNotCrash(t *testing.T) {
	var panicFn = func(key string, value any) (any, bool) { panic("redact boom") }
	var buf bytes.Buffer
	var ctx = Attach(context.Background(), Config{Level: LevelInfo})
	ctx = SuppressConsoleOutput(ctx)
	ctx = AttachWriter(ctx, &buf)
	ctx = WithRedaction(ctx, panicFn)
	ctx = WithFields(ctx, map[string]any{"x": "y"})

	Info(ctx, "first")
	Info(ctx, "second")
}

func TestRedaction_SetButNoFieldsIsSafe(t *testing.T) {
	var out = captureOutput(Config{Level: LevelInfo, Format: JSON}, func(ctx context.Context) {
		ctx = WithRedaction(ctx, func(key string, value any) (any, bool) { return value, true })
		Info(ctx, "no fields here")
	})
	var parsed = parseJSON(t, out)
	if parsed["message"] != "no fields here" {
		t.Errorf("message lost with redaction set and no fields, got %v", parsed["message"])
	}
}

func TestRedaction_NoBuiltInHiddenFields(t *testing.T) {
	var out = captureOutput(Config{Level: LevelInfo, Format: HumanReadable}, func(ctx context.Context) {
		ctx = WithFields(ctx, map[string]any{"app": "myapp"})
		Info(ctx, "msg")
	})
	if !strings.Contains(out, "app=") {
		t.Errorf("with hiddenFields removed, 'app' should render by default, got: %s", out)
	}
}
