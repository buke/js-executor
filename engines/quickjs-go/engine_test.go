// Copyright 2025 Brian Wang <wangbuke@gmail.com>
// SPDX-License-Identifier: Apache-2.0

package quickjsengine

import (
	"errors"
	"testing"

	jsexecutor "github.com/buke/js-executor"
	"github.com/stretchr/testify/require"
)

// TestEngine_Init_Success tests successful initialization of the engine with valid scripts.
func TestEngine_Init_Success(t *testing.T) {
	engine, err := newEngine()
	require.NoError(t, err)
	defer engine.Close()

	scripts := []*jsexecutor.JsScript{
		{FileName: "init.js", Content: "function add(a, b) { return a + b; }"},
	}
	err = engine.Load(scripts)
	require.NoError(t, err)
}

// TestEngine_Init_ScriptError tests initialization failure due to a syntax error in the script.
func TestEngine_Init_ScriptError(t *testing.T) {
	engine, err := newEngine()
	require.NoError(t, err)
	defer engine.Close()

	scripts := []*jsexecutor.JsScript{
		{FileName: "bad.js", Content: "function () { syntax error }"},
	}
	err = engine.Load(scripts)
	require.Error(t, err)
}

// TestEngine_Reload tests reloading the engine with new scripts.
func TestEngine_Reload(t *testing.T) {
	engine, err := newEngine()
	require.NoError(t, err)
	defer engine.Close()

	scripts1 := []*jsexecutor.JsScript{
		{FileName: "a.js", Content: "function foo() { return 1; }"},
	}
	require.NoError(t, engine.Load(scripts1))

	scripts2 := []*jsexecutor.JsScript{
		{FileName: "b.js", Content: "function foo() { return 2; }"},
	}
	require.NoError(t, engine.Reload(scripts2))
}

// TestEngine_Reload_Fails tests the failure path of the Reload method.
func TestEngine_Reload_Fails(t *testing.T) {
	callCount := 0
	statefulFailingOption := func(e jsexecutor.JsEngine) error {
		callCount++
		if callCount > 1 {
			return errors.New("simulated failure on second call")
		}
		return nil
	}

	// The first call to newEngine will succeed, storing the option.
	engine, err := newEngine(statefulFailingOption)
	require.NoError(t, err)
	defer engine.Close()

	// The second call to newEngine, inside Reload, should fail.
	err = engine.Reload(nil)
	require.Error(t, err)
	require.Contains(t, err.Error(), "failed to apply option: simulated failure on second call")
}

// TestEngine_Execute_Success tests successful execution of a JS request.
func TestEngine_Execute_Success(t *testing.T) {
	engine, err := newEngine()
	require.NoError(t, err)
	defer engine.Close()

	scripts := []*jsexecutor.JsScript{
		{FileName: "add.js", Content: "function add(a, b) { return a + b; }"},
	}
	require.NoError(t, engine.Load(scripts))

	req := &jsexecutor.JsRequest{
		Id:      "1",
		Service: "add",
		Args:    []interface{}{2, 3},
	}
	resp, err := engine.Execute(req)
	require.NoError(t, err)
	require.NotNil(t, resp)
}

// TestEngine_Execute_Exception tests execution when the JS function throws or does not exist.
func TestEngine_Execute_Exception(t *testing.T) {
	engine, err := newEngine()
	require.NoError(t, err)
	defer engine.Close()

	scripts := []*jsexecutor.JsScript{
		{FileName: "empty.js", Content: ""},
	}
	require.NoError(t, engine.Load(scripts))

	req := &jsexecutor.JsRequest{
		Id:      "2",
		Service: "foo",
		Args:    []interface{}{},
	}
	_, err = engine.Execute(req)
	require.Error(t, err)
}

// TestEngine_Execute_NilRequest tests execution with a nil request.
func TestEngine_Execute_NilRequest(t *testing.T) {
	engine, err := newEngine()
	require.NoError(t, err)
	defer engine.Close()

	_, err = engine.Execute(nil)
	require.Error(t, err)
}

// TestEngine_Close_Idempotent tests that closing the engine multiple times is safe.
func TestEngine_Close_Idempotent(t *testing.T) {
	engine, err := newEngine()
	require.NoError(t, err)
	require.NoError(t, engine.Close())
	require.NoError(t, engine.Close())
}

// TestEngine_NewEngine_OptionError tests error handling when an option returns an error.
func TestEngine_NewEngine_OptionError(t *testing.T) {
	opt := func(e jsexecutor.JsEngine) error { return errors.New("option error") }
	engine, err := newEngine(opt)
	require.Error(t, err)
	require.Nil(t, engine)
}

// TestEngine_Execute_EvalRpcScriptException tests error handling when evaluating the RPC script fails.
func TestEngine_Execute_EvalRpcScriptException(t *testing.T) {
	engine, err := newEngine()
	require.NoError(t, err)
	defer engine.Close()

	// Set an invalid RPC script to trigger Eval exception
	engine.RpcScript = "throw new Error('bad rpc script');"
	scripts := []*jsexecutor.JsScript{}
	require.NoError(t, engine.Load(scripts))

	req := &jsexecutor.JsRequest{
		Id:      "1",
		Service: "add",
		Args:    []interface{}{1, 2},
	}
	_, err = engine.Execute(req)
	require.Error(t, err)
	require.Contains(t, err.Error(), "failed to evaluate RPC script")
}

// TestEngine_Execute_MarshalRequestError tests error handling when marshaling the request fails.
func TestEngine_Execute_MarshalRequestError(t *testing.T) {
	engine, err := newEngine()
	require.NoError(t, err)
	defer engine.Close()

	engine.RpcScript = "function rpc(req) { return req; }"

	type Unmarshalable struct {
		Ch chan int // Exported field that cannot be marshaled
	}
	req := &jsexecutor.JsRequest{
		Id:      "1",
		Service: "add",
		Args:    []interface{}{Unmarshalable{make(chan int)}},
	}
	_, err = engine.Execute(req)
	t.Logf("marshal error: %v", err)
	require.Error(t, err)
	require.Contains(t, err.Error(), "failed to marshal request")
}

// TestEngine_Execute_UnmarshalResponseError tests error handling when unmarshaling the JS response fails.
func TestEngine_Execute_UnmarshalResponseError(t *testing.T) {
	engine, err := newEngine()
	require.NoError(t, err)
	defer engine.Close()

	engine.RpcScript = `(req) => Symbol("x")`

	scripts := []*jsexecutor.JsScript{}
	require.NoError(t, engine.Load(scripts))

	req := &jsexecutor.JsRequest{
		Id:      "1",
		Service: "add",
		Args:    []interface{}{1, 2},
	}
	_, err = engine.Execute(req)
	require.Error(t, err)
	require.Contains(t, err.Error(), "failed to unmarshal response")
}
