// Copyright 2025 Brian Wang <wangbuke@gmail.com>
// SPDX-License-Identifier: Apache-2.0

package jsexecutor

import (
	"fmt"
	"log/slog"
	"runtime"
	"sync/atomic"
	"time"
	"unsafe"
)

// JsExecutorOption contains configuration options for the JavaScript executor.
type JsExecutorOption struct {
	minPoolSize     uint32        // Minimum number of threads in the pool
	maxPoolSize     uint32        // Maximum number of threads in the pool
	queueSize       uint32        // Size of the task queue per thread
	threadTTL       time.Duration // Thread time-to-live for idle cleanup
	maxExecutions   uint32        // Maximum executions per thread before cleanup
	executeTimeout  time.Duration // Timeout for task execution
	createThreshold float64       // Queue load threshold for creating new threads (0.0-1.0)
	selectThreshold float64       // Queue load threshold for skipping busy threads (0.0-1.0)
}

// JsExecutor manages a pool of JavaScript execution threads.
type JsExecutor struct {
	options       *JsExecutorOption // Configuration options
	pool          *pool             // Thread pool
	engineFactory JsEngineFactory   // JavaScript engine factory function

	// Use atomic pointer for lock-free, zero-copy reads of initScripts.
	initScriptsPtr unsafe.Pointer // Points to []*InitScript (atomic access)

	logger *slog.Logger // Logger instance
}

// getInitScripts returns the current initialization scripts (no copy, read-only).
func (e *JsExecutor) GetInitScripts() []*InitScript {
	ptr := atomic.LoadPointer(&e.initScriptsPtr)
	if ptr == nil {
		return nil
	}
	return *(*[]*InitScript)(ptr)
}

// setInitScripts atomically sets new initialization scripts.
func (e *JsExecutor) SetInitScripts(scripts []*InitScript) {
	if len(scripts) == 0 {
		atomic.StorePointer(&e.initScriptsPtr, nil)
		return
	}

	// Create immutable copy once during write.
	newScripts := make([]*InitScript, len(scripts))
	copy(newScripts, scripts)

	// Atomically replace the pointer.
	atomic.StorePointer(&e.initScriptsPtr, unsafe.Pointer(&newScripts))
}

// AppendInitScripts appends new scripts to the existing initialization scripts.
func (e *JsExecutor) AppendInitScripts(scripts ...*InitScript) {
	if len(scripts) == 0 {
		return
	}

	for {
		oldPtr := atomic.LoadPointer(&e.initScriptsPtr)

		var oldScripts []*InitScript
		if oldPtr != nil {
			oldScripts = *(*[]*InitScript)(oldPtr)
		}

		// Create a new slice with the combined content.
		newScripts := make([]*InitScript, len(oldScripts)+len(scripts))
		copy(newScripts, oldScripts)
		copy(newScripts[len(oldScripts):], scripts)

		// Attempt to atomically swap the pointer.
		if atomic.CompareAndSwapPointer(&e.initScriptsPtr, oldPtr, unsafe.Pointer(&newScripts)) {
			break // Success
		}
		// If CAS fails, another goroutine updated the pointer. Retry the loop.
	}
}

// Start initializes and starts the executor thread pool.
func (e *JsExecutor) Start() error {
	if e.pool == nil {
		return fmt.Errorf("thread pool is not initialized")
	}
	return e.pool.start()
}

// Execute executes a JavaScript request and returns the response.
func (e *JsExecutor) Execute(request *JsRequest) (*JsResponse, error) {
	if e.pool == nil {
		return nil, fmt.Errorf("thread pool is not initialized")
	}
	start := time.Now()
	defer func() {
		if e.logger != nil {
			elapsed := time.Since(start)
			e.logger.Debug("JsExecutor.Execute",
				"id", request.Id,
				"service", request.Service,
				"elapsed", elapsed,
			)
		}
	}()
	task := newTask(request)
	return e.pool.execute(task)
}

// Stop stops the executor and shuts down all threads.
func (e *JsExecutor) Stop() error {
	if e.pool == nil {
		return fmt.Errorf("thread pool is not initialized")
	}
	return e.pool.stop()
}

// Reload reloads all threads with new initialization scripts.
func (e *JsExecutor) Reload(scripts ...*InitScript) error {
	if e.pool == nil {
		return fmt.Errorf("thread pool is not initialized")
	}

	if len(scripts) > 0 {
		e.SetInitScripts(scripts) // Use safe setter method.
	}

	return e.pool.reload()
}

// NewExecutor creates a new JavaScript executor with the given options.
func NewExecutor(opts ...func(*JsExecutor)) (*JsExecutor, error) {
	cpuCount := runtime.GOMAXPROCS(0)

	executor := &JsExecutor{
		logger: slog.Default(), // Default logger
		options: &JsExecutorOption{
			minPoolSize:     uint32(cpuCount),     // Default to CPU count
			maxPoolSize:     uint32(cpuCount * 2), // Default to 2x CPU count
			queueSize:       256,                  // Default queue size
			threadTTL:       0,                    // No TTL by default
			maxExecutions:   0,                    // No execution limit by default
			executeTimeout:  0,                    // No execution timeout by default
			createThreshold: 0.5,                  // Create new thread at 50% load
			selectThreshold: 0.75,                 // Skip thread at 75% load
		},
	}

	// Apply configuration options.
	for _, opt := range opts {
		opt(executor)
	}

	// JavaScript engine factory is required.
	if executor.engineFactory == nil {
		return nil, fmt.Errorf("JavaScript engine factory or factory must be provided")
	}

	executor.pool = newPool(executor)

	return executor, nil
}

// WithJsEngine configures the JavaScript engine builder and options.
func WithJsEngine(engineFactory JsEngineFactory) func(*JsExecutor) {
	return func(executor *JsExecutor) {
		executor.engineFactory = engineFactory
	}
}

// WithLogger configures the logger for the executor.
func WithLogger(logger *slog.Logger) func(*JsExecutor) {
	return func(executor *JsExecutor) {
		executor.logger = logger
	}
}

// WithInitScripts configures the initialization scripts.
func WithInitScripts(scripts ...*InitScript) func(*JsExecutor) {
	return func(executor *JsExecutor) {
		if len(scripts) > 0 {
			executor.SetInitScripts(scripts) // Use safe setter method.
		}
	}
}

// WithMinPoolSize sets the minimum number of threads in the pool.
func WithMinPoolSize(size uint32) func(*JsExecutor) {
	return func(executor *JsExecutor) {
		if size > 0 {
			executor.options.minPoolSize = size
		}
	}
}

// WithMaxPoolSize sets the maximum number of threads in the pool.
func WithMaxPoolSize(size uint32) func(*JsExecutor) {
	return func(executor *JsExecutor) {
		if size > 0 {
			executor.options.maxPoolSize = size
		}
	}
}

// WithQueueSize sets the size of the task queue per thread.
func WithQueueSize(size uint32) func(*JsExecutor) {
	return func(executor *JsExecutor) {
		if size > 0 {
			executor.options.queueSize = size
		}
	}
}

// WithThreadTTL sets the time-to-live for idle threads.
func WithThreadTTL(ttl time.Duration) func(*JsExecutor) {
	return func(executor *JsExecutor) {
		if ttl > 0 {
			executor.options.threadTTL = ttl
		}
	}
}

// WithMaxExecutions sets the maximum executions per thread before cleanup.
func WithMaxExecutions(max uint32) func(*JsExecutor) {
	return func(executor *JsExecutor) {
		if max > 0 {
			executor.options.maxExecutions = max
		}
	}
}

// WithExecuteTimeout sets the timeout for task execution.
func WithExecuteTimeout(timeout time.Duration) func(*JsExecutor) {
	return func(executor *JsExecutor) {
		if timeout > 0 {
			executor.options.executeTimeout = timeout
		}
	}
}

// WithCreateThreshold sets the queue load threshold for creating new threads.
func WithCreateThreshold(threshold float64) func(*JsExecutor) {
	return func(executor *JsExecutor) {
		if threshold > 0 && threshold <= 1.0 {
			executor.options.createThreshold = threshold
		}
	}
}

// WithSelectThreshold sets the queue load threshold for skipping busy threads.
func WithSelectThreshold(threshold float64) func(*JsExecutor) {
	return func(executor *JsExecutor) {
		if threshold > 0 && threshold <= 1.0 {
			executor.options.selectThreshold = threshold
		}
	}
}
