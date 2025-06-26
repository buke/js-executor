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

// JsExecutorOption contains configuration options for the JavaScript executor
type JsExecutorOption struct {
	minPoolSize     uint32        // Minimum number of threads in the pool
	maxPoolSize     uint32        // Maximum number of threads in the pool
	queueSize       uint32        // Size of the task queue per thread
	threadTTL       time.Duration // Thread time-to-live for idle cleanup
	maxExecutions   uint32        // Maximum executions per thread before cleanup
	enqueueTimeout  time.Duration // Timeout for enqueuing tasks
	executeTimeout  time.Duration // Timeout for task execution
	createThreshold float64       // Queue load threshold for creating new threads (0.0-1.0)
	selectThreshold float64       // Queue load threshold for skipping busy threads (0.0-1.0)
}

// JsExecutor manages a pool of JavaScript execution threads
type JsExecutor struct {
	options       *JsExecutorOption // Configuration options
	pool          *pool             // Thread pool
	engineFactory JsEngineFactory   // JavaScript engine factory function

	// Use atomic pointer for lock-free, zero-copy reads of initScripts
	initScriptsPtr unsafe.Pointer // Points to []*InitScript (atomic access)

	logger *slog.Logger // Logger instance
}

// getInitScripts returns the current initialization scripts (no copy, read-only)
func (e *JsExecutor) getInitScripts() []*InitScript {
	ptr := atomic.LoadPointer(&e.initScriptsPtr)
	if ptr == nil {
		return nil
	}
	return *(*[]*InitScript)(ptr)
}

// setInitScripts atomically sets new initialization scripts
func (e *JsExecutor) setInitScripts(scripts []*InitScript) {
	if len(scripts) == 0 {
		atomic.StorePointer(&e.initScriptsPtr, nil)
		return
	}

	// Create immutable copy once during write
	newScripts := make([]*InitScript, len(scripts))
	copy(newScripts, scripts)

	// Atomically replace the pointer
	atomic.StorePointer(&e.initScriptsPtr, unsafe.Pointer(&newScripts))
}

// Start initializes and starts the executor thread pool
func (e *JsExecutor) Start() error {
	if e.pool == nil {
		return fmt.Errorf("thread pool is not initialized")
	}
	return e.pool.start()
}

// Execute executes a JavaScript request and returns the response
func (e *JsExecutor) Execute(request *JsRequest) (*JsResponse, error) {
	if e.pool == nil {
		return nil, fmt.Errorf("thread pool is not initialized")
	}
	task := newTask(request)
	return e.pool.execute(task)
}

// Stop stops the executor and shuts down all threads
func (e *JsExecutor) Stop() error {
	if e.pool == nil {
		return fmt.Errorf("thread pool is not initialized")
	}
	return e.pool.stop()
}

// Reload reloads all threads with new initialization scripts
func (e *JsExecutor) Reload(scripts ...*InitScript) error {
	if e.pool == nil {
		return fmt.Errorf("thread pool is not initialized")
	}

	if len(scripts) > 0 {
		e.setInitScripts(scripts) // Use safe setter method
	}

	return e.pool.reload()
}

// NewExecutor creates a new JavaScript executor with the given options
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
			enqueueTimeout:  30 * time.Second,     // 30 second enqueue timeout
			executeTimeout:  60 * time.Second,     // 60 second execution timeout
			createThreshold: 0.5,                  // Create new thread at 50% load
			selectThreshold: 0.75,                 // Skip thread at 75% load
		},
	}

	// Apply configuration options
	for _, opt := range opts {
		opt(executor)
	}

	// JavaScript engine factory is required
	if executor.engineFactory == nil {
		return nil, fmt.Errorf("JavaScript engine factory or factory must be provided")
	}

	executor.pool = newPool(executor)

	return executor, nil
}

// WithJsEngine configures the JavaScript engine builder and options
func WithJsEngine(engineFactory JsEngineFactory) func(*JsExecutor) {
	return func(executor *JsExecutor) {
		executor.engineFactory = engineFactory
	}
}

// WithLogger configures the logger for the executor
func WithLogger(logger *slog.Logger) func(*JsExecutor) {
	return func(executor *JsExecutor) {
		executor.logger = logger
	}
}

// WithInitJsScripts configures the initialization scripts
func WithInitScripts(scripts ...*InitScript) func(*JsExecutor) {
	return func(executor *JsExecutor) {
		if len(scripts) > 0 {
			executor.setInitScripts(scripts) // Use safe setter method
		}
	}
}

func WithMinPoolSize(size uint32) func(*JsExecutor) {
	return func(executor *JsExecutor) {
		if size > 0 {
			executor.options.minPoolSize = size
		}
	}
}
func WithMaxPoolSize(size uint32) func(*JsExecutor) {
	return func(executor *JsExecutor) {
		if size > 0 {
			executor.options.maxPoolSize = size
		}
	}
}

func WithQueueSize(size uint32) func(*JsExecutor) {
	return func(executor *JsExecutor) {
		if size > 0 {
			executor.options.queueSize = size
		}
	}
}

func WithThreadTTL(ttl time.Duration) func(*JsExecutor) {
	return func(executor *JsExecutor) {
		if ttl > 0 {
			executor.options.threadTTL = ttl
		}
	}
}

func WithMaxExecutions(max uint32) func(*JsExecutor) {
	return func(executor *JsExecutor) {
		if max > 0 {
			executor.options.maxExecutions = max
		}
	}
}

func WithEnqueueTimeout(timeout time.Duration) func(*JsExecutor) {
	return func(executor *JsExecutor) {
		if timeout > 0 {
			executor.options.enqueueTimeout = timeout
		}
	}
}

func WithExecuteTimeout(timeout time.Duration) func(*JsExecutor) {
	return func(executor *JsExecutor) {
		if timeout > 0 {
			executor.options.executeTimeout = timeout
		}
	}
}

func WithCreateThreshold(threshold float64) func(*JsExecutor) {
	return func(executor *JsExecutor) {
		if threshold > 0 && threshold <= 1.0 {
			executor.options.createThreshold = threshold
		}
	}
}

func WithSelectThreshold(threshold float64) func(*JsExecutor) {
	return func(executor *JsExecutor) {
		if threshold > 0 && threshold <= 1.0 {
			executor.options.selectThreshold = threshold
		}
	}
}
