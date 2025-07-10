// Copyright 2025 Brian Wang <wangbuke@gmail.com>
// SPDX-License-Identifier: Apache-2.0

package gojaengine

import (
	"testing"

	jsexecutor "github.com/buke/js-executor"
	"github.com/stretchr/testify/require"
)

// TestIntegration_GojaExecutor_Async tests an async JS function.
func TestIntegration_GojaExecutor_Async(t *testing.T) {
	// Prepare an initialization script that defines an async JS function.
	// Using an async function with a timeout ensures we test the event loop.
	initScript := &jsexecutor.InitScript{
		FileName: "hello_async.js",
		Content: `
            async function hello(name) {
                // Wait for 10ms to ensure the async path is taken.
                await new Promise(resolve => setTimeout(resolve, 10));
                return "Hello, " + name + "!";
            }
        `,
	}

	// Create a JsExecutor with the Goja engine factory and options.
	executor, err := jsexecutor.NewExecutor(
		jsexecutor.WithJsEngine(NewFactory()),
		jsexecutor.WithInitScripts(initScript),
	)
	require.NoError(t, err)
	require.NotNil(t, executor)

	// Start the executor's thread pool.
	err = executor.Start()
	require.NoError(t, err)

	// Prepare a JS request to call the async hello function.
	req := &jsexecutor.JsRequest{
		Id:      "async-1",
		Service: "hello",
		Args:    []interface{}{"Goja Async"},
	}

	// Execute the JS request and verify the result.
	resp, err := executor.Execute(req)
	require.NoError(t, err)
	require.NotNil(t, resp)
	require.Equal(t, "Hello, Goja Async!", resp.Result)

	// Stop the executor.
	err = executor.Stop()
	require.NoError(t, err)
}

// TestIntegration_GojaExecutor_Sync tests a synchronous JS function.
func TestIntegration_GojaExecutor_Sync(t *testing.T) {
	// Prepare an initialization script that defines a simple synchronous function.
	initScript := &jsexecutor.InitScript{
		FileName: "hello_sync.js",
		Content: `
            function hello(name) {
                return "Hi, " + name + "!";
            }
        `,
	}

	// Create a JsExecutor with the Goja engine factory and options.
	executor, err := jsexecutor.NewExecutor(
		jsexecutor.WithJsEngine(NewFactory()),
		jsexecutor.WithInitScripts(initScript),
	)
	require.NoError(t, err)
	require.NotNil(t, executor)

	// Start the executor's thread pool.
	err = executor.Start()
	require.NoError(t, err)

	// Prepare a JS request to call the sync hello function.
	req := &jsexecutor.JsRequest{
		Id:      "sync-1",
		Service: "hello",
		Args:    []interface{}{"Goja Sync"},
	}

	// Execute the JS request and verify the result.
	resp, err := executor.Execute(req)
	require.NoError(t, err)
	require.NotNil(t, resp)
	require.Equal(t, "Hi, Goja Sync!", resp.Result)

	// Stop the executor.
	err = executor.Stop()
	require.NoError(t, err)
}
