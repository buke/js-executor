//go:build !windows

// Copyright 2025 Brian Wang <wangbuke@gmail.com>
// SPDX-License-Identifier: Apache-2.0

package v8engine

// EngineOption holds specific configurations for the V8 engine.
type EngineOption struct {
	// RpcScript allows overriding the default embedded RPC script.
	// If empty, the default script will be used.
	RpcScript string
}

// Option is a function that configures a V8 Engine.
// It's used in the NewFactory function.
//
// Since Engine.Iso and Engine.Ctx are public, users can easily create their own
// custom options for advanced configuration. For example, to inject a Go function
// into the JS context:
//
//	func WithGoFunction(name string, fn v8go.FunctionCallback) v8engine.Option {
//	  return func(e *v8engine.Engine) error {
//	    global := e.Ctx.Global()
//	    tmpl := v8go.NewFunctionTemplate(e.Iso, fn)
//	    fun, err := tmpl.GetFunction(e.Ctx)
//	    if err != nil {
//	      return err
//	    }
//	    return global.Set(name, fun)
//	  }
//	}
type Option func(*Engine) error

// WithRpcScript provides an option to override the default RPC handling script.
// This allows for custom RPC logic or extending the engine's capabilities.
func WithRpcScript(script string) Option {
	return func(e *Engine) error {
		e.Option.RpcScript = script
		return nil
	}
}
