package logs

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"runtime"
	"sort"
	"strings"
	"time"
)

type callerInfo struct {
	File string
	Line int
	Pkg  string
	Func string
}

type entry struct {
	Timestamp time.Time
	Level     Level
	Message   string
	Fields    map[string]any
	Scope     string
	Caller    callerInfo
	Stack     string
	Error     error
}

const (
	colorReset  = "\x1b[0m"
	colorDim    = "\x1b[2;90m"
	colorRed    = "\x1b[31m"
	colorYellow = "\x1b[33m"
	colorBlue   = "\x1b[34m"
	colorGray   = "\x1b[90m"
)

var hiddenFields = map[string]bool{
	"app": true, "env": true, "account_key": true,
	"http_method": true, "http_uri": true, "namespace": true,
	"origin_app_key": true, "origin_http_method": true, "origin_http_uri": true, "origin_remote_ip": true,
	"http_host": true, "http_proto": true, "http_remote_addr": true, "http_scheme": true,
	"http_url": true, "http_user_agent": true, "remote_ip": true,
	"user_id": true, "http_route": true, "http_resp_bytes_length": true, "http_resp_status": true,
	"http_path": true, "time_ms": true,
}

func getColorByLevel(level Level) string {
	switch level {
	case LevelError, LevelFatal:
		return colorRed
	case LevelWarning:
		return colorYellow
	case LevelInfo:
		return colorBlue
	case LevelDebug:
		return colorGray
	default:
		return colorReset
	}
}

func formatFieldValue(value any) string {
	switch v := value.(type) {
	case string:
		return fmt.Sprintf("%q", v)
	case int:
		return fmt.Sprintf("%d", v)
	case int8:
		return fmt.Sprintf("%d", v)
	case int16:
		return fmt.Sprintf("%d", v)
	case int32:
		return fmt.Sprintf("%d", v)
	case int64:
		return fmt.Sprintf("%d", v)
	case uint:
		return fmt.Sprintf("%d", v)
	case uint8:
		return fmt.Sprintf("%d", v)
	case uint16:
		return fmt.Sprintf("%d", v)
	case uint32:
		return fmt.Sprintf("%d", v)
	case uint64:
		return fmt.Sprintf("%d", v)
	case float32:
		return fmt.Sprintf("%v", v)
	case float64:
		return fmt.Sprintf("%v", v)
	case bool:
		return fmt.Sprintf("%t", v)
	default:
		return fmt.Sprintf("%v", v)
	}
}

func formatHuman(e entry) string {
	var buf bytes.Buffer

	var levelColor = getColorByLevel(e.Level)
	var timestamp = e.Timestamp.Format("15:04:05")

	fmt.Fprintf(&buf, "%s%-7s%s[%s] %s",
		levelColor,
		e.Level.String(),
		colorReset,
		timestamp,
		e.Message)

	if e.Caller.File != "" {
		fmt.Fprintf(&buf, " %s(%s:%d)%s", colorDim, e.Caller.File, e.Caller.Line, colorReset)
	}

	var visibleFields = make(map[string]any, len(e.Fields))
	for k, v := range e.Fields {
		if !hiddenFields[k] {
			visibleFields[k] = v
		}
	}

	if len(visibleFields) > 0 {
		buf.WriteString("\n")
		buf.WriteString(colorDim)
		buf.WriteString("    ")

		var keys = make([]string, 0, len(visibleFields))
		for k := range visibleFields {
			keys = append(keys, k)
		}
		sort.Strings(keys)

		for i, k := range keys {
			if i > 0 {
				buf.WriteString(" ")
			}
			fmt.Fprintf(&buf, "%s=%s", k, formatFieldValue(visibleFields[k]))
		}
		buf.WriteString(colorReset)
	}

	if e.Stack != "" {
		fmt.Fprintf(&buf, "\n%s%s%s", colorDim, e.Stack, colorReset)
	}

	if e.Error != nil {
		fmt.Fprintf(&buf, "\n%s%s%s", colorRed, e.Error, colorReset)
	}

	buf.WriteString("\n")
	return buf.String()
}

func formatJSON(e entry) string {
	var m = map[string]any{
		"timestamp": e.Timestamp.Format(time.RFC3339Nano),
		"severity":  e.Level.String(),
		"message":   e.Message,
	}

	var data map[string]any
	if len(e.Fields) > 0 {
		data = make(map[string]any, len(e.Fields)+1)
		for k, v := range e.Fields {
			data[k] = v
		}
	}

	if e.Caller.File != "" {
		if data == nil {
			data = make(map[string]any, 1)
		}
		data["@code"] = formatAtCode(e.Caller)
	}

	if data != nil {
		m["data"] = data
	}

	if e.Stack != "" {
		m["stack_trace"] = e.Stack
	}

	if e.Error != nil {
		m["error"] = e.Error.Error()
	}

	var b, err = json.Marshal(m)
	if err != nil {
		return fmt.Sprintf(`{"severity":"ERROR","message":"failed to marshal log entry: %s"}`, err)
	}
	b = append(b, '\n')
	return string(b)
}

var runningDirPrefix string

func getRunningDirPrefix() string {
	if runningDirPrefix == "" {
		workingDir, err := os.Getwd()
		if err != nil {
			runningDirPrefix = "."
			return runningDirPrefix
		}
		runningDirPrefix = workingDir
		var components = strings.Split(workingDir, "/")
		if len(components) > 1 && components[len(components)-2] == "apps" {
			runningDirPrefix = strings.Join(components[:len(components)-2], "/")
		}
	}
	return runningDirPrefix
}

func getCallerInfo(skip int) callerInfo {
	pc, file, line, ok := runtime.Caller(skip)
	if !ok {
		return callerInfo{}
	}

	var fn = runtime.FuncForPC(pc)
	var pkg, funcName string
	if fn != nil {
		var name = fn.Name()
		if lastSlash := strings.LastIndex(name, "/"); lastSlash >= 0 {
			name = name[lastSlash+1:]
		}
		funcName = name
		if dot := strings.Index(name, "."); dot >= 0 {
			pkg = name[:dot]
		}
	}

	var displayFile = strings.TrimPrefix(file, getRunningDirPrefix())
	displayFile = strings.TrimPrefix(displayFile, "/")

	return callerInfo{
		File: displayFile,
		Line: line,
		Pkg:  pkg,
		Func: funcName,
	}
}

func formatAtCode(c callerInfo) string {
	if c.Func != "" {
		return fmt.Sprintf("%s:%d %s()", c.File, c.Line, c.Func)
	}
	return fmt.Sprintf("%s:%d", c.File, c.Line)
}

func captureStack() string {
	var buf = make([]byte, 4096)
	var n = runtime.Stack(buf, false)
	return string(buf[:n])
}
