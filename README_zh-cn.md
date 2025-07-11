# js-executor
[English](README.md) | 简体中文

[![Test](https://github.com/buke/js-executor/workflows/Test/badge.svg)](https://github.com/buke/js-executor/actions?query=workflow%3ATest)
[![codecov](https://codecov.io/gh/buke/js-executor/graph/badge.svg?token=dEKb74zdFq)](https://codecov.io/gh/buke/js-executor)
[![Go Report Card](https://goreportcard.com/badge/github.com/buke/js-executor)](https://goreportcard.com/report/github.com/buke/js-executor)
[![GoDoc](https://pkg.go.dev/badge/github.com/buke/js-executor?status.svg)](https://pkg.go.dev/github.com/buke/js-executor?tab=doc)

Go 语言 JavaScript 执行池，内置支持 QuickJS / Goja / V8 引擎。

## 概述

**js-executor** 是一个高性能、灵活的 Go 语言 JavaScript 执行池。  
它通过原生操作系统线程池模型并行执行 JavaScript 代码，每个引擎实例运行在独立的原生线程中。  
支持可插拔的引擎后端（如 QuickJS、Goja、V8）、初始化脚本、上下文传递和强大的资源管理。

## 功能特性

- **线程池模型**：基于原生线程池高效并行处理多任务 JavaScript 执行。
- **内置支持 QuickJS / Goja / V8 引擎**。
- **可插拔引擎**：可灵活集成不同的 JavaScript 引擎（如 QuickJS、Goja、V8）。
- **初始化脚本**：支持为所有线程加载和热重载初始化脚本。
- **上下文传递**：每次请求和响应都可传递自定义上下文数据。
- **资源管理**：自动线程生命周期管理，包括空闲超时和最大执行次数。
- **超时与限制**：可配置执行超时、内存限制、栈大小等参数。

## 使用示例

```go
import (
    "fmt"
    jsexecutor "github.com/buke/js-executor"
    quickjsengine "github.com/buke/js-executor/engines/quickjs-go"
)

func main() {
    // 准备初始化脚本
    initScript := &jsexecutor.InitScript{
        FileName: "hello.js",
        Content:  `function hello(name) { return "Hello, " + name + "!"; }`,
    }

    // 创建带 QuickJS 引擎的执行器
    executor, err := jsexecutor.NewExecutor(
        jsexecutor.WithJsEngine(quickjsengine.NewFactory()),
        jsexecutor.WithInitScripts(initScript),
    )
    if err != nil {
        panic(err)
    }
    defer executor.Stop()

    // 启动执行器
    if err := executor.Start(); err != nil {
        panic(err)
    }

    // 执行 JS 函数
    req := &jsexecutor.JsRequest{
        Id:      "1",
        Service: "hello",
        Args:    []interface{}{"世界"},
    }
    resp, err := executor.Execute(req)
    if err != nil {
        panic(err)
    }
    fmt.Println(resp.Result) // 输出: Hello, 世界!
}
```

## 配置项

- **线程池大小**：设置线程池的最小和最大线程数。
- **队列大小**：设置每个线程的任务队列长度。
- **超时设置**：配置执行超时、线程空闲超时、每线程最大执行次数等。
- **引擎参数**：配置内存限制、栈大小、GC 阈值、模块导入等。

## 快速开始

1. 安装依赖：
    ```sh
    go get github.com/buke/js-executor
    go get github.com/buke/js-executor/engines/quickjs-go
    ```

2. 更多用法请参考 [example_test.go](./example_test.go)。

## 支持的引擎

| 引擎    | 仓库地址                                                           | 说明                                         |
|---------|--------------------------------------------------------------------|----------------------------------------------|
| QuickJS | [github.com/buke/quickjs-go](https://github.com/buke/quickjs-go)   | 基于 CGo，性能高。                           |
| Goja    | [github.com/dop251/goja](https://github.com/dop251/goja)           | 纯 Go 实现，无 CGo 依赖。                    |
| V8Go    | [github.com/tommie/v8go](https://github.com/tommie/v8go)           | 基于 CGo，V8 引擎，最快。不支持 Windows。    |

### 基准测试
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

**结果分析：**

*   **性能 (`ns/op`)**:  
    - **Goja** 作为基准（**1x**）。
    - **QuickJS** 约为 Goja 的 **2 倍**速度。
    - **V8Go** 约为 Goja 的 **10 倍**速度，约为 QuickJS 的 **5 倍**速度（高并发、CPU 密集型场景）。
*   **内存 (`B/op`, `allocs/op`)**:  
    - **V8Go** 和 **QuickJS** 的 Go 侧内存分配和分配次数都非常低。
    - **Goja**（纯 Go）内存分配和分配次数明显更高，因为所有 JS 内存都由 Go 运行时和 GC 管理。
    - 注意：CGo 引擎（V8Go、QuickJS）Go 基准测试只统计 Go 侧分配，原生引擎内存未统计。
*   **平台**:  
    - **V8Go** 不支持 Windows。

## 许可

[Apache-2.0](LICENSE)