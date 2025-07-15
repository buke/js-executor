// Copyright 2025 Brian Wang <wangbuke@gmail.com>
// SPDX-License-Identifier: Apache-2.0

package quickjsengine

import (
	"fmt"

	jsexecutor "github.com/buke/js-executor"
)

// EngineOption holds configuration options for a QuickJS engine instance.
type EngineOption struct {
	Timeout            uint64 `json:"timeout"`            // Script execution timeout in seconds (0 = no timeout)
	MemoryLimit        uint64 `json:"memoryLimit"`        // Memory limit in bytes (0 = no limit)
	GCThreshold        int64  `json:"gcThreshold"`        // GC threshold in bytes (-1 = disable, 0 = default)
	MaxStackSize       uint64 `json:"maxStackSize"`       // Stack size in bytes (0 = default)
	CanBlock           bool   `json:"canBlock"`           // Whether the runtime can block (for async operations)
	EnableModuleImport bool   `json:"enableModuleImport"` // Enable ES6 module import support
	Strip              int    `json:"strip"`              // Strip level for bytecode compilation
}

// Option is a function that applies a configuration to an Engine instance.
// type Option func(*Engine) error

// WithGCThreshold sets the garbage collection threshold for the engine.
// Use -1 to disable automatic GC, 0 for default, or a positive value for a custom threshold.
func WithGCThreshold(threshold int64) jsexecutor.JsEngineOption {
	return func(engine jsexecutor.JsEngine) error {
		e := engine.(*Engine)
		if threshold < -1 {
			return fmt.Errorf("invalid GC threshold: %d", threshold)
		}
		e.Option.GCThreshold = threshold
		e.Runtime.SetGCThreshold(threshold)
		return nil
	}
}

// WithMemoryLimit sets the memory limit for the JavaScript runtime in bytes.
// If limit is 0, there is no memory limit.
func WithMemoryLimit(limit uint64) jsexecutor.JsEngineOption {
	return func(engine jsexecutor.JsEngine) error {
		e := engine.(*Engine)
		// 0 means no limit, so it's allowed
		e.Option.MemoryLimit = limit
		e.Runtime.SetMemoryLimit(limit)
		return nil
	}
}

// WithTimeout sets the script execution timeout in seconds.
// If timeout is 0, there is no timeout.
func WithTimeout(timeout uint64) jsexecutor.JsEngineOption {
	return func(engine jsexecutor.JsEngine) error {
		e := engine.(*Engine)
		// 0 means no timeout, so it's allowed
		e.Option.Timeout = timeout
		e.Runtime.SetExecuteTimeout(timeout)
		return nil
	}
}

// WithMaxStackSize sets the stack size for the JavaScript runtime in bytes.
// If size is 0, the default stack size is used.
func WithMaxStackSize(size uint64) jsexecutor.JsEngineOption {
	return func(engine jsexecutor.JsEngine) error {
		e := engine.(*Engine)
		// 0 means use default stack size, so it's allowed
		e.Option.MaxStackSize = size
		e.Runtime.SetMaxStackSize(size)
		return nil
	}
}

// WithCanBlock enables or disables blocking operations in the runtime.
// Enabling this may affect performance and security.
func WithCanBlock(canBlock bool) jsexecutor.JsEngineOption {
	return func(engine jsexecutor.JsEngine) error {
		e := engine.(*Engine)
		e.Option.CanBlock = canBlock
		e.Runtime.SetCanBlock(canBlock)
		return nil
	}
}

// WithEnableModuleImport enables or disables ES6 module import support.
// Enabling this may have security implications.
func WithEnableModuleImport(enable bool) jsexecutor.JsEngineOption {
	return func(engine jsexecutor.JsEngine) error {
		e := engine.(*Engine)
		e.Option.EnableModuleImport = enable
		e.Runtime.SetModuleImport(enable)
		return nil
	}
}

// WithStrip sets the strip level for bytecode compilation.
// 0 = no stripping, higher values strip more debug information.
func WithStrip(strip int) jsexecutor.JsEngineOption {
	return func(engine jsexecutor.JsEngine) error {
		e := engine.(*Engine)
		if strip < 0 || strip > 2 {
			return fmt.Errorf("invalid strip level: %d", strip)
		}
		e.Option.Strip = strip
		e.Runtime.SetStripInfo(strip)
		return nil
	}
}

// WithRpcScript sets the RPC script for the engine.
// The script must not be empty.
func WithRpcScript(script string) jsexecutor.JsEngineOption {
	return func(engine jsexecutor.JsEngine) error {
		e := engine.(*Engine)
		if script == "" {
			return fmt.Errorf("rpc script cannot be empty")
		}
		e.RpcScript = script
		return nil
	}
}
