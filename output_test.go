package logs

import (
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestFormatHuman_BasicMessage(t *testing.T) {
	var e = Entry{
		Timestamp: time.Date(2025, 1, 15, 14, 30, 45, 0, time.UTC),
		Level:     LevelInfo,
		Message:   "hello world",
	}
	var result = formatHuman(e)
	if !strings.Contains(result, "INFO") {
		t.Errorf("expected INFO in output, got: %s", result)
	}
	if !strings.Contains(result, "14:30:45") {
		t.Errorf("expected timestamp in output, got: %s", result)
	}
	if !strings.Contains(result, "hello world") {
		t.Errorf("expected message in output, got: %s", result)
	}
}

func TestFormatHuman_WithFields(t *testing.T) {
	var e = Entry{
		Timestamp: time.Date(2025, 1, 15, 14, 30, 45, 0, time.UTC),
		Level:     LevelInfo,
		Message:   "msg",
		Fields:    map[string]any{"batch": "abc", "count": 42},
	}
	var result = formatHuman(e)
	if !strings.Contains(result, "batch=") {
		t.Errorf("expected batch field in output, got: %s", result)
	}
	if !strings.Contains(result, "count=42") {
		t.Errorf("expected count field in output, got: %s", result)
	}
}

func TestFormatHuman_FieldsSortedAlphabetically(t *testing.T) {
	var e = Entry{
		Timestamp: time.Date(2025, 1, 15, 14, 30, 45, 0, time.UTC),
		Level:     LevelInfo,
		Message:   "msg",
		Fields:    map[string]any{"zebra": "z", "alpha": "a"},
	}
	var result = formatHuman(e)
	var alphaIdx = strings.Index(result, "alpha=")
	var zebraIdx = strings.Index(result, "zebra=")
	if alphaIdx > zebraIdx {
		t.Errorf("expected alpha before zebra, got: %s", result)
	}
}

func TestFormatHuman_ColorByLevel(t *testing.T) {
	var tests = []struct {
		level Level
		color string
	}{
		{LevelError, colorRed},
		{LevelFatal, colorRed},
		{LevelWarning, colorYellow},
		{LevelInfo, colorBlue},
		{LevelDebug, colorGray},
	}
	for _, tt := range tests {
		var e = Entry{Timestamp: time.Now(), Level: tt.level, Message: "test"}
		var result = formatHuman(e)
		if !strings.Contains(result, tt.color) {
			t.Errorf("level %s: expected color %q in output", tt.level, tt.color)
		}
	}
}

func TestFormatHuman_CallerInfo(t *testing.T) {
	var e = Entry{
		Timestamp: time.Now(),
		Level:     LevelInfo,
		Message:   "msg",
		Caller:    CallerInfo{File: "engine/canvas.go", Line: 42, Pkg: "engine"},
	}
	var result = formatHuman(e)
	if !strings.Contains(result, "engine/canvas.go:42") {
		t.Errorf("expected caller info in output, got: %s", result)
	}
}

func TestFormatHuman_StringFieldsQuoted(t *testing.T) {
	var e = Entry{
		Timestamp: time.Now(),
		Level:     LevelInfo,
		Message:   "msg",
		Fields:    map[string]any{"name": "alice"},
	}
	var result = formatHuman(e)
	if !strings.Contains(result, `name="alice"`) {
		t.Errorf("expected quoted string field, got: %s", result)
	}
}

func TestFormatJSON_ValidJSON(t *testing.T) {
	var e = Entry{
		Timestamp: time.Date(2025, 1, 15, 14, 30, 45, 0, time.UTC),
		Level:     LevelInfo,
		Message:   "hello world",
		Fields:    map[string]any{"batch": "abc"},
		Caller:    CallerInfo{File: "engine/canvas.go", Line: 42, Pkg: "engine"},
	}
	var result = formatJSON(e)

	var parsed map[string]any
	if err := json.Unmarshal([]byte(result), &parsed); err != nil {
		t.Fatalf("invalid JSON: %v\noutput: %s", err, result)
	}

	if parsed["severity"] != "INFO" {
		t.Errorf("expected severity INFO, got %v", parsed["severity"])
	}
	if parsed["message"] != "hello world" {
		t.Errorf("expected message 'hello world', got %v", parsed["message"])
	}
	if parsed["timestamp"] == nil {
		t.Error("expected timestamp field")
	}
}

func TestFormatJSON_DataFieldContainsFields(t *testing.T) {
	var e = Entry{
		Timestamp: time.Now(),
		Level:     LevelInfo,
		Message:   "msg",
		Fields:    map[string]any{"batch": "abc", "count": 42},
		Scope:     "engine",
	}
	var result = formatJSON(e)

	var parsed map[string]any
	if err := json.Unmarshal([]byte(result), &parsed); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}

	data, ok := parsed["data"].(map[string]any)
	if !ok {
		t.Fatalf("expected data field as object, got %T", parsed["data"])
	}
	if data["batch"] != "abc" {
		t.Errorf("expected data.batch=abc, got %v", data["batch"])
	}
}

func TestFormatJSON_SourceLocation(t *testing.T) {
	var e = Entry{
		Timestamp: time.Now(),
		Level:     LevelError,
		Message:   "boom",
		Caller:    CallerInfo{File: "engine/canvas.go", Line: 42, Pkg: "engine", Func: "engine.MyFunc"},
	}
	var result = formatJSON(e)

	var parsed map[string]any
	if err := json.Unmarshal([]byte(result), &parsed); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	loc, ok := parsed["logging.googleapis.com/sourceLocation"].(map[string]any)
	if !ok {
		t.Fatalf("expected GCP sourceLocation object, got %T (%v)", parsed["logging.googleapis.com/sourceLocation"], parsed["logging.googleapis.com/sourceLocation"])
	}
	if loc["file"] != "engine/canvas.go" {
		t.Errorf("expected sourceLocation.file, got %v", loc["file"])
	}
	if loc["line"] != "42" {
		t.Errorf("expected sourceLocation.line as the string %q (GCP encodes int64 line as a string), got %v", "42", loc["line"])
	}
	if loc["function"] != "engine.MyFunc" {
		t.Errorf("expected sourceLocation.function, got %v", loc["function"])
	}
	if _, exists := parsed["data"]; exists {
		t.Errorf("caller location must not leak into data, got data=%v", parsed["data"])
	}
}

func TestFormatJSON_SourceLocationLeavesDataToUserFields(t *testing.T) {
	var e = Entry{
		Timestamp: time.Now(),
		Level:     LevelError,
		Message:   "boom",
		Fields:    map[string]any{"batch": "abc", "@code": "user-owned"},
		Caller:    CallerInfo{File: "engine/canvas.go", Line: 42, Pkg: "engine", Func: "engine.MyFunc"},
	}
	var result = formatJSON(e)

	var parsed map[string]any
	if err := json.Unmarshal([]byte(result), &parsed); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	data, ok := parsed["data"].(map[string]any)
	if !ok {
		t.Fatalf("expected jsonPayload.data to be an object, got %T", parsed["data"])
	}
	if data["batch"] != "abc" {
		t.Errorf("expected user field batch=abc preserved, got %v", data["batch"])
	}
	if data["@code"] != "user-owned" {
		t.Errorf("a user field named @code is no longer stomped by caller info, got %v", data["@code"])
	}
}

func TestFormatJSON_NoDataFieldWhenNoFieldsAndNoCaller(t *testing.T) {
	var e = Entry{
		Timestamp: time.Now(),
		Level:     LevelInfo,
		Message:   "msg",
	}
	var result = formatJSON(e)

	var parsed map[string]any
	if err := json.Unmarshal([]byte(result), &parsed); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if _, exists := parsed["data"]; exists {
		t.Error("expected no data field when there are no fields and no caller")
	}
}

func TestFormatJSON_StackTraceIncluded(t *testing.T) {
	var e = Entry{
		Timestamp: time.Now(),
		Level:     LevelError,
		Message:   "msg",
		Stack:     "goroutine 1 [running]:\nmain.main()\n\t/app/main.go:10",
	}
	var result = formatJSON(e)

	var parsed map[string]any
	if err := json.Unmarshal([]byte(result), &parsed); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if parsed["stack_trace"] == nil {
		t.Error("expected stack_trace field for error with stack")
	}
}

func TestFormatFieldValue_Types(t *testing.T) {
	var tests = []struct {
		input    any
		expected string
	}{
		{"hello", `"hello"`},
		{42, "42"},
		{int64(100), "100"},
		{3.14, "3.14"},
		{true, "true"},
		{false, "false"},
	}
	for _, tt := range tests {
		var result = formatFieldValue(tt.input)
		if result != tt.expected {
			t.Errorf("formatFieldValue(%v): expected %s, got %s", tt.input, tt.expected, result)
		}
	}
}

func TestGetCallerInfo_ReturnsNonEmpty(t *testing.T) {
	var info = getCallerInfo(1)
	if info.File == "" {
		t.Error("expected non-empty caller file")
	}
	if info.Line == 0 {
		t.Error("expected non-zero caller line")
	}
}

func TestCaptureStack_ReturnsFrames(t *testing.T) {
	var stack = captureStack()
	if stack == "" {
		t.Error("expected non-empty stack trace")
	}
	if !strings.Contains(stack, "output_test.go") {
		t.Errorf("expected stack to contain this test file, got: %s", stack)
	}
}

func TestGetColorByLevel_UnknownReturnsReset(t *testing.T) {
	if got := getColorByLevel(Level(99)); got != colorReset {
		t.Errorf("expected colorReset for unknown level, got %q", got)
	}
}

func TestFormatFieldValue_AllNumericTypes(t *testing.T) {
	var tests = []struct {
		input    any
		expected string
	}{
		{int8(-8), "-8"},
		{int16(-16), "-16"},
		{int32(-32), "-32"},
		{uint(1), "1"},
		{uint8(2), "2"},
		{uint16(3), "3"},
		{uint32(4), "4"},
		{uint64(5), "5"},
		{float32(1.5), "1.5"},
	}
	for _, tt := range tests {
		if got := formatFieldValue(tt.input); got != tt.expected {
			t.Errorf("formatFieldValue(%v): expected %s, got %s", tt.input, tt.expected, got)
		}
	}
}

func TestFormatFieldValue_DefaultUsesGoSyntax(t *testing.T) {
	var got = formatFieldValue([]int{1, 2, 3})
	if got != "[1 2 3]" {
		t.Errorf("expected default formatting for slice, got %s", got)
	}
}

func TestFormatHuman_IncludesStackAndError(t *testing.T) {
	var e = Entry{
		Timestamp: time.Now(),
		Level:     LevelError,
		Message:   "boom",
		Stack:     "goroutine 1 [running]:\nmain.main()",
		Error:     fmt.Errorf("disk full"),
	}
	var result = formatHuman(e)
	if !strings.Contains(result, "goroutine 1 [running]") {
		t.Errorf("expected stack in human output, got: %s", result)
	}
	if !strings.Contains(result, "disk full") {
		t.Errorf("expected error in human output, got: %s", result)
	}
}

func TestFormatJSON_UnserializableFieldKeepsEntry(t *testing.T) {
	var e = Entry{
		Timestamp: time.Now(),
		Level:     LevelInfo,
		Message:   "unmarshalable",
		Fields:    map[string]any{"fn": func() {}, "ok": "kept"},
	}
	var result = formatJSON(e)

	var parsed map[string]any
	if err := json.Unmarshal([]byte(result), &parsed); err != nil {
		t.Fatalf("entry must stay valid JSON, got: %s", result)
	}
	if parsed["message"] != "unmarshalable" {
		t.Errorf("message must survive a bad field, got %v", parsed["message"])
	}
	data, ok := parsed["data"].(map[string]any)
	if !ok {
		t.Fatalf("expected data object, got %T", parsed["data"])
	}
	if data["ok"] != "kept" {
		t.Errorf("good field must survive, got %v", data["ok"])
	}
	if fn, _ := data["fn"].(string); !strings.Contains(fn, "ERR") {
		t.Errorf("unserializable field should become a placeholder, got %v", data["fn"])
	}
}

func TestGetCallerInfo_UnresolvableReturnsEmpty(t *testing.T) {
	var info = getCallerInfo(10000)
	if info.File != "" || info.Line != 0 {
		t.Errorf("expected empty callerInfo for unresolvable skip, got %+v", info)
	}
}

func TestGetRunningDirPrefix_StripsAppsParent(t *testing.T) {
	var originalGetwd = getwd
	var originalPrefix = runningDirPrefix
	defer func() {
		getwd = originalGetwd
		runningDirPrefix = originalPrefix
	}()

	getwd = func() (string, error) { return "/repo/apps/myservice", nil }
	runningDirPrefix = ""

	if got := getRunningDirPrefix(); got != "/repo" {
		t.Errorf("expected apps parent stripped to /repo, got %s", got)
	}
}

func TestGetRunningDirPrefix_GetwdErrorFallsBackToDot(t *testing.T) {
	var originalGetwd = getwd
	var originalPrefix = runningDirPrefix
	defer func() {
		getwd = originalGetwd
		runningDirPrefix = originalPrefix
	}()

	getwd = func() (string, error) { return "", fmt.Errorf("getwd failed") }
	runningDirPrefix = ""

	if got := getRunningDirPrefix(); got != "." {
		t.Errorf("expected fallback to '.', got %s", got)
	}
}

func TestFormatJSON_ErrorField(t *testing.T) {
	var e = Entry{
		Timestamp: time.Now(),
		Level:     LevelError,
		Message:   "operation failed",
		Error:     fmt.Errorf("connection refused"),
	}
	var result = formatJSON(e)

	var parsed map[string]any
	if err := json.Unmarshal([]byte(result), &parsed); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if parsed["error"] != "connection refused" {
		t.Errorf("expected error field, got %v", parsed["error"])
	}
}

func TestTrimDirPrefix_PathComponentAware(t *testing.T) {
	var tests = []struct {
		name   string
		file   string
		prefix string
		want   string
	}{
		{"under prefix", "/repo/pkg/foo.go", "/repo", "pkg/foo.go"},
		{"equals prefix", "/repo", "/repo", ""},
		{"sibling shares string prefix", "/repo-shared/util.go", "/repo", "/repo-shared/util.go"},
		{"prefix is substring mid-component", "/repository/foo.go", "/repo", "/repository/foo.go"},
		{"unrelated path untouched", "/usr/local/go/src/fmt/print.go", "/repo", "/usr/local/go/src/fmt/print.go"},
	}
	for _, tt := range tests {
		if got := trimDirPrefix(tt.file, tt.prefix); got != tt.want {
			t.Errorf("%s: trimDirPrefix(%q, %q) = %q, want %q", tt.name, tt.file, tt.prefix, got, tt.want)
		}
	}
}

func TestGetRunningDirPrefix_ConcurrentLazyInitRace(t *testing.T) {
	var originalGetwd = getwd
	var originalPrefix = runningDirPrefix
	defer func() {
		getwd = originalGetwd
		runningDirPrefix = originalPrefix
	}()

	getwd = func() (string, error) { return "/repo/apps/myservice", nil }
	runningDirPrefix = ""

	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_ = getRunningDirPrefix()
		}()
	}
	wg.Wait()
}
