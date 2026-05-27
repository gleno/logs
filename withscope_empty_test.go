package logs

import (
	"context"
	"testing"
)

func TestWithScope_EmptyArgNoTrailingDot(t *testing.T) {
	var ctx = Attach(context.Background(), Config{Level: LevelInfo})
	ctx = WithScope(ctx, "engine")
	ctx = WithScope(ctx, "")

	var s = testState(ctx)
	if s.scope != "engine" {
		t.Errorf("expected scope engine, got %q", s.scope)
	}
}
