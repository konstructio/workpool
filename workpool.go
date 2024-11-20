package workpool

import (
	"context"
	"errors"
	"os"
	"os/signal"
	"sync"
	"syscall"
)

// Custom error types
var (
	ErrInvalidNumWorkers    = errors.New("number of workers must be greater than zero")
	ErrInvalidQueueCapacity = errors.New("queue capacity must be greater than zero")
	ErrPoolStopped          = errors.New("pool is stopped")
)

// Job represents a unit of work to be processed by the worker pool. It's the
// responsibility of the job to check the context for cancellation and gracefully
// return if it's cancelled.
type Job func(ctx context.Context) error

// Pool manages a set of worker goroutines to process jobs.
type Pool struct {
	jobQueue   chan Job
	numWorkers int
	ctx        context.Context
	cancel     context.CancelFunc
	wg         sync.WaitGroup
	stopOnce   sync.Once
	stopped    chan struct{}
}

// Initialize creates a new worker pool with the specified number of workers,
// job queue capacity, and context. The error returned relates to input
// validation.
//
//nolint:contextcheck // The context is used to cancel the entire pool.
func Initialize(ctx context.Context, numWorkers int, queueCapacity int) (*Pool, error) {
	// Validate inputs
	if numWorkers <= 0 {
		return nil, ErrInvalidNumWorkers
	}
	if queueCapacity <= 0 {
		return nil, ErrInvalidQueueCapacity
	}
	if ctx == nil {
		ctx = context.Background()
	}

	// Create a cancellable context
	ctx, cancel := context.WithCancel(ctx)

	pool := &Pool{
		numWorkers: numWorkers,
		ctx:        ctx,
		cancel:     cancel,
		jobQueue:   make(chan Job, queueCapacity),
		stopped:    make(chan struct{}),
	}

	// Start worker goroutines
	for i := 0; i < pool.numWorkers; i++ {
		pool.wg.Add(1)
		go pool.worker()
	}

	return pool, nil
}

// worker is the function that each worker goroutine runs.
func (p *Pool) worker() {
	defer p.wg.Done()
	for {
		select {
		case <-p.ctx.Done():
			return
		case job, ok := <-p.jobQueue:
			if !ok {
				return
			}
			// Pass the pool's context to the job
			job(p.ctx)
		}
	}
}

// Submit adds a job to the pool to be executed asynchronously. The error
// returned will be ErrPoolStopped if the pool has been stopped. The job's
// error will be ignored.
func (p *Pool) Submit(job Job) (err error) {
	select {
	case <-p.ctx.Done():
		return ErrPoolStopped
	default:
	}

	// Even with the context check above, there is a small window where the
	// pool could be stopped between the check and the job being added to the
	// queue. This defer/recover block handles the case of attempting to write
	// to a closed channel.
	// Since only one line could panic, we know the error is because the pool
	// was stopped, so we can confidently return ErrPoolStopped.

	defer func() {
		if r := recover(); r != nil {
			err = ErrPoolStopped
		}
	}()

	p.jobQueue <- job
	return nil
}

// SubmitAndWait adds a job to the pool and waits for its completion. The error
// returned will be ErrPoolStopped if the pool has been stopped, otherwise it
// will be the error returned by the job.
func (p *Pool) SubmitAndWait(job Job) error {
	// Create a wrapper function that will send the job's error to a channel
	// and return the error to the caller, allowing us to wait for the job to
	// complete.
	errChan := make(chan error, 1)
	wrappedJob := func(ctx context.Context) error {
		err := job(ctx)
		errChan <- err
		return err
	}

	// Submit the wrapped job to the pool
	if err := p.Submit(wrappedJob); err != nil {
		return err
	}

	select {
	case <-p.ctx.Done():
		return ErrPoolStopped
	case err := <-errChan:
		return err
	}
}

// Stop gracefully shuts down the worker pool. Any in-progress jobs will be
// allowed to finish and will be notified that the pool is stopping by
// having their context cancelled.
func (p *Pool) Stop() {
	p.stopOnce.Do(func() {
		p.cancel()
		close(p.jobQueue)
		p.wg.Wait()
		close(p.stopped)
	})
}

// RunAndWait runs the pool and waits until Stop is called or an OS signal is received.
func (p *Pool) RunAndWait() {
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	select {
	case <-p.stopped:
		return
	case <-sigChan:
		p.Stop()
		return
	}
}
