// Copyright 2025 Brian Wang <wangbuke@gmail.com>
// SPDX-License-Identifier: Apache-2.0

package jsexecutor

import (
	"errors"
	"sync"
	"testing"
	"time"
)

// mockEngine is reused from executor_test.go

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

func TestThread_Reload_Timeout(t *testing.T) {
	exec := &JsExecutor{
		engineFactory: mockEngineFactory(),
		options:       &JsExecutorOption{queueSize: 2},
	}
	th := newThread(exec, "t11", 11)
	_ = th.initEngine()

	// Fill actionQueue to block reload
	th.actionQueue <- &threadActionRequest{action: actionReload, done: make(chan error, 1)}
	err := th.reload()
	if err == nil || err.Error() == "" {
		t.Error("Expected timeout error on reload")
	}
}

func TestThread_Stop_Timeout(t *testing.T) {
	exec := &JsExecutor{
		engineFactory: mockEngineFactory(),
		options:       &JsExecutorOption{queueSize: 2},
	}
	th := newThread(exec, "t12", 12)
	_ = th.initEngine()
	// Fill actionQueue to block stop
	th.actionQueue <- &threadActionRequest{action: actionReload, done: make(chan error, 1)}
	th.stop() // Should not panic, just log timeout
}

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
