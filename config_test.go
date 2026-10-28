package logs

import (
	"context"
	"errors"
	"sync"
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
	var original = defaultState.formatter()
	defer SetDefaultFormat(original)

	SetDefaultFormat(GCPLogFormatter{})
	if _, ok := defaultState.formatter().(GCPLogFormatter); !ok {
		t.Errorf("expected default formatter GCPLogFormatter, got %T", defaultState.formatter())
	}
}

func TestSetFormat_ReturnsContextWithNewFormat(t *testing.T) {
	var ctx = Attach(context.Background(), Config{Level: LevelInfo, Formatter: HumanLogFormatter{}})
	var child = SetFormat(ctx, GCPLogFormatter{})

	if _, ok := testState(child).config.Formatter.(GCPLogFormatter); !ok {
		t.Errorf("expected child formatter GCPLogFormatter, got %T", testState(child).config.Formatter)
	}
}

func TestSetFormat_DoesNotMutateParent(t *testing.T) {
	var ctx = Attach(context.Background(), Config{Level: LevelInfo, Formatter: HumanLogFormatter{}})
	SetFormat(ctx, GCPLogFormatter{})

	if _, ok := testState(ctx).config.Formatter.(HumanLogFormatter); !ok {
		t.Errorf("SetFormat mutated parent state: parent formatter is now %T", testState(ctx).config.Formatter)
	}
}

func TestSetFormat_DoesNotMutateDefaultState(t *testing.T) {
	var original = defaultState.formatter()
	defer SetDefaultFormat(original)

	SetFormat(context.Background(), GCPLogFormatter{})

	if defaultState.formatter() != original {
		t.Errorf("SetFormat on bare context mutated global default formatter to %v", defaultState.formatter())
	}
}

func TestFlush_SyncsConsoleAndWriters(t *testing.T) {
	var console = &fakeSyncer{}
	var writer = &fakeSyncer{}
	var s = &state{console: &sink{w: console}, writers: []*sink{{w: writer}}}
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
	var s = &state{console: &sink{w: console}}
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
		console: &sink{w: &fakeSyncer{}},
		writers: []*sink{{w: &fakeSyncer{syncErr: wantErr}}},
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

func TestParseLevel_AcceptsEveryLevelInAnyCase(t *testing.T) {
	var cases = map[string]Level{
		"debug": LevelDebug, "INFO": LevelInfo, " Warn ": LevelWarning, "warning": LevelWarning,
		"Error": LevelError, "fatal": LevelFatal,
	}
	for text, want := range cases {
		got, err := ParseLevel(text)
		if err != nil || got != want {
			t.Errorf("ParseLevel(%q) = %v, %v; want %v", text, got, err, want)
		}
	}
}

func TestParseLevel_RefusesAnUnknownLevel(t *testing.T) {
	if _, err := ParseLevel("loud"); err == nil {
		t.Error("expected an error for an unknown level")
	}
}

func TestEnabled_FollowsTheContextLevel(t *testing.T) {
	var ctx = Attach(context.Background(), Config{Level: LevelWarning})
	if Enabled(ctx, LevelInfo) {
		t.Error("expected info disabled at warning level")
	}
	if !Enabled(ctx, LevelWarning) || !Enabled(ctx, LevelError) {
		t.Error("expected warning and above enabled at warning level")
	}
}

func TestEnabled_IsFalseWhenAllOutputIsSuppressed(t *testing.T) {
	if Enabled(SuppressAllOutput(context.Background()), LevelFatal) {
		t.Error("expected nothing enabled when all output is suppressed")
	}
}

func TestEnabled_FatalBypassesLevelFilter(t *testing.T) {
	var ctx = Attach(context.Background(), Config{Level: LevelFatal + 1})
	if !Enabled(ctx, LevelFatal) || Enabled(ctx, LevelError) {
		t.Fatal("only fatal should bypass a level above fatal")
	}
}

func TestSetDefaultFormat_IsSafeBesideConcurrentLogging(t *testing.T) {
	var original = defaultState.formatter()
	defer SetDefaultFormat(original)

	var ctx = SuppressConsoleOutput(context.Background())
	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(2)
		go func() {
			defer wg.Done()
			SetDefaultFormat(GCPLogFormatter{})
			SetDefaultFormat(HumanLogFormatter{})
		}()
		go func() {
			defer wg.Done()
			Info(ctx, "concurrent")
			_ = Enabled(nil, LevelInfo)
		}()
	}
	wg.Wait()
}

func TestSetDefaultFormat_LeavesAttachedContextsAlone(t *testing.T) {
	var original = defaultState.formatter()
	defer SetDefaultFormat(original)

	var ctx = SetFormat(context.Background(), HumanLogFormatter{})
	SetDefaultFormat(GCPLogFormatter{})
	if getState(ctx).formatter() != (HumanLogFormatter{}) {
		t.Error("expected a context's own format to survive a new default")
	}
}
