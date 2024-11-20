package workpool

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"
)

func TestInitializeValidParameters(t *testing.T) {
	ctx := context.Background()
	numWorkers := 5
	queueCapacity := 10

	p, err := Initialize(ctx, numWorkers, queueCapacity)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if p.numWorkers != numWorkers {
		t.Fatalf("expected numWorkers %d, got %d", numWorkers, p.numWorkers)
	}

	if cap(p.jobQueue) != queueCapacity {
		t.Fatalf("expected queueCapacity %d, got %d", queueCapacity, cap(p.jobQueue))
	}

	p.Stop()
}

func TestInitializeInvalidParameters(t *testing.T) {
	ctx := context.Background()

	_, err := Initialize(ctx, 0, 10)
	if !errors.Is(err, ErrInvalidNumWorkers) {
		t.Fatalf("expected ErrInvalidNumWorkers, got %v", err)
	}

	_, err = Initialize(ctx, 5, 0)
	if !errors.Is(err, ErrInvalidQueueCapacity) {
		t.Fatalf("expected ErrInvalidQueueCapacity, got %v", err)
	}
}

func TestSubmitJob(t *testing.T) {
	ctx := context.Background()
	p, err := Initialize(ctx, 2, 5)
	if err != nil {
		t.Fatalf("Initialize failed: %v", err)
	}
	defer p.Stop()

	jobExecuted := make(chan bool, 1)
	err = p.Submit(func(ctx context.Context) error {
		jobExecuted <- true
		return nil
	})
	if err != nil {
		t.Fatalf("Submit failed: %v", err)
	}

	select {
	case <-jobExecuted:
		// Job executed successfully
	case <-time.After(1 * time.Second):
		t.Fatal("Job was not executed within 1 second")
	}
}

func TestSubmitAfterStop(t *testing.T) {
	ctx := context.Background()
	p, err := Initialize(ctx, 2, 5)
	if err != nil {
		t.Fatalf("Initialize failed: %v", err)
	}
	p.Stop()

	err = p.Submit(func(ctx context.Context) error {
		return nil
	})
	if err != ErrPoolStopped {
		t.Fatalf("expected ErrPoolStopped, got %v", err)
	}
}

func TestSubmitAndWait(t *testing.T) {
	ctx := context.Background()
	p, err := Initialize(ctx, 2, 5)
	if err != nil {
		t.Fatalf("Initialize failed: %v", err)
	}
	defer p.Stop()

	jobExecuted := false
	err = p.SubmitAndWait(func(ctx context.Context) error {
		jobExecuted = true
		return nil
	})
	if err != nil {
		t.Fatalf("SubmitAndWait failed: %v", err)
	}

	if !jobExecuted {
		t.Fatal("Job was not executed")
	}
}

func TestSubmitAndWaitAfterStop(t *testing.T) {
	ctx := context.Background()
	p, err := Initialize(ctx, 2, 5)
	if err != nil {
		t.Fatalf("Initialize failed: %v", err)
	}
	p.Stop()

	err = p.SubmitAndWait(func(ctx context.Context) error {
		return nil
	})
	if err != ErrPoolStopped {
		t.Fatalf("expected ErrPoolStopped, got %v", err)
	}
}

func TestJobReturnsError(t *testing.T) {
	ctx := context.Background()
	p, err := Initialize(ctx, 2, 5)
	if err != nil {
		t.Fatalf("Initialize failed: %v", err)
	}
	defer p.Stop()

	expectedErr := errors.New("job error")
	err = p.SubmitAndWait(func(ctx context.Context) error {
		return expectedErr
	})
	if err != expectedErr {
		t.Fatalf("expected %v, got %v", expectedErr, err)
	}
}

func TestContextCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	p, err := Initialize(ctx, 2, 5)
	if err != nil {
		t.Fatalf("Initialize failed: %v", err)
	}
	defer p.Stop()

	jobStarted := make(chan bool)
	jobCancelled := make(chan bool)

	err = p.Submit(func(ctx context.Context) error {
		jobStarted <- true
		select {
		case <-ctx.Done():
			jobCancelled <- true
		case <-time.After(2 * time.Second):
			// Job completed
		}
		return nil
	})
	if err != nil {
		t.Fatalf("Submit failed: %v", err)
	}

	// Wait for the job to start
	<-jobStarted
	// Cancel the context
	cancel()

	select {
	case <-jobCancelled:
		// Job detected context cancellation
	case <-time.After(1 * time.Second):
		t.Fatal("Job did not detect context cancellation within 1 second")
	}
}

func TestPoolStop(t *testing.T) {
	ctx := context.Background()
	p, err := Initialize(ctx, 2, 5)
	if err != nil {
		t.Fatalf("Initialize failed: %v", err)
	}

	var mu sync.Mutex
	jobCount := 0
	for i := 0; i < 10; i++ {
		err = p.Submit(func(ctx context.Context) error {
			time.Sleep(100 * time.Millisecond)
			mu.Lock()
			jobCount++
			mu.Unlock()
			return nil
		})
		if err != nil {
			t.Fatalf("Submit failed: %v", err)
		}
	}

	// Stop the pool after a short delay
	time.AfterFunc(200*time.Millisecond, func() {
		p.Stop()
	})

	// Wait for the pool to stop
	p.wg.Wait()

	mu.Lock()
	count := jobCount
	mu.Unlock()

	if count == 0 {
		t.Fatal("No jobs were executed")
	} else {
		t.Logf("Jobs executed before and after pool stopped: %d", count)
	}
}

func TestNoJobsProcessedAfterStop(t *testing.T) {
	ctx := context.Background()
	p, err := Initialize(ctx, 2, 5)
	if err != nil {
		t.Fatalf("Initialize failed: %v", err)
	}
	p.Stop()

	jobExecuted := make(chan bool, 1)
	err = p.Submit(func(ctx context.Context) error {
		jobExecuted <- true
		return nil
	})
	if err != ErrPoolStopped {
		t.Fatalf("expected ErrPoolStopped, got %v", err)
	}

	select {
	case <-jobExecuted:
		t.Fatal("Job was executed after pool was stopped")
	case <-time.After(500 * time.Millisecond):
		// No job executed, as expected
	}
}
