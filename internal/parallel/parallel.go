// Package parallel runs independent per-item work across a bounded pool.
package parallel

import (
	"runtime"
	"sync"
	"sync/atomic"
)

// maxWorkers bounds every fan-out here. The callers are IO-bound - small file
// reads and stats - so they saturate the disk queue long before the CPU, and
// more goroutines past this only add scheduling noise.
const maxWorkers = 8

// ForEachIndex calls fn for every index in [0, n), possibly concurrently, and
// returns once all of them have run.
//
// fn must be safe to call from several goroutines at once: write only to the
// slot for its own index, and treat everything shared as read-only.
func ForEachIndex(n int, fn func(i int)) {
	if n <= 0 {
		return
	}
	workers := min(min(n, runtime.NumCPU()), maxWorkers)
	if workers <= 1 {
		for i := range n {
			fn(i)
		}
		return
	}

	var next atomic.Int64
	var wg sync.WaitGroup
	wg.Add(workers)
	for range workers {
		go func() {
			defer wg.Done()
			for {
				i := int(next.Add(1)) - 1
				if i >= n {
					return
				}
				fn(i)
			}
		}()
	}
	wg.Wait()
}
