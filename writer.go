package logs

import (
	"bytes"
	"context"
	"io"
	"strings"
	"sync"
)

type logWriter struct {
	ctx   context.Context
	level Level
	mu    sync.Mutex
	buf   bytes.Buffer
}

func NewWriter(ctx context.Context, level Level) io.WriteCloser {
	return &logWriter{ctx: ctx, level: level}
}

func (w *logWriter) Write(p []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()

	w.buf.Write(p)

	for {
		line, err := w.buf.ReadString('\n')
		if err != nil {
			w.buf.WriteString(line)
			break
		}
		line = strings.TrimRight(line, "\n\r")
		if line != "" {
			emit(w.ctx, w.level, line, nil)
		}
	}
	return len(p), nil
}

func (w *logWriter) Close() error {
	w.mu.Lock()
	defer w.mu.Unlock()

	var remaining = strings.TrimRight(w.buf.String(), "\n\r")
	if remaining != "" {
		emit(w.ctx, w.level, remaining, nil)
	}
	w.buf.Reset()
	return nil
}
