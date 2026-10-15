# logs

A context-based structured logger for Go: log levels, structured fields, scopes, and pluggable writers, with human-readable or JSON output. Standard library only — zero external dependencies.

## Install

```sh
go get github.com/gleno/logs
```

## Features

- **Zero dependencies** — standard library only.
- **Context-based** — configuration, fields, scopes, and writers all travel on `context.Context`. No global logger to thread through your code; pass the context you already have.
- **Five levels** — `Debug`, `Info`, `Warning`, `Error`, `Fatal`, with level-based filtering.
- **Structured fields** — attach key/value pairs to a context; they appear on every log line emitted with it.
- **Scopes** — build dot-separated scope paths (`engine.ingestion`) that nest as you descend through your call tree.
- **Two output formats** — colorized human-readable output for terminals, or structured JSON for log aggregation.
- **Caller info** — file, line, and function captured automatically on every entry, with paths shown relative to your module root.
- **Stack traces** — captured automatically at and above a configurable level.
- **Error helpers** — `Err`/`Errf` log an `error` value with its message attached as a structured field.
- **Pluggable writers** — fan out to any number of `io.Writer`s, suppress the console, or silence everything.
- **Hidden fields** — keep chosen fields out of human-readable output while JSON still carries them.
- **`log/slog` handler** — route `slog` records from your code or libraries through the same context, fields and writers.
- **`io.Writer` adapter** — turn the logger into an `io.WriteCloser` to capture output from libraries that write to a stream.

## Usage

### Basic logging

Every logging call takes a `context.Context`. A bare `context.Background()` works out of the box (defaults to `Info` level, human-readable output, writing to stderr).

```go
package main

import (
	"context"

	"github.com/gleno/logs"
)

func main() {
	ctx := context.Background()

	logs.Debug(ctx, "starting up")            // filtered out at default Info level
	logs.Info(ctx, "server listening")
	logs.Infof(ctx, "listening on port %d", 8080)
	logs.Warn(ctx, "disk usage high")
	logs.Error(ctx, "request failed")

	// Fatal logs at Fatal level, then calls os.Exit(1).
	// logs.Fatal(ctx, "cannot continue")
}
```

Each level has a `...f` formatting variant: `Debugf`, `Infof`, `Warnf`, `Errorf`, `Fatalf`.

### Logging errors

`Err` and `Errf` log at `Error` level and attach the underlying `error`. Both are no-ops when the error is `nil`, so you can call them unconditionally.

```go
if err := doWork(ctx); err != nil {
	logs.Err(ctx, err)                          // message is err.Error()
	logs.Errf(ctx, err, "batch %s failed", id)  // custom message, error attached
}
```

### Setting the output format

Set the process-wide default format with `SetDefaultFormat`, usually once at startup. Use `JSON` in production, `HumanReadable` for local development. It is safe to call while other goroutines log; contexts that set their own format keep it.

```go
func main() {
	logs.SetDefaultFormat(logs.JSON)
	// ...
}
```

To override the format for a specific context (rather than globally), use `SetFormat`, which returns a derived context:

```go
ctx = logs.SetFormat(ctx, logs.HumanReadable)
logs.Info(ctx, "this line is human-readable")
```

### Configuring a context

`Attach` binds an explicit `Config` to a context. This is how you set the minimum level, format, and the threshold at which stack traces are captured.

```go
ctx := logs.Attach(context.Background(), logs.Config{
	Level:          logs.LevelDebug,   // emit everything from Debug up
	Format:         logs.JSON,
	StackTraceFrom: logs.LevelError,   // capture a stack trace at Error and Fatal
})

logs.Debug(ctx, "now visible")
```

`Config` fields:

| Field | Type | Meaning |
|---|---|---|
| `Level` | `Level` | Minimum level to emit; lower levels are dropped. |
| `Format` | `OutputFormat` | `HumanReadable` or `JSON`. |
| `StackTraceFrom` | `Level` | Entries at or above this level include a stack trace. Defaults to `LevelError` when left zero. |

The levels are `LevelDebug`, `LevelInfo`, `LevelWarning`, `LevelError`, `LevelFatal`. The formats are `HumanReadable` and `JSON`.

`ParseLevel` reads a level from text such as an environment variable. It accepts `debug`, `info`, `warn`/`warning`, `error` and `fatal` in any case:

```go
level, err := logs.ParseLevel(os.Getenv("LOG_LEVEL"))
if err != nil {
	level = logs.LevelInfo
}
ctx = logs.Attach(ctx, logs.Config{Level: level})
```

`Enabled` reports whether a level would be emitted with a context, so you can skip building an expensive message:

```go
if logs.Enabled(ctx, logs.LevelDebug) {
	logs.Debugf(ctx, "state: %s", dumpState())
}
```

### Structured fields

`WithFields` returns a derived context carrying key/value pairs. Every log emitted with that context (and its descendants) includes them. Fields accumulate as you derive further contexts; `nil` values are dropped, and parent contexts are never mutated.

```go
ctx = logs.WithFields(ctx, map[string]any{
	"request_id": reqID,
	"user_id":    userID,
})

logs.Info(ctx, "handling request") // both fields attached

ctx = logs.WithFields(ctx, map[string]any{"attempt": 2})
logs.Info(ctx, "retrying")          // request_id, user_id, and attempt all present
```

In JSON output, fields appear under the `data` key. In human-readable output, they render as `key="value"` pairs beneath the message.

### Hiding fields from human-readable output

`HideFields` returns a derived context whose human-readable output leaves out the named fields. JSON output still carries them, so they stay searchable in your log store without cluttering a terminal. No field is hidden by default.

```go
ctx = logs.WithFields(ctx, map[string]any{"user_agent": ua, "route": route})
ctx = logs.HideFields(ctx, "user_agent")

logs.Info(ctx, "served") // terminal shows route only; JSON has both
```

### Scopes

`WithScope` builds a dot-separated scope path that grows as you nest. The scope is also exposed as a `scope` field.

```go
ctx = logs.WithScope(ctx, "engine")
ctx = logs.WithScope(ctx, "ingestion")

logs.Info(ctx, "started") // scope = "engine.ingestion"
```

### Writers and output control

By default, logs go to stderr (the "console"). You can attach additional writers, suppress the console, or silence everything — each returns a derived context.

```go
var buf bytes.Buffer

ctx = logs.AttachWriter(ctx, &buf)        // fan out to an extra writer (console still active)
ctx = logs.SuppressConsoleOutput(ctx)     // stop writing to stderr; attached writers still receive output
ctx = logs.SuppressAllOutput(ctx)         // drop everything, including attached writers
```

`Flush` syncs the console and any attached writers that implement `Sync() error` (for example, files):

```go
defer logs.Flush(ctx)
```

### Using `log/slog`

`NewSlogHandler` returns a `slog.Handler` that writes through the context it was made from: its level, format, fields, hidden fields and writers. Use it for your own `slog` calls or to hand a `*slog.Logger` to a library.

```go
logger := slog.New(logs.NewSlogHandler(ctx))
logger.With("component", "db").Info("connected", "pool", "main")
```

Records map to levels as `Debug` → `LevelDebug`, `Info` → `LevelInfo`, `Warn` → `LevelWarning`, and `Error` and above → `LevelError`. Attributes become fields; groups qualify their keys with dots (`db.pool`); `error` values are recorded as their message; `LogValuer`s are resolved. The caller shown is the `slog` call site.

### Caller paths

File paths in caller info are shown relative to the nearest directory above the working directory that holds a `go.mod`, or relative to the working directory when there is none. Binaries built with `-trimpath` already carry module-relative paths and are shown as built.

### Capturing a stream as log lines

`NewWriter` returns an `io.WriteCloser` that emits each line written to it as a log entry at the given level — useful for redirecting output from a third-party library or subprocess into the logger.

```go
w := logs.NewWriter(ctx, logs.LevelInfo)
defer w.Close() // flushes any trailing partial line

fmt.Fprintln(w, "line one") // emitted as an Info log
fmt.Fprintln(w, "line two")
```
