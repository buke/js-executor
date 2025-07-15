package jsexecutor_test

import (
	"fmt"

	jsexecutor "github.com/buke/js-executor"
	quickjsengine "github.com/buke/js-executor/engines/quickjs-go"
)

// Example demonstrates how to use JsExecutor with QuickJS engine.
// It shows how to initialize the executor, start it, execute a simple JS function, and stop the executor.
func Example() {
	// Prepare a simple JS function as an initialization script
	jsScript := &jsexecutor.JsScript{
		FileName: "hello.js",
		Content:  `function hello(name) { return "Hello, " + name + "!"; }`,
	}

	// Create a QuickJS engine builder and a new executor with the jsScript
	executor, err := jsexecutor.NewExecutor(
		jsexecutor.WithJsEngine(quickjsengine.NewFactory(
			quickjsengine.WithEnableModuleImport(true),
			quickjsengine.WithCanBlock(true),
		)),
		jsexecutor.WithJsScripts(jsScript),
	)
	if err != nil {
		fmt.Printf("Failed to create executor: %v\n", err)
		return
	}

	// Start the executor
	if err := executor.Start(); err != nil {
		fmt.Printf("Failed to start executor: %v\n", err)
		return
	}

	// Prepare a JS request to call the hello function
	req := &jsexecutor.JsRequest{
		Id:      "1",
		Service: "hello",
		Args:    []interface{}{"World"},
	}

	// Execute the JS request
	resp, err := executor.Execute(req)
	if err != nil {
		fmt.Printf("Execution error: %v\n", err)
		return
	}
	fmt.Printf("Result: %v\n", resp.Result)

	// Stop the executor
	if err := executor.Stop(); err != nil {
		fmt.Printf("Failed to stop executor: %v\n", err)
		return
	}

	// Output:
	// Result: Hello, World!
}
