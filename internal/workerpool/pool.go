package workerpool

import (
	"context"
	"sync"
)

// Task represents a unit of work
type Task func(ctx context.Context) error

// WorkerPool manages a pool of workers for executing tasks
type WorkerPool struct {
	tasks   chan Task
	results chan error
	workers int
	wg      sync.WaitGroup
	ctx     context.Context
	cancel  context.CancelFunc
}

// NewWorkerPool creates a new worker pool
func NewWorkerPool(workers int) *WorkerPool {
	ctx, cancel := context.WithCancel(context.Background())
	return &WorkerPool{
		tasks:   make(chan Task, workers*2),
		results: make(chan error, workers*2),
		workers: workers,
		ctx:     ctx,
		cancel:  cancel,
	}
}

// Start launches the worker goroutines
func (wp *WorkerPool) Start() {
	for i := 0; i < wp.workers; i++ {
		wp.wg.Add(1)
		go wp.worker()
	}
}

// Stop gracefully stops the worker pool
func (wp *WorkerPool) Stop() {
	close(wp.tasks)
	wp.cancel()
	wp.wg.Wait()
	close(wp.results)
}

// Submit submits a task to the pool (blocking if pool is full)
func (wp *WorkerPool) Submit(task Task) {
	select {
	case wp.tasks <- task:
	case <-wp.ctx.Done():
	}
}

// TrySubmit tries to submit a task without blocking
func (wp *WorkerPool) TrySubmit(task Task) bool {
	select {
	case wp.tasks <- task:
		return true
	default:
		return false
	}
}

func (wp *WorkerPool) worker() {
	defer wp.wg.Done()

	for task := range wp.tasks {
		if wp.ctx.Err() != nil {
			return
		}

		// Execute task with context
		err := task(wp.ctx)

		// Send result (non-blocking)
		select {
		case wp.results <- err:
		default:
			// Results channel full, drop result
		}
	}
}

// Results returns a channel for receiving task results
func (wp *WorkerPool) Results() <-chan error {
	return wp.results
}
