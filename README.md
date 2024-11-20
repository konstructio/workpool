# Workpool

[![GoDoc](https://godoc.org/github.com/konstructio/workpool?status.svg)](https://godoc.org/github.com/konstructio/workpool)

`workpool` is a Go library that provides a simple and efficient worker pool implementation. It allows you to manage and execute tasks asynchronously with a controlled level of concurrency. The library handles task submission, worker management, and graceful shutdown with context cancellation.

## Features

- **Fixed Number of Workers**: Create a pool with a specified number of worker goroutines.
- **Asynchronous Task Execution**: Submit tasks to be executed asynchronously.
- **Synchronous Task Submission**: Submit tasks and wait for their completion.
- **Graceful Shutdown**: Stop the pool gracefully, allowing workers to finish their current tasks.
- **Context Propagation**: Each task receives a context to handle cancellations and timeouts.
- **Configurable Queue Capacity**: Specify the capacity of the job queue to control memory usage and backpressure.

## Installation

To install the `workpool` library, use `go get`:

```bash
go get github.com/konstructio/workpool@latest
```

## Usage

Import the package in your Go code:

```go
import "github.com/konstructio/workpool"
```

### Creating a Pool

Create a new worker pool by specifying the context, number of workers, and job queue capacity:

```go
ctx := context.Background()
numWorkers := 5
queueCapacity := 100

pool, err := workpool.Initialize(ctx, numWorkers, queueCapacity)
if err != nil {
    // Handle error
}
```

### Submitting Tasks Asynchronously

Submit tasks to the pool to be executed asynchronously. This function returns an error if the pool is stopped, but otherwise doesn't return any other error, as the task is executed asynchronously. If you need the error message from the task, you can use `SubmitAndWait()` instead.

```go
err := pool.Submit(func(ctx context.Context) error {
    // Task logic here
    fmt.Println("Executing async task")
    return nil
})
if err != nil {
    // Handle error (e.g., pool is stopped)
}
```

### Submitting Tasks and Waiting for Completion

Submit a task and wait for its completion using `SubmitAndWait`. This function returns an error if the pool is stopped. Contrary to `Submit()`, this function will also wait for the Job to be executed then return any potential error returned by the Job execution.

```go
err := pool.SubmitAndWait(func(ctx context.Context) error {
    // Task logic here
    fmt.Println("Executing synchronous task")
    return nil
})
if err != nil {
    // Handle error (e.g., pool is stopped)
}

err := pool.SubmitAndWait(func(ctx context.Context) error {
    // Task logic here
    return fmt.Errorf("task failed")
})
if err != nil {
    fmt.Println("Task error:", err)
}
```

### Graceful Shutdown

Use `Stop()` to stop the pool gracefully, allowing workers to finish their current tasks. The pool will not accept new tasks after calling `Stop`, and it will not forcefully stop the workers from completing their current tasks.

```go
pool.Stop()
```

Alternatively, you can also cancel the context associated with the `Initialize()` function call to stop the pool:

```go
poolCtx, cancel := context.WithCancel(ctx)

pool, err := workpool.Initialize(poolCtx, numWorkers, queueCapacity)
if err != nil { }

go func() {
  pool.RunAndWait()
}()

// stop the pool
cancel()
```


### Running the Pool and Waiting for Shutdown

Use `RunAndWait` to run the pool and block until `Stop` is called or an OS interrupt signal is received:

```go
go func() {
  // Use a goroutine to prevent blocking the program
  pool.RunAndWait()
}()

// Simulate work and then stop the pool after 5 seconds
time.Sleep(5 * time.Second)
pool.Stop()
```

### Example: Basic Usage

```go
package main

import (
    "context"
    "fmt"
    "time"

    "github.com/konstructio/workpool"
)

func main() {
    ctx := context.Background()
    pool, err := workpool.Initialize(ctx, 5, 100)
    if err != nil {
        fmt.Println("Error initializing pool:", err)
        return
    }

    defer pool.Stop()

    // Submit tasks asynchronously
    for i := 0; i < 10; i++ {
        taskID := i
        err := pool.Submit(func(ctx context.Context) error {
            fmt.Printf("Async Task %d started\n", taskID)
            time.Sleep(1 * time.Second)
            fmt.Printf("Async Task %d completed\n", taskID)
            return nil
        })
        if err != nil {
            fmt.Printf("Error submitting task %d: %v\n", taskID, err)
        }
    }

    // Submit a task and wait for its completion
    err = pool.SubmitAndWait(func(ctx context.Context) error {
        fmt.Println("Synchronous Task started")
        time.Sleep(2 * time.Second)
        fmt.Println("Synchronous Task completed")
        return nil
    })
    if err != nil {
        fmt.Println("Error executing synchronous task:", err)
    }

    // Run the pool and wait for shutdown
    go func() {
        time.Sleep(5 * time.Second)
        pool.Stop()
    }()
    pool.RunAndWait()
}
```

### Example: Handling Context Cancellation in Tasks

Tasks receive a context that can be used to handle cancellations or timeouts:

```go
err := pool.Submit(func(ctx context.Context) error {
    select {
    case <-ctx.Done():
        // Handle cancellation
        fmt.Println("Task was cancelled")
        return ctx.Err()
    case <-time.After(3 * time.Second):
        // Perform task
        fmt.Println("Task completed")
    }
    return nil
})
if err != nil {
    // Handle error
}
```

## API Reference

### Functions

- **Initialize(ctx context.Context, numWorkers int, queueCapacity int) (*Pool, error)**
  - Creates a new worker pool.
  - `ctx`: The context for the pool; can be used to set deadlines or cancellation.
  - `numWorkers`: Number of worker goroutines.
  - `queueCapacity`: Capacity of the job queue.
  - Returns a pointer to the `Pool` and an error if the parameters are invalid.

### Pool Methods

- **(p *Pool) Submit(job Job) error**
  - Submits a job to be executed asynchronously.
  - Returns an error if the pool is stopped.

- **(p *Pool) SubmitAndWait(job Job) error**
  - Submits a job and waits for its completion.
  - Returns any error returned by the job or if the pool is stopped.

- **(p *Pool) Stop()**
  - Stops the pool gracefully, allowing workers to finish current tasks.

- **(p *Pool) RunAndWait()**
  - Runs the pool and blocks until `Stop` is called or an OS interrupt signal is received.

### Types

- **Job**
  - A function type `func(ctx context.Context) error`.
  - Represents a unit of work to be executed by the pool.
