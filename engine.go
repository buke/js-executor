// Copyright 2025 Brian Wang <wangbuke@gmail.com>
// SPDX-License-Identifier: Apache-2.0

package jsexecutor

// JsRequest represents a JavaScript execution request
type JsRequest struct {
	Id      string                 `json:"id"`      // Unique identifier for the request
	Service string                 `json:"service"` // Service/function name to call
	Args    []interface{}          `json:"args"`    // Arguments to pass to the function
	Context map[string]interface{} `json:"context"` // Additional context data
}

// JsResponse represents the result of JavaScript execution
type JsResponse struct {
	Id      string                 `json:"id"`      // Request ID that this response corresponds to
	Result  interface{}            `json:"result"`  // Execution result
	Context map[string]interface{} `json:"context"` // Updated context data
}

// InitJsScript represents a JavaScript initialization script
type InitJsScript struct {
	Content  string // Script content
	FileName string // Script file name for debugging purposes
}

// JsEngineBuilder creates JavaScript engine instances
type JsEngineBuilder interface {
	// Build creates a new JavaScript engine with the given options
	Build(options ...JsEngineOption) (JsEngine, error)
}

// JsEngine represents a JavaScript execution engine
type JsEngine interface {
	// Init initializes the engine with the given scripts
	Init(scripts []*InitJsScript) error

	// Reload reloads the engine with new scripts
	Reload(scripts []*InitJsScript) error

	// Execute executes a JavaScript request and returns the response
	Execute(req *JsRequest) (*JsResponse, error)

	// Close closes the engine and releases resources
	Close() error
}

// JsEngineOption is a function that configures a JavaScript engine
type JsEngineOption func(JsEngine) error
