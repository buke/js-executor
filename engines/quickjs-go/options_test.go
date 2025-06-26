// Copyright 2025 Brian Wang <wangbuke@gmail.com>
// SPDX-License-Identifier: Apache-2.0

package quickjsengine

import (
	"testing"

	"github.com/stretchr/testify/require"
)

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
