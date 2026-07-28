//go:build !windows

package securemem

import (
	"runtime"
	"sync"
)

type heapProtector struct{}

type heapHandle struct {
	mu        sync.Mutex
	secret    []byte
	destroyed bool
}

func newPlatformProtector() Protector { return heapProtector{} }

func (heapProtector) Store(secret []byte) (Handle, error) {
	h := &heapHandle{secret: append([]byte(nil), secret...)}
	return h, nil
}

func (h *heapHandle) With(fn func([]byte) error) error {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.destroyed {
		return ErrUnavailable
	}
	copyOfSecret := append([]byte(nil), h.secret...)
	defer func() {
		wipe(copyOfSecret)
		runtime.KeepAlive(copyOfSecret)
	}()
	return fn(copyOfSecret)
}

func (h *heapHandle) Destroy() error {
	h.mu.Lock()
	defer h.mu.Unlock()
	if !h.destroyed {
		wipe(h.secret)
		h.secret = nil
		h.destroyed = true
	}
	return nil
}
