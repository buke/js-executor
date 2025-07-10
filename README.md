# js-executor
English | [简体中文](README_zh-cn.md)

[![Test](https://github.com/buke/js-executor/workflows/Test/badge.svg)](https://github.com/buke/js-executor/actions?query=workflow%3ATest)
[![codecov](https://codecov.io/gh/buke/js-executor/graph/badge.svg?token=dEKb74zdFq)](https://codecov.io/gh/buke/js-executor)
[![Go Report Card](https://goreportcard.com/badge/github.com/buke/js-executor)](https://goreportcard.com/report/github.com/buke/js-executor)
[![GoDoc](https://pkg.go.dev/badge/github.com/buke/js-executor?status.svg)](https://pkg.go.dev/github.com/buke/js-executor?tab=doc)


JavaScript execution thread pool for Go, with built-in support for QuickJS and Goja engines.

## Overview

**js-executor** is a high-performance, flexible JavaScript execution pool for Go.  
It provides a thread pool model for executing JavaScript code in parallel, running each engine instance in a native OS thread.  
It supports pluggable engine backends (such as QuickJS and Goja), initialization scripts, context passing, and robust resource management.

## Supported Engines

| Engine   | Repository                                                       | Notes                               |
|----------|------------------------------------------------------------------|-------------------------------------|
| QuickJS  | [github.com/buke/quickjs-go](https://github.com/buke/quickjs-go) | CGo-based, high performance.        |
| Goja     | [github.com/dop251/goja](https://github.com/dop251/goja)         | Pure Go, no CGo dependency.         |

### Benchmark
```shell
$ go test -run=^$ -bench=. -benchmem

goos: darwin
goarch: arm64
pkg: github.com/buke/js-executor
cpu: Apple M4
BenchmarkExecutor_QuickJS-10               26292             44961 ns/op            1092 B/op         46 allocs/op
BenchmarkExecutor_Goja-10                  12428             99048 ns/op           50058 B/op        720 allocs/op
PASS
ok      github.com/buke/js-executor     4.055s
```

**Analysis:**

*   **Performance (`ns/op`)**: QuickJS is approximately **2.2x faster** in this high-concurrency, CPU-bound test(Fibonacci). Its low-level C implementation and minimal pressure on the Go garbage collector (GC) allow it to excel under heavy load.
*   **Memory (`B/op`, `allocs/op`)**: The memory statistics highlight a crucial difference.
    *   **Goja**: As a pure Go engine, its memory usage is fully tracked by Go's tools. The higher numbers reflect the total cost of JS execution within the Go runtime, which can lead to increased GC pressure.
    *   **QuickJS**: The reported numbers **only show the Go-side allocation overhead**. The memory used by the C-based QuickJS engine itself is **not measured** by Go's benchmark tool. This results in extremely low GC pressure on the Go application, which is a key reason for its high performance.

## Features

- **Thread Pool Model**: Efficiently handles multiple JavaScript tasks in parallel using native threads.
- **Pluggable Engine Support**: Easily integrates with different JavaScript engines (e.g., QuickJS, Goja).
- **Initialization Scripts**: Supports loading and reloading of initialization scripts for all threads.
- **Context Passing**: Allows passing custom context data with each request and response.
- **Resource Management**: Automatic thread lifecycle management, including idle timeout and max execution limits.
- **Timeout and Limits**: Configurable execution timeout, memory limit, stack size, and more.

## Usage Example

The following example demonstrates how to use the **QuickJS** engine.

```go
import (
    "fmt"
    jsexecutor "github.com/buke/js-executor"
    quickjsengine "github.com/buke/js-executor/engines/quickjs-go"
)

func main() {
    // Prepare an initialization script
    initScript := &jsexecutor.InitScript{
        FileName: "hello.js",
        Content:  `function hello(name) { return "Hello, " + name + "!"; }`,
    }

    // Create a new executor with QuickJS engine
    executor, err := jsexecutor.NewExecutor(
        jsexecutor.WithJsEngine(quickjsengine.NewFactory()),
        jsexecutor.WithInitScripts(initScript),
    )
    if err != nil {
        panic(err)
    }
    defer executor.Stop()

    // Start the executor
    if err := executor.Start(); err != nil {
        panic(err)
    }

    // Execute a JS function
    req := &jsexecutor.JsRequest{
        Id:      "1",
        Service: "hello",
        Args:    []interface{}{"World"},
    }
    resp, err := executor.Execute(req)
    if err != nil {
        panic(err)
    }
    fmt.Println(resp.Result) // Output: Hello, World!
}
```

## Configuration

- **Pool Size**: Set minimum and maximum thread pool size.
- **Queue Size**: Set per-thread task queue size.
- **Timeouts**: Configure execution timeout, thread idle timeout, and max executions per thread.
- **Engine Options**: Configure memory limit, stack size, GC threshold, module import, etc.

## Quick Start

1. Install dependencies:
    ```sh
    go get github.com/buke/js-executor
    go get github.com/buke/js-executor/engines/quickjs-go
    ```

2. See [example_test.go](./example_test.go) for more usage examples.

## License

[Apache-2.0](LICENSE)