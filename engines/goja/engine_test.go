// Copyright 2025 Brian Wang <wangbuke@gmail.com>
// SPDX-License-Identifier: Apache-2.0

package gojaengine

import (
	"fmt"
	"testing"

	jsexecutor "github.com/buke/js-executor"
	"github.com/dop251/goja"
	"github.com/stretchr/testify/require"
)

// Helper to temporarily replace the package-level rpcScript for a test.
func withMockRpcScript(script string, t *testing.T, testFunc func(t *testing.T)) {
	originalScript := rpcScript
	rpcScript = script
	defer func() {
		rpcScript = originalScript
	}()
	testFunc(t)
}

func TestNewFactory(t *testing.T) {
	factory := NewFactory()
	require.NotNil(t, factory)

	engine, err := factory()
	require.NoError(t, err)
	require.NotNil(t, engine)
	defer engine.Close()

	_, ok := engine.(*Engine)
	require.True(t, ok)
}

func TestNewFactory_OptionError(t *testing.T) {
	errorOption := func(engine jsexecutor.JsEngine) error {
		return fmt.Errorf("a deliberate config error")
	}
	factory := NewFactory(errorOption)
	_, err := factory()
	require.Error(t, err)
	require.Contains(t, err.Error(), "a deliberate config error")
}

func TestEngine_Init(t *testing.T) {
	engine, err := NewFactory()()
	require.NoError(t, err)
	require.NotNil(t, engine)
	defer engine.Close()

	jsScript := &jsexecutor.JsScript{
		FileName: "test.js",
		Content:  "var a = 10;",
	}
	err = engine.Load([]*jsexecutor.JsScript{jsScript})
	require.NoError(t, err)

	gojaEngine := engine.(*Engine)
	done := make(chan goja.Value, 1)
	gojaEngine.Loop.RunOnLoop(func(vm *goja.Runtime) {
		done <- vm.Get("a")
	})
	result := <-done
	require.Equal(t, int64(10), result.Export())
}

func TestEngine_Init_Error(t *testing.T) {
	engine, err := NewFactory()()
	require.NoError(t, err)
	require.NotNil(t, engine)
	defer engine.Close()

	jsScript := &jsexecutor.JsScript{
		FileName: "error.js",
		Content:  "var a =;", // Syntax error
	}
	err = engine.Load([]*jsexecutor.JsScript{jsScript})
	require.Error(t, err)
	require.Contains(t, err.Error(), "failed to execute init script")
}

func TestEngine_Reload(t *testing.T) {
	engine, err := NewFactory()()
	require.NoError(t, err)
	require.NotNil(t, engine)
	defer engine.Close()

	jsScript1 := &jsexecutor.JsScript{FileName: "v1.js", Content: "var version = 1;"}
	err = engine.Load([]*jsexecutor.JsScript{jsScript1})
	require.NoError(t, err)

	jsScript2 := &jsexecutor.JsScript{FileName: "v2.js", Content: "var version = 2;"}
	err = engine.Reload([]*jsexecutor.JsScript{jsScript2})
	require.NoError(t, err)

	gojaEngine := engine.(*Engine)
	done := make(chan goja.Value, 1)
	gojaEngine.Loop.RunOnLoop(func(vm *goja.Runtime) {
		done <- vm.Get("version")
	})
	result := <-done
	require.Equal(t, int64(2), result.Export())
}

func TestEngine_Reload_Error(t *testing.T) {
	errorOption := func(engine jsexecutor.JsEngine) error { return fmt.Errorf("reload config error") }
	engine, err := NewFactory(errorOption)()
	require.Error(t, err) // The initial creation fails
	require.Nil(t, engine)

	// Test reload on a valid engine, but with faulty original options
	engine, err = NewFactory()()
	require.NoError(t, err)
	engine.(*Engine).opts = []jsexecutor.JsEngineOption{errorOption} // Manually set faulty original options

	err = engine.Reload(nil)
	require.Error(t, err)
	require.Contains(t, err.Error(), "reload config error")
}

func TestEngine_Execute_NilRequest(t *testing.T) {
	engine, err := NewFactory()()
	require.NoError(t, err)
	defer engine.Close()

	_, err = engine.Execute(nil)
	require.Error(t, err)
	require.Equal(t, "request cannot be nil", err.Error())
}

func TestEngine_Execute_RejectedPromise(t *testing.T) {
	engine, err := NewFactory()()
	require.NoError(t, err)
	defer engine.Close()

	jsScript := &jsexecutor.JsScript{
		FileName: "reject.js",
		Content:  "function fail() { return Promise.reject('a serious error'); }",
	}
	err = engine.Load([]*jsexecutor.JsScript{jsScript})
	require.NoError(t, err)

	req := &jsexecutor.JsRequest{Service: "fail"}
	_, err = engine.Execute(req)
	require.Error(t, err)
	require.Contains(t, err.Error(), "js execution error: a serious error")
}

func TestEngine_Execute_NonPromiseReturn(t *testing.T) {
	engine, err := NewFactory()()
	require.NoError(t, err)
	defer engine.Close()

	jsScript := &jsexecutor.JsScript{
		FileName: "non_promise.js",
		Content:  "function nonPromise() { return { message: 'not a promise' }; }",
	}
	err = engine.Load([]*jsexecutor.JsScript{jsScript})
	require.NoError(t, err)

	req := &jsexecutor.JsRequest{Service: "nonPromise"}

	resp, err := engine.Execute(req)
	require.NoError(t, err)
	require.NotNil(t, resp)

	resultMap, ok := resp.Result.(map[string]interface{})
	require.True(t, ok)
	require.Equal(t, "not a promise", resultMap["message"])
}

func TestEngine_Execute_RpcScriptLoadError(t *testing.T) {
	withMockRpcScript("this is invalid syntax", t, func(t *testing.T) {
		engine, err := NewFactory()()
		require.NoError(t, err)
		defer engine.Close()

		_, err = engine.Execute(&jsexecutor.JsRequest{})
		require.Error(t, err)
		require.Contains(t, err.Error(), "failed to load rpc script")
	})
}

func TestEngine_Execute_RpcScriptNotAFunction(t *testing.T) {
	withMockRpcScript("42", t, func(t *testing.T) {
		engine, err := NewFactory()()
		require.NoError(t, err)
		defer engine.Close()

		_, err = engine.Execute(&jsexecutor.JsRequest{})
		require.Error(t, err)
		require.Contains(t, err.Error(), "rpc script did not return a function")
	})
}

func TestEngine_Execute_RpcCallError(t *testing.T) {
	withMockRpcScript("() => { throw new Error('rpc call failed'); }", t, func(t *testing.T) {
		engine, err := NewFactory()()
		require.NoError(t, err)
		defer engine.Close()

		_, err = engine.Execute(&jsexecutor.JsRequest{})
		require.Error(t, err)
		require.Contains(t, err.Error(), "failed to call rpc function")
	})
}

func TestEngine_Execute_RpcReturnsNilObject(t *testing.T) {
	withMockRpcScript("() => null", t, func(t *testing.T) {
		engine, err := NewFactory()()
		require.NoError(t, err)
		defer engine.Close()

		_, err = engine.Execute(&jsexecutor.JsRequest{})
		require.Error(t, err)
		require.Contains(t, err.Error(), "rpc call did not return a promise-like object")
	})
}

func TestEngine_Execute_RpcReturnsNonPromiseObject(t *testing.T) {
	withMockRpcScript("() => ({})", t, func(t *testing.T) {
		engine, err := NewFactory()()
		require.NoError(t, err)
		defer engine.Close()

		_, err = engine.Execute(&jsexecutor.JsRequest{})
		require.Error(t, err)
		require.Contains(t, err.Error(), "rpc call did not return a promise (missing .then method)")
	})
}

func TestEngine_Execute_OnSuccessExportError(t *testing.T) {
	// This script returns a Promise that resolves with a value that cannot be exported to JsResponse.
	// A JS Symbol cannot be exported to a Go struct pointer.
	withMockRpcScript("() => Promise.resolve(Symbol('unexportable'))", t, func(t *testing.T) {
		engine, err := NewFactory()()
		require.NoError(t, err)
		defer engine.Close()

		_, err = engine.Execute(&jsexecutor.JsRequest{})
		require.Error(t, err)
		require.Contains(t, err.Error(), "failed to export result")
	})
}

func TestEngine_Execute_InvokeThenError(t *testing.T) {
	// This script returns a "thenable" object whose .then method throws an error when called.
	// This covers the final error branch in the Execute method.
	mockScript := `() => ({
        then: function(onSuccess, onError) {
            throw new Error('I am a broken .then method');
        }
    })`
	withMockRpcScript(mockScript, t, func(t *testing.T) {
		engine, err := NewFactory()()
		require.NoError(t, err)
		defer engine.Close()

		_, err = engine.Execute(&jsexecutor.JsRequest{})
		require.Error(t, err)
		require.Contains(t, err.Error(), "failed to invoke promise.then")
	})
}

func TestEngine_Execute_Sync(t *testing.T) {
	engine, err := NewFactory()()
	require.NoError(t, err)
	require.NotNil(t, engine)
	defer engine.Close()

	jsScript := &jsexecutor.JsScript{
		FileName: "sync.js",
		Content:  "function hello(name) { return 'Hello, ' + name; }",
	}
	err = engine.Load([]*jsexecutor.JsScript{jsScript})
	require.NoError(t, err)

	req := &jsexecutor.JsRequest{
		Id:      "req-sync",
		Service: "hello",
		Args:    []interface{}{"World"},
	}

	resp, err := engine.Execute(req)
	require.NoError(t, err)
	require.NotNil(t, resp)
	require.Equal(t, "Hello, World", resp.Result)
	require.Equal(t, "req-sync", resp.Id)
}

func TestEngine_Execute_Async(t *testing.T) {
	engine, err := NewFactory()()
	require.NoError(t, err)
	require.NotNil(t, engine)
	defer engine.Close()

	jsScript := &jsexecutor.JsScript{
		FileName: "async.js",
		Content:  "async function hello(name) { return Promise.resolve('Hello, ' + name); }",
	}
	err = engine.Load([]*jsexecutor.JsScript{jsScript})
	require.NoError(t, err)

	req := &jsexecutor.JsRequest{
		Id:      "req-async",
		Service: "hello",
		Args:    []interface{}{"Async World"},
	}

	resp, err := engine.Execute(req)
	require.NoError(t, err)
	require.NotNil(t, resp)
	require.Equal(t, "Hello, Async World", resp.Result)
	require.Equal(t, "req-async", resp.Id)
}
