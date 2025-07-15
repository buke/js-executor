// Copyright 2025 Brian Wang <wangbuke@gmail.com>
// SPDX-License-Identifier: Apache-2.0

package quickjsengine

import (
	_ "embed"
	"fmt"

	jsexecutor "github.com/buke/js-executor"
	"github.com/buke/quickjs-go"
)

//go:embed engine_rpc.js
var rpcScript string

// Engine represents a QuickJS engine instance with its runtime, context, and options.
type Engine struct {
	Runtime   *quickjs.Runtime // QuickJS runtime instance
	Ctx       *quickjs.Context // QuickJS context instance
	Option    *EngineOption    // Engine configuration options
	RpcScript string           // Embedded RPC script for executing requests
	opts      []Option         // Store original options for reloading
}

// Init executes the provided initialization scripts in the engine context.
// Each script is evaluated in order. If any script fails, an error is returned.
func (e *Engine) Init(scripts []*jsexecutor.InitScript) error {
	for _, script := range scripts {
		if err := e.Ctx.Eval(script.Content, quickjs.EvalFileName(script.FileName), quickjs.EvalAwait(true)); err.IsException() {
			return fmt.Errorf("failed to execute init script %s: %w", script.FileName, e.Ctx.Exception())
		}
	}
	return nil
}

// Reload performs a "hard" reload of the engine by creating a new runtime and context.
// It re-applies the original options and then initializes with the provided scripts.
func (e *Engine) Reload(scripts []*jsexecutor.InitScript) error {
	// Close the current engine and release its resources.
	e.Close()

	e.Runtime = quickjs.NewRuntime()
	e.Ctx = e.Runtime.NewContext()

	// Apply additional engine options
	for _, option := range e.opts {
		if err := option(e); err != nil {
			e.Close()
			return fmt.Errorf("failed to apply option: %w", err)
		}
	}

	// Initialize the new engine with the provided scripts.
	return e.Init(scripts)
}

// Execute runs a JavaScript request using the embedded RPC script as the entry point.
// The request is marshaled to JS, passed to the RPC function, and the response is unmarshaled back to Go.
func (e *Engine) Execute(req *jsexecutor.JsRequest) (*jsexecutor.JsResponse, error) {
	if req == nil {
		return nil, fmt.Errorf("request cannot be nil")
	}

	// Evaluate the RPC script to get the function
	fn := e.Ctx.Eval(e.RpcScript, quickjs.EvalFileName("engine_rpc.js"))
	defer fn.Free()
	if fn.IsException() {
		return nil, fmt.Errorf("failed to evaluate RPC script: %w", e.Ctx.Exception())
	}

	// Marshal the request to a JS value
	jsReq, err := e.Ctx.Marshal(req)
	defer jsReq.Free()
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	// Call the RPC function with the marshaled request
	jsResp := fn.Execute(e.Ctx.Null(), jsReq).Await()
	defer jsResp.Free()
	if jsResp.IsException() {
		return nil, fmt.Errorf("failed to call function: %w", e.Ctx.Exception())
	}

	// Unmarshal the JS response back to Go
	res := &jsexecutor.JsResponse{}
	if err := e.Ctx.Unmarshal(jsResp, res); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %w", err)
	}

	return res, nil
}

// Close releases all resources associated with the engine, including context and runtime.
func (e *Engine) Close() error {
	if e.Ctx != nil {
		e.Ctx.Close()
		e.Ctx = nil
	}
	if e.Runtime != nil {
		e.Runtime.Close()
		e.Runtime = nil
	}
	return nil
}

// newEngine creates a new QuickJS engine instance with the given options.
// It initializes the runtime, context, and applies all provided engine options.
func newEngine(options ...Option) (*Engine, error) {
	// Create QuickJS runtime
	rt := quickjs.NewRuntime()

	// Create QuickJS context
	ctx := rt.NewContext()

	// Create engine instance with default options
	engine := &Engine{
		Runtime: rt,
		Ctx:     ctx,
		Option: &EngineOption{
			MemoryLimit:        0,     // Default memory limit (no limit)
			GCThreshold:        -1,    // Default GC threshold. -1 means no threshold
			Timeout:            0,     // Default timeout (no timeout)
			MaxStackSize:       0,     // Default max stack size
			CanBlock:           false, // Blocking not allowed by default
			EnableModuleImport: false, // Module import disabled by default
			Strip:              1,     // Default strip behavior
		},
		RpcScript: rpcScript, // Use embedded RpcScript
		opts:      options,   // Store for Reload
	}

	// Apply additional engine options
	for _, option := range options {
		if err := option(engine); err != nil {
			engine.Close()
			return nil, err
		}
	}

	return engine, nil
}

// NewFactory returns a JsEngineFactory that creates QuickJS engines with the given options.
func NewFactory(options ...Option) jsexecutor.JsEngineFactory {
	return func() (jsexecutor.JsEngine, error) {
		return newEngine(options...)
	}
}
