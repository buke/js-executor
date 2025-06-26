// Copyright 2025 Brian Wang <wangbuke@gmail.com>
// SPDX-License-Identifier: Apache-2.0

package jsexecutor

import (
	"fmt"
	"runtime"
	"sync/atomic"
	"time"
)

const (
	defaultActionTimeout = 5 * time.Second
)

// threadAction represents an action that can be performed on a thread
type threadAction int

const (
	actionStop   threadAction = iota // Stop the thread
	actionReload                     // Reload the thread's JavaScript engine
)

// threadActionRequest represents a request to perform an action on a thread
type threadActionRequest struct {
	action threadAction // The action to perform
	done   chan error   // Channel to signal completion and return any error
}

// thread represents a single JavaScript execution thread
type thread struct {
	executor *JsExecutor // Reference to the parent executor
	name     string      // Human-readable name for the thread
	threadId uint32      // Unique identifier for the thread

	taskQueue   chan *task                // Channel for receiving tasks to execute
	actionQueue chan *threadActionRequest // Channel for receiving control actions

	lastUsedNano int64  // Timestamp of last task execution (atomic, nanoseconds)
	taskID       uint32 // Number of tasks executed by this thread (atomic)

	jsEngine JsEngine // JavaScript engine instance
}

// newThread creates a new thread instance
func newThread(executor *JsExecutor, name string, threadId uint32) *thread {
	return &thread{
		executor:     executor,
		name:         name,
		threadId:     threadId,
		taskQueue:    make(chan *task, executor.options.queueSize),
		actionQueue:  make(chan *threadActionRequest, 1),
		lastUsedNano: time.Now().UnixNano(),
		taskID:       0,
	}
}

// initEngine initializes the JavaScript engine for this thread
func (t *thread) initEngine() error {
	// Create a new JavaScript engine instance
	jsEngine, err := t.executor.engineFactory()
	if err != nil {
		return fmt.Errorf("failed to create JS engine: %w", err)
	}
	t.jsEngine = jsEngine

	// Get initialization scripts safely
	scripts := t.executor.getInitScripts()

	// Initialize the engine with scripts
	if err := t.jsEngine.Init(scripts); err != nil {
		return fmt.Errorf("failed to init JS engine: %w", err)
	}

	return nil
}

// run is the main thread loop that processes tasks and actions
func (t *thread) run() {
	// Lock this goroutine to an OS thread for consistent execution environment
	runtime.LockOSThread()

	// Cleanup when the thread exits
	defer func() {
		if r := recover(); r != nil {
			if t.executor.logger != nil {
				t.executor.logger.Error("Thread panic",
					"thread", t.name,
					"error", r)
			}
		}
		// Close the JavaScript engine
		if t.jsEngine != nil {
			if err := t.jsEngine.Close(); err != nil {
				if t.executor.logger != nil {
					t.executor.logger.Error("Failed to close JS engine",
						"thread", t.name,
						"error", err)
				}
			}
		}
	}()

	// Initialize the JavaScript engine
	if err := t.initEngine(); err != nil {
		panic(fmt.Sprintf("failed to initialize JS engine for thread %s: %v", t.name, err))
	}

	var pendingAction *threadActionRequest

	// Main execution loop
	for {
		// Execute pending action if task queue is empty
		if pendingAction != nil && len(t.taskQueue) == 0 {
			t.executeAction(pendingAction)
			pendingAction = nil
			continue
		}

		select {
		case task := <-t.taskQueue:
			if task == nil {
				return // Channel closed, exit
			}
			t.executeTask(task)

		case actionReq := <-t.actionQueue:
			switch actionReq.action {
			case actionStop:
				// Handle stop immediately, cancel any pending action
				if pendingAction != nil {
					pendingAction.done <- fmt.Errorf("thread is shutting down")
				}
				t.executeAction(actionReq)
				return

			case actionReload:
				// Handle reload when queue is empty
				if len(t.taskQueue) == 0 {
					t.executeAction(actionReq)
				} else {
					pendingAction = actionReq
					if t.executor.logger != nil {
						t.executor.logger.Info("Reload request received, waiting for queue to empty",
							"thread", t.name,
							"queueSize", len(t.taskQueue))
					}
				}
			}
		}
	}
}

// executeAction executes a thread action (stop or reload)
func (t *thread) executeAction(req *threadActionRequest) {
	switch req.action {
	case actionStop:
		if t.executor.logger != nil {
			t.executor.logger.Info("Thread stopping", "thread", t.name)
		}

		// Close the JavaScript engine
		if t.jsEngine != nil {
			if err := t.jsEngine.Close(); err != nil {
				if t.executor.logger != nil {
					t.executor.logger.Error("Failed to close JS engine",
						"thread", t.name,
						"error", err)
				}
			}
			t.jsEngine = nil
		}

		req.done <- nil

	case actionReload:
		if t.executor.logger != nil {
			t.executor.logger.Info("Thread starting reload", "thread", t.name)
		}

		// Get initialization scripts safely
		scripts := t.executor.getInitScripts()
		err := t.jsEngine.Reload(scripts)
		req.done <- err

		if err != nil {
			if t.executor.logger != nil {
				t.executor.logger.Error("Thread reload failed",
					"thread", t.name,
					"error", err)
			}
		} else {
			if t.executor.logger != nil {
				t.executor.logger.Info("Thread reload completed successfully",
					"thread", t.name)
			}
		}
	}
}

// executeTask executes a single JavaScript task
func (t *thread) executeTask(task *task) {
	// Update task execution statistics on completion
	defer func() {
		if r := recover(); r != nil {
			// Handle panic during task execution
			task.resultChan <- &taskResult{
				response: nil,
				err:      fmt.Errorf("panic in thread %s: %v", t.name, r),
			}
			if t.executor.logger != nil {
				t.executor.logger.Error("Task execution panic",
					"thread", t.name,
					"taskID", t.getTaskCount(),
					"error", r)
			}
		}
		// Update thread statistics atomically
		atomic.StoreInt64(&t.lastUsedNano, time.Now().UnixNano())
		atomic.AddUint32(&t.taskID, 1)
	}()

	task.status = taskStatusRunning

	// Execute the JavaScript request
	response, err := t.jsEngine.Execute(task.request)
	task.resultChan <- &taskResult{
		response: response,
		err:      err,
	}
	task.status = taskStatusCompleted
}

// reload sends a reload request to the thread
func (t *thread) reload() error {
	req := &threadActionRequest{
		action: actionReload,
		done:   make(chan error, 1),
	}

	// Send reload request with timeout
	select {
	case t.actionQueue <- req:
	case <-time.After(defaultActionTimeout):
		return fmt.Errorf("timeout sending reload request to thread %s", t.name)
	}

	// Wait for completion
	return <-req.done
}

// stop gracefully stops the thread
func (t *thread) stop() {
	req := &threadActionRequest{
		action: actionStop,
		done:   make(chan error, 1),
	}

	// Send stop request with timeout
	select {
	case t.actionQueue <- req:
	case <-time.After(defaultActionTimeout):
		if t.executor.logger != nil {
			t.executor.logger.Warn("Timeout sending stop signal", "thread", t.name)
		}
		return
	}

	// Wait for completion
	<-req.done

	// Close channels to prevent further operations
	close(t.taskQueue)
	close(t.actionQueue)
}

// getTaskCount returns the number of tasks executed by this thread (thread-safe)
func (t *thread) getTaskCount() uint32 {
	return atomic.LoadUint32(&t.taskID)
}

// getLastUsed returns the timestamp of the last task execution (thread-safe)
func (t *thread) getLastUsed() time.Time {
	return time.Unix(0, atomic.LoadInt64(&t.lastUsedNano))
}
