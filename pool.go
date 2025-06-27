// Copyright 2025 Brian Wang <wangbuke@gmail.com>
// SPDX-License-Identifier: Apache-2.0

package jsexecutor

import (
	"fmt"
	"strconv"
	"sync"
	"sync/atomic"
	"time"
)

// ThreadIdKey is the context key for specifying thread ID in requests
const ThreadIdKey = "__threadId"

// pool manages a collection of JavaScript execution threads
type pool struct {
	sync.RWMutex                       // Read-write mutex for thread map access
	executor        *JsExecutor        // Reference to the parent executor
	threads         map[uint32]*thread // Map of thread ID to thread instance
	threadCount     uint32             // Atomic: current number of threads in the pool
	roundRobinList  []uint32           // List of thread IDs for round-robin selection
	roundRobinIndex uint32             // Current index for round-robin selection (atomic)
	threadIdCounter uint32             // Counter for generating unique thread IDs (atomic)
	stopCleanup     chan struct{}      // Chanoel to signal cleanup goroutine to stop
	replenishChan   chan struct{}      // Channel to signal thread replenishment
}

// threadCleanupInfo holds information about a thread to be cleaned up
type threadCleanupInfo struct {
	id     uint32  // Thread ID
	thread *thread // Thread instance
}

// newPool creates a new thread pool
func newPool(e *JsExecutor) *pool {
	return &pool{
		executor:        e,
		threads:         make(map[uint32]*thread),
		roundRobinList:  make([]uint32, 0),
		roundRobinIndex: 0,
		threadIdCounter: 0,
		stopCleanup:     make(chan struct{}),
		replenishChan:   make(chan struct{}, 1),
		threadCount:     0,
	}
}

// start initializes the thread pool with minimum number of threads
func (p *pool) start() error {
	// Create minimum number of threads
	for i := uint32(0); i < p.executor.options.minPoolSize; i++ {
		if _, err := p.createThread(); err != nil {
			return fmt.Errorf("failed to create thread %d: %w", i, err)
		}
	}

	// Start cleanup goroutine if TTL or max executions are configured
	if p.executor.options.threadTTL > 0 || p.executor.options.maxExecutions > 0 {
		go p.retireThreads()
	}

	return nil
}

// stop shuts down the thread pool and all threads
func (p *pool) stop() error {
	close(p.stopCleanup)
	close(p.replenishChan)

	p.Lock()
	defer p.Unlock()

	// Stop all threads
	for _, t := range p.threads {
		t.stop()
	}

	// Clear the thread collections
	p.threads = make(map[uint32]*thread)
	p.roundRobinList = make([]uint32, 0)
	atomic.StoreUint32(&p.threadCount, 0)
	return nil
}

// createThread creates and starts a new thread with atomic thread count control
func (p *pool) createThread() (*thread, error) {
	newCount := atomic.AddUint32(&p.threadCount, 1)
	if newCount > p.executor.options.maxPoolSize {
		atomic.AddUint32(&p.threadCount, ^uint32(0)) // -1
		return nil, fmt.Errorf("max pool size reached")
	}

	threadId := atomic.AddUint32(&p.threadIdCounter, 1)

	t := newThread(p.executor, "thread-"+strconv.FormatUint(uint64(threadId), 10), threadId)

	// Start the thread goroutine
	go t.run()

	// Add thread to the pool (requires lock for structural changes)
	p.Lock()
	p.threads[threadId] = t
	p.roundRobinList = append(p.roundRobinList, threadId)
	p.Unlock()

	if p.executor.logger != nil {
		p.executor.logger.Info("Thread created",
			"thread", t.name,
			"totalThreads", atomic.LoadUint32(&p.threadCount))
	}

	return t, nil
}

// selectThread selects an appropriate thread for executing a request
func (p *pool) selectThread(req *JsRequest) *thread {
	p.RLock()
	defer p.RUnlock()

	// 1. Check if a specific thread is requested via context
	if req.Context != nil {
		if threadIdVal, exists := req.Context[ThreadIdKey]; exists {
			if threadId, ok := threadIdVal.(uint32); ok {
				if t, found := p.threads[threadId]; found {
					return t
				}
			}
		}
	}

	// 2. Use round-robin selection with load balancing
	listLen := len(p.roundRobinList)
	if listLen == 0 {
		return nil
	}

	// Try to find a thread that's not too busy
	for i := 0; i < listLen; i++ {
		index := atomic.AddUint32(&p.roundRobinIndex, 1) % uint32(listLen)
		threadId := p.roundRobinList[index]

		if t, exists := p.threads[threadId]; exists {
			queueThreshold := int(float64(p.executor.options.queueSize) * p.executor.options.selectThreshold)
			if len(t.taskQueue) < queueThreshold {
				return t
			}
		}
	}

	// 3. If all threads are busy, return the next thread in round-robin order
	index := atomic.LoadUint32(&p.roundRobinIndex) % uint32(len(p.roundRobinList))
	threadId := p.roundRobinList[index]
	return p.threads[threadId]
}

// getOrCreateThread gets an existing thread or creates a new one if needed
func (p *pool) getOrCreateThread(req *JsRequest) (*thread, error) {

	// Try to get an existing thread first
	if t := p.selectThread(req); t != nil {
		queueThreshold := int(float64(p.executor.options.queueSize) * p.executor.options.createThreshold)
		if len(t.taskQueue) < queueThreshold {
			return t, nil
		}
	}

	// Use atomic threadCount for concurrency safety
	currentThreadCount := atomic.LoadUint32(&p.threadCount)
	if currentThreadCount < p.executor.options.maxPoolSize {
		if p.executor.logger != nil {
			p.executor.logger.Info("Creating new thread due to high load",
				"currentThreads", currentThreadCount,
				"maxPoolSize", p.executor.options.maxPoolSize)
		}
		if t, err := p.createThread(); err == nil {
			return t, nil
		}
	}

	// At max capacity, return an existing thread
	return p.selectThread(req), nil
}

// execute executes a task using an appropriate thread
func (p *pool) execute(task *task) (*JsResponse, error) {
	// Get a thread for execution
	t, err := p.getOrCreateThread(task.request)
	if err != nil {
		return nil, fmt.Errorf("failed to get thread: %w", err)
	}

	if t == nil {
		return nil, fmt.Errorf("no available thread to execute task")
	}

	// Enqueue the task
	t.taskQueue <- task

	// Wait for the result with timeout
	select {
	case result := <-task.resultChan:
		if result.err != nil {
			return nil, result.err
		}
		return result.response, nil
	case <-time.After(p.executor.options.executeTimeout):
		return nil, fmt.Errorf("timeout waiting for task result")
	}
}

// reload reloads all threads with new scripts
func (p *pool) reload() error {
	p.Lock()
	defer p.Unlock()

	for _, t := range p.threads {
		if err := t.reload(); err != nil {
			return fmt.Errorf("failed to reload thread %s: %w", t.name, err)
		}
	}

	return nil
}

// retireThreads runs the background cleanup process
func (p *pool) retireThreads() {
	var ticker *time.Ticker
	if p.executor.options.threadTTL > 0 {
		ticker = time.NewTicker(p.executor.options.threadTTL / 2)
	} else {
		ticker = time.NewTicker(time.Minute)
	}
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			p.performCleanup()
		case <-p.replenishChan:
			p.replenish()
		case <-p.stopCleanup:
			return
		}
	}
}

// removeThreadFromRoundRobin removes a thread ID from the round-robin list
func (p *pool) removeThreadFromRoundRobin(threadId uint32) {
	for i, id := range p.roundRobinList {
		if id == threadId {
			p.roundRobinList = append(p.roundRobinList[:i], p.roundRobinList[i+1:]...)
			break
		}
	}
}

// shouldRemoveThread determines if a thread should be removed based on TTL and execution count
func (p *pool) shouldRemoveThread(t *thread, now time.Time) bool {
	// Check TTL if configured
	if p.executor.options.threadTTL > 0 {
		if now.Sub(t.getLastUsed()) > p.executor.options.threadTTL {
			return true
		}
	}

	// Check max executions if configured
	if p.executor.options.maxExecutions > 0 {
		if t.getTaskCount() >= p.executor.options.maxExecutions {
			return true
		}
	}

	return false
}

// performCleanup performs the actual cleanup of idle or overused threads
func (p *pool) performCleanup() {
	now := time.Now()

	// Phase 1: Collect information about threads to remove (read lock)
	p.RLock()
	currentThreadCount := atomic.LoadUint32(&p.threadCount)
	if currentThreadCount <= p.executor.options.minPoolSize {
		p.RUnlock()
		return
	}

	var threadsToRemove []threadCleanupInfo
	for threadId, t := range p.threads {
		if p.shouldRemoveThread(t, now) {
			// Ensure we don't remove too many threads
			if currentThreadCount-uint32(len(threadsToRemove)) > p.executor.options.minPoolSize {
				threadsToRemove = append(threadsToRemove, threadCleanupInfo{
					id:     threadId,
					thread: t,
				})
			}
		}
	}
	p.RUnlock()

	if len(threadsToRemove) == 0 {
		return
	}

	// Phase 2: Remove threads from pool (write lock, minimal time)
	p.Lock()
	actualRemovedCount := 0
	for _, info := range threadsToRemove {
		// Double-check thread still exists (avoid concurrent removal)
		if _, exists := p.threads[info.id]; exists {
			delete(p.threads, info.id)
			p.removeThreadFromRoundRobin(info.id)
			actualRemovedCount++
			atomic.AddUint32(&p.threadCount, ^uint32(0)) // -1 for each removed thread
		}
	}
	newThreadCount := atomic.LoadUint32(&p.threadCount)
	p.Unlock()

	// Phase 3: Stop threads asynchronously (no locks)
	for _, info := range threadsToRemove {
		taskCount := info.thread.getTaskCount()
		lastUsed := info.thread.getLastUsed()

		go func(th *thread, tc uint32, lu time.Time) {
			th.retire()
			reason := "idle timeout"
			if p.executor.options.maxExecutions > 0 && tc >= p.executor.options.maxExecutions {
				reason = "max executions reached"
			}
			if p.executor.logger != nil {
				p.executor.logger.Info("Thread removed",
					"thread", th.name,
					"reason", reason,
					"executions", tc,
					"idleTime", now.Sub(lu),
					"remainingThreads", newThreadCount)
			}
		}(info.thread, taskCount, lastUsed)
	}
}

// replenish checks if the pool needs more threads and creates them if necessary
func (p *pool) replenish() {
	for {
		current := atomic.LoadUint32(&p.threadCount)
		if current >= p.executor.options.minPoolSize {
			break
		}
		if _, err := p.createThread(); err != nil {
			if p.executor.logger != nil {
				p.executor.logger.Error("Failed to create replenishment thread", "error", err)
			}
			break
		}
	}
}
