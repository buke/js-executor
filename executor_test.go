// Copyright 2025 Brian Wang <wangbuke@gmail.com>
// SPDX-License-Identifier: Apache-2.0

package jsexecutor

import (
	"errors"
	"log/slog"
	"reflect"
	"sync"
	"testing"
	"time"
)

// mockEngine is a simple mock implementation of JsEngine for testing.
type mockEngine struct {
	mu           sync.Mutex    // Mutex for concurrent access
	initCalled   bool          // Whether Init was called
	reloadCalled bool          // Whether Reload was called
	closeCalled  bool          // Whether Close was called
	initScripts  []*InitScript // Scripts passed to Init/Reload
	executedReq  *JsRequest    // Last executed request
	executeResp  *JsResponse   // Response to return from Execute
	executeErr   error         // Error to return from Execute

	initFunc    func(scripts []*InitScript) error         // Custom Init behavior (if set)
	reloadFunc  func(scripts []*InitScript) error         // Custom Reload behavior (if set)
	executeFunc func(req *JsRequest) (*JsResponse, error) // Custom Execute behavior (if set)
	closeFunc   func() error                              // Custom Close behavior (if set)
}

// Init mocks the initialization of the JavaScript engine.
func (m *mockEngine) Init(scripts []*InitScript) error {
	m.mu.Lock()
	m.initCalled = true
	m.initScripts = scripts
	m.mu.Unlock()
	if m.initFunc != nil {
		return m.initFunc(scripts)
	}
	return nil
}

// Reload mocks reloading the JavaScript engine with new scripts.
func (m *mockEngine) Reload(scripts []*InitScript) error {
	m.reloadCalled = true
	m.initScripts = scripts
	if m.reloadFunc != nil {
		return m.reloadFunc(scripts)
	}
	return nil
}

// Execute mocks executing a JavaScript request.
func (m *mockEngine) Execute(req *JsRequest) (*JsResponse, error) {
	m.mu.Lock()
	m.executedReq = req
	m.mu.Unlock()
	if m.executeFunc != nil {
		return m.executeFunc(req)
	}
	return m.executeResp, m.executeErr
}

// Close mocks closing the JavaScript engine.
func (m *mockEngine) Close() error {
	m.closeCalled = true
	if m.closeFunc != nil {
		return m.closeFunc()
	}
	return nil
}

// mockEngineFactory returns a new mockEngine instance as JsEngineFactory.
func mockEngineFactory() JsEngineFactory {
	return func() (JsEngine, error) {
		return &mockEngine{
			executeFunc: func(req *JsRequest) (*JsResponse, error) {
				return &JsResponse{Id: req.Id, Result: "ok"}, nil
			},
		}, nil
	}
}

// TestJsExecutor_Start_Stop tests starting and stopping the executor.
func TestJsExecutor_Start_Stop(t *testing.T) {
	executor, err := NewExecutor(
		WithJsEngine(mockEngineFactory()),
	)
	if err != nil {
		t.Fatalf("Failed to create executor: %v", err)
	}

	// Start the executor
	if err := executor.Start(); err != nil {
		t.Fatalf("Failed to start executor: %v", err)
	}

	// Stop the executor
	if err := executor.Stop(); err != nil {
		t.Fatalf("Failed to stop executor: %v", err)
	}
}

// TestJsExecutor_Execute tests executing a JavaScript request.
func TestJsExecutor_Execute(t *testing.T) {
	executor, err := NewExecutor(
		WithJsEngine(mockEngineFactory()),
	)
	if err != nil {
		t.Fatalf("Failed to create executor: %v", err)
	}
	if err := executor.Start(); err != nil {
		t.Fatalf("Failed to start executor: %v", err)
	}
	defer executor.Stop()

	req := &JsRequest{
		Id:      "1",
		Service: "testService",
		Args:    []interface{}{"foo"},
	}
	resp, err := executor.Execute(req)
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}
	if resp == nil || resp.Result != "ok" {
		t.Errorf("Unexpected response: %+v", resp)
	}
}

// TestJsExecutor_Reload tests reloading initialization scripts.
func TestJsExecutor_Reload(t *testing.T) {
	executor, err := NewExecutor(
		WithJsEngine(mockEngineFactory()),
	)
	if err != nil {
		t.Fatalf("Failed to create executor: %v", err)
	}
	if err := executor.Start(); err != nil {
		t.Fatalf("Failed to start executor: %v", err)
	}
	defer executor.Stop()

	scripts := []*InitScript{
		{FileName: "a.js", Content: "var a = 1;"},
	}
	if err := executor.Reload(scripts...); err != nil {
		t.Fatalf("Reload failed: %v", err)
	}
	got := executor.getInitScripts()
	if !reflect.DeepEqual(got, scripts) {
		t.Errorf("Reload did not set scripts correctly, got: %+v, want: %+v", got, scripts)
	}
}

// TestJsExecutor_WithLogger tests setting a custom logger.
func TestJsExecutor_WithLogger(t *testing.T) {
	logger := slog.Default()
	executor, err := NewExecutor(
		WithJsEngine(mockEngineFactory()),
		WithLogger(logger),
	)
	if err != nil {
		t.Fatalf("Failed to create executor: %v", err)
	}
	if executor.logger != logger {
		t.Errorf("Logger not set correctly")
	}
}

// TestJsExecutor_WithThresholds tests setting create and select thresholds.
func TestJsExecutor_WithThresholds(t *testing.T) {
	executor, err := NewExecutor(
		WithJsEngine(mockEngineFactory()),
		WithCreateThreshold(0.7),
		WithSelectThreshold(0.9),
	)
	if err != nil {
		t.Fatalf("Failed to create executor: %v", err)
	}
	if executor.options.createThreshold != 0.7 || executor.options.selectThreshold != 0.9 {
		t.Errorf("Thresholds not set correctly: got %+v", executor.options)
	}
}

// TestJsExecutor_WithInitScripts tests setting initialization scripts via option.
func TestJsExecutor_WithInitScripts(t *testing.T) {
	scripts := []*InitScript{
		{FileName: "init.js", Content: "var x = 1;"},
	}
	executor, err := NewExecutor(
		WithJsEngine(mockEngineFactory()),
		WithInitScripts(scripts...),
	)
	if err != nil {
		t.Fatalf("Failed to create executor: %v", err)
	}
	got := executor.getInitScripts()
	if !reflect.DeepEqual(got, scripts) {
		t.Errorf("Init scripts not set correctly, got: %+v, want: %+v", got, scripts)
	}
}

// TestJsExecutor_WithThreadTTL tests setting the thread time-to-live option.
func TestJsExecutor_WithThreadTTL(t *testing.T) {
	executor, err := NewExecutor(
		WithJsEngine(mockEngineFactory()),
		WithThreadTTL(10*time.Second),
	)
	if err != nil {
		t.Fatalf("Failed to create executor: %v", err)
	}
	if executor.options.threadTTL != 10*time.Second {
		t.Errorf("threadTTL not set correctly, got: %v, want: %v", executor.options.threadTTL, 10*time.Second)
	}
}

// TestJsExecutor_WithMaxExecutions tests setting the maxExecutions option.
func TestJsExecutor_WithMaxExecutions(t *testing.T) {
	executor, err := NewExecutor(
		WithJsEngine(mockEngineFactory()),
		WithMaxExecutions(123),
	)
	if err != nil {
		t.Fatalf("Failed to create executor: %v", err)
	}
	if executor.options.maxExecutions != 123 {
		t.Errorf("maxExecutions not set correctly, got: %v, want: %v", executor.options.maxExecutions, 123)
	}
}

// TestJsExecutor_WithExecuteTimeout tests setting the executeTimeout option.
func TestJsExecutor_WithExecuteTimeout(t *testing.T) {
	executor, err := NewExecutor(
		WithJsEngine(mockEngineFactory()),
		WithExecuteTimeout(5*time.Second),
	)
	if err != nil {
		t.Fatalf("Failed to create executor: %v", err)
	}
	if executor.options.executeTimeout != 5*time.Second {
		t.Errorf("executeTimeout not set correctly, got: %v, want: %v", executor.options.executeTimeout, 5*time.Second)
	}
}

// TestJsExecutor_Execute_ErrorWhenPoolNotStarted tests error when executing without starting the pool.
func TestJsExecutor_Execute_ErrorWhenPoolNotStarted(t *testing.T) {
	executor := &JsExecutor{}
	_, err := executor.Execute(&JsRequest{Id: "1"})
	if err == nil {
		t.Error("Expected error when pool is not initialized")
	}
}

// TestJsExecutor_Reload_ErrorWhenPoolNotStarted tests error when reloading without starting the pool.
func TestJsExecutor_Reload_ErrorWhenPoolNotStarted(t *testing.T) {
	executor := &JsExecutor{}
	err := executor.Reload(&InitScript{FileName: "a.js", Content: "var a = 1;"})
	if err == nil {
		t.Error("Expected error when pool is not initialized")
	}
}

// TestJsExecutor_Stop_ErrorWhenPoolNotStarted tests error when stopping without starting the pool.
func TestJsExecutor_Stop_ErrorWhenPoolNotStarted(t *testing.T) {
	executor := &JsExecutor{}
	err := executor.Stop()
	if err == nil {
		t.Error("Expected error when pool is not initialized")
	}
}

// TestJsExecutor_Start_ErrorWhenPoolNotInitialized tests error when starting without initializing the pool.
func TestJsExecutor_Start_ErrorWhenPoolNotInitialized(t *testing.T) {
	executor := &JsExecutor{}
	err := executor.Start()
	if err == nil {
		t.Error("Expected error when pool is not initialized")
	}
}

// TestJsExecutor_EngineErrorPropagation tests error propagation from the engine.
func TestJsExecutor_EngineErrorPropagation(t *testing.T) {
	engine := &mockEngine{
		executeErr: errors.New("mock execute error"),
	}
	executor, err := NewExecutor(
		WithJsEngine(func() (JsEngine, error) { return engine, nil }),
	)
	if err != nil {
		t.Fatalf("Failed to create executor: %v", err)
	}
	if err := executor.Start(); err != nil {
		t.Fatalf("Failed to start executor: %v", err)
	}
	defer executor.Stop()
	_, err = executor.Execute(&JsRequest{Id: "err"})
	if err == nil || err.Error() != "mock execute error" {
		t.Errorf("Expected engine error to propagate, got: %v", err)
	}
}

// TestNewExecutor_ErrorWhenNoEngineFactory tests error when no engine factory is provided.
func TestNewExecutor_ErrorWhenNoEngineFactory(t *testing.T) {
	_, err := NewExecutor()
	if err == nil {
		t.Error("Expected error when engineFactory is nil")
	}
}

// TestJsExecutor_SetInitScripts_EmptyScripts tests setting and clearing init scripts.
func TestJsExecutor_SetInitScripts_EmptyScripts(t *testing.T) {
	executor, err := NewExecutor(
		WithJsEngine(mockEngineFactory()),
	)
	if err != nil {
		t.Fatalf("Failed to create executor: %v", err)
	}
	// Set non-empty first
	scripts := []*InitScript{{FileName: "a.js", Content: "var a = 1;"}}
	executor.setInitScripts(scripts)
	if got := executor.getInitScripts(); !reflect.DeepEqual(got, scripts) {
		t.Errorf("Expected scripts to be set")
	}
	// Now set empty
	executor.setInitScripts([]*InitScript{})
	if got := executor.getInitScripts(); got != nil {
		t.Errorf("Expected getInitScripts to return nil when set with empty slice, got: %+v", got)
	}
}

// TestJsExecutor_WithInitScripts_Empty tests WithInitScripts with no scripts.
func TestJsExecutor_WithInitScripts_Empty(t *testing.T) {
	executor, err := NewExecutor(
		WithJsEngine(mockEngineFactory()),
	)
	if err != nil {
		t.Fatalf("Failed to create executor: %v", err)
	}
	WithInitScripts()(
		executor,
	)
	if got := executor.getInitScripts(); got != nil {
		t.Errorf("Expected getInitScripts to return nil when WithInitScripts is called with no scripts")
	}
}

// TestJsExecutor_ConcurrentReloadAndExecute tests concurrent reload and execute calls.
func TestJsExecutor_ConcurrentReloadAndExecute(t *testing.T) {
	executor, _ := NewExecutor(WithJsEngine(mockEngineFactory()))
	_ = executor.Start()
	defer executor.Stop()
	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(2)
		go func() {
			defer wg.Done()
			_ = executor.Reload(&InitScript{FileName: "a.js", Content: "var a=1;"})
		}()
		go func() {
			defer wg.Done()
			_, _ = executor.Execute(&JsRequest{Id: "c"})
		}()
	}
	wg.Wait()
}
