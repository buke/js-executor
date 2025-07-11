//go:build !windows

// Copyright 2025 Brian Wang <wangbuke@gmail.com>
// SPDX-License-Identifier: Apache-2.0

package v8engine

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// TestWithRpcScript tests the WithRpcScript option.
// It verifies that a custom RPC script can be set on the engine's options.
func TestWithRpcScript(t *testing.T) {
	engine, err := newEngine()
	require.NoError(t, err)
	defer engine.Close()

	// Test with a valid, non-empty script
	customScript := "async () => ({ result: 'custom' });"
	err = WithRpcScript(customScript)(engine)
	require.NoError(t, err)
	require.Equal(t, customScript, engine.Option.RpcScript)

	// Test with an empty script. The option itself doesn't validate,
	// it just sets the value. The engine's creation logic will decide
	// whether to use the default script or this empty one.
	err = WithRpcScript("")(engine)
	require.NoError(t, err)
	require.Equal(t, "", engine.Option.RpcScript)
}
