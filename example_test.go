package jsexecutor_test

import (
	"fmt"

	jsexecutor "github.com/buke/js-executor"
	quickjsengine "github.com/buke/js-executor/engines/quickjs-go"
)

func Example() {
	// Prepare a simple JS function as init script
	initScript := &jsexecutor.InitScript{
		FileName: "hello.js",
		Content:  `function hello(name) { return "Hello, " + name + "!"; }`,
	}

	// Create a QuickJS engine builder
	executor, err := jsexecutor.NewExecutor(
		jsexecutor.WithJsEngine(quickjsengine.NewFactory(
			quickjsengine.WithEnableModuleImport(true),
			quickjsengine.WithCanBlock(true),
		)),
		// jsexecutor.WithLogger(nil), // Use nil for no logging
		jsexecutor.WithInitScripts(initScript),
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
