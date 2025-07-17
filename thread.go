// Copyright 2025 Brian Wang <wangbuke@gmail.com>
// SPDX-License-Identifier: Apache-2.0

package jsexecutor

import (
	"fmt"
	"runtime"
	"sync/atomic"
	"time"
)

// threadAction represents an action that can be performed on a thread.
type threadAction int

const (
	actionStop   threadAction = iota // Stop the thread
	actionReload                     // Reload the thread's JavaScript engine
	actionRetire                     // Retire the thread
)

// String returns the string representation of a threadAction.
func (a threadAction) String() string {
	switch a {
	case actionStop:
		return "stop"
	case actionReload:
		return "reload"
	case actionRetire:
		return "retire"
	default:
		return "unknown"
	}
}

// threadActionRequest represents a request to perform an action on a thread.
type threadActionRequest struct {
	action threadAction // The action to perform
	done   chan error   // Channel to signal completion and return any error
}

// thread represents a single JavaScript execution thread.
type thread struct {
	executor *JsExecutor // Reference to the parent executor
	name     string      // Human-readable name for the thread
	threadId uint32      // Unique identifier for the thread

	taskQueue   chan *task                // Channel for receiving tasks to execute
	actionQueue chan *threadActionRequest // Channel for receiving control actions
	initCh      chan error                // Channel to signal initialization completion

	lastUsedNano int64  // Timestamp of last task execution (atomic, nanoseconds)
	taskID       uint32 // Number of tasks executed by this thread (atomic)

	jsEngine JsEngine // JavaScript engine instance
}

// newThread creates a new thread instance.
func newThread(executor *JsExecutor, name string, threadId uint32) *thread {
	return &thread{
		executor:     executor,
		name:         name,
		threadId:     threadId,
		taskQueue:    make(chan *task, executor.options.queueSize),
		actionQueue:  make(chan *threadActionRequest, 1),
		initCh:       make(chan error, 1),
		lastUsedNano: time.Now().UnixNano(),
		taskID:       0,
	}
}

// getTaskCount returns the number of tasks executed by this thread (thread-safe).
func (t *thread) getTaskCount() uint32 {
	return atomic.LoadUint32(&t.taskID)
}

// getLastUsed returns the timestamp of the last task execution (thread-safe).
func (t *thread) getLastUsed() time.Time {
	return time.Unix(0, atomic.LoadInt64(&t.lastUsedNano))
}

// initEngine initializes the JavaScript engine for this thread.
func (t *thread) initEngine() error {
	// Create a new JavaScript engine instance
	jsEngine, err := t.executor.engineFactory()
	if err != nil {
		return fmt.Errorf("failed to create JS engine: %w", err)
	}
	t.jsEngine = jsEngine

	// Get initialization scripts safely
	scripts := t.executor.GetJsScripts()

	// Initialize the engine with scripts
	if err := t.jsEngine.Load(scripts); err != nil {
		return fmt.Errorf("failed to init JS engine: %w", err)
	}

	return nil
}

// run is the main thread loop that processes tasks and actions.
func (t *thread) run() {
	// Lock this goroutine to an OS thread for consistent execution environment
	runtime.LockOSThread()

	// Cleanup when the thread exits
	defer func() {
		if t.jsEngine != nil {
			if err := t.jsEngine.Close(); err != nil {
				if t.executor != nil && t.executor.logger != nil {
					t.executor.logger.Error("Failed to close JS engine",
						"thread", t.name,
						"error", err)
				}
			}
		}
	}()

	// Use a queue to store all pending actions
	var pendingActions []*threadActionRequest

	// Initialize the JavaScript engine
	if err := t.initEngine(); err != nil {
		t.initCh <- err
		close(t.initCh)
		if t.executor != nil && t.executor.logger != nil {
			t.executor.logger.Error("Failed to initialize JS engine",
				"thread", t.name,
				"error", err,
			)
		}
		return
	}
	// Signal successful initialization
	t.initCh <- nil
	close(t.initCh)

	// Main execution loop
	for {
		// Execute all pending actions if task queue is empty
		for len(pendingActions) > 0 && len(t.taskQueue) == 0 {
			action := pendingActions[0]
			pendingActions = pendingActions[1:]
			t.executeAction(action)
		}

		select {
		case task := <-t.taskQueue:
			if task == nil {
				return // Channel closed, exit the thread
			}
			t.executeTask(task)
			if t.checkAndRetireIfNeeded(&pendingActions) {
				if t.executor.logger != nil {
					t.executor.logger.Debug("Thread reached max executions, retiring",
						"thread", t.name,
						"taskCount", t.getTaskCount(),
					)
				}
				t.notifyPoolReplenish()
				continue
			}
		case actionReq := <-t.actionQueue:
			// Queue all control actions, execute in order after tasks are done
			pendingActions = append(pendingActions, actionReq)
			continue
		}
	}
}

// executeAction executes a thread action (stop, reload, retire, or default).
func (t *thread) executeAction(req *threadActionRequest) {
	defer func() {
		if r := recover(); r != nil {
			err := fmt.Errorf("panic in executeAction: %v", r)
			if t.executor != nil && t.executor.logger != nil {
				t.executor.logger.Error("Panic recovered in executeAction", "thread", t.name, "action", req.action.String(), "error", r)
			}
			// Ensure the done channel is notified even on panic
			if req.done != nil {
				req.done <- err
			}
		}
	}()

	if req == nil {
		if t.executor != nil && t.executor.logger != nil {
			t.executor.logger.Error("executeAction called with nil request", "thread", t.name)
		}
		return
	}

	switch req.action {
	case actionReload:
		// Get initialization scripts safely
		scripts := t.executor.GetJsScripts()
		err := t.jsEngine.Load(scripts)
		if err != nil && t.executor.logger != nil {
			t.executor.logger.Error("Thread reload failed",
				"thread", t.name,
				"error", err)
		}
		req.done <- err

	case actionStop:
		// Close the JavaScript engine
		if t.jsEngine != nil {
			err := t.jsEngine.Close()
			if err != nil && t.executor.logger != nil {
				t.executor.logger.Error("Failed to close JS engine",
					"thread", t.name,
					"error", err)
			}
			t.jsEngine = nil
			req.done <- err
		} else {
			// If engine is already nil, just signal completion
			req.done <- nil
		}

	case actionRetire:
		// Close the JavaScript engine
		if t.jsEngine != nil {
			err := t.jsEngine.Close()
			if err != nil && t.executor.logger != nil {
				t.executor.logger.Error("Failed to close JS engine",
					"thread", t.name,
					"error", err)
			}
			t.jsEngine = nil
			req.done <- err
		} else {
			// If engine is already nil, just signal completion
			req.done <- nil
		}

	default:
		// Handle unknown action types
		req.done <- nil
	}
}

// executeTask executes a single JavaScript task.
func (t *thread) executeTask(task *task) {
	// Update task execution statistics on completion
	defer func() {
		if r := recover(); r != nil {
			// Handle panic during task execution
			task.resultChan <- &taskResult{
				response: nil,
				err:      fmt.Errorf("panic in thread %s: %v", t.name, r),
			}
			if t.executor != nil && t.executor.logger != nil {
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

// reload sends a reload request to the thread and waits for completion.
func (t *thread) reload() error {
	req := &threadActionRequest{
		action: actionReload,
		done:   make(chan error, 1),
	}
	// Send reload request
	t.actionQueue <- req
	// Wait for completion
	return <-req.done
}

// stop gracefully stops the thread and closes its channels.
func (t *thread) stop() {
	req := &threadActionRequest{
		action: actionStop,
		done:   make(chan error, 1),
	}
	// Send stop request
	t.actionQueue <- req
	// Wait for completion
	<-req.done
	// Close channels to prevent further operations
	close(t.taskQueue)
	close(t.actionQueue)
}

// retire cleans up the thread, closing its channels and releasing resources.
func (t *thread) retire() {
	req := &threadActionRequest{
		action: actionRetire,
		done:   make(chan error, 1),
	}
	// Send cleanup request
	t.actionQueue <- req
	// Wait for completion
	<-req.done
	// Close channels to prevent further operations
	close(t.taskQueue)
	close(t.actionQueue)
}

// checkAndRetireIfNeeded checks if the thread has reached maxExecutions and retires if needed.
// If retire is needed, it appends a retire request to pendingActions and returns true.
func (t *thread) checkAndRetireIfNeeded(pendingActions *[]*threadActionRequest) bool {
	if t.executor.options.maxExecutions > 0 && t.getTaskCount() >= t.executor.options.maxExecutions {
		t.notifyPoolReplenish()
		*pendingActions = append(*pendingActions, &threadActionRequest{
			action: actionRetire,
			done:   make(chan error, 1),
		})
		return true
	}
	return false
}

// notifyPoolReplenish notifies the pool to check and replenish threads if needed.
func (t *thread) notifyPoolReplenish() {
	select {
	case t.executor.pool.replenishChan <- struct{}{}:
	default:
	}
}
