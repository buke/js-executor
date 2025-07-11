//go:build !windows

package jsexecutor_test

import (
	"fmt"
	"sync"
	"testing"

	jsexecutor "github.com/buke/js-executor"
	v8engine "github.com/buke/js-executor/engines/v8go"
	"github.com/stretchr/testify/require"
)

// TestIntegration_ExecutorWithV8Go tests basic integration of JsExecutor with the V8Go engine.
func TestIntegration_ExecutorWithV8Go(t *testing.T) {
	initScript := &jsexecutor.InitScript{
		FileName: "hello.js",
		Content:  `function hello(name) { return "Hi, " + name + "!"; }`,
	}
	executor, err := jsexecutor.NewExecutor(
		jsexecutor.WithJsEngine(v8engine.NewFactory()),
		jsexecutor.WithInitScripts(initScript),
	)
	require.NoError(t, err)
	require.NotNil(t, executor)

	require.NoError(t, executor.Start())

	// Prepare a request to call the "hello" function
	req := &jsexecutor.JsRequest{
		Id:      "1",
		Service: "hello",
		Args:    []interface{}{"World"},
	}
	resp, err := executor.Execute(req)
	require.NoError(t, err)
	require.NotNil(t, resp)
	require.Equal(t, "Hi, World!", resp.Result)

	require.NoError(t, executor.Stop())
}

// TestIntegration_ExecutorWithV8Go_ConcurrentTasks tests concurrent task execution with V8Go engine.
func TestIntegration_ExecutorWithV8Go_ConcurrentTasks(t *testing.T) {
	initScript := &jsexecutor.InitScript{
		FileName: "hello.js",
		Content:  `function hello(name) { return "Hi, " + name + "!"; }`,
	}
	executor, err := jsexecutor.NewExecutor(
		jsexecutor.WithJsEngine(v8engine.NewFactory()),
		jsexecutor.WithInitScripts(initScript),
		jsexecutor.WithMinPoolSize(2),
		jsexecutor.WithMaxPoolSize(4),
		jsexecutor.WithQueueSize(2),
	)
	require.NoError(t, err)
	require.NotNil(t, executor)

	require.NoError(t, executor.Start())
	defer executor.Stop()

	const (
		goroutineCount    = 16
		tasksPerGoroutine = 256
		totalTasks        = goroutineCount * tasksPerGoroutine
	)
	results := make([]string, totalTasks)
	errs := make([]error, totalTasks)

	var wg sync.WaitGroup
	wg.Add(goroutineCount)
	for g := 0; g < goroutineCount; g++ {
		go func(gid int) {
			defer wg.Done()
			for i := 0; i < tasksPerGoroutine; i++ {
				idx := gid*tasksPerGoroutine + i
				req := &jsexecutor.JsRequest{
					Id:      fmt.Sprintf("%d", idx),
					Service: "hello",
					Args:    []interface{}{fmt.Sprintf("User%d", idx)},
				}
				resp, err := executor.Execute(req)
				if err == nil && resp != nil {
					results[idx] = fmt.Sprintf("%v", resp.Result)
				}
				errs[idx] = err
			}
		}(g)
	}
	wg.Wait()

	// Verify all results and errors
	for i := 0; i < totalTasks; i++ {
		require.NoError(t, errs[i], "task %d failed: %v", i, errs[i])
		require.Equal(t, fmt.Sprintf("Hi, User%d!", i), results[i])
	}
}
