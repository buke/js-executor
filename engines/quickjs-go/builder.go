// Copyright 2025 Brian Wang <wangbuke@gmail.com>
// SPDX-License-Identifier: Apache-2.0

package quickjsengine

import (
	jsexecutor "github.com/buke/js-executor"
)

// Builder implements jsexecutor.JsEngineBuilder for QuickJS
type Builder struct {
}

// NewBuilder creates a new QuickJS engine builder with default settings
func NewBuilder() *Builder {
	return &Builder{}
}

// Build creates a new QuickJS engine instance with the configured options
func (b *Builder) Build(options ...jsexecutor.JsEngineOption) (jsexecutor.JsEngine, error) {
	return NewEngine(options...)
}
