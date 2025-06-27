// Copyright 2025 Brian Wang <wangbuke@gmail.com>
// SPDX-License-Identifier: Apache-2.0

package jsexecutor

import (
	"errors"
	"log/slog"
	"sync"
	"testing"
	"time"
)

// TestThreadAction_String tests the string representation of threadAction.
func TestThreadAction_String(t *testing.T) {
	tests := []struct {
		action   threadAction
		expected string
	}{
		{actionStop, "stop"},
		{actionReload, "reload"},
		{actionRetire, "retire"},
		{threadAction(999), "unknown"},
	}
	for _, tt := range tests {
		if got := tt.action.String(); got != tt.expected {
			t.Errorf("threadAction(%d).String() = %q, want %q", tt.action, got, tt.expected)
		}
	}
}

// TestThread_InitEngine_Success tests successful initialization of the JS engine.
func TestThread_InitEngine_Success(t *testing.T) {
	exec := &JsExecutor{
		engineFactory: mockEngineFactory(),
		options:       &JsExecutorOption{queueSize: 2},
	}
	th := newThread(exec, "t1", 1)
	if err := th.initEngine(); err != nil {
		t.Fatalf("initEngine failed: %v", err)
	}
	if th.jsEngine == nil {
		t.Error("jsEngine should be set after initEngine")
	}
}

// TestThread_InitEngine_FactoryError tests error handling when engineFactory fails.
func TestThread_InitEngine_FactoryError(t *testing.T) {
	exec := &JsExecutor{
		engineFactory: func() (JsEngine, error) { return nil, errors.New("factory error") },
		options:       &JsExecutorOption{queueSize: 2},
	}
	th := newThread(exec, "t2", 2)
	err := th.initEngine()
	if err == nil || err.Error() != "failed to create JS engine: factory error" {
		t.Errorf("Expected factory error, got: %v", err)
	}
}

// TestThread_InitEngine_InitError tests error handling when engine.Init fails.
func TestThread_InitEngine_InitError(t *testing.T) {
	engine := &mockEngine{
		initFunc: func(scripts []*InitScript) error { return errors.New("init error") },
	}
	exec := &JsExecutor{
		engineFactory: func() (JsEngine, error) { return engine, nil },
		options:       &JsExecutorOption{queueSize: 2},
	}
	th := newThread(exec, "t3", 3)
	err := th.initEngine()
	if err == nil || err.Error() != "failed to init JS engine: init error" {
		t.Errorf("Expected init error, got: %v", err)
	}
}

// TestThread_ExecuteTask tests execution of a single JavaScript task.
func TestThread_ExecuteTask(t *testing.T) {
	exec := &JsExecutor{
		engineFactory: mockEngineFactory(),
		options:       &JsExecutorOption{queueSize: 2},
	}
	th := newThread(exec, "t4", 4)
	_ = th.initEngine()
	req := &JsRequest{Id: "abc"}
	task := newTask(req)
	th.executeTask(task)
	result := <-task.resultChan
	if result.err != nil {
		t.Errorf("executeTask returned error: %v", result.err)
	}
	if result.response == nil || result.response.Id != "abc" {
		t.Errorf("Unexpected response: %+v", result.response)
	}
}

// TestThread_Reload tests the reload operation of a thread.
func TestThread_Reload(t *testing.T) {
	exec := &JsExecutor{
		engineFactory: mockEngineFactory(),
		options:       &JsExecutorOption{queueSize: 2},
	}
	th := newThread(exec, "t5", 5)
	_ = th.initEngine()
	go th.run()
	time.Sleep(10 * time.Millisecond)
	err := th.reload()
	if err != nil {
		t.Errorf("reload failed: %v", err)
	}
	th.stop()
}

// TestThread_Stop tests the stop operation and channel closure.
func TestThread_Stop(t *testing.T) {
	exec := &JsExecutor{
		engineFactory: mockEngineFactory(),
		options:       &JsExecutorOption{queueSize: 2},
	}
	th := newThread(exec, "t6", 6)
	_ = th.initEngine()
	go th.run()
	time.Sleep(10 * time.Millisecond)
	th.stop()
	// After stop, channels should be closed
	select {
	case _, ok := <-th.taskQueue:
		if ok {
			t.Error("taskQueue should be closed after stop")
		}
	default:
	}
	select {
	case _, ok := <-th.actionQueue:
		if ok {
			t.Error("actionQueue should be closed after stop")
		}
	default:
	}
}

// TestThread_Run_And_Execute tests running the thread and executing a task.
func TestThread_Run_And_Execute(t *testing.T) {
	exec := &JsExecutor{
		engineFactory: mockEngineFactory(),
		options:       &JsExecutorOption{queueSize: 2},
	}
	th := newThread(exec, "t7", 7)
	go th.run()
	time.Sleep(10 * time.Millisecond)
	req := &JsRequest{Id: "run1"}
	task := newTask(req)
	th.taskQueue <- task
	result := <-task.resultChan
	if result.err != nil {
		t.Errorf("run/execute returned error: %v", result.err)
	}
	th.stop()
}

// TestThread_ExecuteAction_ReloadError tests error handling in reload action.
func TestThread_ExecuteAction_ReloadError(t *testing.T) {
	engine := &mockEngine{
		reloadFunc: func(scripts []*InitScript) error { return errors.New("reload error") },
	}
	exec := &JsExecutor{
		engineFactory: func() (JsEngine, error) { return engine, nil },
		options:       &JsExecutorOption{queueSize: 2},
	}
	th := newThread(exec, "t8", 8)
	_ = th.initEngine()
	req := &threadActionRequest{
		action: actionReload,
		done:   make(chan error, 1),
	}
	th.executeAction(req)
	err := <-req.done
	if err == nil || err.Error() != "reload error" {
		t.Errorf("Expected reload error, got: %v", err)
	}
}

// TestThread_GetTaskCountAndLastUsed tests task count and last used timestamp.
func TestThread_GetTaskCountAndLastUsed(t *testing.T) {
	exec := &JsExecutor{
		engineFactory: mockEngineFactory(),
		options:       &JsExecutorOption{queueSize: 2},
	}
	th := newThread(exec, "t9", 9)
	th.initEngine()
	before := th.getTaskCount()
	req := &JsRequest{Id: "count"}
	task := newTask(req)
	th.executeTask(task)
	<-task.resultChan
	after := th.getTaskCount()
	if after != before+1 {
		t.Errorf("getTaskCount did not increment, before=%d after=%d", before, after)
	}
	if th.getLastUsed().IsZero() {
		t.Error("getLastUsed should not be zero")
	}
}

// TestThread_Run_PanicRecovery tests panic recovery in the thread's run loop.
func TestThread_Run_PanicRecovery(t *testing.T) {
	engine := &mockEngine{
		executeResp: nil,
		executeErr:  errors.New("panic!"),
	}
	exec := &JsExecutor{
		engineFactory: func() (JsEngine, error) { return engine, nil },
		options:       &JsExecutorOption{queueSize: 2},
	}
	th := newThread(exec, "t10", 10)
	go th.run()
	time.Sleep(10 * time.Millisecond)
	req := &JsRequest{Id: "panic"}
	task := newTask(req)
	th.taskQueue <- task
	result := <-task.resultChan
	if result.err == nil {
		t.Error("Expected error from panic in executeTask")
	}
	th.stop()
}

// TestThread_Concurrent_ExecuteTask tests concurrent execution of tasks.
func TestThread_Concurrent_ExecuteTask(t *testing.T) {
	exec := &JsExecutor{
		engineFactory: mockEngineFactory(),
		options:       &JsExecutorOption{queueSize: 10},
	}
	th := newThread(exec, "t13", 13)
	_ = th.initEngine()
	var wg sync.WaitGroup
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			req := &JsRequest{Id: "c"}
			task := newTask(req)
			th.executeTask(task)
			<-task.resultChan
		}(i)
	}
	wg.Wait()
}

// TestThread_Run_DeferCloseError tests error handling in deferred engine close.
func TestThread_Run_DeferCloseError(t *testing.T) {
	done := make(chan struct{})
	engine := &mockEngine{
		closeFunc: func() error {
			close(done)
			return errors.New("close error")
		},
		executeFunc: func(req *JsRequest) (*JsResponse, error) {
			panic("trigger panic in execute")
		},
	}
	exec := &JsExecutor{
		engineFactory: func() (JsEngine, error) { return engine, nil },
		logger:        slog.Default(),
		options:       &JsExecutorOption{queueSize: 2},
	}
	th := newThread(exec, "t_defer", 1001)
	_ = th.initEngine()
	go th.run()
	time.Sleep(10 * time.Millisecond)
	// Trigger panic to enter defer
	task := newTask(&JsRequest{Id: "panic"})
	th.taskQueue <- task
	time.Sleep(20 * time.Millisecond)
	// Close taskQueue to exit run loop and trigger defer
	close(th.taskQueue)
	select {
	case <-done:
		// ok, Close was called
	case <-time.After(1 * time.Second):
		t.Fatal("timeout waiting for Close in defer")
	}
}

// TestThread_Run_PendingActionExecute tests that pending actions are executed after tasks.
func TestThread_Run_PendingActionExecute(t *testing.T) {
	exec := &JsExecutor{
		engineFactory: mockEngineFactory(),
		logger:        slog.Default(),
		options:       &JsExecutorOption{queueSize: 1},
	}
	th := newThread(exec, "t_pending", 1002)
	_ = th.initEngine()
	done := make(chan error, 1)
	go th.run()
	time.Sleep(10 * time.Millisecond)

	// 1. Enqueue a task to make the queue non-empty
	task := newTask(&JsRequest{Id: "a"})
	th.taskQueue <- task

	// 2. Send an action request, pendingAction will be set
	th.actionQueue <- &threadActionRequest{action: actionReload, done: done}
	time.Sleep(50 * time.Millisecond) // Give main loop a chance to schedule

	// 3. Consume the task, making the queue empty
	<-task.resultChan
	time.Sleep(50 * time.Millisecond)

	// 4. pendingAction will be executed in the next loop
	select {
	case <-done:
		// ok
	case <-time.After(2 * time.Second):
		t.Fatal("pendingAction was not executed")
	}
	th.stop()
}

// TestThread_Run_CheckAndRetireIfNeeded_Logger tests retire logic and logger coverage.
func TestThread_Run_CheckAndRetireIfNeeded_Logger(t *testing.T) {
	exec := &JsExecutor{
		engineFactory: mockEngineFactory(),
		logger:        slog.Default(),
		options: &JsExecutorOption{
			queueSize:     2,
			maxExecutions: 1, // Only allow 1 execution
		},
	}
	exec.pool = &pool{replenishChan: make(chan struct{}, 1)}

	th := newThread(exec, "t_retire", 1004)
	_ = th.initEngine()
	go th.run()
	time.Sleep(10 * time.Millisecond)

	// Enqueue two tasks to ensure getTaskCount() >= maxExecutions triggers retire branch
	task1 := newTask(&JsRequest{Id: "max1"})
	th.taskQueue <- task1
	<-task1.resultChan

	task2 := newTask(&JsRequest{Id: "max2"})
	th.taskQueue <- task2
	<-task2.resultChan

	time.Sleep(100 * time.Millisecond) // Wait for main loop to reach retire branch
}

// TestThread_ExecuteAction_Stop_CloseError tests error handling in stop action's engine close.
func TestThread_ExecuteAction_Stop_CloseError(t *testing.T) {
	exec := &JsExecutor{
		engineFactory: func() (JsEngine, error) {
			return &mockEngine{
				closeFunc: func() error { return errors.New("close error") },
			}, nil
		},
		logger:  slog.Default(),
		options: &JsExecutorOption{queueSize: 1},
	}
	th := newThread(exec, "t_stop_close_err", 100)
	_ = th.initEngine()
	req := &threadActionRequest{
		action: actionStop,
		done:   make(chan error, 1),
	}
	th.executeAction(req)
	<-req.done
}

// TestThread_ExecuteAction_Retire_CloseError tests error handling in retire action's engine close.
func TestThread_ExecuteAction_Retire_CloseError(t *testing.T) {
	exec := &JsExecutor{
		engineFactory: func() (JsEngine, error) {
			return &mockEngine{
				closeFunc: func() error { return errors.New("close error") },
			}, nil
		},
		logger:  slog.Default(),
		options: &JsExecutorOption{queueSize: 1},
	}
	th := newThread(exec, "t_retire_close_err", 101)
	_ = th.initEngine()
	req := &threadActionRequest{
		action: actionRetire,
		done:   make(chan error, 1),
	}
	th.executeAction(req)
	<-req.done
}

// TestThread_ExecuteAction_Reload_Error tests error handling in reload action.
func TestThread_ExecuteAction_Reload_Error(t *testing.T) {
	exec := &JsExecutor{
		engineFactory: func() (JsEngine, error) {
			return &mockEngine{
				reloadFunc: func(scripts []*InitScript) error { return errors.New("reload error") },
			}, nil
		},
		logger:  slog.Default(),
		options: &JsExecutorOption{queueSize: 1},
	}
	th := newThread(exec, "t_reload_err", 102)
	_ = th.initEngine()
	req := &threadActionRequest{
		action: actionReload,
		done:   make(chan error, 1),
	}
	th.executeAction(req)
	err := <-req.done
	if err == nil || err.Error() != "reload error" {
		t.Fatalf("expected reload error, got %v", err)
	}
}

// TestThread_ExecuteAction_Default tests the default branch of executeAction.
func TestThread_ExecuteAction_Default(t *testing.T) {
	exec := &JsExecutor{
		engineFactory: mockEngineFactory(),
		options:       &JsExecutorOption{queueSize: 1},
	}
	th := newThread(exec, "t_default", 999)
	_ = th.initEngine()
	req := &threadActionRequest{
		action: threadAction(999), // Undefined action
		done:   make(chan error, 1),
	}
	th.executeAction(req)
	err := <-req.done
	if err != nil {
		t.Errorf("expected nil error for default action, got %v", err)
	}
}
