// Copyright 2025 Brian Wang <wangbuke@gmail.com>
// SPDX-License-Identifier: Apache-2.0

package jsexecutor

import (
	"fmt"
	"log/slog"
	"os"
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
	engineBuilder JsEngineBuilder   // JavaScript engine builder
	engineOptions []JsEngineOption  // Options for creating JavaScript engines

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

	// Set default logger if none provided
	if executor.logger == nil {
		executor.logger = slog.New(slog.NewTextHandler(os.Stdout, nil))
	}

	// JavaScript engine builder is required
	if executor.engineBuilder == nil {
		return nil, fmt.Errorf("JS engine builder is required")
	}

	executor.pool = newPool(executor)

	return executor, nil
}

// WithJsEngine configures the JavaScript engine builder and options
func (e *JsExecutor) WithJsEngine(builder JsEngineBuilder, opts ...JsEngineOption) func(*JsExecutor) {
	return func(executor *JsExecutor) {
		executor.engineBuilder = builder
		executor.engineOptions = opts
	}
}

// WithOptions configures the executor with custom options
func (e *JsExecutor) WithOptions(opt *JsExecutorOption) func(*JsExecutor) {
	return func(executor *JsExecutor) {
		if opt.minPoolSize > 0 {
			executor.options.minPoolSize = opt.minPoolSize
		}
		if opt.maxPoolSize > 0 {
			executor.options.maxPoolSize = opt.maxPoolSize
		}
		if opt.queueSize > 0 {
			executor.options.queueSize = opt.queueSize
		}
		if opt.threadTTL > 0 {
			executor.options.threadTTL = opt.threadTTL
		}
		if opt.maxExecutions > 0 {
			executor.options.maxExecutions = opt.maxExecutions
		}
		if opt.enqueueTimeout > 0 {
			executor.options.enqueueTimeout = opt.enqueueTimeout
		}
		if opt.executeTimeout > 0 {
			executor.options.executeTimeout = opt.executeTimeout
		}
		if opt.createThreshold > 0 && opt.createThreshold <= 1.0 {
			executor.options.createThreshold = opt.createThreshold
		}
		if opt.selectThreshold > 0 && opt.selectThreshold <= 1.0 {
			executor.options.selectThreshold = opt.selectThreshold
		}
	}
}

// WithLogger configures the logger for the executor
func (e *JsExecutor) WithLogger(logger *slog.Logger) func(*JsExecutor) {
	return func(executor *JsExecutor) {
		executor.logger = logger
	}
}

// WithThresholds configures the load thresholds for thread management
func (e *JsExecutor) WithThresholds(create, selectThreshold float64) func(*JsExecutor) {
	return func(executor *JsExecutor) {
		if create > 0 && create <= 1.0 {
			executor.options.createThreshold = create
		}
		if selectThreshold > 0 && selectThreshold <= 1.0 {
			executor.options.selectThreshold = selectThreshold
		}
	}
}

// WithInitJsScripts configures the initialization scripts
func (e *JsExecutor) WithInitJsScripts(scripts ...*InitScript) func(*JsExecutor) {
	return func(executor *JsExecutor) {
		if len(scripts) > 0 {
			executor.setInitScripts(scripts) // Use safe setter method
		}
	}
}
