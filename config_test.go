package logs

import (
	"context"
	"errors"
	"io"
	"testing"
)

type fakeSyncer struct {
	syncErr error
	synced  bool
}

func (f *fakeSyncer) Write(p []byte) (int, error) { return len(p), nil }

func (f *fakeSyncer) Sync() error {
	f.synced = true
	return f.syncErr
}

func TestLevel_String_Unknown(t *testing.T) {
	if got := Level(99).String(); got != "UNKNOWN" {
		t.Errorf("expected UNKNOWN for out-of-range level, got %s", got)
	}
}

func TestSetDefaultFormat_MutatesDefaultState(t *testing.T) {
	var original = defaultState.config.Format
	defer SetDefaultFormat(original)

	SetDefaultFormat(JSON)
	if defaultState.config.Format != JSON {
		t.Errorf("expected default format JSON, got %v", defaultState.config.Format)
	}
}

func TestSetFormat_ReturnsContextWithNewFormat(t *testing.T) {
	var ctx = Attach(context.Background(), Config{Level: LevelInfo, Format: HumanReadable})
	var child = SetFormat(ctx, JSON)

	if testState(child).config.Format != JSON {
		t.Errorf("expected child format JSON, got %v", testState(child).config.Format)
	}
}

func TestSetFormat_DoesNotMutateParent(t *testing.T) {
	var ctx = Attach(context.Background(), Config{Level: LevelInfo, Format: HumanReadable})
	SetFormat(ctx, JSON)

	if testState(ctx).config.Format != HumanReadable {
		t.Errorf("SetFormat mutated parent state: parent format is now %v", testState(ctx).config.Format)
	}
}

func TestSetFormat_DoesNotMutateDefaultState(t *testing.T) {
	var original = defaultState.config.Format
	defer func() { defaultState.config.Format = original }()

	SetFormat(context.Background(), JSON)

	if defaultState.config.Format != original {
		t.Errorf("SetFormat on bare context mutated global default state to %v", defaultState.config.Format)
	}
}

func TestFlush_SyncsConsoleAndWriters(t *testing.T) {
	var console = &fakeSyncer{}
	var writer = &fakeSyncer{}
	var s = &state{console: console, writers: []io.Writer{writer}}
	var ctx = context.WithValue(context.Background(), stateKey, s)

	if err := Flush(ctx); err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if !console.synced {
		t.Error("expected console to be synced")
	}
	if !writer.synced {
		t.Error("expected writer to be synced")
	}
}

func TestFlush_IgnoresConsoleSyncError(t *testing.T) {
	var console = &fakeSyncer{syncErr: errors.New("sync /dev/stderr: operation not supported by device")}
	var s = &state{console: console}
	var ctx = context.WithValue(context.Background(), stateKey, s)

	if err := Flush(ctx); err != nil {
		t.Errorf("console is a best-effort display sink; its sync error should not propagate, got %v", err)
	}
	if !console.synced {
		t.Error("expected console sync to still be attempted")
	}
}

func TestFlush_ReturnsWriterSyncError(t *testing.T) {
	var wantErr = errors.New("writer sync failed")
	var s = &state{
		console: &fakeSyncer{},
		writers: []io.Writer{&fakeSyncer{syncErr: wantErr}},
	}
	var ctx = context.WithValue(context.Background(), stateKey, s)

	if err := Flush(ctx); !errors.Is(err, wantErr) {
		t.Errorf("expected writer sync error, got %v", err)
	}
}

func TestFlush_IgnoresNonSyncers(t *testing.T) {
	var ctx = Attach(context.Background(), Config{Level: LevelInfo})
	ctx = SuppressConsoleOutput(ctx)
	ctx = AttachWriter(ctx, &nopWriter{})

	if err := Flush(ctx); err != nil {
		t.Errorf("expected no error when nothing implements syncer, got %v", err)
	}
}

type nopWriter struct{}

func (nopWriter) Write(p []byte) (int, error) { return len(p), nil }
