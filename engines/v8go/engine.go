//go:build !windows

// Copyright 2025 Brian Wang <wangbuke@gmail.com>
// SPDX-License-Identifier: Apache-2.0

package v8engine

import (
	_ "embed"
	"encoding/json"
	"fmt"

	jsexecutor "github.com/buke/js-executor"
	"github.com/tommie/v8go"
)

var (
	// Make these functions variables so they can be mocked in tests.
	v8NewIsolate  = v8go.NewIsolate
	v8NewContext  = v8go.NewContext
	jsonUnmarshal = json.Unmarshal
	v8NewValue    = v8go.NewValue
)

//go:embed engine_rpc.js
var rpcScript string

// Engine implements the jsexecutor.JsEngine interface using the V8 engine.
// It encapsulates a V8 Isolate and Context.
type Engine struct {
	// Iso is the V8 Isolate, representing a single-threaded VM instance.
	// It is exposed publicly to allow for advanced custom options.
	Iso *v8go.Isolate

	// Ctx is the V8 Context, representing the execution environment.
	// It is exposed publicly to allow for advanced custom options.
	Ctx *v8go.Context

	// Option holds the engine-specific configurations.
	Option *EngineOption

	// RpcScript contains the JavaScript code for handling RPC calls.
	RpcScript string
}

// NewFactory creates a new jsexecutor.JsEngineFactory for the V8 engine.
func NewFactory(opts ...Option) jsexecutor.JsEngineFactory {
	return func() (jsexecutor.JsEngine, error) {
		return newEngine(opts...)
	}
}

// newEngine creates and initializes a new V8 Engine instance.
func newEngine(opts ...Option) (*Engine, error) {
	e := &Engine{
		Option:    &EngineOption{},
		RpcScript: rpcScript, // Set default RPC script
	}

	// Apply user-provided options
	for _, opt := range opts {
		if err := opt(e); err != nil {
			return nil, fmt.Errorf("failed to apply option: %w", err)
		}
	}

	// Override default RPC script if provided in options
	if e.Option.RpcScript != "" {
		e.RpcScript = e.Option.RpcScript
	}

	// Create a new V8 Isolate
	iso := v8NewIsolate()
	if iso == nil {
		return nil, fmt.Errorf("failed to create v8 isolate")
	}
	e.Iso = iso

	// Create a new V8 Context
	ctx := v8NewContext(iso)
	if ctx == nil {
		iso.Dispose() // Clean up isolate if context creation fails
		return nil, fmt.Errorf("failed to create v8 context")
	}
	e.Ctx = ctx

	return e, nil
}

// Init executes initialization scripts in the V8 context.
func (e *Engine) Init(scripts []*jsexecutor.InitScript) error {
	for _, script := range scripts {
		if _, err := e.Ctx.RunScript(script.Content, script.FileName); err != nil {
			return fmt.Errorf("failed to execute init script %s: %w", script.FileName, err)
		}
	}
	return nil
}

// Reload creates a new V8 context and re-initializes it, preserving the Isolate.
func (e *Engine) Reload(scripts []*jsexecutor.InitScript) error {
	if e.Ctx != nil {
		e.Ctx.Close()
	}

	newCtx := v8NewContext(e.Iso)
	if newCtx == nil {
		return fmt.Errorf("failed to create new v8 context for reload")
	}
	e.Ctx = newCtx

	return e.Init(scripts)
}

// Execute runs a JavaScript request using the RPC script.
func (e *Engine) Execute(req *jsexecutor.JsRequest) (*jsexecutor.JsResponse, error) {
	// Run the RPC script to get the handler function
	rpcVal, err := e.Ctx.RunScript(e.RpcScript, "engine_rpc.js")
	if err != nil {
		return nil, fmt.Errorf("failed to run rpc script: %w", err)
	}
	if !rpcVal.IsFunction() {
		return nil, fmt.Errorf("rpc script did not return a function")
	}

	// Marshal the Go request to a V8 value
	jsReq, err := v8NewValue(e.Iso, req)
	if err != nil {
		// v8go.NewValue doesn't support complex types, so we marshal to JSON string first.
		jsonReq, jsonErr := json.Marshal(req)
		if jsonErr != nil {
			return nil, fmt.Errorf("failed to json marshal request: %w", jsonErr)
		}
		jsReq, err = v8NewValue(e.Iso, string(jsonReq))
		if err != nil {
			return nil, fmt.Errorf("failed to create v8 value from json string: %w", err)
		}
	}

	// Call the async RPC function, which returns a Promise
	rpcFn, _ := rpcVal.AsFunction()
	promiseVal, err := rpcFn.Call(e.Ctx.Global(), jsReq)
	if err != nil {
		return nil, fmt.Errorf("rpc function call failed: %w", err)
	}
	promise, err := promiseVal.AsPromise()
	if err != nil {
		return nil, fmt.Errorf("rpc call did not return a promise: %w", err)
	}

	// Block and wait for the promise to resolve
	jsResultVal := promise.Result()

	// Check the promise state. If it was rejected, the result is the error.
	if promise.State() == v8go.Rejected {
		// The result of a rejected promise is the error object itself.
		// We return it as a Go error.
		return nil, fmt.Errorf("js execution error: %s", jsResultVal.String())
	}

	// Unmarshal the V8 result back to a Go struct
	jsonBytes, _ := jsResultVal.MarshalJSON()
	// Add a check for empty result, which can happen with circular references in v8go.
	if len(jsonBytes) == 0 {
		return nil, fmt.Errorf("failed to marshal response value to json: result is empty")
	}

	res := &jsexecutor.JsResponse{}
	if err := jsonUnmarshal(jsonBytes, res); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %w", err)
	}

	return res, nil
}

// Close releases all resources associated with the V8 engine.
func (e *Engine) Close() error {
	if e.Ctx != nil {
		e.Ctx.Close()
		e.Ctx = nil
	}
	if e.Iso != nil {
		e.Iso.Dispose()
		e.Iso = nil
	}
	return nil
}
