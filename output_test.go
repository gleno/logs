package logs

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"
	"time"
)

func TestFormatHuman_BasicMessage(t *testing.T) {
	var e = entry{
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
	var e = entry{
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
	var e = entry{
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
		var e = entry{Timestamp: time.Now(), Level: tt.level, Message: "test"}
		var result = formatHuman(e)
		if !strings.Contains(result, tt.color) {
			t.Errorf("level %s: expected color %q in output", tt.level, tt.color)
		}
	}
}

func TestFormatHuman_HiddenFieldsFiltered(t *testing.T) {
	var e = entry{
		Timestamp: time.Now(),
		Level:     LevelInfo,
		Message:   "msg",
		Fields:    map[string]any{"app": "test", "env": "prod", "real_field": "keep"},
	}
	var result = formatHuman(e)
	if strings.Contains(result, "app=") {
		t.Errorf("expected hidden field 'app' to be filtered, got: %s", result)
	}
	if !strings.Contains(result, "real_field=") {
		t.Errorf("expected real_field to be kept, got: %s", result)
	}
}

func TestFormatHuman_CallerInfo(t *testing.T) {
	var e = entry{
		Timestamp: time.Now(),
		Level:     LevelInfo,
		Message:   "msg",
		Caller:    callerInfo{File: "engine/canvas.go", Line: 42, Pkg: "engine"},
	}
	var result = formatHuman(e)
	if !strings.Contains(result, "engine/canvas.go:42") {
		t.Errorf("expected caller info in output, got: %s", result)
	}
}

func TestFormatHuman_StringFieldsQuoted(t *testing.T) {
	var e = entry{
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
	var e = entry{
		Timestamp: time.Date(2025, 1, 15, 14, 30, 45, 0, time.UTC),
		Level:     LevelInfo,
		Message:   "hello world",
		Fields:    map[string]any{"batch": "abc"},
		Caller:    callerInfo{File: "engine/canvas.go", Line: 42, Pkg: "engine"},
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
	var e = entry{
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

func TestFormatJSON_AtCodeInData(t *testing.T) {
	var e = entry{
		Timestamp: time.Now(),
		Level:     LevelError,
		Message:   "boom",
		Caller:    callerInfo{File: "engine/canvas.go", Line: 42, Pkg: "engine", Func: "engine.MyFunc"},
	}
	var result = formatJSON(e)

	var parsed map[string]any
	if err := json.Unmarshal([]byte(result), &parsed); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	data, ok := parsed["data"].(map[string]any)
	if !ok {
		t.Fatalf("expected jsonPayload.data to be an object, got %T (%v)", parsed["data"], parsed["data"])
	}
	code, ok := data["@code"].(string)
	if !ok {
		t.Fatalf("expected jsonPayload.data.@code to be a string, got %T", data["@code"])
	}
	if !strings.Contains(code, "engine/canvas.go:42") {
		t.Errorf("expected @code to contain file:line, got %q", code)
	}
	if !strings.Contains(code, "MyFunc") {
		t.Errorf("expected @code to contain function name, got %q", code)
	}
	if _, exists := parsed["caller"]; exists {
		t.Errorf("expected no top-level caller field, got %v", parsed["caller"])
	}
}

func TestFormatJSON_AtCodeCoexistsWithUserFields(t *testing.T) {
	var e = entry{
		Timestamp: time.Now(),
		Level:     LevelError,
		Message:   "boom",
		Fields:    map[string]any{"batch": "abc"},
		Caller:    callerInfo{File: "engine/canvas.go", Line: 42, Pkg: "engine", Func: "engine.MyFunc"},
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
	if _, ok := data["@code"].(string); !ok {
		t.Errorf("expected @code alongside user fields, got %v", data)
	}
	if data["batch"] != "abc" {
		t.Errorf("expected user field batch=abc preserved, got %v", data["batch"])
	}
}

func TestFormatJSON_NoDataFieldWhenNoFieldsAndNoCaller(t *testing.T) {
	var e = entry{
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
	var e = entry{
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

func TestFormatJSON_ErrorField(t *testing.T) {
	var e = entry{
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
