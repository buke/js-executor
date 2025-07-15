// Copyright 2025 Brian Wang <wangbuke@gmail.com>
// SPDX-License-Identifier: Apache-2.0

package gojaengine

import (
	jsexecutor "github.com/buke/js-executor"
	"github.com/dop251/goja"
	"github.com/dop251/goja_nodejs/console"
	"github.com/dop251/goja_nodejs/require"
)

// EngineOption holds configuration for a Goja engine instance.
type EngineOption struct {
	MaxCallStackSize int
	EnableConsole    bool
	EnableRequire    bool
	FieldNameMapper  goja.FieldNameMapper
}

// WithMaxCallStackSize sets the maximum call stack size for the runtime.
// A value of 0 or less means no limit.
func WithMaxCallStackSize(size int) jsexecutor.JsEngineOption {
	return func(engine jsexecutor.JsEngine) error {
		e := engine.(*Engine)
		e.Option.MaxCallStackSize = size
		done := make(chan struct{})
		e.Loop.RunOnLoop(func(vm *goja.Runtime) {
			vm.SetMaxCallStackSize(size)
			close(done)
		})
		<-done
		return nil
	}
}

// WithEnableConsole enables the console object (console.log, etc.) in the JS runtime.
func WithEnableConsole() jsexecutor.JsEngineOption {
	return func(engine jsexecutor.JsEngine) error {
		e := engine.(*Engine)
		e.Option.EnableConsole = true
		done := make(chan struct{})
		e.Loop.RunOnLoop(func(vm *goja.Runtime) {
			console.Enable(vm)
			close(done)
		})
		<-done
		return nil
	}
}

// WithRequire enables the require() function for loading CommonJS modules.
func WithRequire() jsexecutor.JsEngineOption {
	return func(engine jsexecutor.JsEngine) error {
		e := engine.(*Engine)
		e.Option.EnableRequire = true
		done := make(chan struct{})
		e.Loop.RunOnLoop(func(vm *goja.Runtime) {
			// Creates a new module registry and enables require()
			new(require.Registry).Enable(vm)
			close(done)
		})
		<-done
		return nil
	}
}

// WithFieldNameMapper sets the field name mapper for Go-to-JS struct conversions.
// This controls how Go struct field names are exposed in JavaScript.
func WithFieldNameMapper(mapper goja.FieldNameMapper) jsexecutor.JsEngineOption {
	return func(engine jsexecutor.JsEngine) error {
		e := engine.(*Engine)
		if mapper != nil {
			e.Option.FieldNameMapper = mapper
			done := make(chan struct{})
			e.Loop.RunOnLoop(func(vm *goja.Runtime) {
				vm.SetFieldNameMapper(mapper)
				close(done)
			})
			<-done
		}
		return nil
	}
}
