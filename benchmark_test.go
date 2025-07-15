//go:build !windows

package jsexecutor_test

import (
	"testing"

	jsexecutor "github.com/buke/js-executor"
	gojaengine "github.com/buke/js-executor/engines/goja"
	quickjsengine "github.com/buke/js-executor/engines/quickjs-go"
	v8engine "github.com/buke/js-executor/engines/v8go"
)

// A simple CPU-intensive script for benchmarking.
// The Fibonacci function is a good candidate as it's pure computation.
const benchmarkJsScript = `
function fib(n) {
    if (n < 2) {
        return n;
    }
    return fib(n - 1) + fib(n - 2);
}
`

// runExecutorBenchmark is a helper function to run a benchmark test for a given engine factory.
func runExecutorBenchmark(b *testing.B, factory jsexecutor.JsEngineFactory) {
	jsScript := &jsexecutor.JsScript{
		FileName: "benchmark.js",
		Content:  benchmarkJsScript,
	}

	// Create a new executor with the specified engine
	executor, err := jsexecutor.NewExecutor(
		jsexecutor.WithJsEngine(factory),
		jsexecutor.WithJsScripts(jsScript),
		jsexecutor.WithMinPoolSize(16), // Use a fixed pool size for stable benchmark results
		jsexecutor.WithMaxPoolSize(16), // by setting min and max to the same value.
	)
	if err != nil {
		b.Fatalf("Failed to create executor: %v", err)
	}

	if err := executor.Start(); err != nil {
		b.Fatalf("Failed to start executor: %v", err)
	}
	defer executor.Stop()

	b.ResetTimer() // Start timing after setup

	// Run the benchmark in parallel to test the pool's concurrency
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			req := &jsexecutor.JsRequest{
				Service: "fib",
				Args:    []interface{}{15}, // A number that takes some time but not too long
			}
			_, err := executor.Execute(req)
			if err != nil {
				// In a benchmark, we often stop on error to ensure we're measuring success cases.
				b.Errorf("Execute failed: %v", err)
			}
		}
	})
}

// BenchmarkExecutor_Goja benchmarks the executor with the Goja engine.
func BenchmarkExecutor_Goja(b *testing.B) {
	runExecutorBenchmark(b, gojaengine.NewFactory())
}

// BenchmarkExecutor_QuickJS benchmarks the executor with the QuickJS engine.
func BenchmarkExecutor_QuickJS(b *testing.B) {
	runExecutorBenchmark(b, quickjsengine.NewFactory())
}

// BenchmarkExecutor_V8Go benchmarks the executor with the V8 engine.
func BenchmarkExecutor_V8Go(b *testing.B) {
	runExecutorBenchmark(b, v8engine.NewFactory())
}
