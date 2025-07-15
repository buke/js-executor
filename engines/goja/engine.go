// Copyright 2025 Brian Wang <wangbuke@gmail.com>
// SPDX-License-Identifier: Apache-2.0

package gojaengine

import (
	_ "embed"
	"fmt"

	jsexecutor "github.com/buke/js-executor"
	"github.com/dop251/goja"
	"github.com/dop251/goja_nodejs/eventloop"
)

//go:embed engine_rpc.js
var rpcScript string

// Engine implements the jsexecutor.JsEngine interface using the Goja JS engine.
// It uses an event loop to ensure thread-safe execution of JavaScript.
type Engine struct {
	Loop   *eventloop.EventLoop // The event loop that owns and serializes access to the runtime.
	Option *EngineOption        // Engine configuration options.
	opts   []Option             // Store original options for reloading.
}

// NewFactory returns a jsexecutor.JsEngineFactory for creating Goja engines.
// The factory is configured with the provided options.
func NewFactory(opts ...Option) jsexecutor.JsEngineFactory {
	return func() (jsexecutor.JsEngine, error) {
		return newEngine(opts...)
	}
}

// newEngine creates a new Goja engine instance.
// It initializes a full-featured event loop that supports timers.
func newEngine(opts ...Option) (*Engine, error) {
	// The eventloop creates its own internal goja.Runtime
	loop := eventloop.NewEventLoop()

	e := &Engine{
		Loop:   loop,
		Option: &EngineOption{}, // Initialize with default options
		opts:   opts,            // Store for Reload
	}

	// Start the event loop *before* applying options
	loop.Start()

	// Apply the default FieldNameMapper first.
	// This can be overridden by user-provided options.
	WithFieldNameMapper(goja.TagFieldNameMapper("json", true))(e)

	// Apply all provided options. Each option will block until it's applied.
	for _, opt := range opts {
		if err := opt(e); err != nil {
			loop.Stop() // Ensure loop is stopped on configuration error
			return nil, fmt.Errorf("failed to apply option: %w", err)
		}
	}

	return e, nil
}

// Init runs initialization scripts on the engine's event loop.
func (e *Engine) Init(scripts []*jsexecutor.JsScript) error {
	done := make(chan error, 1)
	e.Loop.RunOnLoop(func(vm *goja.Runtime) {
		for _, script := range scripts {
			if _, err := vm.RunScript(script.FileName, script.Content); err != nil {
				done <- fmt.Errorf("failed to execute init script %s: %w", script.FileName, err)
				return
			}
		}
		done <- nil // Signal success
	})
	return <-done
}

// Reload creates a new event loop and re-initializes the engine.
// This is a "hard" reload, replacing the entire JS environment.
func (e *Engine) Reload(scripts []*jsexecutor.JsScript) error {
	// Stop the old loop and release its resources.
	e.Close()

	// Create a new engine with the original options.
	newE, err := newEngine(e.opts...)
	if err != nil {
		return fmt.Errorf("failed to create new engine on reload: %w", err)
	}

	// Replace the current engine's state with the new one.
	e.Loop = newE.Loop
	e.Option = newE.Option
	e.opts = newE.opts

	// Initialize the new engine with the provided scripts.
	return e.Init(scripts)
}

// Execute runs a JavaScript request and returns the response.
// It schedules the execution on the event loop and handles async results.
func (e *Engine) Execute(req *jsexecutor.JsRequest) (*jsexecutor.JsResponse, error) {
	if req == nil {
		return nil, fmt.Errorf("request cannot be nil")
	}

	resultChan := make(chan *jsexecutor.JsResponse, 1)
	errorChan := make(chan error, 1)

	// Schedule the job on the persistent event loop.
	e.Loop.RunOnLoop(func(vm *goja.Runtime) {
		fnValue, err := vm.RunScript("engine_rpc.js", rpcScript)
		if err != nil {
			errorChan <- fmt.Errorf("failed to load rpc script: %w", err)
			return
		}
		fn, ok := goja.AssertFunction(fnValue)
		if !ok {
			errorChan <- fmt.Errorf("rpc script did not return a function")
			return
		}

		resPromise, err := fn(goja.Undefined(), vm.ToValue(req))
		if err != nil {
			errorChan <- fmt.Errorf("failed to call rpc function: %w", err)
			return
		}

		// Check for null or undefined BEFORE calling ToObject to prevent a panic.
		if goja.IsUndefined(resPromise) || goja.IsNull(resPromise) {
			errorChan <- fmt.Errorf("rpc call did not return a promise-like object")
			return
		}

		promiseObj := resPromise.ToObject(vm)

		then, ok := goja.AssertFunction(promiseObj.Get("then"))
		if !ok {
			errorChan <- fmt.Errorf("rpc call did not return a promise (missing .then method)")
			return
		}

		onSuccess := func(call goja.FunctionCall) goja.Value {
			var response jsexecutor.JsResponse
			if err := vm.ExportTo(call.Argument(0), &response); err != nil {
				errorChan <- fmt.Errorf("failed to export result: %w", err)
			} else {
				resultChan <- &response
			}
			return goja.Undefined()
		}

		onError := func(call goja.FunctionCall) goja.Value {
			errorChan <- fmt.Errorf("js execution error: %s", call.Argument(0).String())
			return goja.Undefined()
		}

		// Correctly call the 'then' function on the promise object.
		if _, err := then(promiseObj, vm.ToValue(onSuccess), vm.ToValue(onError)); err != nil {
			errorChan <- fmt.Errorf("failed to invoke promise.then: %w", err)
		}
	})

	// Wait for the result.
	select {
	case response := <-resultChan:
		return response, nil
	case err := <-errorChan:
		return nil, err
	}
}

// Close stops the event loop and releases associated resources.
func (e *Engine) Close() error {
	if e.Loop != nil {
		e.Loop.Stop()
	}
	return nil
}
