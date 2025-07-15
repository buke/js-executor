//go:build !windows

// Copyright 2025 Brian Wang <wangbuke@gmail.com>
// SPDX-License-Identifier: Apache-2.0

package v8engine

import (
	"fmt"

	jsexecutor "github.com/buke/js-executor"
)

// EngineOption holds specific configurations for the V8 engine.
type EngineOption struct{}

// WithRpcScript provides an option to override the default RPC handling script.
// This allows for custom RPC logic or extending the engine's capabilities.
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
