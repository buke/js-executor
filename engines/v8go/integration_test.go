// Copyright 2025 Brian Wang <wangbuke@gmail.com>
// SPDX-License-Identifier: Apache-2.0

package v8engine

import (
	"testing"

	jsexecutor "github.com/buke/js-executor"
	"github.com/stretchr/testify/require"
)

// TestIntegration_V8GoExecutor performs an integration test for JsExecutor with the V8 engine.
// It verifies that the executor can be created, started, execute a JS function, and stopped successfully.
func TestIntegration_V8GoExecutor(t *testing.T) {
	// Prepare an initialization script that defines a JS function
	initScript := &jsexecutor.InitScript{
		FileName: "hello.js",
		Content:  `function hello(name) { return "Hello, " + name + "!"; }`,
	}

	// Create a JsExecutor with the V8 engine and the initialization script
	// Note: The V8 engine factory currently doesn't have specific options like QuickJS does.
	executor, err := jsexecutor.NewExecutor(
		jsexecutor.WithJsEngine(NewFactory()),
		jsexecutor.WithInitScripts(initScript),
	)
	require.NoError(t, err)
	require.NotNil(t, executor)

	// Start the executor
	err = executor.Start()
	require.NoError(t, err)

	// Prepare a JS request to call the hello function
	req := &jsexecutor.JsRequest{
		Id:      "1",
		Service: "hello",
		Args:    []interface{}{"Integration"},
	}

	// Execute the JS request and verify the result
	resp, err := executor.Execute(req)
	require.NoError(t, err)
	require.NotNil(t, resp)
	require.Equal(t, "Hello, Integration!", resp.Result)

	// Stop the executor
	err = executor.Stop()
	require.NoError(t, err)
}
