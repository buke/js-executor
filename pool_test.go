// Copyright 2025 Brian Wang <wangbuke@gmail.com>
// SPDX-License-Identifier: Apache-2.0

package jsexecutor

import (
	"errors"
	"log/slog"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// mockEngineFactoryWithError returns a factory that fails on reload or execute if specified.
func mockEngineFactoryWithError(reloadErr, execErr error) JsEngineFactory {
	return func() (JsEngine, error) {
		return &mockEngine{
			reloadFunc: func(scripts []*JsScript) error {
				return reloadErr
			},
			executeFunc: func(req *JsRequest) (*JsResponse, error) {
				if execErr != nil {
					return nil, execErr
				}
				return &JsResponse{Id: req.Id, Result: "ok"}, nil
			},
		}, nil
	}
}

// TestPool_NewPoolAndStartStop tests pool creation, start, and stop.
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

// TestPool_CreateThreadAndSelect tests thread creation and selection.
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

// TestPool_CreateThread_MaxPoolSize tests createThread when exceeding maxPoolSize.
func TestPool_CreateThread_MaxPoolSize(t *testing.T) {
	exec := &JsExecutor{
		engineFactory: mockEngineFactory(),
		options: &JsExecutorOption{
			minPoolSize: 1,
			maxPoolSize: 1,
			queueSize:   2,
		},
	}
	p := newPool(exec)
	_, err := p.createThread()
	if err != nil {
		t.Fatalf("first createThread failed: %v", err)
	}
	_, err = p.createThread()
	if err == nil {
		t.Error("expected error when exceeding maxPoolSize")
	}
}

// TestPool_GetOrCreateThread tests getOrCreateThread normal and fallback logic.
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

// TestPool_GetOrCreateThread_CreateThreadError tests error path when createThread fails.
func TestPool_GetOrCreateThread_CreateThreadError(t *testing.T) {
	exec := &JsExecutor{
		engineFactory: mockEngineFactory(),
		options: &JsExecutorOption{
			minPoolSize:     1,
			maxPoolSize:     1,
			queueSize:       2,
			createThreshold: 0.0,
		},
	}
	p := newPool(exec)
	// Fill up the pool
	_, _ = p.createThread()
	// Now force createThread to fail
	req := &JsRequest{}
	_, err := p.getOrCreateThread(req)
	if err != nil {
		t.Logf("getOrCreateThread error as expected: %v", err)
	}
}

// TestPool_Execute tests normal execution path.
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

// TestPool_Execute_ThreadNil tests execute when getOrCreateThread returns nil.
func TestPool_Execute_ThreadNil(t *testing.T) {
	exec := &JsExecutor{
		engineFactory: mockEngineFactory(),
		options: &JsExecutorOption{
			minPoolSize:    0,
			maxPoolSize:    0, // No threads available
			queueSize:      2,
			executeTimeout: 1 * time.Second,
		},
	}
	p := newPool(exec)
	task := newTask(&JsRequest{Id: "fail"})
	// Simulate no threads available by setting empty pool
	emptyIds := []uint32{}
	p.threadIds.Store(&emptyIds) // Use pointer instead of direct slice
	_, err := p.execute(task)
	if err == nil {
		t.Error("expected error when no available thread")
	}
}

// TestPool_Execute_Timeout tests execute timeout.
func TestPool_Execute_Timeout(t *testing.T) {
	exec := &JsExecutor{
		engineFactory: func() (JsEngine, error) {
			return &mockEngine{
				executeFunc: func(req *JsRequest) (*JsResponse, error) {
					time.Sleep(100 * time.Millisecond) // longer than executeTimeout
					return &JsResponse{Id: req.Id}, nil
				},
			}, nil
		},
		options: &JsExecutorOption{
			minPoolSize:    1,
			maxPoolSize:    1,
			queueSize:      1,
			executeTimeout: 10 * time.Millisecond,
		},
	}
	p := newPool(exec)
	_ = p.start()
	defer p.stop()
	req := &JsRequest{Id: "timeout"}
	task := newTask(req)
	_, err := p.execute(task)
	if err == nil || err.Error() != "timeout waiting for task result" {
		t.Errorf("expected timeout error, got: %v", err)
	}
}

// TestPool_Execute_ResultError tests execute when result.err is not nil.
func TestPool_Execute_ResultError(t *testing.T) {
	exec := &JsExecutor{
		engineFactory: mockEngineFactoryWithError(nil, errors.New("exec error")),
		options: &JsExecutorOption{
			minPoolSize:    1,
			maxPoolSize:    1,
			queueSize:      1,
			executeTimeout: 1 * time.Second,
		},
	}
	p := newPool(exec)
	_ = p.start()
	defer p.stop()
	req := &JsRequest{Id: "err"}
	task := newTask(req)
	resp, err := p.execute(task)
	if err == nil || err.Error() != "exec error" {
		t.Errorf("expected exec error, got: %v, resp: %+v", err, resp)
	}
}

// TestPool_Reload tests normal reload and error path.
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

	// Test reload error
	exec2 := &JsExecutor{
		engineFactory: mockEngineFactoryWithError(errors.New("reload fail"), nil),
		options: &JsExecutorOption{
			minPoolSize: 1,
			maxPoolSize: 1,
			queueSize:   1,
		},
	}
	p2 := newPool(exec2)
	_ = p2.start()
	defer p2.stop()
	err := p2.reload()
	if err == nil || err.Error() == "" {
		t.Error("expected reload error")
	}
}

// TestPool_Stop tests pool stop logic.
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

// TestPool_AddThreadToList tests adding a thread to the round-robin list.
func TestPool_AddThreadToList(t *testing.T) {
	exec := &JsExecutor{
		options: &JsExecutorOption{},
	}
	p := newPool(exec)

	// Test adding to empty list
	p.addThreadToList(1)
	threadIds := *p.threadIds.Load().(*[]uint32) // Dereference pointer to get slice
	if len(threadIds) != 1 || threadIds[0] != 1 {
		t.Errorf("addThreadToList failed, got %v, want [1]", threadIds)
	}

	// Test adding to existing list
	p.addThreadToList(2)
	threadIds = *p.threadIds.Load().(*[]uint32) // Dereference pointer to get slice
	if len(threadIds) != 2 || threadIds[1] != 2 {
		t.Errorf("addThreadToList failed, got %v, want [1, 2]", threadIds)
	}
}

// TestPool_RemoveThreadFromList tests removing a thread from the round-robin list.
func TestPool_RemoveThreadFromList(t *testing.T) {
	exec := &JsExecutor{
		options: &JsExecutorOption{},
	}
	p := newPool(exec)
	ids := []uint32{1, 2, 3, 4}
	p.threadIds.Store(&ids) // Store pointer to slice

	p.removeThreadFromList(3)
	threadIds := *p.threadIds.Load().(*[]uint32) // Dereference pointer to get slice
	expected := []uint32{1, 2, 4}
	if len(threadIds) != len(expected) {
		t.Errorf("removeThreadFromList failed, got length %d, want %d", len(threadIds), len(expected))
		return
	}
	for i, v := range expected {
		if threadIds[i] != v {
			t.Errorf("removeThreadFromList failed, got %v, want %v", threadIds, expected)
			break
		}
	}
}

// TestPool_ShouldRemoveThread tests thread removal logic based on TTL and maxExecutions.
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
	atomic.StoreUint32(&th.taskID, 2)
	if !p.shouldRemoveThread(th, time.Now()) {
		t.Error("shouldRemoveThread should return true for overused thread")
	}
	// Should not remove if active and not overused
	atomic.StoreUint32(&th.taskID, 0)
	th.lastUsedNano = time.Now().UnixNano()
	if p.shouldRemoveThread(th, time.Now()) {
		t.Error("shouldRemoveThread should return false for active thread")
	}
}

// TestPool_PerformCleanup_MinPoolSize tests performCleanup when threadCount <= minPoolSize.
func TestPool_PerformCleanup_MinPoolSize(t *testing.T) {
	exec := &JsExecutor{
		options: &JsExecutorOption{
			minPoolSize:   2,
			threadTTL:     1 * time.Millisecond,
			maxExecutions: 1,
			queueSize:     1,
		},
	}
	p := newPool(exec)
	th := newThread(exec, "t1", 1)
	p.threads.Store(uint32(1), th)
	ids := []uint32{1}
	p.threadIds.Store(&ids) // Store pointer to slice
	atomic.StoreUint32(&p.threadCount, 1)
	th.lastUsedNano = time.Now().Add(-2 * time.Millisecond).UnixNano()
	atomic.StoreUint32(&th.taskID, 2)
	p.performCleanup()
	// Should not remove thread since threadCount <= minPoolSize
	if _, ok := p.threads.Load(uint32(1)); !ok {
		t.Error("performCleanup should not remove thread when at minPoolSize")
	}
}

// TestPool_PerformCleanup_NoThreadsToRemove tests performCleanup when no threads to remove.
func TestPool_PerformCleanup_NoThreadsToRemove(t *testing.T) {
	exec := &JsExecutor{
		options: &JsExecutorOption{
			minPoolSize:   0,
			threadTTL:     1 * time.Hour,
			maxExecutions: 100,
			queueSize:     1,
		},
	}
	p := newPool(exec)
	th := newThread(exec, "t1", 1)
	p.threads.Store(uint32(1), th)
	ids := []uint32{1}
	p.threadIds.Store(&ids) // Store pointer to slice
	atomic.StoreUint32(&p.threadCount, 1)
	th.lastUsedNano = time.Now().UnixNano()
	atomic.StoreUint32(&th.taskID, 0)
	p.performCleanup()
	// Should not remove thread since not idle or overused
	if _, ok := p.threads.Load(uint32(1)); !ok {
		t.Error("performCleanup should not remove thread if not idle/overused")
	}
}

// TestPool_PerformCleanup tests performCleanup normal removal.
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
	p.threads.Store(uint32(1), th)
	ids := []uint32{1}
	p.threadIds.Store(&ids) // Store pointer to slice
	atomic.StoreUint32(&p.threadCount, 1)
	th.lastUsedNano = time.Now().Add(-2 * time.Millisecond).UnixNano()
	atomic.StoreUint32(&th.taskID, 2)
	p.performCleanup()
	if _, ok := p.threads.Load(uint32(1)); ok {
		t.Error("performCleanup should remove thread")
	}
	threadIds := *p.threadIds.Load().(*[]uint32) // Dereference pointer to get slice
	if len(threadIds) != 0 {
		t.Error("performCleanup should update threadIds list")
	}
}

// TestPool_Replenish tests replenish logic including createThread error.
func TestPool_Replenish(t *testing.T) {
	exec := &JsExecutor{
		engineFactory: func() (JsEngine, error) { return nil, errors.New("fail") },
		options: &JsExecutorOption{
			minPoolSize: 2,
			maxPoolSize: 2,
			queueSize:   1,
		},
	}
	p := newPool(exec)
	// Should break on createThread error
	p.replenish()
}

// TestPool_Concurrency tests concurrent execution in the pool.
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

func TestPool_RetireThreads_Coverage(t *testing.T) {
	exec := &JsExecutor{
		engineFactory: mockEngineFactory(),
		options: &JsExecutorOption{
			minPoolSize:   1,
			maxPoolSize:   2,
			queueSize:     1,
			threadTTL:     10 * time.Millisecond, // very short TTL
			maxExecutions: 0,
		},
	}
	p := newPool(exec)
	if err := p.start(); err != nil {
		t.Fatalf("pool start failed: %v", err)
	}
	defer p.stop()

	// 1. Test ticker.C branch (performCleanup)
	time.Sleep(30 * time.Millisecond) // Wait for at least one cleanup tick

	// 2. Test replenishChan branch
	select {
	case p.replenishChan <- struct{}{}:
		// Give some time for replenish to run
		time.Sleep(10 * time.Millisecond)
	default:
		t.Log("replenishChan already full")
	}
}

func TestPool_Start_CreateThreadError(t *testing.T) {
	exec := &JsExecutor{
		engineFactory: mockEngineFactory(),
		options: &JsExecutorOption{
			minPoolSize: 2, // minPoolSize > maxPoolSize triggers error
			maxPoolSize: 1,
			queueSize:   1,
		},
	}
	p := newPool(exec)
	err := p.start()
	if err == nil || err.Error() == "" {
		t.Error("expected error when createThread fails in start")
	}
}

func TestPool_SelectThread_ByContext(t *testing.T) {
	exec := &JsExecutor{
		engineFactory: mockEngineFactory(),
		options: &JsExecutorOption{
			minPoolSize: 1,
			maxPoolSize: 1,
			queueSize:   1,
		},
	}
	p := newPool(exec)
	th, err := p.createThread()
	if err != nil {
		t.Fatalf("createThread failed: %v", err)
	}
	// Set thread ID in context
	ctx := map[string]interface{}{
		ThreadIdKey: th.threadId,
	}
	req := &JsRequest{Context: ctx}
	selected := p.selectThread(req)
	if selected != th {
		t.Errorf("selectThread by context failed, got %v, want %v", selected, th)
	}
}

func TestPool_Execute_GetOrCreateThreadError(t *testing.T) {
	exec := &JsExecutor{
		engineFactory: func() (JsEngine, error) { return nil, errors.New("engine error") },
		options: &JsExecutorOption{
			minPoolSize:    0,
			maxPoolSize:    1,
			queueSize:      1,
			executeTimeout: 1 * time.Second,
		},
	}
	p := newPool(exec)
	atomic.StoreUint32(&p.threadCount, 1)
	task := newTask(&JsRequest{Id: "fail"})
	_, err := p.execute(task)
	if err == nil || err.Error() == "" || err.Error() != "failed to get thread: no available thread in pool" {
		t.Errorf("expected error from getOrCreateThread, got: %v", err)
	}
}

func TestPool_RetireThreads_NoTTL(t *testing.T) {
	exec := &JsExecutor{
		engineFactory: mockEngineFactory(),
		options: &JsExecutorOption{
			minPoolSize:   1,
			maxPoolSize:   1,
			queueSize:     1,
			threadTTL:     0, // threadTTL=0 triggers else branch
			maxExecutions: 1, // maxExecutions triggers retireThreads
		},
	}
	p := newPool(exec)
	if err := p.start(); err != nil {
		t.Fatalf("pool start failed: %v", err)
	}
	// Just ensure goroutine starts and enters else branch, no need to wait 1 minute
	// Stop pool to ensure goroutine exits
	if err := p.stop(); err != nil {
		t.Fatalf("pool stop failed: %v", err)
	}
}

func TestPool_PerformCleanup_MaxExecutionsReasonAndLogger(t *testing.T) {
	done := make(chan struct{})
	exec := &JsExecutor{
		engineFactory: func() (JsEngine, error) {
			return &mockEngine{
				closeFunc: func() error {
					close(done)
					return nil
				},
			}, nil
		},
		logger: slog.Default(),
		options: &JsExecutorOption{
			minPoolSize:   0,
			maxPoolSize:   1,
			queueSize:     1,
			threadTTL:     1 * time.Hour,
			maxExecutions: 1,
		},
	}
	p := newPool(exec)
	th, _ := p.createThread()
	atomic.StoreUint32(&th.taskID, 1)
	th.lastUsedNano = time.Now().UnixNano()
	p.threads.Store(th.threadId, th)
	ids := []uint32{th.threadId}
	p.threadIds.Store(&ids) // Store pointer to slice
	atomic.StoreUint32(&p.threadCount, 1)

	p.performCleanup()
	select {
	case <-done:
		// ok
	case <-time.After(1 * time.Second):
		t.Fatal("timeout waiting for cleanup goroutine")
	}
}

func TestPool_Replenish_LoggerError(t *testing.T) {
	exec := &JsExecutor{
		engineFactory: func() (JsEngine, error) { return nil, errors.New("fail") },
		logger:        slog.Default(),
		options: &JsExecutorOption{
			minPoolSize: 2,
			maxPoolSize: 1, // createThread will fail
			queueSize:   1,
		},
	}
	p := newPool(exec)
	// replenish will try createThread, which fails and triggers logger.Error
	p.replenish()
}

// TestPool_Execute_NoTimeout tests execute when executeTimeout is 0.
func TestPool_Execute_NoTimeout(t *testing.T) {
	exec := &JsExecutor{
		engineFactory: mockEngineFactory(),
		options: &JsExecutorOption{
			minPoolSize:    1,
			maxPoolSize:    2,
			queueSize:      2,
			executeTimeout: 0, // No timeout
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

// TestPool_SelectThread_EmptyThreadIds tests selectThread when threadIds is empty.
func TestPool_SelectThread_EmptyThreadIds(t *testing.T) {
	exec := &JsExecutor{
		engineFactory: mockEngineFactory(),
		options: &JsExecutorOption{
			minPoolSize: 1,
			maxPoolSize: 1,
			queueSize:   1,
		},
	}
	p := newPool(exec)
	emptyIds := []uint32{}
	p.threadIds.Store(&emptyIds) // Store pointer to slice
	req := &JsRequest{}
	selected := p.selectThread(req)
	if selected != nil {
		t.Error("selectThread should return nil when threadIds is empty")
	}
}

// TestPool_SelectThread_ThreadNotFound tests selectThread when thread is not found in sync.Map.
func TestPool_SelectThread_ThreadNotFound(t *testing.T) {
	exec := &JsExecutor{
		engineFactory: mockEngineFactory(),
		options: &JsExecutorOption{
			minPoolSize:     1,
			maxPoolSize:     1,
			queueSize:       1,
			selectThreshold: 0.5,
		},
	}
	p := newPool(exec)
	ids := []uint32{999}    // Non-existent thread ID
	p.threadIds.Store(&ids) // Store pointer to slice
	req := &JsRequest{}
	selected := p.selectThread(req)
	if selected != nil {
		t.Error("selectThread should return nil when thread is not found")
	}
}

// TestPool_AddRemoveThreadToList_Concurrent tests concurrent add/remove operations.
func TestPool_AddRemoveThreadToList_Concurrent(t *testing.T) {
	exec := &JsExecutor{
		options: &JsExecutorOption{},
	}
	p := newPool(exec)

	var wg sync.WaitGroup

	// Concurrent adds
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(id uint32) {
			defer wg.Done()
			p.addThreadToList(id)
		}(uint32(i))
	}

	wg.Wait()

	threadIds := *p.threadIds.Load().(*[]uint32) // Dereference pointer to get slice
	if len(threadIds) != 10 {
		t.Errorf("expected 10 thread IDs, got %d", len(threadIds))
	}

	// Concurrent removes
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func(id uint32) {
			defer wg.Done()
			p.removeThreadFromList(id)
		}(uint32(i))
	}

	wg.Wait()

	threadIds = *p.threadIds.Load().(*[]uint32) // Dereference pointer to get slice
	if len(threadIds) != 5 {
		t.Errorf("expected 5 thread IDs after removal, got %d", len(threadIds))
	}
}
