package logs

import (
	"bytes"
	"context"
	"strings"
	"testing"
)

func writerContext(buf *bytes.Buffer) context.Context {
	var ctx = Attach(context.Background(), Config{Level: LevelDebug, Format: HumanReadable})
	ctx = SuppressConsoleOutput(ctx)
	return AttachWriter(ctx, buf)
}

func TestWriter_EmitsCompleteLines(t *testing.T) {
	var buf bytes.Buffer
	var w = NewWriter(writerContext(&buf), LevelInfo)

	n, err := w.Write([]byte("first line\nsecond line\n"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if n != len("first line\nsecond line\n") {
		t.Errorf("expected Write to report full byte count, got %d", n)
	}
	if !strings.Contains(buf.String(), "first line") {
		t.Errorf("expected first line emitted, got: %s", buf.String())
	}
	if !strings.Contains(buf.String(), "second line") {
		t.Errorf("expected second line emitted, got: %s", buf.String())
	}
}

func TestWriter_BuffersPartialLineUntilNewline(t *testing.T) {
	var buf bytes.Buffer
	var w = NewWriter(writerContext(&buf), LevelInfo)

	w.Write([]byte("partial "))
	if strings.Contains(buf.String(), "partial") {
		t.Errorf("partial line without newline should not be emitted yet, got: %s", buf.String())
	}

	w.Write([]byte("complete\n"))
	if !strings.Contains(buf.String(), "partial complete") {
		t.Errorf("expected buffered partial joined with rest, got: %s", buf.String())
	}
}

func TestWriter_SkipsBlankLines(t *testing.T) {
	var buf bytes.Buffer
	var w = NewWriter(writerContext(&buf), LevelInfo)

	w.Write([]byte("\n\n\n"))
	if buf.Len() != 0 {
		t.Errorf("expected blank lines to be skipped, got: %s", buf.String())
	}
}

func TestWriter_CloseFlushesRemainder(t *testing.T) {
	var buf bytes.Buffer
	var w = NewWriter(writerContext(&buf), LevelWarning)

	w.Write([]byte("no trailing newline"))
	if err := w.Close(); err != nil {
		t.Fatalf("unexpected close error: %v", err)
	}
	if !strings.Contains(buf.String(), "no trailing newline") {
		t.Errorf("expected Close to flush buffered remainder, got: %s", buf.String())
	}
	if !strings.Contains(buf.String(), "WARNING") {
		t.Errorf("expected remainder emitted at writer level, got: %s", buf.String())
	}
}

func TestWriter_CloseWithEmptyBufferEmitsNothing(t *testing.T) {
	var buf bytes.Buffer
	var w = NewWriter(writerContext(&buf), LevelInfo)

	if err := w.Close(); err != nil {
		t.Fatalf("unexpected close error: %v", err)
	}
	if buf.Len() != 0 {
		t.Errorf("expected no output closing an empty writer, got: %s", buf.String())
	}
}
