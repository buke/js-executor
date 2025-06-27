// Copyright 2025 Brian Wang <wangbuke@gmail.com>
// SPDX-License-Identifier: Apache-2.0

package jsexecutor

import (
	"reflect"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestPool_NewPoolAndStartStop(t *testing.T) {
	exec := &JsExecutor{
		engineFactory: mockEngineFactory(),
		options: &JsExecutorOption{
			minPoolSize: 1,
			maxPoolSize: 2,
			queueSize:   2,
		},
	}
	p := newPool(exec)
	if p == nil {
		t.Fatal("newPool returned nil")
	}
	if err := p.start(); err != nil {
		t.Fatalf("pool start failed: %v", err)
	}
	if err := p.stop(); err != nil {
		t.Fatalf("pool stop failed: %v", err)
	}
}

func TestPool_CreateThreadAndSelect(t *testing.T) {
	exec := &JsExecutor{
		engineFactory: mockEngineFactory(),
		options: &JsExecutorOption{
			minPoolSize: 1,
			maxPoolSize: 2,
			queueSize:   2,
		},
	}
	p := newPool(exec)
	th, err := p.createThread()
	if err != nil {
		t.Fatalf("createThread failed: %v", err)
	}
	if th == nil {
		t.Fatal("createThread returned nil thread")
	}
	req := &JsRequest{}
	selected := p.selectThread(req)
	if selected == nil {
		t.Fatal("selectThread returned nil")
	}
}

func TestPool_GetOrCreateThread(t *testing.T) {
	exec := &JsExecutor{
		engineFactory: mockEngineFactory(),
		options: &JsExecutorOption{
			minPoolSize:     1,
			maxPoolSize:     2,
			queueSize:       2,
			createThreshold: 1.0, // Only create new thread if queue is full
		},
	}
	p := newPool(exec)
	req := &JsRequest{}
	th1, err := p.getOrCreateThread(req)
	if err != nil {
		t.Fatalf("getOrCreateThread failed: %v", err)
	}
	th2, err := p.getOrCreateThread(req)
	if err != nil {
		t.Fatalf("getOrCreateThread failed: %v", err)
	}
	if th1 != th2 {
		t.Error("getOrCreateThread should return the same thread if pool is not full")
	}
}

func TestPool_Execute(t *testing.T) {
	exec := &JsExecutor{
		engineFactory: mockEngineFactory(),
		options: &JsExecutorOption{
			minPoolSize:    1,
			maxPoolSize:    2,
			queueSize:      2,
			executeTimeout: 1 * time.Second,
		},
	}
	p := newPool(exec)
	if err := p.start(); err != nil {
		t.Fatalf("pool start failed: %v", err)
	}
	defer p.stop()
	req := &JsRequest{Id: "1"}
	task := newTask(req)
	resp, err := p.execute(task)
	if err != nil {
		t.Fatalf("pool execute failed: %v", err)
	}
	if resp == nil || resp.Id != "1" {
		t.Errorf("unexpected execute result: %+v", resp)
	}
}

func TestPool_Reload(t *testing.T) {
	exec := &JsExecutor{
		engineFactory: mockEngineFactory(),
		options: &JsExecutorOption{
			minPoolSize: 1,
			maxPoolSize: 2,
			queueSize:   2,
		},
	}
	p := newPool(exec)
	if err := p.start(); err != nil {
		t.Fatalf("pool start failed: %v", err)
	}
	defer p.stop()
	if err := p.reload(); err != nil {
		t.Errorf("pool reload failed: %v", err)
	}
}

func TestPool_Stop(t *testing.T) {
	exec := &JsExecutor{
		engineFactory: mockEngineFactory(),
		options: &JsExecutorOption{
			minPoolSize: 1,
			maxPoolSize: 2,
			queueSize:   2,
		},
	}
	p := newPool(exec)
	if err := p.start(); err != nil {
		t.Fatalf("pool start failed: %v", err)
	}
	if err := p.stop(); err != nil {
		t.Errorf("pool stop failed: %v", err)
	}
}

func TestPool_RemoveThreadFromRoundRobin(t *testing.T) {
	p := &pool{
		roundRobinList: []uint32{1, 2, 3, 4},
	}
	p.removeThreadFromRoundRobin(3)
	expected := []uint32{1, 2, 4}
	if !reflect.DeepEqual(p.roundRobinList, expected) {
		t.Errorf("removeThreadFromRoundRobin failed, got %v, want %v", p.roundRobinList, expected)
	}
}

func TestPool_ShouldRemoveThread(t *testing.T) {
	exec := &JsExecutor{
		options: &JsExecutorOption{
			threadTTL:     1 * time.Millisecond,
			maxExecutions: 1,
		},
	}
	p := newPool(exec)
	th := &thread{}
	// Should remove if idle for longer than TTL
	th.lastUsedNano = time.Now().Add(-2 * time.Millisecond).UnixNano()
	if !p.shouldRemoveThread(th, time.Now()) {
		t.Error("shouldRemoveThread should return true for idle thread")
	}
	// Should remove if exceeded maxExecutions
	th.lastUsedNano = time.Now().UnixNano()
	th.taskID = 2
	if !p.shouldRemoveThread(th, time.Now()) {
		t.Error("shouldRemoveThread should return true for overused thread")
	}
	// Should not remove if active and not overused
	th.taskID = 0
	th.lastUsedNano = time.Now().UnixNano()
	if p.shouldRemoveThread(th, time.Now()) {
		t.Error("shouldRemoveThread should return false for active thread")
	}
}

func TestPool_PerformCleanup(t *testing.T) {
	exec := &JsExecutor{
		options: &JsExecutorOption{
			minPoolSize:   0,
			threadTTL:     1 * time.Millisecond,
			maxExecutions: 1,
			queueSize:     1,
		},
	}
	p := newPool(exec)
	th := newThread(exec, "t1", 1)
	p.threads = map[uint32]*thread{1: th}
	p.roundRobinList = []uint32{1}
	atomic.StoreUint32(&p.threadCount, 1)
	th.lastUsedNano = time.Now().Add(-2 * time.Millisecond).UnixNano()
	th.taskID = 2
	p.performCleanup()
	if _, ok := p.threads[1]; ok {
		t.Error("performCleanup should remove thread")
	}
	if len(p.roundRobinList) != 0 {
		t.Error("performCleanup should update roundRobinList")
	}
}

func TestPool_Concurrency(t *testing.T) {
	exec := &JsExecutor{
		engineFactory: mockEngineFactory(),
		options: &JsExecutorOption{
			minPoolSize:    2,
			maxPoolSize:    4,
			queueSize:      2,
			executeTimeout: 1 * time.Second,
		},
	}
	p := newPool(exec)
	if err := p.start(); err != nil {
		t.Fatalf("pool start failed: %v", err)
	}
	defer p.stop()
	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			req := &JsRequest{Id: "c"}
			task := newTask(req)
			_, err := p.execute(task)
			if err != nil {
				t.Errorf("concurrent execute failed: %v", err)
			}
		}(i)
	}
	wg.Wait()
}
