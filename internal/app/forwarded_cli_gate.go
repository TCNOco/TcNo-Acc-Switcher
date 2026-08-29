package app

import "sync"

type forwardedCLIGate struct {
	mu       sync.Mutex
	ready    bool
	draining bool
	queue    [][]string
	dispatch func([]string)
}

func newForwardedCLIGate(dispatch func([]string)) *forwardedCLIGate {
	return &forwardedCLIGate{dispatch: dispatch}
}

func (g *forwardedCLIGate) Submit(argv []string) {
	argv = append([]string(nil), argv...)

	g.mu.Lock()
	g.queue = append(g.queue, argv)
	if !g.ready || g.draining {
		g.mu.Unlock()
		return
	}
	g.draining = true
	g.mu.Unlock()

	g.drain()
}

func (g *forwardedCLIGate) Ready() {
	g.mu.Lock()
	if g.ready {
		g.mu.Unlock()
		return
	}
	g.ready = true
	if g.draining || len(g.queue) == 0 {
		g.mu.Unlock()
		return
	}
	g.draining = true
	g.mu.Unlock()

	g.drain()
}

func (g *forwardedCLIGate) drain() {
	for {
		g.mu.Lock()
		if len(g.queue) == 0 {
			g.draining = false
			g.mu.Unlock()
			return
		}
		argv := g.queue[0]
		g.queue[0] = nil
		g.queue = g.queue[1:]
		g.mu.Unlock()

		g.dispatch(argv)
	}
}
