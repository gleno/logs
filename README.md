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
- **Caller info** — file, line, and function captured automatically on every entry.
- **Stack traces** — captured automatically at and above a configurable level.
- **Error helpers** — `Err`/`Errf` log an `error` value with its message attached as a structured field.
- **Pluggable writers** — fan out to any number of `io.Writer`s, suppress the console, or silence everything.
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

Set the process-wide default format once at startup with `SetDefaultFormat`. Use `JSON` in production, `HumanReadable` for local development.

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

### Capturing a stream as log lines

`NewWriter` returns an `io.WriteCloser` that emits each line written to it as a log entry at the given level — useful for redirecting output from a third-party library or subprocess into the logger.

```go
w := logs.NewWriter(ctx, logs.LevelInfo)
defer w.Close() // flushes any trailing partial line

fmt.Fprintln(w, "line one") // emitted as an Info log
fmt.Fprintln(w, "line two")
```
