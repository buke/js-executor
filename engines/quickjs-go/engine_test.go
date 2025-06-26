// Copyright 2025 Brian Wang <wangbuke@gmail.com>
// SPDX-License-Identifier: Apache-2.0

package quickjsengine

import (
	"errors"
	"testing"

	jsexecutor "github.com/buke/js-executor"
	"github.com/stretchr/testify/require"
)

func TestEngine_Init_Success(t *testing.T) {
	engine, err := newEngine()
	require.NoError(t, err)
	defer engine.Close()

	scripts := []*jsexecutor.InitScript{
		{FileName: "init.js", Content: "function add(a, b) { return a + b; }"},
	}
	err = engine.Init(scripts)
	require.NoError(t, err)
}

func TestEngine_Init_ScriptError(t *testing.T) {
	engine, err := newEngine()
	require.NoError(t, err)
	defer engine.Close()

	scripts := []*jsexecutor.InitScript{
		{FileName: "bad.js", Content: "function () { syntax error }"},
	}
	err = engine.Init(scripts)
	require.Error(t, err)
}

func TestEngine_Reload(t *testing.T) {
	engine, err := newEngine()
	require.NoError(t, err)
	defer engine.Close()

	scripts1 := []*jsexecutor.InitScript{
		{FileName: "a.js", Content: "function foo() { return 1; }"},
	}
	require.NoError(t, engine.Init(scripts1))

	scripts2 := []*jsexecutor.InitScript{
		{FileName: "b.js", Content: "function foo() { return 2; }"},
	}
	require.NoError(t, engine.Reload(scripts2))
}

func TestEngine_Execute_Success(t *testing.T) {
	engine, err := newEngine()
	require.NoError(t, err)
	defer engine.Close()

	scripts := []*jsexecutor.InitScript{
		{FileName: "add.js", Content: "function add(a, b) { return a + b; }"},
	}
	require.NoError(t, engine.Init(scripts))

	req := &jsexecutor.JsRequest{
		Id:      "1",
		Service: "add",
		Args:    []interface{}{2, 3},
	}
	resp, err := engine.Execute(req)
	require.NoError(t, err)
	require.NotNil(t, resp)
}

func TestEngine_Execute_Exception(t *testing.T) {
	engine, err := newEngine()
	require.NoError(t, err)
	defer engine.Close()

	scripts := []*jsexecutor.InitScript{
		{FileName: "empty.js", Content: ""},
	}
	require.NoError(t, engine.Init(scripts))

	req := &jsexecutor.JsRequest{
		Id:      "2",
		Service: "foo",
		Args:    []interface{}{},
	}
	_, err = engine.Execute(req)
	require.Error(t, err)
}

func TestEngine_Execute_NilRequest(t *testing.T) {
	engine, err := newEngine()
	require.NoError(t, err)
	defer engine.Close()

	_, err = engine.Execute(nil)
	require.Error(t, err)
}

func TestEngine_Close_Idempotent(t *testing.T) {
	engine, err := newEngine()
	require.NoError(t, err)
	require.NoError(t, engine.Close())
	require.NoError(t, engine.Close())
}

func TestEngine_NewEngine_OptionError(t *testing.T) {
	opt := func(e *Engine) error { return errors.New("option error") }
	engine, err := newEngine(opt)
	require.Error(t, err)
	require.Nil(t, engine)
}

func TestEngine_Execute_EvalRpcScriptException(t *testing.T) {
	engine, err := newEngine()
	require.NoError(t, err)
	defer engine.Close()

	// Set an invalid RPC script to trigger Eval exception
	engine.RpcScript = "throw new Error('bad rpc script');"
	scripts := []*jsexecutor.InitScript{}
	require.NoError(t, engine.Init(scripts))

	req := &jsexecutor.JsRequest{
		Id:      "1",
		Service: "add",
		Args:    []interface{}{1, 2},
	}
	_, err = engine.Execute(req)
	require.Error(t, err)
	require.Contains(t, err.Error(), "failed to evaluate RPC script")
}

func TestEngine_Execute_MarshalRequestError(t *testing.T) {
	engine, err := newEngine()
	require.NoError(t, err)
	defer engine.Close()

	engine.RpcScript = "function rpc(req) { return req; }"

	type Unmarshalable struct {
		Ch chan int // Exported field
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

func TestEngine_Execute_UnmarshalResponseError(t *testing.T) {
	engine, err := newEngine()
	require.NoError(t, err)
	defer engine.Close()

	engine.RpcScript = `(req) => Symbol("x")`

	scripts := []*jsexecutor.InitScript{}
	require.NoError(t, engine.Init(scripts))

	req := &jsexecutor.JsRequest{
		Id:      "1",
		Service: "add",
		Args:    []interface{}{1, 2},
	}
	_, err = engine.Execute(req)
	require.Error(t, err)
	require.Contains(t, err.Error(), "failed to unmarshal response")
}
