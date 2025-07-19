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

// ThreadIdKey is the context key for specifying thread ID in requests.
const ThreadIdKey = "__threadId"

// pool manages a collection of JavaScript execution threads using lock-free algorithms.
type pool struct {
	executor        *JsExecutor   // Reference to the parent executor
	threads         sync.Map      // Lock-free map of thread ID to thread instance
	threadIds       atomic.Value  // Stores *[]uint32 for round-robin selection (copy-on-write)
	threadCount     uint32        // Atomic: current number of threads in the pool
	roundRobinIndex uint32        // Current index for round-robin selection (atomic)
	threadIdCounter uint32        // Counter for generating unique thread IDs (atomic)
	stopCleanup     chan struct{} // Channel to signal cleanup goroutine to stop
	replenishChan   chan struct{} // Channel to signal thread replenishment
}

// threadCleanupInfo holds information about a thread to be cleaned up.
type threadCleanupInfo struct {
	id     uint32  // Thread ID
	thread *thread // Thread instance
}

// newPool creates a new thread pool with lock-free data structures.
func newPool(e *JsExecutor) *pool {
	p := &pool{
		executor:        e,
		threadCount:     0,
		roundRobinIndex: 0,
		threadIdCounter: 0,
		stopCleanup:     make(chan struct{}),
		replenishChan:   make(chan struct{}, 1),
	}
	// Initialize empty thread ID list with pointer
	emptyIds := make([]uint32, 0)
	p.threadIds.Store(&emptyIds)
	return p
}

// start initializes the thread pool with the minimum number of threads.
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

	if p.executor.logger != nil {
		p.executor.logger.Debug("Thread pool started",
			"minPoolSize", p.executor.options.minPoolSize,
			"maxPoolSize", p.executor.options.maxPoolSize,
			"queueSize", p.executor.options.queueSize,
			"threadTTL", p.executor.options.threadTTL,
			"maxExecutions", p.executor.options.maxExecutions,
			"executeTimeout", p.executor.options.executeTimeout,
			"createThreshold", p.executor.options.createThreshold,
			"selectThreshold", p.executor.options.selectThreshold,
			"initialThreads", p.threadCount,
		)
	}
	return nil
}

// stop shuts down the thread pool and all threads.
func (p *pool) stop() error {
	close(p.stopCleanup)
	close(p.replenishChan)

	// Stop all threads by ranging over sync.Map
	p.threads.Range(func(key, value interface{}) bool {
		t := value.(*thread)
		t.stop()
		return true
	})

	// Clear the thread collections
	p.threads = sync.Map{}
	emptyIds := make([]uint32, 0)
	p.threadIds.Store(&emptyIds)
	atomic.StoreUint32(&p.threadCount, 0)

	if p.executor.logger != nil {
		p.executor.logger.Debug("Thread pool stopped")
	}

	return nil
}

// addThreadToList adds a thread ID to the round-robin list using copy-on-write.
func (p *pool) addThreadToList(threadId uint32) {
	for {
		oldIdsPtr := p.threadIds.Load().(*[]uint32)
		oldIds := *oldIdsPtr
		newIds := make([]uint32, len(oldIds)+1)
		copy(newIds, oldIds)
		newIds[len(oldIds)] = threadId

		// Use CompareAndSwap with pointer to slice
		if p.threadIds.CompareAndSwap(oldIdsPtr, &newIds) {
			break
		}
		// If CAS fails, retry with the updated list
	}
}

// removeThreadFromList removes a thread ID from the round-robin list using copy-on-write.
func (p *pool) removeThreadFromList(threadId uint32) {
	for {
		oldIdsPtr := p.threadIds.Load().(*[]uint32)
		oldIds := *oldIdsPtr
		newIds := make([]uint32, 0, len(oldIds))

		// Copy all IDs except the one to remove
		for _, id := range oldIds {
			if id != threadId {
				newIds = append(newIds, id)
			}
		}

		// Use CompareAndSwap with pointer to slice
		if p.threadIds.CompareAndSwap(oldIdsPtr, &newIds) {
			break
		}
		// If CAS fails, retry with the updated list
	}
}

// createThread creates and starts a new thread with atomic thread count control.
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

	// Wait for the thread to finish initialization
	if err := <-t.initCh; err != nil {
		// Initialization failed, so we decrement the thread count and return the error.
		// The thread is not in the pool yet, so no need to remove it from maps.
		atomic.AddUint32(&p.threadCount, ^uint32(0)) // -1
		return nil, fmt.Errorf("thread initialization failed: %w", err)
	}

	// Add thread to sync.Map (lock-free)
	p.threads.Store(threadId, t)

	// Add thread ID to round-robin list (copy-on-write)
	p.addThreadToList(threadId)

	return t, nil
}

// selectThread selects an appropriate thread for executing a request using lock-free round-robin.
func (p *pool) selectThread(req *JsRequest) *thread {
	// 1. Check if a specific thread is requested via context
	if req.Context != nil {
		if threadIdVal, exists := req.Context[ThreadIdKey]; exists {
			if threadId, ok := threadIdVal.(uint32); ok {
				if t, found := p.threads.Load(threadId); found {
					return t.(*thread)
				}
			}
		}
	}

	// 2. Get current thread ID snapshot for round-robin selection
	threadIds := *p.threadIds.Load().(*[]uint32)
	listLen := len(threadIds)
	if listLen == 0 {
		return nil
	}

	// 3. Try to find a thread that's not too busy (load balancing)
	startIndex := atomic.AddUint32(&p.roundRobinIndex, 1) % uint32(listLen)
	for i := 0; i < listLen; i++ {
		index := (startIndex + uint32(i)) % uint32(listLen)
		threadId := threadIds[index]

		if t, exists := p.threads.Load(threadId); exists {
			thread := t.(*thread)
			queueThreshold := int(float64(p.executor.options.queueSize) * p.executor.options.selectThreshold)
			if len(thread.taskQueue) < queueThreshold {
				return thread
			}
		}
	}

	// 4. If all threads are busy, return the next thread in round-robin order
	threadId := threadIds[startIndex]
	if t, exists := p.threads.Load(threadId); exists {
		return t.(*thread)
	}

	return nil
}

// getOrCreateThread gets an existing thread or creates a new one if needed.
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
			p.executor.logger.Debug("Creating new thread due to high load",
				"currentThreads", currentThreadCount,
				"maxPoolSize", p.executor.options.maxPoolSize)
		}
		if t, err := p.createThread(); err == nil {
			return t, nil
		}
	}

	// At max capacity, return an existing thread
	t := p.selectThread(req)
	if t == nil {
		return nil, fmt.Errorf("no available thread in pool")
	}
	return t, nil
}

// execute executes a task using an appropriate thread.
func (p *pool) execute(task *task) (*JsResponse, error) {
	// Get a thread for execution
	t, err := p.getOrCreateThread(task.request)
	if err != nil {
		return nil, fmt.Errorf("failed to get thread: %w", err)
	}

	// Enqueue the task
	t.taskQueue <- task

	// Wait for the result, only apply timeout if executeTimeout > 0
	if p.executor.options.executeTimeout > 0 {
		select {
		case result := <-task.resultChan:
			if result.err != nil {
				return nil, result.err
			}
			return result.response, nil
		case <-time.After(p.executor.options.executeTimeout):
			return nil, fmt.Errorf("timeout waiting for task result")
		}
	} else {
		result := <-task.resultChan
		if result.err != nil {
			return nil, result.err
		}
		return result.response, nil
	}
}

// reload reloads all threads with new scripts.
func (p *pool) reload() error {
	var reloadError error

	// Range over sync.Map to reload all threads
	p.threads.Range(func(key, value interface{}) bool {
		t := value.(*thread)
		if err := t.reload(); err != nil {
			reloadError = fmt.Errorf("failed to reload thread %s: %w", t.name, err)
			return false // Stop iteration on error
		}
		return true // Continue iteration
	})

	if p.executor.logger != nil {
		if reloadError == nil {
			p.executor.logger.Debug("All threads reloaded successfully",
				"threadCount", p.threadCount,
			)
		}
	}

	return reloadError
}

// retireThreads runs the background cleanup process for idle or overused threads.
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

// shouldRemoveThread determines if a thread should be removed based on TTL and execution count.
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

// performCleanup performs the actual cleanup of idle or overused threads using lock-free operations.
func (p *pool) performCleanup() {
	now := time.Now()
	currentThreadCount := atomic.LoadUint32(&p.threadCount)

	if currentThreadCount <= p.executor.options.minPoolSize {
		return
	}

	// Collect threads to remove by ranging over sync.Map
	var threadsToRemove []threadCleanupInfo
	p.threads.Range(func(key, value interface{}) bool {
		threadId := key.(uint32)
		t := value.(*thread)

		if p.shouldRemoveThread(t, now) {
			// Ensure we don't remove too many threads
			if currentThreadCount-uint32(len(threadsToRemove)) > p.executor.options.minPoolSize {
				threadsToRemove = append(threadsToRemove, threadCleanupInfo{
					id:     threadId,
					thread: t,
				})
			}
		}
		return true // Continue iteration
	})

	if len(threadsToRemove) == 0 {
		return
	}

	// Remove threads from pool and update counters
	actualRemovedCount := 0
	for _, info := range threadsToRemove {
		// Remove from sync.Map (this is atomic)
		if _, loaded := p.threads.LoadAndDelete(info.id); loaded {
			// Remove from round-robin list
			p.removeThreadFromList(info.id)
			atomic.AddUint32(&p.threadCount, ^uint32(0)) // -1
			actualRemovedCount++
		}
	}

	newThreadCount := atomic.LoadUint32(&p.threadCount)

	// Stop threads asynchronously (no locks needed)
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
				p.executor.logger.Debug("Thread removed",
					"thread", th.name,
					"reason", reason,
					"executions", tc,
					"idleTime", now.Sub(lu),
					"remainingThreads", newThreadCount)
			}
		}(info.thread, taskCount, lastUsed)
	}
}

// replenish checks if the pool needs more threads and creates them if necessary.
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
