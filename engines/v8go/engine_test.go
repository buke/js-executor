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
		failingOption := func(engine jsexecutor.JsEngine) error {
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
		defer func() {
			v8NewIsolate = originalNewIsolate
		}()

		_, err := newEngine()
		require.Error(t, err)
		require.Contains(t, err.Error(), "failed to create v8 isolate")
	})

	t.Run("Context Creation Fails", func(t *testing.T) {
		originalNewContext := v8NewContext
		v8NewContext = func(opt ...v8go.ContextOption) *v8go.Context {
			return nil
		}
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
		script := &jsexecutor.JsScript{
			FileName: "test.js",
			Content:  "var a = 1;",
		}
		err := engine.Load([]*jsexecutor.JsScript{script})
		require.NoError(t, err)
	})

	t.Run("Invalid Script", func(t *testing.T) {
		script := &jsexecutor.JsScript{
			FileName: "invalid.js",
			Content:  "var a =;",
		}
		err := engine.Load([]*jsexecutor.JsScript{script})
		require.Error(t, err)
	})
}

// TestEngine_Execute tests the success path and business logic failures of the Execute method.
func TestEngine_Execute(t *testing.T) {
	engine, err := newEngine()
	require.NoError(t, err)
	defer engine.Close()

	// Init a simple function
	initScript := &jsexecutor.JsScript{FileName: "test.js", Content: "function add(a, b) { return a + b; }"}
	err = engine.Load([]*jsexecutor.JsScript{initScript})
	require.NoError(t, err)

	t.Run("Success", func(t *testing.T) {
		req := &jsexecutor.JsRequest{
			Service: "add",
			Args:    []interface{}{2, 3},
		}
		resp, err := engine.Execute(req)
		require.NoError(t, err)
		require.NotNil(t, resp)
		require.Equal(t, 5.0, resp.Result)
	})

	t.Run("JS Function Call Fails (Business Error)", func(t *testing.T) {
		req := &jsexecutor.JsRequest{
			Service: "nonExistentFunc",
		}
		resp, err := engine.Execute(req)
		require.Error(t, err)
		require.Nil(t, resp)
		require.Contains(t, err.Error(), "nonExistentFunc is not defined")
	})
}

// TestEngine_Execute_Fails tests the technical failure paths of the Execute method.
func TestEngine_Execute_Fails(t *testing.T) {
	t.Run("RPC Script Execution Fails", func(t *testing.T) {
		engine, err := newEngine(WithRpcScript(`throw new Error('boom');`))
		require.NoError(t, err)
		defer engine.Close()

		_, err = engine.Execute(&jsexecutor.JsRequest{})
		require.Error(t, err)
		require.Contains(t, err.Error(), "failed to run rpc script")
		require.Contains(t, err.Error(), "boom")
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

		originalNewValue := v8NewValue
		v8NewValue = func(iso *v8go.Isolate, val any) (*v8go.Value, error) {
			if _, ok := val.(*jsexecutor.JsRequest); ok {
				return nil, errors.New("mock error: cannot convert struct")
			}
			return nil, errors.New("mock error: cannot convert string")
		}
		defer func() { v8NewValue = originalNewValue }()

		_, err = engine.Execute(&jsexecutor.JsRequest{})
		require.Error(t, err)
		require.Contains(t, err.Error(), "failed to create v8 value from json string")
	})

	t.Run("Response Marshal Fails (Circular Reference)", func(t *testing.T) {
		circularScript := `function circular() { let a = {}; a.b = a; return a; }`
		engine, err := newEngine()
		require.NoError(t, err)
		defer engine.Close()
		err = engine.Load([]*jsexecutor.JsScript{{FileName: "circular.js", Content: circularScript}})
		require.NoError(t, err)

		req := &jsexecutor.JsRequest{Service: "circular"}
		_, err = engine.Execute(req)
		require.Error(t, err)
		require.Contains(t, err.Error(), "failed to marshal response value to json: result is empty")
	})

	t.Run("Response Unmarshal Fails", func(t *testing.T) {
		mismatchScript := `async () => ({ id: 123, result: "ok" })`
		engine, err := newEngine(WithRpcScript(mismatchScript))
		require.NoError(t, err)
		defer engine.Close()

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
