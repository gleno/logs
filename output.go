package logs

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"sync"
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
	Hidden    map[string]bool
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
		if !e.Hidden[k] {
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

var runningDirPrefixMu sync.Mutex

var getwd = os.Getwd

var statFile = os.Stat

func getRunningDirPrefix() string {
	runningDirPrefixMu.Lock()
	defer runningDirPrefixMu.Unlock()
	if runningDirPrefix == "" {
		workingDir, err := getwd()
		if err != nil {
			runningDirPrefix = "."
			return runningDirPrefix
		}
		runningDirPrefix = findModuleRoot(workingDir)
	}
	return runningDirPrefix
}

func findModuleRoot(dir string) string {
	for candidate := dir; ; candidate = filepath.Dir(candidate) {
		if _, err := statFile(filepath.Join(candidate, "go.mod")); err == nil {
			return candidate
		}
		if filepath.Dir(candidate) == candidate {
			return dir
		}
	}
}

func getCallerInfo(skip int) callerInfo {
	pc, file, line, ok := runtime.Caller(skip)
	if !ok {
		return callerInfo{}
	}
	return describeCaller(runtime.FuncForPC(pc), file, line)
}

func getCallerInfoAt(pc uintptr) callerInfo {
	if pc == 0 {
		return callerInfo{}
	}
	var frame, _ = runtime.CallersFrames([]uintptr{pc}).Next()
	return describeCaller(frame.Func, frame.File, frame.Line)
}

func describeCaller(fn *runtime.Func, file string, line int) callerInfo {
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

	var displayFile = trimDirPrefix(file, getRunningDirPrefix())

	return callerInfo{
		File: displayFile,
		Line: line,
		Pkg:  pkg,
		Func: funcName,
	}
}

func trimDirPrefix(file, prefix string) string {
	if file == prefix {
		return ""
	}
	if strings.HasPrefix(file, prefix+"/") {
		return file[len(prefix)+1:]
	}
	return file
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
