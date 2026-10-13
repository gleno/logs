package logs

import (
	"io"
	"sync"
)

type syncer interface {
	Sync() error
}

type sink struct {
	mu sync.Mutex
	w  io.Writer
}

func (k *sink) write(p []byte) {
	k.mu.Lock()
	defer k.mu.Unlock()
	k.w.Write(p)
}

func (k *sink) sync() error {
	k.mu.Lock()
	defer k.mu.Unlock()
	if f, ok := k.w.(syncer); ok {
		return f.Sync()
	}
	return nil
}
