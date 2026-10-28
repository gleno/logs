package logs

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"strings"
	"testing"
	"time"
)

func TestSlog_ReworkPipelinePreservesRecordAndRedactsAllFields(t *testing.T) {
	var got Entry
	var ctx = Attach(context.Background(), Config{
		Level: LevelDebug,
		Formatter: funcFormatter(func(e Entry) []byte {
			got = e
			return nil
		}),
		StackTraceFrom: StackFrom(LevelDebug),
	})
	ctx = SuppressConsoleOutput(WithScope(ctx, "service"))
	ctx = WithFields(ctx, map[string]any{"context_secret": "hidden", "request": "r1"})
	ctx = WithRedaction(ctx, func(key string, value any) (any, bool) {
		return value, !strings.HasSuffix(key, "secret")
	})
	var timestamp = time.Unix(123, 0)
	var record = slog.NewRecord(timestamp, slog.LevelInfo, "message", 0)
	record.Add("record_secret", "hidden", "count", 3)
	var handler = NewSlogHandler(ctx).WithAttrs([]slog.Attr{slog.String("handler_secret", "hidden")})
	if err := handler.Handle(context.Background(), record); err != nil {
		t.Fatal(err)
	}
	if got.Timestamp != timestamp || got.Message != "message" || got.Scope != "service" || got.Stack == "" {
		t.Fatalf("record metadata, scope, or configured stack was lost: %+v", got)
	}
	if len(got.Fields) != 2 || got.Fields["request"] != "r1" || got.Fields["count"] != int64(3) {
		t.Fatalf("expected context, handler, and record fields to share redaction: %v", got.Fields)
	}
}

func TestSlog_ConcurrentWritesUseSharedSink(t *testing.T) {
	var buf bytes.Buffer
	var ctx = Attach(context.Background(), Config{Formatter: GCPLogFormatter{}})
	ctx = AttachWriter(SuppressConsoleOutput(ctx), &buf)
	var logger = slog.New(NewSlogHandler(ctx))
	var done = make(chan struct{}, 20)
	for i := 0; i < cap(done); i++ {
		go func() {
			logger.Info("concurrent", "count", 1)
			done <- struct{}{}
		}()
	}
	for i := 0; i < cap(done); i++ {
		<-done
	}
	var lines = strings.Split(strings.TrimSpace(buf.String()), "\n")
	if len(lines) != cap(done) {
		t.Fatalf("expected %d complete records, got %d", cap(done), len(lines))
	}
	for _, line := range lines {
		if !json.Valid([]byte(line)) {
			t.Fatalf("torn JSON record: %s", line)
		}
	}
}

func captureSlogJSON(t *testing.T, config Config, fn func(logger *slog.Logger)) map[string]any {
	t.Helper()
	config.Formatter = GCPLogFormatter{}
	var output = captureOutput(config, func(ctx context.Context) {
		fn(slog.New(NewSlogHandler(ctx)))
	})
	if output == "" {
		return nil
	}
	var decoded map[string]any
	if err := json.Unmarshal([]byte(strings.TrimSpace(output)), &decoded); err != nil {
		t.Fatalf("output is not one JSON entry: %v\n%s", err, output)
	}
	return decoded
}

func dataOf(t *testing.T, decoded map[string]any) map[string]any {
	t.Helper()
	var data, ok = decoded["data"].(map[string]any)
	if !ok {
		t.Fatalf("expected a data object, got %v", decoded)
	}
	return data
}

func TestSlog_MapsLevels(t *testing.T) {
	var cases = []struct {
		level slog.Level
		want  string
	}{
		{slog.LevelDebug, "DEBUG"},
		{slog.LevelInfo, "INFO"},
		{slog.LevelWarn, "WARNING"},
		{slog.LevelError, "ERROR"},
		{slog.LevelError + 4, "ERROR"},
	}
	for _, c := range cases {
		var decoded = captureSlogJSON(t, Config{Level: LevelDebug, StackTraceFrom: StackFrom(LevelFatal)}, func(logger *slog.Logger) {
			logger.Log(context.Background(), c.level, "m")
		})
		if decoded["severity"] != c.want {
			t.Errorf("slog level %v: expected severity %s, got %v", c.level, c.want, decoded["severity"])
		}
	}
}

func TestSlog_RecordsBelowTheContextLevelAreDropped(t *testing.T) {
	var decoded = captureSlogJSON(t, Config{Level: LevelWarning}, func(logger *slog.Logger) {
		logger.Info("quiet")
	})
	if decoded != nil {
		t.Errorf("expected nothing below the context level, got %v", decoded)
	}
}

func TestSlog_EnabledFollowsTheContext(t *testing.T) {
	var ctx = Attach(context.Background(), Config{Level: LevelWarning})
	var handler = NewSlogHandler(ctx)
	if handler.Enabled(context.Background(), slog.LevelInfo) {
		t.Error("expected info to be disabled at warning level")
	}
	if !handler.Enabled(context.Background(), slog.LevelWarn) {
		t.Error("expected warn to be enabled at warning level")
	}
	if NewSlogHandler(SuppressAllOutput(ctx)).Enabled(context.Background(), slog.LevelError) {
		t.Error("expected nothing enabled when all output is suppressed")
	}
}

func TestSlog_AttrsBecomeFieldsBesideTheContextFields(t *testing.T) {
	var output = captureOutput(Config{Level: LevelInfo, Formatter: GCPLogFormatter{}}, func(ctx context.Context) {
		ctx = WithFields(ctx, map[string]any{"request": "r1"})
		slog.New(NewSlogHandler(ctx)).With("component", "db").Info("m", "rows", 3)
	})
	var decoded map[string]any
	if err := json.Unmarshal([]byte(output), &decoded); err != nil {
		t.Fatal(err)
	}
	var data = dataOf(t, decoded)
	if data["request"] != "r1" || data["component"] != "db" || data["rows"] != float64(3) {
		t.Errorf("expected context fields, handler attrs and record attrs together, got %v", data)
	}
}

func TestSlog_GroupsQualifyKeys(t *testing.T) {
	var decoded = captureSlogJSON(t, Config{Level: LevelInfo}, func(logger *slog.Logger) {
		logger.WithGroup("db").With("pool", "main").Info("m", slog.Group("query", "ms", 12), "table", "runs")
	})
	var data = dataOf(t, decoded)
	for key, want := range map[string]any{"db.pool": "main", "db.query.ms": float64(12), "db.table": "runs"} {
		if data[key] != want {
			t.Errorf("expected %s=%v, got %v (all: %v)", key, want, data[key], data)
		}
	}
}

func TestSlog_ErrorValuesAreTheirMessage(t *testing.T) {
	var decoded = captureSlogJSON(t, Config{Level: LevelInfo, StackTraceFrom: StackFrom(LevelFatal)}, func(logger *slog.Logger) {
		logger.Error("failed", "err", errors.New("boom"))
	})
	if got := dataOf(t, decoded)["err"]; got != "boom" {
		t.Errorf("expected the error's message, got %v", got)
	}
}

type secretValuer struct{}

func (secretValuer) LogValue() slog.Value { return slog.StringValue("redacted") }

func TestSlog_LogValuersAreResolved(t *testing.T) {
	var decoded = captureSlogJSON(t, Config{Level: LevelInfo}, func(logger *slog.Logger) {
		logger.Info("m", "token", secretValuer{})
	})
	if got := dataOf(t, decoded)["token"]; got != "redacted" {
		t.Errorf("expected the resolved value, got %v", got)
	}
}

func TestSlog_CallerIsTheSlogCallSite(t *testing.T) {
	var decoded = captureSlogJSON(t, Config{Level: LevelInfo}, func(logger *slog.Logger) {
		logger.Info("m")
	})
	var location, _ = decoded[gcpSourceLocationKey].(map[string]any)
	var code, _ = location["file"].(string)
	if !strings.HasSuffix(code, "slog_test.go") {
		t.Errorf("expected the caller to be this test file, got %q", code)
	}
}

func TestSlog_WithAttrsDoesNotChangeTheParent(t *testing.T) {
	var decoded = captureSlogJSON(t, Config{Level: LevelInfo}, func(logger *slog.Logger) {
		_ = logger.With("child", true)
		logger.Info("m", "own", 1)
	})
	if _, leaked := dataOf(t, decoded)["child"]; leaked {
		t.Errorf("expected the parent logger to stay without the child's attrs, got %v", decoded)
	}
}
