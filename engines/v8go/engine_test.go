//go:build !windows

// Copyright 2025 Brian Wang <wangbuke@gmail.com>
// SPDX-License-Identifier: Apache-2.0

package v8engine

import (
	"errors"
	"testing"

	jsexecutor "github.com/buke/js-executor"
	"github.com/stretchr/testify/require"
	"github.com/tommie/v8go"
)

// TestNewEngine tests the creation of a new V8 engine.
func TestNewEngine(t *testing.T) {
	t.Run("Success", func(t *testing.T) {
		engine, err := newEngine()
		require.NoError(t, err)
		require.NotNil(t, engine)
		require.NotNil(t, engine.Iso)
		require.NotNil(t, engine.Ctx)
		require.Equal(t, rpcScript, engine.RpcScript)
		engine.Close()
	})

	t.Run("With Option", func(t *testing.T) {
		customScript := "() => 'custom'"
		engine, err := newEngine(WithRpcScript(customScript))
		require.NoError(t, err)
		require.NotNil(t, engine)
		require.Equal(t, customScript, engine.RpcScript)
		engine.Close()
	})

	t.Run("With Failing Option", func(t *testing.T) {
		expectedErr := errors.New("option failed")
		failingOption := func(e *Engine) error {
			return expectedErr
		}
		engine, err := newEngine(failingOption)
		require.Error(t, err)
		require.ErrorIs(t, err, expectedErr)
		require.Nil(t, engine)
	})
}

// TestNewEngine_Fails tests the failure paths of newEngine.
func TestNewEngine_Fails(t *testing.T) {
	t.Run("Isolate Creation Fails", func(t *testing.T) {
		// Monkey-patch the function to simulate failure
		originalNewIsolate := v8NewIsolate
		v8NewIsolate = func() *v8go.Isolate {
			return nil
		}
		// Restore the original function after the test
		defer func() {
			v8NewIsolate = originalNewIsolate
		}()

		_, err := newEngine()
		require.Error(t, err)
		require.Contains(t, err.Error(), "failed to create v8 isolate")
	})

	t.Run("Context Creation Fails", func(t *testing.T) {
		// Monkey-patch the context creation to simulate failure
		originalNewContext := v8NewContext
		// The mock function must have the correct signature to match the original.
		v8NewContext = func(opt ...v8go.ContextOption) *v8go.Context {
			return nil
		}
		// Restore the original function after the test
		defer func() {
			v8NewContext = originalNewContext
		}()

		_, err := newEngine()
		require.Error(t, err)
		require.Contains(t, err.Error(), "failed to create v8 context")
	})
}

// TestEngine_Init tests the Init method.
func TestEngine_Init(t *testing.T) {
	engine, err := newEngine()
	require.NoError(t, err)
	defer engine.Close()

	t.Run("Success", func(t *testing.T) {
		script := &jsexecutor.InitScript{
			FileName: "test.js",
			Content:  "var a = 1;",
		}
		err := engine.Init([]*jsexecutor.InitScript{script})
		require.NoError(t, err)
	})

	t.Run("Invalid Script", func(t *testing.T) {
		script := &jsexecutor.InitScript{
			FileName: "invalid.js",
			Content:  "var a =;",
		}
		err := engine.Init([]*jsexecutor.InitScript{script})
		require.Error(t, err)
	})
}

// TestEngine_Reload tests the Reload method.
func TestEngine_Reload(t *testing.T) {
	engine, err := newEngine()
	require.NoError(t, err)
	defer engine.Close()

	// Set a variable in the initial context
	_, err = engine.Ctx.RunScript("var initialVar = 'old';", "setup.js")
	require.NoError(t, err)

	// Reload with a new script
	newScript := &jsexecutor.InitScript{FileName: "new.js", Content: "var newVar = 'new';"}
	err = engine.Reload([]*jsexecutor.InitScript{newScript})
	require.NoError(t, err)

	// Verify the old variable does not exist
	val, err := engine.Ctx.RunScript("typeof initialVar", "check.js")
	require.NoError(t, err)
	require.Equal(t, "undefined", val.String())

	// Verify the new variable exists
	val, err = engine.Ctx.RunScript("newVar", "check.js")
	require.NoError(t, err)
	require.Equal(t, "new", val.String())
}

// TestEngine_Reload_Fails tests the failure path of the Reload method.
func TestEngine_Reload_Fails(t *testing.T) {
	engine, err := newEngine()
	require.NoError(t, err)
	defer engine.Close()

	// Monkey-patch the context creation to simulate failure
	originalNewContext := v8NewContext
	// The mock function must have the correct signature to match the original.
	v8NewContext = func(opt ...v8go.ContextOption) *v8go.Context {
		return nil
	}
	defer func() {
		v8NewContext = originalNewContext
	}()

	err = engine.Reload([]*jsexecutor.InitScript{})
	require.Error(t, err)
	require.Contains(t, err.Error(), "failed to create new v8 context for reload")
}

// TestEngine_Execute tests the success path and business logic failures of the Execute method.
func TestEngine_Execute(t *testing.T) {
	engine, err := newEngine()
	require.NoError(t, err)
	defer engine.Close()

	// Init a simple function
	initScript := &jsexecutor.InitScript{FileName: "test.js", Content: "function add(a, b) { return a + b; }"}
	err = engine.Init([]*jsexecutor.InitScript{initScript})
	require.NoError(t, err)

	t.Run("Success", func(t *testing.T) {
		req := &jsexecutor.JsRequest{
			Service: "add",
			Args:    []interface{}{2, 3},
		}
		resp, err := engine.Execute(req)
		require.NoError(t, err)
		require.NotNil(t, resp)
		// Note: JSON unmarshaling will result in float64 for numbers
		require.Equal(t, 5.0, resp.Result)
	})

	t.Run("JS Function Call Fails (Business Error)", func(t *testing.T) {
		req := &jsexecutor.JsRequest{
			Service: "nonExistentFunc",
		}
		// The error is now returned directly from the Execute method.
		resp, err := engine.Execute(req)
		require.Error(t, err)
		require.Nil(t, resp)
		require.Contains(t, err.Error(), "nonExistentFunc is not defined")
	})
}

// TestEngine_Execute_Fails tests the technical failure paths of the Execute method.
func TestEngine_Execute_Fails(t *testing.T) {
	t.Run("RPC Script Execution Fails", func(t *testing.T) {
		// This script is syntactically valid but throws an error immediately when run.
		// This tests the error path of `e.Ctx.RunScript`.
		engine, err := newEngine(WithRpcScript(`throw new Error('boom');`))
		require.NoError(t, err)
		defer engine.Close()

		_, err = engine.Execute(&jsexecutor.JsRequest{})
		require.Error(t, err)
		require.Contains(t, err.Error(), "failed to run rpc script")
		require.Contains(t, err.Error(), "boom") // Check for the original error message
	})
	t.Run("RPC Script is not a function", func(t *testing.T) {
		engine, err := newEngine(WithRpcScript(`"a string, not a function"`))
		require.NoError(t, err)
		defer engine.Close()

		_, err = engine.Execute(&jsexecutor.JsRequest{})
		require.Error(t, err)
		require.Contains(t, err.Error(), "rpc script did not return a function")
	})

	t.Run("RPC Function Call Fails", func(t *testing.T) {
		// This script is a non-async function that throws a synchronous error.
		// This should cause the `rpcFn.Call` method itself to return an error.
		engine, err := newEngine(WithRpcScript(`() => { throw new Error('synchronous boom'); }`))
		require.NoError(t, err)
		defer engine.Close()

		_, err = engine.Execute(&jsexecutor.JsRequest{})
		require.Error(t, err)
		require.Contains(t, err.Error(), "rpc function call failed")
		require.Contains(t, err.Error(), "synchronous boom")
	})

	t.Run("RPC Script returns non-promise", func(t *testing.T) {
		engine, err := newEngine(WithRpcScript(`() => "not a promise"`))
		require.NoError(t, err)
		defer engine.Close()

		_, err = engine.Execute(&jsexecutor.JsRequest{})
		require.Error(t, err)
		require.Contains(t, err.Error(), "rpc call did not return a promise")
	})

	t.Run("Request Marshal Fails", func(t *testing.T) {
		engine, err := newEngine()
		require.NoError(t, err)
		defer engine.Close()

		// Channels cannot be marshaled to JSON
		req := &jsexecutor.JsRequest{
			Args: []interface{}{make(chan int)},
		}
		_, err = engine.Execute(req)
		require.Error(t, err)
		require.Contains(t, err.Error(), "failed to json marshal request")
	})

	t.Run("V8 Value Creation from JSON Fails", func(t *testing.T) {
		engine, err := newEngine()
		require.NoError(t, err)
		defer engine.Close()

		// Mock v8NewValue to fail on the second call (the one with the JSON string)
		originalNewValue := v8NewValue
		v8NewValue = func(iso *v8go.Isolate, val any) (*v8go.Value, error) {
			// The first call with the struct should fail to enter the fallback logic.
			if _, ok := val.(*jsexecutor.JsRequest); ok {
				return nil, errors.New("mock error: cannot convert struct")
			}
			// The second call with the string must also fail to hit our target branch.
			return nil, errors.New("mock error: cannot convert string")
		}
		defer func() { v8NewValue = originalNewValue }()

		_, err = engine.Execute(&jsexecutor.JsRequest{})
		require.Error(t, err)
		require.Contains(t, err.Error(), "failed to create v8 value from json string")
	})

	t.Run("Response Marshal Fails (Circular Reference)", func(t *testing.T) {
		// This script creates an object with a circular reference, which cannot be JSON.stringified.
		circularScript := `function circular() { let a = {}; a.b = a; return a; }`
		engine, err := newEngine()
		require.NoError(t, err)
		defer engine.Close()
		err = engine.Init([]*jsexecutor.InitScript{{FileName: "circular.js", Content: circularScript}})
		require.NoError(t, err)

		req := &jsexecutor.JsRequest{Service: "circular"}
		_, err = engine.Execute(req)
		require.Error(t, err)
		// v8go's MarshalJSON on a circular reference returns an empty byte slice without an error.
		// We catch this specific case.
		require.Contains(t, err.Error(), "failed to marshal response value to json: result is empty")
	})

	t.Run("Response Unmarshal Fails", func(t *testing.T) {
		// This script returns a valid JSON, but its structure doesn't match JsResponse.
		mismatchScript := `async () => ({ id: 123, result: "ok" })`
		engine, err := newEngine(WithRpcScript(mismatchScript))
		require.NoError(t, err)
		defer engine.Close()

		// We mock json.Unmarshal to trigger the error reliably.
		originalUnmarshal := jsonUnmarshal
		jsonUnmarshal = func(data []byte, v any) error {
			return errors.New("unmarshal error")
		}
		defer func() { jsonUnmarshal = originalUnmarshal }()

		_, err = engine.Execute(&jsexecutor.JsRequest{})
		require.Error(t, err)
		require.Contains(t, err.Error(), "failed to unmarshal response")
	})
}

// TestEngine_Close tests the Close method.
func TestEngine_Close(t *testing.T) {
	engine, err := newEngine()
	require.NoError(t, err)

	// First close should be successful
	err = engine.Close()
	require.NoError(t, err)
	require.Nil(t, engine.Iso)
	require.Nil(t, engine.Ctx)

	// Second close should also be successful (idempotent)
	err = engine.Close()
	require.NoError(t, err)
}
