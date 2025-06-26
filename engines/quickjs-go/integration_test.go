package quickjsengine

import (
	"testing"

	jsexecutor "github.com/buke/js-executor"
	"github.com/stretchr/testify/require"
)

// Integration test: create a JsExecutor with QuickJS engine and run a full JS workflow.
func TestIntegration_QuickJSExecutor(t *testing.T) {
	// Prepare an init script that defines a JS function
	initScript := &jsexecutor.InitScript{
		FileName: "hello.js",
		Content:  `function hello(name) { return "Hello, " + name + "!"; }`,
	}

	// Create a JsExecutor with QuickJS engine and the init script
	executor, err := jsexecutor.NewExecutor(
		jsexecutor.WithJsEngine(NewFactory(
			WithEnableModuleImport(true),
			WithCanBlock(true),
		)),
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

	// Execute the JS request
	resp, err := executor.Execute(req)
	require.NoError(t, err)
	require.NotNil(t, resp)
	require.Equal(t, "Hello, Integration!", resp.Result)

	// Stop the executor
	err = executor.Stop()
	require.NoError(t, err)
}
