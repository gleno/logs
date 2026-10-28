package logs

import (
	"bytes"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

type CallerInfo struct {
	File string
	Line int
	Pkg  string
	Func string
}

type Entry struct {
	Timestamp time.Time
	Level     Level
	Message   string
	Fields    map[string]any
	Scope     string
	Caller    CallerInfo
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

func formatHuman(e Entry) string {
	var buf bytes.Buffer

	var levelColor = getColorByLevel(e.Level)
	var timestamp = e.Timestamp.Format("15:04:05")

	fmt.Fprintf(&buf, "%s%-7s%s[%s] ",
		levelColor,
		e.Level.String(),
		colorReset,
		timestamp)

	if e.Scope != "" {
		fmt.Fprintf(&buf, "%s[%s]%s ", colorDim, e.Scope, colorReset)
	}

	buf.WriteString(e.Message)

	if e.Caller.File != "" {
		fmt.Fprintf(&buf, " %s(%s:%d)%s", colorDim, e.Caller.File, e.Caller.Line, colorReset)
	}

	if len(e.Fields) > 0 {
		buf.WriteString("\n")
		buf.WriteString(colorDim)
		buf.WriteString("    ")

		var keys = make([]string, 0, len(e.Fields))
		for k := range e.Fields {
			keys = append(keys, k)
		}
		sort.Strings(keys)

		for i, k := range keys {
			if i > 0 {
				buf.WriteString(" ")
			}
			fmt.Fprintf(&buf, "%s=%s", k, formatFieldValue(e.Fields[k]))
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

func formatJSON(e Entry) string {
	if b, ok := tryMarshal(jsonEntryMap(e, e.Fields)); ok {
		return string(append(b, '\n'))
	}

	var b, _ = json.Marshal(jsonEntryMap(e, sanitizeFields(e.Fields)))
	return string(append(b, '\n'))
}

func jsonEntryMap(e Entry, fields map[string]any) map[string]any {
	var m = map[string]any{
		"timestamp": e.Timestamp.Format(time.RFC3339Nano),
		"severity":  e.Level.String(),
		"message":   e.Message,
	}

	if e.Scope != "" {
		m["scope"] = e.Scope
	}

	if len(fields) > 0 {
		var data = make(map[string]any, len(fields))
		for k, v := range fields {
			data[k] = v
		}
		m["data"] = data
	}

	if e.Caller.File != "" {
		m[gcpSourceLocationKey] = sourceLocation(e.Caller)
	}

	if e.Stack != "" {
		m["stack_trace"] = e.Stack
	}

	if e.Error != nil {
		m["error"] = e.Error.Error()
	}

	return m
}

func tryMarshal(m map[string]any) (b []byte, ok bool) {
	defer func() {
		if recover() != nil {
			b, ok = nil, false
		}
	}()
	var err error
	b, err = json.Marshal(m)
	return b, err == nil
}

func sanitizeFields(fields map[string]any) map[string]any {
	var out = make(map[string]any, len(fields))
	for k, v := range fields {
		out[k] = sanitizeValue(v)
	}
	return out
}

func sanitizeValue(v any) any {
	switch f := v.(type) {
	case float64:
		if s, ok := nonFiniteString(f); ok {
			return s
		}
	case float32:
		if s, ok := nonFiniteString(float64(f)); ok {
			return s
		}
	}
	if marshalsOK(v) {
		return v
	}
	return fmt.Sprintf("!ERR(%T)", v)
}

func nonFiniteString(f float64) (string, bool) {
	switch {
	case math.IsNaN(f):
		return "NaN", true
	case math.IsInf(f, 1):
		return "+Inf", true
	case math.IsInf(f, -1):
		return "-Inf", true
	}
	return "", false
}

func marshalsOK(v any) (ok bool) {
	defer func() {
		if recover() != nil {
			ok = false
		}
	}()
	var _, err = json.Marshal(v)
	return err == nil
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

func getCallerInfo(skip int) CallerInfo {
	pc, file, line, ok := runtime.Caller(skip)
	if !ok {
		return CallerInfo{}
	}
	return describeCaller(runtime.FuncForPC(pc), file, line)
}

func getCallerInfoAt(pc uintptr) CallerInfo {
	if pc == 0 {
		return CallerInfo{}
	}
	var frame, _ = runtime.CallersFrames([]uintptr{pc}).Next()
	return describeCaller(frame.Func, frame.File, frame.Line)
}

func describeCaller(fn *runtime.Func, file string, line int) CallerInfo {
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

	return CallerInfo{
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

const gcpSourceLocationKey = "logging.googleapis.com/sourceLocation"

func sourceLocation(c CallerInfo) map[string]any {
	var loc = map[string]any{
		"file": c.File,
		"line": strconv.Itoa(c.Line),
	}
	if c.Func != "" {
		loc["function"] = c.Func
	}
	return loc
}

func captureStack() string {
	var buf = make([]byte, 4096)
	var n = runtime.Stack(buf, false)
	return string(buf[:n])
}
