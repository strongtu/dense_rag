package watcher

import "sync"

// Pool is a bounded worker pool that limits concurrency.
type Pool struct {
	wg   sync.WaitGroup
	sem  chan struct{}
}

// NewPool creates a worker pool with the given concurrency limit.
// If size <= 0 it defaults to 4.
func NewPool(size int) *Pool {
	if size <= 0 {
		size = 4
	}
	return &Pool{
		sem: make(chan struct{}, size),
	}
}

// Submit enqueues fn for execution. It blocks if the pool is at capacity
// until a worker slot becomes available.
func (p *Pool) Submit(fn func()) {
	p.sem <- struct{}{} // acquire slot (blocks if full)
	p.wg.Add(1)
	go func() {
		defer func() {
			<-p.sem // release slot
			p.wg.Done()
		}()
		fn()
	}()
}

// Shutdown waits for all submitted work to complete.
func (p *Pool) Shutdown() {
	p.wg.Wait()
}
