package logs

import (
	"bytes"
	"context"
	"testing"
)

func testState(ctx context.Context) *state {
	return getState(ctx)
}

func TestWithFields_AccumulatesFields(t *testing.T) {
	var ctx = Attach(context.Background(), Config{Level: LevelInfo})
	ctx = WithFields(ctx, map[string]any{"a": 1})
	ctx = WithFields(ctx, map[string]any{"b": "two"})

	var s = testState(ctx)
	if s.fields["a"] != 1 {
		t.Errorf("expected field a=1, got %v", s.fields["a"])
	}
	if s.fields["b"] != "two" {
		t.Errorf("expected field b=two, got %v", s.fields["b"])
	}
}

func TestWithFields_DoesNotMutateParent(t *testing.T) {
	var ctx = Attach(context.Background(), Config{Level: LevelInfo})
	ctx = WithFields(ctx, map[string]any{"a": 1})

	var child = WithFields(ctx, map[string]any{"b": 2})

	var parentState = testState(ctx)
	var childState = testState(child)

	if parentState.fields["b"] != nil {
		t.Errorf("parent should not have field b, got %v", parentState.fields["b"])
	}
	if childState.fields["a"] != 1 {
		t.Errorf("child should inherit field a=1, got %v", childState.fields["a"])
	}
	if childState.fields["b"] != 2 {
		t.Errorf("child should have field b=2, got %v", childState.fields["b"])
	}
}

func TestWithFields_FiltersNilValues(t *testing.T) {
	var ctx = Attach(context.Background(), Config{Level: LevelInfo})
	ctx = WithFields(ctx, map[string]any{"a": 1, "b": nil})

	var s = testState(ctx)
	if s.fields["a"] != 1 {
		t.Errorf("expected field a=1, got %v", s.fields["a"])
	}
	if _, exists := s.fields["b"]; exists {
		t.Error("expected nil field b to be filtered out")
	}
}

func TestWithFields_EmptyFieldsReturnsSameContext(t *testing.T) {
	var ctx = Attach(context.Background(), Config{Level: LevelInfo})
	var ctx2 = WithFields(ctx, map[string]any{})
	var ctx3 = WithFields(ctx, nil)

	if testState(ctx) != testState(ctx2) {
		t.Error("empty fields should return same state")
	}
	if testState(ctx) != testState(ctx3) {
		t.Error("nil fields should return same state")
	}
}

func TestWithScope_BuildsDotSeparatedPath(t *testing.T) {
	var ctx = Attach(context.Background(), Config{Level: LevelInfo})
	ctx = WithScope(ctx, "engine")
	ctx = WithScope(ctx, "ingestion")

	var s = testState(ctx)
	if s.scope != "engine.ingestion" {
		t.Errorf("expected scope engine.ingestion, got %s", s.scope)
	}
}

func TestWithScope_AddsToFields(t *testing.T) {
	var ctx = Attach(context.Background(), Config{Level: LevelInfo})
	ctx = WithScope(ctx, "engine")

	var s = testState(ctx)
	if s.fields["scope"] != "engine" {
		t.Errorf("expected scope field, got %v", s.fields["scope"])
	}
}

func TestSuppressConsoleOutput(t *testing.T) {
	var ctx = Attach(context.Background(), Config{Level: LevelInfo})
	ctx = SuppressConsoleOutput(ctx)

	var s = testState(ctx)
	if s.console != nil {
		t.Error("expected console to be nil after suppression")
	}
}

func TestAttachWriter(t *testing.T) {
	var ctx = Attach(context.Background(), Config{Level: LevelInfo})
	var buf bytes.Buffer
	ctx = AttachWriter(ctx, &buf)

	var s = testState(ctx)
	if len(s.writers) != 1 {
		t.Errorf("expected 1 writer, got %d", len(s.writers))
	}
}

func TestAttachWriter_Multiple(t *testing.T) {
	var ctx = Attach(context.Background(), Config{Level: LevelInfo})
	var buf1, buf2 bytes.Buffer
	ctx = AttachWriter(ctx, &buf1)
	ctx = AttachWriter(ctx, &buf2)

	var s = testState(ctx)
	if len(s.writers) != 2 {
		t.Errorf("expected 2 writers, got %d", len(s.writers))
	}
}

func TestSuppressAllOutput(t *testing.T) {
	var ctx = Attach(context.Background(), Config{Level: LevelInfo})
	var buf bytes.Buffer
	ctx = AttachWriter(ctx, &buf)
	ctx = SuppressAllOutput(ctx)

	var s = testState(ctx)
	if !s.suppressAll {
		t.Error("expected suppressAll to be true")
	}
}

func TestNoAttachFallback(t *testing.T) {
	var ctx = context.Background()
	var s = testState(ctx)
	if s == nil {
		t.Fatal("expected default state for bare context")
	}
	if s.console == nil {
		t.Error("expected default console writer")
	}
	if s.config.Level != LevelInfo {
		t.Errorf("expected default level Info, got %v", s.config.Level)
	}
}

func TestSuppressConsoleOutput_WritersStillWork(t *testing.T) {
	var ctx = Attach(context.Background(), Config{Level: LevelInfo})
	var buf bytes.Buffer
	ctx = AttachWriter(ctx, &buf)
	ctx = SuppressConsoleOutput(ctx)

	var s = testState(ctx)
	if s.console != nil {
		t.Error("console should be suppressed")
	}
	if len(s.writers) != 1 {
		t.Error("attached writer should still be present")
	}
}

func TestGetState_NilContext(t *testing.T) {
	var s = getState(nil)
	if s == nil {
		t.Fatal("expected default state for nil context")
	}
}
