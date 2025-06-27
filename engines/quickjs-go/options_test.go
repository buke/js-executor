// Copyright 2025 Brian Wang <wangbuke@gmail.com>
// SPDX-License-Identifier: Apache-2.0

package quickjsengine

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// TestWithGCThreshold tests the WithGCThreshold option for normal, disable, and invalid values.
func TestWithGCThreshold(t *testing.T) {
	engine, err := newEngine()
	require.NoError(t, err)
	defer engine.Close()

	// Normal setting
	err = WithGCThreshold(1024)(engine)
	require.NoError(t, err)
	require.Equal(t, int64(1024), engine.Option.GCThreshold)

	// Disable automatic GC
	err = WithGCThreshold(-1)(engine)
	require.NoError(t, err)
	require.Equal(t, int64(-1), engine.Option.GCThreshold)

	// Invalid value
	err = WithGCThreshold(-2)(engine)
	require.Error(t, err)
}

// TestWithMemoryLimit tests the WithMemoryLimit option for normal and zero (no limit) values.
func TestWithMemoryLimit(t *testing.T) {
	engine, err := newEngine()
	require.NoError(t, err)
	defer engine.Close()

	err = WithMemoryLimit(1024 * 1024)(engine)
	require.NoError(t, err)
	require.Equal(t, uint64(1024*1024), engine.Option.MemoryLimit)

	// 0 = no limit
	err = WithMemoryLimit(0)(engine)
	require.NoError(t, err)
	require.Equal(t, uint64(0), engine.Option.MemoryLimit)
}

// TestWithTimeout tests the WithTimeout option for normal and zero (no timeout) values.
func TestWithTimeout(t *testing.T) {
	engine, err := newEngine()
	require.NoError(t, err)
	defer engine.Close()

	err = WithTimeout(10)(engine)
	require.NoError(t, err)
	require.Equal(t, uint64(10), engine.Option.Timeout)

	// 0 = no limit
	err = WithTimeout(0)(engine)
	require.NoError(t, err)
	require.Equal(t, uint64(0), engine.Option.Timeout)
}

// TestWithMaxStackSize tests the WithMaxStackSize option for normal and zero (default) values.
func TestWithMaxStackSize(t *testing.T) {
	engine, err := newEngine()
	require.NoError(t, err)
	defer engine.Close()

	err = WithMaxStackSize(1024 * 1024)(engine)
	require.NoError(t, err)
	require.Equal(t, uint64(1024*1024), engine.Option.MaxStackSize)

	// 0 = default
	err = WithMaxStackSize(0)(engine)
	require.NoError(t, err)
	require.Equal(t, uint64(0), engine.Option.MaxStackSize)
}

// TestWithCanBlock tests the WithCanBlock option for enabling and disabling blocking.
func TestWithCanBlock(t *testing.T) {
	engine, err := newEngine()
	require.NoError(t, err)
	defer engine.Close()

	err = WithCanBlock(true)(engine)
	require.NoError(t, err)
	require.Equal(t, true, engine.Option.CanBlock)

	err = WithCanBlock(false)(engine)
	require.NoError(t, err)
	require.Equal(t, false, engine.Option.CanBlock)
}

// TestWithEnableModuleImport tests the WithEnableModuleImport option for enabling and disabling ES6 module import.
func TestWithEnableModuleImport(t *testing.T) {
	engine, err := newEngine()
	require.NoError(t, err)
	defer engine.Close()

	err = WithEnableModuleImport(true)(engine)
	require.NoError(t, err)
	require.Equal(t, true, engine.Option.EnableModuleImport)

	err = WithEnableModuleImport(false)(engine)
	require.NoError(t, err)
	require.Equal(t, false, engine.Option.EnableModuleImport)
}

// TestWithStrip tests the WithStrip option for valid and invalid strip levels.
func TestWithStrip(t *testing.T) {
	engine, err := newEngine()
	require.NoError(t, err)
	defer engine.Close()

	err = WithStrip(0)(engine)
	require.NoError(t, err)
	require.Equal(t, 0, engine.Option.Strip)

	err = WithStrip(1)(engine)
	require.NoError(t, err)
	require.Equal(t, 1, engine.Option.Strip)

	err = WithStrip(2)(engine)
	require.NoError(t, err)
	require.Equal(t, 2, engine.Option.Strip)

	// Invalid values
	err = WithStrip(-1)(engine)
	require.Error(t, err)
	err = WithStrip(3)(engine)
	require.Error(t, err)
}

// TestWithRpcScript tests the WithRpcScript option for valid and invalid (empty) script values.
func TestWithRpcScript(t *testing.T) {
	engine, err := newEngine()
	require.NoError(t, err)
	defer engine.Close()

	err = WithRpcScript("function rpc(){}")(engine)
	require.NoError(t, err)
	require.Equal(t, "function rpc(){}", engine.RpcScript)

	// Empty string is invalid
	err = WithRpcScript("")(engine)
	require.Error(t, err)
}
