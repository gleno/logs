# logs

Context-based structured logging for Go: levels, scopes, fields, and pluggable
writers, with human-readable or JSON output. Zero external dependencies.

```sh
go get github.com/gleno/logs
```

Set the default output format once at startup:

```go
logs.SetDefaultFormat(logs.JSON)
```
