# js-executor
English | [简体中文](README_zh-cn.md)

[![Test](https://github.com/buke/js-executor/workflows/Test/badge.svg)](https://github.com/buke/js-executor/actions?query=workflow%3ATest)
[![codecov](https://codecov.io/gh/buke/js-executor/graph/badge.svg?token=dEKb74zdFq)](https://codecov.io/gh/buke/js-executor)
[![Go Report Card](https://goreportcard.com/badge/github.com/buke/js-executor)](https://goreportcard.com/report/github.com/buke/js-executor)
[![GoDoc](https://pkg.go.dev/badge/github.com/buke/js-executor?status.svg)](https://pkg.go.dev/github.com/buke/js-executor?tab=doc)

JavaScript execution thread pool for Go, with built-in support for QuickJS / Goja / V8 engines.

## Overview

**js-executor**  provides a thread pool model for executing JavaScript code in parallel, running each engine instance in a native OS thread, and supports pluggable engine backends (such as QuickJS, Goja, and V8), initialization scripts, context passing, and robust resource management.

## Features

- **Thread Pool Model**: Efficiently handles multiple JavaScript tasks in parallel using native threads.
- **Built-in support for QuickJS / Goja / V8 engines**.
- **Pluggable Engine Support**: Easily integrates with different JavaScript engines (e.g., QuickJS, Goja, V8).
- **Initialization Scripts**: Supports loading and reloading of initialization scripts for all threads.
- **Context Passing**: Allows passing custom context data with each request and response.
- **Resource Management**: Automatic thread lifecycle management, including idle timeout and max execution limits.
- **Timeout and Limits**: Configurable execution timeout, memory limit, stack size, and more.

## Usage Example

The following example demonstrates how to use the **QuickJS**, **Goja**, and **V8Go** engines.

### QuickJS Example

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

### Goja Example

```go
import (
    "fmt"
    jsexecutor "github.com/buke/js-executor"
    gojaengine "github.com/buke/js-executor/engines/goja"
)

func main() {
    // Prepare an initialization script
    initScript := &jsexecutor.InitScript{
        FileName: "hello.js",
        Content:  `function hello(name) { return "Hello, " + name + "!"; }`,
    }

    // Create a new executor with Goja engine
    executor, err := jsexecutor.NewExecutor(
        jsexecutor.WithJsEngine(gojaengine.NewFactory()),
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
        Id:      "goja-1",
        Service: "hello",
        Args:    []interface{}{"Goja"},
    }
    resp, err := executor.Execute(req)
    if err != nil {
        panic(err)
    }
    fmt.Println(resp.Result) // Output: Hello, Goja!
}
```

### V8Go Example

> **Note:** V8Go is not supported on Windows.

```go
//go:build !windows

import (
    "fmt"
    jsexecutor "github.com/buke/js-executor"
    v8engine "github.com/buke/js-executor/engines/v8go"
)

func main() {
    // Prepare an initialization script
    initScript := &jsexecutor.InitScript{
        FileName: "hello.js",
        Content:  `function hello(name) { return "Hello, " + name + "!"; }`,
    }

    // Create a new executor with V8Go engine
    executor, err := jsexecutor.NewExecutor(
        jsexecutor.WithJsEngine(v8engine.NewFactory()),
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
        Id:      "v8go-1",
        Service: "hello",
        Args:    []interface{}{"V8Go"},
    }
    resp, err := executor.Execute(req)
    if err != nil {
        panic(err)
    }
    fmt.Println(resp.Result) // Output: Hello, V8Go!
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


## Supported Engines

| Engine   | Repository                                                           | Notes                                         |
|----------|----------------------------------------------------------------------|-----------------------------------------------|
| QuickJS  | [github.com/buke/quickjs-go](https://github.com/buke/quickjs-go)     | CGo-based, high performance.                  |
| Goja     | [github.com/dop251/goja](https://github.com/dop251/goja)             | Pure Go, no CGo dependency.                   |
| V8Go     | [github.com/tommie/v8go](https://github.com/tommie/v8go)             | CGo-based, V8 engine, fastest. Not supported on Windows. |

### Benchmark
```shell
$ go test -run=^$ -bench=. -benchmem

goos: darwin
goarch: arm64
pkg: github.com/buke/js-executor
cpu: Apple M4
BenchmarkExecutor_Goja-10                  13106             92237 ns/op           52404 B/op        761 allocs/op
BenchmarkExecutor_QuickJS-10               26239             45663 ns/op            1092 B/op         46 allocs/op
BenchmarkExecutor_V8Go-10                 134680              8719 ns/op             986 B/op         31 allocs/op
PASS
ok      github.com/buke/js-executor     5.725s
```

**Analysis:**

*   **Performance (`ns/op`)**:  
    - **Goja** is the baseline (**1x**).
    - **QuickJS** is about **2x faster** than Goja.
    - **V8Go** is about **10x faster** than Goja, and about **5x faster** than QuickJS, in this high-concurrency, CPU-bound test (Fibonacci).
*   **Memory (`B/op`, `allocs/op`)**:  
    - **V8Go** and **QuickJS** have very low Go-side memory usage and allocations per operation.
    - **Goja** (pure Go) shows much higher memory usage and allocations, as all JS memory is tracked by Go's runtime and GC.
    - Note: For CGo engines (V8Go, QuickJS), the Go benchmark only measures Go-side allocations; native engine memory is not included.
*   **Platform**:  
    - **V8Go** is not supported on Windows.

## License

[Apache-2.0](LICENSE)