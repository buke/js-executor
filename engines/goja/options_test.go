// Copyright 2025 Brian Wang <wangbuke@gmail.com>
// SPDX-License-Identifier: Apache-2.0

package gojaengine

import (
	"testing"

	"github.com/dop251/goja"
	"github.com/stretchr/testify/require"
)

func TestWithMaxCallStackSize(t *testing.T) {
	engine, err := NewFactory(WithMaxCallStackSize(128))()
	require.NoError(t, err)
	defer engine.Close()

	gojaEngine := engine.(*Engine)
	require.Equal(t, 128, gojaEngine.Option.MaxCallStackSize)
}

func TestWithEnableConsole(t *testing.T) {
	engine, err := NewFactory(WithEnableConsole())()
	require.NoError(t, err)
	defer engine.Close()

	gojaEngine := engine.(*Engine)
	require.True(t, gojaEngine.Option.EnableConsole)

	// Verify console object exists in VM
	done := make(chan bool, 1)
	gojaEngine.Loop.RunOnLoop(func(vm *goja.Runtime) {
		v := vm.Get("console")
		done <- v != nil && !goja.IsUndefined(v)
	})
	require.True(t, <-done)
}

func TestWithRequire(t *testing.T) {
	engine, err := NewFactory(WithRequire())()
	require.NoError(t, err)
	defer engine.Close()

	gojaEngine := engine.(*Engine)
	require.True(t, gojaEngine.Option.EnableRequire)

	// Verify require function exists in VM
	done := make(chan bool, 1)
	gojaEngine.Loop.RunOnLoop(func(vm *goja.Runtime) {
		v := vm.Get("require")
		done <- v != nil && !goja.IsUndefined(v)
	})
	require.True(t, <-done)
}

func TestWithFieldNameMapper(t *testing.T) {
	// This tests the default mapper set in newEngine
	engine, err := NewFactory()()
	require.NoError(t, err)
	defer engine.Close()

	type MyStruct struct {
		MyField string `json:"myField"`
	}

	gojaEngine := engine.(*Engine)
	done := make(chan goja.Value, 1)
	gojaEngine.Loop.RunOnLoop(func(vm *goja.Runtime) {
		s := MyStruct{MyField: "test"}
		vm.Set("myVar", s)
		result, err := vm.RunString("myVar.myField")
		if err != nil {
			t.Log(err)
		}
		done <- result
	})

	result := <-done
	require.Equal(t, "test", result.String())
}

// TestWithFieldNameMapper_Nil covers the branch where a nil mapper is passed.
func TestWithFieldNameMapper_Nil(t *testing.T) {
	// This test ensures that passing a nil mapper does not cause an error
	// and that the default mapper remains in effect.
	engine, err := NewFactory(WithFieldNameMapper(nil))()
	require.NoError(t, err)
	require.NotNil(t, engine)
	defer engine.Close()

	// Verify the default mapper is still active by checking its behavior.
	type MyStruct struct {
		MyField string `json:"myField"`
	}

	gojaEngine := engine.(*Engine)
	done := make(chan goja.Value, 1)
	gojaEngine.Loop.RunOnLoop(func(vm *goja.Runtime) {
		s := MyStruct{MyField: "test"}
		vm.Set("myVar", s)
		result, err := vm.RunString("myVar.myField")
		if err != nil {
			t.Log(err)
		}
		done <- result
	})

	result := <-done
	require.Equal(t, "test", result.String())
}
